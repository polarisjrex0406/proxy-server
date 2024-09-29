package main

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"net/http"

	"github.com/go-redis/redis/v8"
	_ "github.com/lib/pq"
	"github.com/omimic12/proxy-server/config"
	"github.com/omimic12/proxy-server/pkg"
)

var (
	ctx         = context.Background()
	redisClient *redis.Client
	db          *sql.DB
)

func main() {
	config.LoadConfig()
	// Test PostgreSQL caching via Redis
	// Connect to PostgresSQL
	dbms := config.GetConfig("DB_TYPE")
	var err error
	db, err = sql.Open(dbms, config.ConnectionString())
	if err != nil {
		log.Fatal(err)
	}
	if err = db.Ping(); err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	// Connect to Redis
	redisClient = redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer redisClient.Close()

	var addr = flag.String("addr", ":8080", "proxy address")
	caCertFile := flag.String("cacertfile", "/root/.local/share/mkcert/rootCA.pem", "certificate .pem file for trusted CA")
	caKeyFile := flag.String("cakeyfile", "/root/.local/share/mkcert/rootCA-key.pem", "key .pem file for trusted CA")
	flag.Parse()

	proxy := pkg.CreateMitmProxy(*caCertFile, *caKeyFile, ctx, redisClient, db)

	log.Println("Starting proxy server on", *addr)
	if err := http.ListenAndServe(*addr, proxy); err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}
