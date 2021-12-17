package main

import (
	"net/http"

	"github.com/gorilla/mux"
)

type router struct {
	r *mux.Router
}

// newRouter is a Mux HTTP router constructor.
func newRouter() *router {
	r := mux.NewRouter()
	return &router{r}
}

func (r *router) Get(uri string, f func(http.ResponseWriter, *http.Request), mid ...mux.MiddlewareFunc) {
	sub := r.r.Path(uri).Subrouter()
	sub.HandleFunc("", f).Methods(http.MethodGet, http.MethodOptions)
	sub.Use(mid...)
}

func (r *router) Post(uri string, f func(http.ResponseWriter, *http.Request), mid ...mux.MiddlewareFunc) {
	sub := r.r.Path(uri).Subrouter()
	sub.HandleFunc("", f).Methods(http.MethodPost, http.MethodOptions)
	sub.Use(mid...)
}

func (r *router) Serve(port string) error {
	return http.ListenAndServe(port, r.r)
}
