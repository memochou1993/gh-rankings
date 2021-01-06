package handler

import (
	"encoding/json"
	"log"
	"net/http"
)

func response(w http.ResponseWriter, code int, payload interface{}) {
	w.WriteHeader(code)

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func closeBody(r *http.Request) {
	if err := r.Body.Close(); err != nil {
		log.Fatalln(err.Error())
	}
}
