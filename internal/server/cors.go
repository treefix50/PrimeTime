package server

import "net/http"

func setCORSHeaders(w http.ResponseWriter, enabled bool) {
	if enabled {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	}
}
