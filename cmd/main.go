package main

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"net/http"

	"github.com/go-redis/redis/v8"
	_ "github.com/lib/pq"
	"github.com/omimic12/proxy-server/internal"
)

var (
	ctx         = context.Background()
	redisClient *redis.Client
	DB          *sql.DB
)

func main() {
	internal.LoadConfig()
	// Test PostgreSQL caching via Redis
	// Connect to PostgresSQL
	dbms := internal.GetConfig("DB_TYPE")
	var err error
	DB, err = sql.Open(dbms, internal.ConnectionString())
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

	var addr = flag.String("addr", ":8080", "proxy address")
	caCertFile := flag.String("cacertfile", "/root/.local/share/mkcert/rootCA.pem", "certificate .pem file for trusted CA")
	caKeyFile := flag.String("cakeyfile", "/root/.local/share/mkcert/rootCA-key.pem", "key .pem file for trusted CA")
	flag.Parse()

	// proxy := &internal.ForwardProxy{
	// 	Ctx:         ctx,
	// 	RedisClient: redisClient,
	// 	DB:          DB,
	// }
	proxy := internal.CreateMitmProxy(*caCertFile, *caKeyFile)

	log.Println("Starting proxy server on", *addr)
	if err := http.ListenAndServe(*addr, proxy); err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}
