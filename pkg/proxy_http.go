package pkg

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/omimic12/proxy-server/constants"
	"github.com/omimic12/proxy-server/pkg/zerocopy"
	"github.com/pkg/errors"
)

const (
	strHeaderBasicRealm = "Basic realm=\"\"\r\n\r\n"
)

func (p *Proxy) ListenHTTP(ctx context.Context, port int) error {
	p.config.HTTPServer.Addr = fmt.Sprintf(":%d", port)
	go p.config.HTTPServer.ListenAndServe() //nolint:errcheck

	<-ctx.Done()
	return p.config.HTTPServer.Shutdown(ctx)
}

func (p *Proxy) handlerHTTPS(w http.ResponseWriter, req *http.Request) {
	fmt.Println("handlerHTTPS Started:")
	fmt.Println(req.Header)
	fmt.Println("handlerHTTPS Ended:")
}

func (p *Proxy) handlerHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodConnect {
		p.serveHTTPS(w, req)
		return
	}

	var err error
	username, password, err := extractCredentials(req, req)
	if err != nil {
		w.WriteHeader(http.StatusProxyAuthRequired)
		w.Header().Add(constants.HeaderProxyAuthenticate, strHeaderBasicRealm)
		return
	}

	request := acquireRequest()
	request.Protocol = HTTP
	request.Done = make(chan struct{}, 1)
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		// metrics.Errors400BadRequest.Inc()
		w.WriteHeader(http.StatusBadRequest)
		releaseRequest(request)
		return
	}

	userIP := net.ParseIP(host)
	if userIP == nil {
		// metrics.Errors400BadRequest.Inc()
		w.WriteHeader(http.StatusBadRequest)
		releaseRequest(request)
		return
	}
	request.UserIP = userIP.String()

	err = parseRequest(req.Host, username, password, request, p.config.Parser)
	if err != nil {
		// metrics.Errors400BadRequest.Inc()
		w.WriteHeader(http.StatusBadRequest)
		releaseRequest(request)
		return
	}

	cleanRequestHeaders(req)

	purchase, err := p.config.Auth.Authenticate(req.Context(), request.Password)

	if err == ErrMissingAuth || err == ErrPurchaseNotFound {
		// metrics.Errors407AuthRequired.Inc()
		w.WriteHeader(http.StatusProxyAuthRequired)
		w.Header().Add(constants.HeaderProxyAuthenticate, strHeaderBasicRealm)
		releaseRequest(request)
		return
	} else if err == ErrNotEnoughData {
		w.WriteHeader(http.StatusPaymentRequired)
		// metrics.Errors402PaymentRequired.Inc()
		releaseRequest(request)
		return
	} else if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		p.logError(err, request)
		// metrics.Errors500Internal.Inc()
		releaseRequest(request)
		return
	}

	if err = hasAccess(purchase, request); err == ErrDomainBlocked || err == ErrIPNotAllowed {
		p.config.Logger.Info(request.UserIP)
		w.WriteHeader(http.StatusForbidden)
		// metrics.Errors403Forbidden.Inc()
		releaseRequest(request)
		return
	} else if err == ErrInvalidTargeting {
		w.WriteHeader(http.StatusBadRequest)
		p.logError(err, request)
		// metrics.Errors400BadRequest.Inc()
		releaseRequest(request)
		return
	} else if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		p.logError(err, request)
		// metrics.Errors500Internal.Inc()
		releaseRequest(request)
		return
	}

	if purchase.Threads > 0 &&
		p.config.ConnectionTracker.Watch(request.ID, request.PurchaseUUID, request.Done) >= purchase.Threads {
		p.config.ConnectionTracker.Stop(request.ID, request.PurchaseUUID)
		w.WriteHeader(http.StatusTooManyRequests)
		// metrics.Errors429TooManyRequests.Inc()
		releaseRequest(request)
		return
	}

	err = p.selectProvider(purchase, request)
	if err == ErrDomainBlocked {
		// p.stopTracker(purchase, request)
		w.WriteHeader(http.StatusForbidden)
		releaseRequest(request) //nolint:errcheck
		// metrics.Errors403Forbidden.Inc()
		return
	} else if err == ErrFailedSelectProvider {
		// p.stopTracker(purchase, request)
		w.WriteHeader(http.StatusBadGateway)
		p.logError(errors.Wrap(err, "failed to select provider"), request)
		// metrics.Errors502Internal.Inc()
		releaseRequest(request) //nolint:errcheck
		return
	} else if err != nil {
		// p.stopTracker(purchase, request)
		w.WriteHeader(http.StatusInternalServerError)
		p.logError(errors.Wrap(err, "error during provider selection"), request)
		// metrics.Errors500Internal.Inc()
		releaseRequest(request) //nolint:errcheck
		return
	}

	p.serveHTTP(purchase, request, w, req)
}

