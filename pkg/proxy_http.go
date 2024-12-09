package pkg

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"

	"github.com/omimic12/proxy-server/constants"
	"github.com/pkg/errors"
)

const (
	strHeaderBasicRealm = "Basic realm=\"\"\r\n\r\n"
)

func (p *Proxy) ListenHTTP(ctx context.Context, port int) error {
	go p.config.HTTPServer.ListenAndServe() //nolint:errcheck

	<-ctx.Done()
	return p.config.HTTPServer.Shutdown(ctx)
}

func (p *Proxy) handlerHTTP(w http.ResponseWriter, req *http.Request) {
	var err error
	username, password, err := extractCredentials(req)
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
		p.serveHTTPS(purchase, request)
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

	// Parse the target URL from the request
	targetURL := req.URL.String()
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		http.Error(w, "Invalid target URL", http.StatusBadRequest)
		return
	}

	// Create a new request to the target URL through the real proxy
	r, err := http.NewRequest(req.Method, parsedURL.String(), req.Body)
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
	r.Header.Set("Proxy-Authorization", "Basic "+string(credentials))

	// Set up the real proxy
	proxyURL, err := url.Parse(hostname)
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

func (p *Proxy) serveHTTPS(purchase *Purchase, request *Request) {
}
