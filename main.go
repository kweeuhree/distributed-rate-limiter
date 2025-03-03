package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/kweeuhree/toolkit"
	"github.com/redis/go-redis/v9"
)

type RedisEnv struct {
	Conn string
	Pass string
}

type TokenConfig struct {
	maxTokens  int
	windowSize int
}

type application struct {
	rdb *redis.Client
	sha string
	tc  *TokenConfig
	t   *toolkit.Tools
}

func main() {
	addr := flag.String("addr", ":4000", "HTTP server addr")
	flag.Parse()

	infoLog, errorLog := setupLogger()

	rdb, sha, err := setupRedis()
	if err != nil {
		errorLog.Fatal(err)
	}
	defer rdb.Close()

	infoLog.Println("Connected to Redis...")

	app := &application{
		rdb: rdb,
		sha: sha,
		// 10 max tokens, 60-second window
		tc: setupTokenConfig(10, 60),
		// Custom toolkit with centralized logging
		t: setupToolkit(infoLog, errorLog),
	}

	srv := &http.Server{
		Addr:     *addr,
		ErrorLog: errorLog,
		Handler:  app.routes(),
	}

	app.t.InfoLog.Printf("Starting server on port %s", *addr)
	if err := srv.ListenAndServe(); err != nil {
		errorLog.Fatal(err)
	}
}

func setupRedis() (*redis.Client, string, error) {
	redisEnv, err := loadEnvVariables()
	if err != nil {
		return nil, "", err
	}

	rdb, err := redisEnv.openRedis()
	if err != nil {
		return nil, "", err
	}

	// Lua scripting ensures atomicity and avoids race conditions:
	// Lua scripts in Redis execute atomically, which means they
	// can’t be interrupted by other operations while running.
	script, err := os.ReadFile("rateLimiter.lua")
	if err != nil {
		return nil, "", fmt.Errorf("failed to read Lua script: %w", err)
	}
	// Store the compiled and loaded Lua script in Redis' server cache,
	// and get its SHA hash
	sha, err := rdb.ScriptLoad(context.Background(), string(script)).Result()
	if err != nil {
		return nil, "", fmt.Errorf("failed to load Lua script: %w", err)
	}

	return rdb, sha, nil
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

func (r *RedisEnv) openRedis() (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         r.Conn,
		Username:     "default",
		Password:     r.Pass,
		DB:           0,
		PoolSize:     10,
		MinIdleConns: 2,
		PoolTimeout:  30 * time.Second,
	})

	// Verify Redis connection
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("could not start Redis connection: %w", err)
	}

	return rdb, nil
}

func setupLogger() (*log.Logger, *log.Logger) {
	infoLog := log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime)
	errorLog := log.New(os.Stderr, "ERROR\t", log.Ldate|log.Ltime|log.Lshortfile)
	return infoLog, errorLog
}

func setupToolkit(infoLog, errorLog *log.Logger) *toolkit.Tools {
	return &toolkit.Tools{
		ErrorLog: errorLog,
		InfoLog:  infoLog,
	}
}

func setupTokenConfig(maxTokens, windowSize int) *TokenConfig {
	return &TokenConfig{
		maxTokens:  maxTokens,
		windowSize: windowSize,
	}
}
