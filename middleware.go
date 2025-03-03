package main

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

type StatusMessage struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

var TooManyRequestsResponse = &StatusMessage{
	Status:  "error",
	Message: "Too many requests.",
}

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
		// Create a client with their IP and request context
		client := newCurrentClient(app.t.GetClientIP(r), context.Background())
		now := time.Now().UnixMicro()
		// Check if the request is allowed by executing the cached Lua script
		allowed, err := app.rdb.EvalSha(
			client.ctx, app.sha,
			[]string{client.redisKey}, // Keys
			app.tc.maxTokens,          // Maximum number of tokens allowed
			app.tc.windowSize,         // Window size (expiration time)
			now,                       // Current timestamp in microseconds
			1,                         // Tokens requested (1 per request)
		).Int()
		if err != nil {
			panic(err)
		}

		// Reject the request if rate limit is exceeded
		if allowed != 1 {
			app.t.ErrorLog.Printf("too many requests from IP %s", client.IP)
			app.t.WriteJSON(w, http.StatusTooManyRequests, TooManyRequestsResponse)
			return
		}
		next.ServeHTTP(w, r)
	})
}
