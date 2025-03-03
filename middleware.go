package main

import (
	"context"
	"fmt"
	"net/http"
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

		client := newCurrentClient(app.t.GetClientIP(r), context.Background())
		exists, err := app.clientExists(client)
		if err != nil {
			app.t.ErrorLog.Printf("failed to check if client exists: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		switch {
		case exists > 0:
			if err := app.processExistingClient(client); err != nil {
				if err == ErrTooManyRequests {
					app.t.ErrorLog.Printf("too many requests from ip: %s", client.IP)
					http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
					return
				}

				app.t.ErrorLog.Printf("redis error: %v", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
		case exists == 0:
			if err := app.trackNewIP(client); err != nil {
				http.Error(w, "failed to start tracking new ip", http.StatusInternalServerError)
				return
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
