package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

type application struct {
	infoLog  *log.Logger
	errorLog *log.Logger
	rdb      *redis.Client
}

type RedisEnv struct {
	Conn string
	Pass string
}

func main() {
	addr := flag.String("addr", ":4000", "HTTP server addr")
	flag.Parse()

	infoLog, errorLog := setupLogger()

	redisEnv, err := loadEnvVariables()
	if err != nil {
		errorLog.Fatal("could not connect to Redis:", err)
	}

	rdb, err := redisEnv.openRedis()
	if err != nil {
		errorLog.Fatal(err)
	}
	defer rdb.Close()

	infoLog.Println("Connected to Redis...")

	app := &application{
		infoLog:  infoLog,
		errorLog: errorLog,
		rdb:      rdb,
	}

	srv := &http.Server{
		Addr:     *addr,
		ErrorLog: errorLog,
		Handler:  app.routes(),
	}

	app.infoLog.Printf("Starting server on port %s", *addr)
	if err := srv.ListenAndServe(); err != nil {
		errorLog.Fatal(err)
	}
}

func (r *RedisEnv) openRedis() (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     r.Conn,
		Username: "default",
		Password: r.Pass,
		DB:       0,
	})

	if err := rdb.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}

	return rdb, nil
}

func loadEnvVariables() (*RedisEnv, error) {
	godotenv.Load()

	redisConn := os.Getenv("REDIS_CONN_ADDRESS")
	redisPassword := os.Getenv("REDIS_PASSWORD")
	if redisConn == "" || redisPassword == "" {
		return nil, fmt.Errorf("failed loading with the following vars: %s, %s", redisConn, redisPassword)
	}
	return &RedisEnv{
		Conn: redisConn,
		Pass: redisPassword,
	}, nil
}

func setupLogger() (*log.Logger, *log.Logger) {
	infoLog := log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime)
	errorLog := log.New(os.Stderr, "ERROR\t", log.Ldate|log.Ltime|log.Lshortfile)
	return infoLog, errorLog
}
