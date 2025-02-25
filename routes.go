package main

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/justinas/alice"
)

func (app *application) routes() http.Handler {
	router := httprouter.New()

	v1 := "/api/v1"
	router.HandlerFunc(http.MethodGet, v1+"/get", app.v1GetEndpoint)
	router.HandlerFunc(http.MethodPost, v1+"/post", app.v1PostEndpoint)

	chain := alice.New(app.LogRequest, app.RateLimiterMiddleware)

	return chain.Then(router)
}
