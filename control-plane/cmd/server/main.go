package main

import (
	"net/http"

	"tenara/control-plane/internal/httpapi"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.Handle("/", httpapi.Handler())
	if err := http.ListenAndServe(":8080", mux); err != nil {
		panic(err)
	}
}
