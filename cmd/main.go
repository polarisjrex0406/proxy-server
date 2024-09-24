package main

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
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

func ConnectionString() string {
	// Get database settings
	dbUser := GetConfig("DB_USER")
	dbPassword := GetConfig("DB_PSWD")
	dbName := GetConfig("DB_NAME")
	dbSSLMode := GetConfig("DB_SSL_MODE")
	// Construct the connection string
	dsn := fmt.Sprintf("user=%s password=%s dbname=%s host=%s port=%s sslmode=%s",
		dbUser, dbPassword, dbName, "localhost", "5432", dbSSLMode)
	return dsn
}

var (
	ctx         = context.Background()
	redisClient *redis.Client
	DB          *sql.DB
)

func GetCachedData(redisClient *redis.Client, DB *sql.DB, query string) (string, error) {
	// Check Redis cache
	cacheKey := query
	cachedData, err := redisClient.Get(ctx, cacheKey).Result()
	if err == redis.Nil {
		// Cache miss, query PostgreSQL
		var result string
		err = DB.QueryRow(query).Scan(&result)
		if err != nil {
			return "", err
		}

		// Cache the result in Redis
		err = redisClient.Set(ctx, cacheKey, result, 1*time.Hour).Err()
		if err != nil {
			return "", err
		}

		return result, nil
	} else if err != nil {
		return "", err
	}

	// Return cached result
	return cachedData, nil
}

func main() {
	LoadConfig()
	// Test PostgreSQL caching via Redis
	// Connect to PostgresSQL
	dbms := GetConfig("DB_TYPE")
	var err error
	DB, err = sql.Open(dbms, ConnectionString())
	if err != nil {
		log.Fatal(err)
	}
	if err = DB.Ping(); err != nil {
		log.Fatal(err)
	}
	defer DB.Close()
	// Connect to Redis
	redisClient = redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer redisClient.Close()

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
	// Example usage
	query := "SELECT pswd FROM tbl_proxy_credentials ORDER BY id ASC"
	data, err := GetCachedData(redisClient, DB, query)
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
