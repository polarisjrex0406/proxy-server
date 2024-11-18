package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/go-redis/redis/v8"
	_ "github.com/lib/pq"
	"github.com/omimic12/proxy-server/config"
	"github.com/omimic12/proxy-server/database"
	"github.com/omimic12/proxy-server/pkg"
)

var (
	ctx         = context.Background()
	redisClient *redis.Client
	db          *sql.DB
)

func main() {
	// 1. Load configuration
	cfg, err := config.GetConfig()
	if err != nil {
		panic(err)
	}

	// Test PostgreSQL caching via Redis
	// Connect to PostgresSQL
	db = database.Connect()
	defer db.Close()

	// Connect to Redis
	redisClient = redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
	})
	defer redisClient.Close()

	addr := flag.String("addr", fmt.Sprintf(":%d", cfg.Server.Port), "proxy address")
	caCertFile := flag.String("cacertfile", "/root/.local/share/mkcert/rootCA.pem", "certificate .pem file for trusted CA")
	caKeyFile := flag.String("cakeyfile", "/root/.local/share/mkcert/rootCA-key.pem", "key .pem file for trusted CA")
	flag.Parse()

	proxy := pkg.CreateMitmProxy(*caCertFile, *caKeyFile, ctx, redisClient, db)

	log.Println("Starting proxy server on", *addr)
	if err := http.ListenAndServe(*addr, proxy); err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}
