package main

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func (app *application) routes() *httprouter.Router {
	router := httprouter.New()

	v1 := "/api/v1"
	router.HandlerFunc(http.MethodGet, v1+"/get", app.v1GetEndpoint)
	router.HandlerFunc(http.MethodPost, v1+"/post", app.v1PostEndpoint)

	return router
}
