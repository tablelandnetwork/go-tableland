package main

import (
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

type router struct {
	r *mux.Router
}

// newRouter is a Mux HTTP router constructor.
func newRouter() *router {
	r := mux.NewRouter()
	r.PathPrefix("/").Methods(http.MethodOptions) // accept OPTIONS on all routes and do nothing
	return &router{r}
}

// Get creates a subroute on the specified URI that only accepts GET. You can provide specific middlewares.
func (r *router) Get(uri string, f func(http.ResponseWriter, *http.Request), mid ...mux.MiddlewareFunc) {
	sub := r.r.Path(uri).Subrouter()
	sub.HandleFunc("", f).Methods(http.MethodGet)
	sub.Use(mid...)
}

// Post creates a subroute on the specified URI that only accepts POST. You can provide specific middlewares.
func (r *router) Post(uri string, f func(http.ResponseWriter, *http.Request), mid ...mux.MiddlewareFunc) {
	sub := r.r.Path(uri).Subrouter()
	sub.HandleFunc("", f).Methods(http.MethodPost)
	sub.Use(mid...)
}

// Delete creates a subroute on the specified URI that only accepts DELETE. You can provide specific middlewares.
func (r *router) Delete(uri string, f func(http.ResponseWriter, *http.Request), mid ...mux.MiddlewareFunc) {
	sub := r.r.Path(uri).Subrouter()
	sub.HandleFunc("", f).Methods(http.MethodDelete)
	sub.Use(mid...)
}

// Use adds middlewares to all routes. Should be used when a middleware should be execute all all routes (e.g. CORS).
func (r *router) Use(mid ...mux.MiddlewareFunc) {
	r.r.Use(mid...)
}

// Serve starts listening on the specified port.
func (r *router) Serve(port string) error {
	srv := &http.Server{
		Addr:         port,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 20 * time.Second,
		IdleTimeout:  120 * time.Second,
		Handler:      r.r,
	}
	return srv.ListenAndServe()
}
