package main

import "net/http"

func (app *application) v1GetEndpoint(w http.ResponseWriter, r *http.Request) {
	app.infoLog.Println("getting")
}

func (app *application) v1PostEndpoint(w http.ResponseWriter, r *http.Request) {
	app.infoLog.Println("posting")
}
