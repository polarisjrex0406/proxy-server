package pkg

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/go-redis/redis/v8"
	"github.com/omimic12/proxy-server/config"
	"github.com/omimic12/proxy-server/utils"
)

// mitmProxy is a type implementing http.Handler that serves as a MITM proxy
// for CONNECT tunnels. Create new instances of mitmProxy using createMitmProxy.
type mitmProxy struct {
	caCert      *x509.Certificate
	caKey       any
	ctx         context.Context
	redisClient *redis.Client
	db          *sql.DB
}

// CreateMitmProxy creates a new MITM proxy. It should be passed the filenames
// for the certificate and private key of a certificate authority trusted by the
// client's machine.
func CreateMitmProxy(caCertFile, caKeyFile string, ctx context.Context, redisClient *redis.Client, db *sql.DB) *mitmProxy {
	caCert, caKey, err := loadX509KeyPair(caCertFile, caKeyFile)
	if err != nil {
		log.Fatal("Error loading CA certificate/key:", err)
	}
	log.Printf("loaded CA certificate and key; IsCA=%v\n", caCert.IsCA)

	return &mitmProxy{
		caCert:      caCert,
		caKey:       caKey,
		ctx:         ctx,
		redisClient: redisClient,
		db:          db,
	}
}

func (p *mitmProxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		log.Fatal("error splitting host/port:", err)
	}
	if host != "65.108.225.146" {
		// http.Error(w, "this proxy only supports for testing", http.StatusForbidden)
		// return
	}

	if req.Method == http.MethodConnect {
		p.proxyConnect(w, req)
	} else {
		p.proxyHandler(w, req)
	}
}

// proxyConnect implements the MITM proxy for CONNECT tunnels.
func (p *mitmProxy) proxyConnect(w http.ResponseWriter, proxyReq *http.Request) {
	log.Printf("CONNECT requested to %v (from %v)", proxyReq.Host, proxyReq.RemoteAddr)

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
	host, _, err := net.SplitHostPort(proxyReq.Host)
	fmt.Println("host =", host)
	if err != nil {
		log.Fatal("error splitting host/port:", err)
	}

	// Create a fake TLS certificate for the target host, signed by our CA. The
	// certificate will be valid for 10 days - this number can be changed.
	pemCert, pemKey := createCert([]string{host}, p.caCert, p.caKey, 240)
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
	// connWriter := bufio.NewWriter(tlsConn)

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
		// Handle Basic Authentication
		proxyAuth := r.Header.Get("Proxy-Authorization")
		credentials, _ := utils.DecodeBasicAuth(proxyAuth)
		parts := strings.Split(credentials, ":")
		username := parts[0]
		fmt.Println(username)

		// Take the original request and changes its destination to be forwarded
		// to the target server.
		changeRequestToTarget(r, proxyReq.Host)

		// Proxy Settings
		realProxyHost, realProxyPort, realProxyUsername, realProxyPassword := getProxySettings(username)

		proxyURL, err := url.Parse(fmt.Sprintf("http://%s:%s@%s:%s", realProxyUsername, realProxyPassword, realProxyHost, realProxyPort))
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
		fmt.Printf("Response Protocol: %v\n", resp.Proto)
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

func getProxySettings(providerName string) (string, string, string, string) {
	cfg, err := config.GetConfig()
	if err != nil {
		panic(err)
	}

	var host, port, username, password string
	if providerName == "ttproxy" {
		host = cfg.Provider.TTProxy.ProxyCredentials.Host
		port = fmt.Sprintf("%d", cfg.Provider.TTProxy.ProxyCredentials.Port)
	} else if providerName == "dataimpulse" {
		host = cfg.Provider.DataImpulse.ProxyCredentials.Host
		port = fmt.Sprintf("%d", cfg.Provider.DataImpulse.ProxyCredentials.Port)
	} else if providerName == "proxyverse" {
		host = cfg.Provider.Proxyverse.ProxyCredentials.Host
		port = fmt.Sprintf("%d", cfg.Provider.Proxyverse.ProxyCredentials.Port)
		username = cfg.Provider.Proxyverse.ProxyCredentials.Username
		password = cfg.Provider.Proxyverse.ProxyCredentials.Password
	} else if providerName == "databay" {
		host = cfg.Provider.Databay.ProxyCredentials.Host
		port = fmt.Sprintf("%d", cfg.Provider.Databay.ProxyCredentials.Port)
		username = cfg.Provider.Databay.ProxyCredentials.Username
		password = cfg.Provider.Databay.ProxyCredentials.Password
	}

	return host, port, username, password
}

func (p *mitmProxy) proxyHandler(w http.ResponseWriter, r *http.Request) {
	// Parse the target URL from the request
	targetURL := r.URL.String()
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		http.Error(w, "Invalid target URL", http.StatusBadRequest)
		return
	}

	// Handle Basic Authentication
	proxyAuth := r.Header.Get("Proxy-Authorization")
	credentials, _ := utils.DecodeBasicAuth(proxyAuth)
	parts := strings.Split(credentials, ":")
	username := parts[0]
	password := parts[1]
	// Example usage
	query := fmt.Sprintf("SELECT pswd FROM tbl_purchases WHERE customer_id = %d AND pswd = '%v'", 1, password)
	data, err := GetCachedData(p.ctx, p.redisClient, p.db, query)
	if err != nil {
		// log.Fatalf("Error getting data: %v\n", err)
	} else {
		fmt.Println(data)
	}

	// Load Proxy Settings
	realProxyHost, realProxyPort, realProxyUsername, realProxyPassword := getProxySettings(username)

	// Create a new request to the target URL through the real proxy
	req, err := http.NewRequest(r.Method, parsedURL.String(), r.Body)
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}

	// Copy headers from the original request
	for key, values := range r.Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	// Concatenate username and password
	realCredentials := fmt.Sprintf("%s:%s", realProxyUsername, realProxyPassword)
	// Encode to Base64
	encodedCredentials := base64.StdEncoding.EncodeToString([]byte(realCredentials))
	// Set the Authorization header
	req.Header.Set("Proxy-Authorization", "Basic "+encodedCredentials)
	fmt.Println("HTTP Outgoing realCredentials: ", realCredentials)

	// Set up the real proxy
	proxyURL, err := url.Parse(fmt.Sprintf("http://%s:%s", realProxyHost, realProxyPort))
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
	fmt.Println(proxyURL.String())
	resp, err := client.Do(req)
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
