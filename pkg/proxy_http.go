package pkg

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/omimic12/proxy-server/constants"
	"github.com/omimic12/proxy-server/pkg/zerocopy"
	"github.com/pkg/errors"
)

const (
	strHeaderBasicRealm = "Basic realm=\"\"\r\n\r\n"
)

var (
	okHTTP11Response = []byte("HTTP/1.1 200 OK\r\n\r\n")
)

func (p *Proxy) ListenHTTP(ctx context.Context, port int) error {
	p.config.HTTPServer.Addr = fmt.Sprintf(":%d", port)
	go p.config.HTTPServer.ListenAndServe() //nolint:errcheck

	<-ctx.Done()
	return p.config.HTTPServer.Shutdown(ctx)
}

func (p *Proxy) handlerHTTP(w http.ResponseWriter, req *http.Request) {
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

	if req.Method == http.MethodConnect {
		p.serveHTTPS(purchase, request, w, req)
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

func (p *Proxy) serveHTTPS(purchase *Purchase, request *Request, w http.ResponseWriter, req *http.Request) {
	log.Printf("CONNECT requested to %v (from %v)", req.Host, req.RemoteAddr)

	// dialDuration := time.Now()
	upstream, err := request.Provider.Dial([]byte(req.RequestURI), request)
	if err != nil {
		// ctx.SetStatusCode(fasthttp.StatusGatewayTimeout)
		// metrics.Errors504GatewayTimeout.Inc()
		// p.stopTracker(purchase, request)
		p.logError(err, request)
		// metrics.IncProviderErrors(request.Provider.Name())
		releaseRequest(request)
		return
	}
	// metrics.IncProviderConnections(request.Provider.Name())
	// metrics.ObserveProviderDialTime(request.Provider.Name(), float64(time.Since(dialDuration).Milliseconds()))

	// metrics.ConnectionsHTTPS.Inc()

	// request.Inc(headerSize(ctx))

	// var noResponse bool
	// if !ctx.Request.Header.IsHTTP11() {
	// 	noResponse = true
	// }

	// ctx.HijackSetNoResponse(noResponse)

	// "Hijack" the client connection to get a TCP (or TLS) socket we can read
	// and write arbitrary data to/from.
	hj, ok := w.(http.Hijacker)
	if !ok {
		log.Fatal("http server doesn't support hijacking connection")
	}

	client, _, err := hj.Hijack()
	if err != nil {
		log.Fatal("http hijacking failed")
	}

	_, err = client.Write(okHTTP11Response)
	if err != nil {
		// p.stopTracker(purchase, request)
		_ = upstream.Close() //nolint:errcheck
		_ = client.Close()   //nolint:errcheck
		releaseRequest(request)
		return
	}
	_ = p.tunnel(purchase, request, upstream, client)

	// ctx.Hijack(func(client net.Conn) {
	// 	if noResponse {
	// 		//http1.0 clients
	// 		_, err := client.Write(okHTTP11Response)
	// 		if err != nil {
	// 			p.stopTracker(purchase, request)
	// 			_ = upstream.Close() //nolint:errcheck
	// 			_ = client.Close()   //nolint:errcheck
	// 			releaseRequest(request)
	// 			return
	// 		}
	// 	}

	// 	_ = p.tunnel(purchase, request, upstream, client) //nolint:errcheck

	// 	releaseRequest(request)
	// })

	// ctx.SetStatusCode(fasthttp.StatusOK)

	// ctx.Success("", nil)
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
