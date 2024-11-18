package database

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/omimic12/proxy-server/config"
)

const driverName = "postgres"

func Connect() *sql.DB {
	db, err := sql.Open(driverName, connectionString())
	if err != nil {
		log.Fatal(err)
	}
	if err = db.Ping(); err != nil {
		log.Fatal(err)
	}
	return db
}

func connectionString() string {
	cfg, err := config.GetConfig()
	if err != nil {
		return ""
	}
	// Construct the connection string
	dsn := fmt.Sprintf("user=%s password=%s dbname=%s host=%s port=%d sslmode=%s",
		cfg.Postgres.User,
		cfg.Postgres.Password,
		cfg.Postgres.DBName,
		cfg.Postgres.Host,
		cfg.Postgres.Port,
		cfg.Postgres.SSLMode,
	)
	return dsn
}