func (p *Proxy) serveHTTP(purchase *Purchase, request *Request, w http.ResponseWriter, req *http.Request) {
	defer func() {
		// p.stopTracker(purchase, request)
		releaseRequest(request)
	}()

	hostname, _, _, credentials, err := request.Provider.Credentials(request) // FIXME looks awkward
	if err != nil {
		return
	}
	fmt.Println("hostname =", hostname)
	fmt.Println("credentials =", string(credentials))

	// Create a new request to the target URL through the real proxy
	r, err := http.NewRequest(req.Method, req.URL.String(), req.Body)
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}

	// Copy headers from the original request
	for key, values := range req.Header {
		for _, value := range values {
			r.Header.Add(key, value)
		}
	}
	// Set the Authorization header
	if len(credentials) > 0 {
		r.Header.Set(constants.HeaderProxyAuthorization, "Basic "+zerocopy.String(credentials))
	}

	proxyStr := fmt.Sprintf("http://%s", hostname)

	// Set up the real proxy
	proxyURL, err := url.Parse(proxyStr)
	if err != nil {
		http.Error(w, "Failed to set up proxy", http.StatusInternalServerError)
		return
	}

	// Create a client with the proxy
	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
	}

	// Send the request to the real proxy
	resp, err := client.Do(r)
	if err != nil {
		fmt.Println("err =", err)
		http.Error(w, "Failed to reach real proxy", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy the response headers and status code
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)

	// Write the response body
	if _, err := io.Copy(w, resp.Body); err != nil {
		http.Error(w, "Failed to write response", http.StatusInternalServerError)
		return
	}
}

