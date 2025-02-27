package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

var ErrTooManyRequests = fmt.Errorf("too many requests")

type currentClient struct {
	IP       string
	redisKey string
	ctx      context.Context
}

func newCurrentClient(ip string, ctx context.Context) *currentClient {
	return &currentClient{
		IP:       ip,
		redisKey: fmt.Sprintf("rate_limit:%s", ip),
		ctx:      ctx,
	}
}

func (app *application) RateLimiterMiddleware(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		client := newCurrentClient(GetClientIP(r), context.Background())
		exists, err := app.clientExists(client)
		if err != nil {
			app.errorLog.Printf("failed to check if client exists: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		switch {
		case exists > 0:
			if err := app.processExistingClient(client); err != nil {
				if err == ErrTooManyRequests {
					app.errorLog.Printf("too many requests from ip: %s", client.IP)
					http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				}

				app.errorLog.Printf("redis error: %v", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
		case exists == 0:
			if err := app.trackNewIP(client); err != nil {
				http.Error(w, "failed to start tracking new ip", http.StatusInternalServerError)
			}
		}

		next.ServeHTTP(w, r)
	})
}

func (app *application) clientExists(client *currentClient) (int, error) {
	exists, err := app.rdb.Exists(client.ctx, client.redisKey).Result()
	if err != nil {
		return 0, err
	}
	return int(exists), nil
}

func (app *application) processExistingClient(client *currentClient) error {
	tokens, err := app.getClientTokens(client)
	if err != nil {
		return err
	}

	switch {
	case tokens > 0:
		_, err := app.rdb.HIncrBy(client.ctx, client.redisKey, "tokens", -1).Result()
		if err != nil {
			return err
		}
	case tokens <= 0:
		return ErrTooManyRequests
	}

	return nil
}

func (app *application) trackNewIP(client *currentClient) error {
	txPipe := app.rdb.TxPipeline()

	hashFields := map[string]interface{}{
		"tokens":    app.tc.maxTokens - 1,
		"timestamp": time.Now().String(),
	}

	txPipe.HSet(client.ctx, fmt.Sprintf("rate_limit:%s", client.IP), hashFields)
	txPipe.Expire(client.ctx, fmt.Sprintf("rate_limit:%s", client.IP), time.Duration(app.tc.windowSize)*time.Second)

	_, err := txPipe.Exec(client.ctx)
	if err != nil {
		return err
	}

	return nil
}

func (app *application) getClientTokens(client *currentClient) (int, error) {
	tokens, err := app.rdb.HGet(client.ctx, client.redisKey, "tokens").Int()

	if err != nil {
		return 0, fmt.Errorf("fedis error exists > 0: %+v", err)
	}

	return tokens, nil
}

func (app *application) LogRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		app.infoLog.Printf("%s - %s %s %s", r.RemoteAddr, r.Proto, r.Method,
			r.URL.RequestURI())
		next.ServeHTTP(w, r)
	})
}
func (app *application) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create a deferred function (which will always be run in the event
		// of a panic as Go unwinds the stack).
		defer func() {
			// Use the builtin recover function to check if there has been a
			// panic or not. If there has...
			if err := recover(); err != nil {
				// Set a "Connection: close" header on the response.
				w.Header().Set("Connection", "close")
				// Call the app.serverError helper method to return a 500
				// Internal Server response.
				app.errorLog.Println(w, fmt.Errorf("%s", err))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func GetClientIP(r *http.Request) string {
	// Check for the X-Forwarded-For header
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		// If multiple IPs are present, split by comma and return the first part
		ips := strings.Split(forwarded, ",")
		return strings.TrimSpace(ips[0])
	}

	// If X-Forwarded-For is not present, return RemoteAddr
	// This will return the IP address of the immediate connection,
	// which might be the client or a proxy
	return strings.Split(r.RemoteAddr, ":")[0]
}
