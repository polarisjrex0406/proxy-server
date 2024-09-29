package config

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	PrimaryDB struct {
		Type     string `long:"primary-db-type" env:"PRIMARY_DB_TYPE" default:"postgres" description:""`
		Name     string `long:"primary-db-name" env:"PRIMARY_DB_NAME" default:"mimicproxy" description:""`
		Username string `long:"primary-db-username" env:"PRIMARY_DB_USERNAME" default:"postgres" description:""`
		Password string `long:"primary-db-password" env:"PRIMARY_DB_PASSWORD" default:"123456aA@" description:""`
		SSLMode  string `long:"primary-db-ssl-mode" env:"PRIMARY_DB_SSLMODE" default:"disable" description:""`
	}
}

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
