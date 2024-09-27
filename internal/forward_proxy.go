package internal

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-redis/redis/v8"
)

type ForwardProxy struct {
	Ctx         context.Context
	RedisClient *redis.Client
	DB          *sql.DB
}

func (p *ForwardProxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		log.Fatal("error splitting host/port:", err)
	}
	if host != "136.243.90.46" {
		return
	}
	fmt.Printf("ForwardProxy - ServeHTTP - %v\n", req.Method)

	if req.Method == http.MethodConnect {
		ProxyConnect(w, req)
	} else {
		ProxyHandler(p, w, req)
		// http.Error(w, "this proxy only supports CONNECT", http.StatusMethodNotAllowed)
	}
}

func ProxyConnect(w http.ResponseWriter, req *http.Request) {
	log.Printf("CONNECT requested to %v (from %v)", req.Host, req.RemoteAddr)
	targetConn, err := net.Dial("tcp", req.Host)
	if err != nil {
		log.Println("failed to dial to target", req.Host)
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
	hj, ok := w.(http.Hijacker)
	if !ok {
		log.Fatal("http server doesn't support hijacking connection")
	}

	clientConn, _, err := hj.Hijack()
	if err != nil {
		log.Fatal("http hijacking failed")
	}

	log.Println("tunnel established")
	go TunnelConn(targetConn, clientConn)
	go TunnelConn(clientConn, targetConn)
}

// Create a custom writer that prints the data
type PrintWriter struct {
	writer io.Writer
}

func (pw *PrintWriter) Write(p []byte) (n int, err error) {
	// Print the data to the console
	// fmt.Print(string(p))
	// Write the data to the underlying writer
	return pw.writer.Write(p)
}

func TunnelConn(dst io.WriteCloser, src io.ReadCloser) {
	// Create a PrintWriter that wraps the destination
	printWriter := &PrintWriter{writer: dst}

	// Copy data from src to printWriter (which prints and writes to dst)
	io.Copy(printWriter, src)

	// Close the streams
	dst.Close()
	src.Close()
}

func ProxyHandler(p *ForwardProxy, w http.ResponseWriter, r *http.Request) {
	// Parse the target URL from the request
	targetURL := r.URL.String()
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		http.Error(w, "Invalid target URL", http.StatusBadRequest)
		return
	}

	// Handle Basic Authentication
	proxyAuth := r.Header.Get("Proxy-Authorization")
	credentials, _ := DecodeBasicAuth(proxyAuth)
	parts := strings.Split(credentials, ":")
	username := parts[0]
	// password := parts[1]
	// Example usage
	query := "SELECT pswd FROM tbl_proxy_credentials ORDER BY id ASC"
	data, err := GetCachedData(p.Ctx, p.RedisClient, p.DB, query)
	if err != nil {
		log.Fatalf("Error getting data: %v\n", err)
	}

	fmt.Println(data)

	// Load Proxy Settings
	realProxyHost, realProxyPort, realProxyUsername, realProxyPassword := GetProxySettings(username)

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

// decodeBasicAuth decodes the Basic Authorization header
func DecodeBasicAuth(auth string) (string, error) {
	// Remove "Basic " prefix
	encoded := strings.TrimPrefix(auth, "Basic ")
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}
