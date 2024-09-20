package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

func LoadConfig() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
}

func GetConfig(configName string) string {
	return os.Getenv(configName)
}

func GetProxySettings(providerName string) (string, string, string, string) {
	host := GetConfig(strings.ToUpper(providerName) + "_HOST")
	port := GetConfig(strings.ToUpper(providerName) + "_PORT")
	username := GetConfig(strings.ToUpper(providerName) + "_USERNAME")
	password := GetConfig(strings.ToUpper(providerName) + "_PASSWORD")
	return host, port, username, password
}

func main() {
	// Define the proxy server address
	proxyAddr := "136.243.175.139:8080"

	// Start the proxy server
	fmt.Printf("Starting proxy server at %s\n", proxyAddr)
	http.HandleFunc("/", ProxyHandler)
	if err := http.ListenAndServe(proxyAddr, nil); err != nil {
		fmt.Println("Error starting server:", err)
	}
}

func ProxyHandler(w http.ResponseWriter, r *http.Request) {
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

	// Load Proxy Settings
	LoadConfig()
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
			if key == "Proxy-Authorization" {
				// Concatenate username and password
				realCredentials := fmt.Sprintf("%s:%s", realProxyUsername, realProxyPassword)
				// Encode to Base64
				encodedCredentials := base64.StdEncoding.EncodeToString([]byte(realCredentials))
				// Set the Authorization header
				req.Header.Add(key, "Basic "+encodedCredentials)
			} else {
				req.Header.Add(key, value)
			}
		}
	}

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
