package server

import (
	"log"
	"net/http"
	"time"
)

func logMiddleware(next http.Handler, corsEnabled bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		if corsEnabled {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		sw := newStatusResponseWriter(w)
		next.ServeHTTP(sw, r)
		log.Printf("%s %s (%s)", r.Method, r.URL.Path, time.Since(start))
	})
}
