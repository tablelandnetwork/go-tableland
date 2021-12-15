package main

import (
	"net/http"

	"github.com/gorilla/mux"
)

type router struct{}

var (
	dispatcher = mux.NewRouter()
)

// NewRouter is a Mux HTTP router constructor
func NewRouter() *router {
	return &router{}
}

func (*router) Get(uri string, f func(http.ResponseWriter, *http.Request)) {
	dispatcher.HandleFunc(uri, f).Methods("GET")
}

func (*router) Post(uri string, f func(http.ResponseWriter, *http.Request)) {
	dispatcher.HandleFunc(uri, f).Methods("POST")
}

func (*router) Serve(port string) error {
	return http.ListenAndServe(port, dispatcher)
}
