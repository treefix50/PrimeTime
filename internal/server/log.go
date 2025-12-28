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
		log.Printf(
			"level=info msg=\"http request\" method=%s path=%s status=%d duration=%s",
			r.Method,
			r.URL.Path,
			sw.Status(),
			time.Since(start),
		)
	})
}
