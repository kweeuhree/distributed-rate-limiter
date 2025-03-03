package main

import "net/http"

func (app *application) v1GetEndpoint(w http.ResponseWriter, r *http.Request) {
	app.t.InfoLog.Println("getting")
}

func (app *application) v1PostEndpoint(w http.ResponseWriter, r *http.Request) {
	app.t.InfoLog.Println("posting")
}