func (p *Proxy) serveHTTPS(w http.ResponseWriter, req *http.Request) {
	log.Printf("CONNECT requested to %v (from %v)", req.Host, req.RemoteAddr)

	// "Hijack" the client connection to get a TCP (or TLS) socket we can read
	// and write arbitrary data to/from.
	hj, ok := w.(http.Hijacker)
	if !ok {
		log.Fatal("http server doesn't support hijacking connection")
	}

	clientConn, _, err := hj.Hijack()
	if err != nil {
		log.Fatal("http hijacking failed")
	}

	// proxyReq.Host will hold the CONNECT target host, which will typically have
	// a port - e.g. example.org:443
	// To generate a fake certificate for example.org, we have to first split off
	// the host from the port.
	host, _, err := net.SplitHostPort(req.Host)
	if err != nil {
		log.Fatal("error splitting host/port:", err)
	}

	// Create a fake TLS certificate for the target host, signed by our CA. The
	// certificate will be valid for 10 days - this number can be changed.
	pemCert, pemKey := createCert([]string{host}, p.config.CACert, p.config.CAKey, 240)
	tlsCert, err := tls.X509KeyPair(pemCert, pemKey)
	if err != nil {
		log.Fatal(err)
	}

	// Send an HTTP OK response back to the client; this initiates the CONNECT
	// tunnel. From this point on the client will assume it's connected directly
	// to the target.
	if _, err := clientConn.Write([]byte("HTTP/1.1 200 OK\r\n\r\n")); err != nil {
		log.Fatal("error writing status to client:", err)
	}

	// Configure a new TLS server, pointing it at the client connection, using
	// our certificate. This server will now pretend being the target.
	tlsConfig := &tls.Config{
		PreferServerCipherSuites: true,
		CurvePreferences:         []tls.CurveID{tls.X25519, tls.CurveP256},
		MinVersion:               tls.VersionTLS12,
		Certificates:             []tls.Certificate{tlsCert},
	}

	tlsConn := tls.Server(clientConn, tlsConfig)
	defer tlsConn.Close()

	// Create a buffered reader for the client connection; this is required to
	// use http package functions with this connection.
	connReader := bufio.NewReader(tlsConn)

	// Run the proxy in a loop until the client closes the connection.
	for {
		r, err := http.ReadRequest(connReader)
		if err == io.EOF {
			break
		} else if err != nil {
			log.Fatal(err)
		}

		// We can dump the request; log it, modify it...
		if b, err := httputil.DumpRequest(r, false); err == nil {
			log.Printf("incoming request:\n%s\n", string(b))
		}

		username, password, err := extractCredentials(req, r)
		if err != nil {
			w.WriteHeader(http.StatusProxyAuthRequired)
			w.Header().Add(constants.HeaderProxyAuthenticate, strHeaderBasicRealm)
			return
		}

		request := acquireRequest()
		request.Protocol = HTTP
		request.Done = make(chan struct{}, 1)
		host, _, err := net.SplitHostPort(req.RemoteAddr)
		if err != nil {
			// metrics.Errors400BadRequest.Inc()
			// w.WriteHeader(http.StatusBadRequest)
			if _, err := tlsConn.Write([]byte("HTTP/1.1 400 Bad Request\r\nContent-Length: 0\r\n\r\n")); err != nil {
				log.Fatal("error writing status to client:", err)
			}
			releaseRequest(request)
			return
		}

		userIP := net.ParseIP(host)
		if userIP == nil {
			// metrics.Errors400BadRequest.Inc()
			w.WriteHeader(http.StatusBadRequest)
			releaseRequest(request)
			return
		}
		request.UserIP = userIP.String()

		err = parseRequest(req.Host, username, password, request, p.config.Parser)
		if err != nil {
			// metrics.Errors400BadRequest.Inc()
			w.WriteHeader(http.StatusBadRequest)
			releaseRequest(request)
			return
		}

		cleanRequestHeaders(req)

		purchase, err := p.config.Auth.Authenticate(req.Context(), request.Password)

		if err == ErrMissingAuth || err == ErrPurchaseNotFound {
			// metrics.Errors407AuthRequired.Inc()
			w.WriteHeader(http.StatusProxyAuthRequired)
			w.Header().Add(constants.HeaderProxyAuthenticate, strHeaderBasicRealm)
			releaseRequest(request)
			return
		} else if err == ErrNotEnoughData {
			w.WriteHeader(http.StatusPaymentRequired)
			// metrics.Errors402PaymentRequired.Inc()
			releaseRequest(request)
			return
		} else if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			p.logError(err, request)
			// metrics.Errors500Internal.Inc()
			releaseRequest(request)
			return
		}

		if err = hasAccess(purchase, request); err == ErrDomainBlocked || err == ErrIPNotAllowed {
			p.config.Logger.Info(request.UserIP)
			w.WriteHeader(http.StatusForbidden)
			// metrics.Errors403Forbidden.Inc()
			releaseRequest(request)
			return
		} else if err == ErrInvalidTargeting {
			w.WriteHeader(http.StatusBadRequest)
			p.logError(err, request)
			// metrics.Errors400BadRequest.Inc()
			releaseRequest(request)
			return
		} else if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			p.logError(err, request)
			// metrics.Errors500Internal.Inc()
			releaseRequest(request)
			return
		}

		if purchase.Threads > 0 &&
			p.config.ConnectionTracker.Watch(request.ID, request.PurchaseUUID, request.Done) >= purchase.Threads {
			p.config.ConnectionTracker.Stop(request.ID, request.PurchaseUUID)
			w.WriteHeader(http.StatusTooManyRequests)
			// metrics.Errors429TooManyRequests.Inc()
			releaseRequest(request)
			return
		}

		err = p.selectProvider(purchase, request)
		if err == ErrDomainBlocked {
			// p.stopTracker(purchase, request)
			w.WriteHeader(http.StatusForbidden)
			releaseRequest(request) //nolint:errcheck
			// metrics.Errors403Forbidden.Inc()
			return
		} else if err == ErrFailedSelectProvider {
			// p.stopTracker(purchase, request)
			w.WriteHeader(http.StatusBadGateway)
			p.logError(errors.Wrap(err, "failed to select provider"), request)
			// metrics.Errors502Internal.Inc()
			releaseRequest(request) //nolint:errcheck
			return
		} else if err != nil {
			// p.stopTracker(purchase, request)
			w.WriteHeader(http.StatusInternalServerError)
			p.logError(errors.Wrap(err, "error during provider selection"), request)
			// metrics.Errors500Internal.Inc()
			releaseRequest(request) //nolint:errcheck
			return
		}

		// Handle Basic Authentication
		hostname, proxyUsername, proxyPassword, proxyCredentials, err := request.Provider.Credentials(request) // FIXME looks awkward
		if err != nil {
			return
		}

		// Take the original request and changes its destination to be forwarded
		// to the target server.
		changeRequestToTarget(r, req.Host)

		proxyStr := hostname
		// Set the Authorization header
		if len(proxyCredentials) > 0 {
			r.Header.Set(constants.HeaderProxyAuthorization, "Basic "+zerocopy.String(proxyCredentials))
			proxyStr = fmt.Sprintf("%s:%s@%s", zerocopy.String(proxyUsername), zerocopy.String(proxyPassword), proxyStr)
		}
		proxyStr = fmt.Sprintf("http://%s", proxyStr)

		proxyURL, err := url.Parse(proxyStr)
		if err != nil {
			http.Error(w, "Failed to set up proxy", http.StatusInternalServerError)
			return
		}
		// Create a client with the proxy
		client := &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyURL(proxyURL),
			},
		}

		// Send the request to the target server and log the response.
		resp, err := client.Do(r)
		if err != nil {
			log.Fatal("error sending request to target:", err)
		}

		if b, err := httputil.DumpResponse(resp, false); err == nil {
			log.Printf("target response:\n%s\n", string(b))
		}
		defer resp.Body.Close()

		// Send the target server's response back to the client.
		resp.ProtoMajor = 1
		resp.ProtoMinor = 1
		if err := resp.Write(tlsConn); err != nil {
			log.Println("error writing response back:", err)
		}
	}
}

// changeRequestToTarget modifies req to be re-routed to the given target;
// the target should be taken from the Host of the original tunnel (CONNECT)
// request.
func changeRequestToTarget(req *http.Request, targetHost string) {
	targetUrl := addrToUrl(targetHost)
	targetUrl.Path = req.URL.Path
	targetUrl.RawQuery = req.URL.RawQuery
	req.URL = targetUrl
	// Make sure this is unset for sending the request through a client
	req.RequestURI = ""
}

func addrToUrl(addr string) *url.URL {
	if !strings.HasPrefix(addr, "https") {
		addr = "https://" + addr
	}
	u, err := url.Parse(addr)
	if err != nil {
		log.Fatal(err)
	}
	return u
}
