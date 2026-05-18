package main

import (
	"encoding/json"
	"log"
	"net/http"
)
//wrapper
func respondWithError(w http.ResponseWriter, code int, msg string, err error) {
	//checks err status
	if err != nil {
		log.Println(err)
	}
	//checks code status
	if code > 499 {
		log.Printf("Responding with 5XX error: %s", msg)
	}
	//creates error struct
	type errorResponse struct {
		Error string `json:"error"`
	}
	//responds with json, no returns so always runs
	respondWithJSON(w, code, errorResponse{
		Error: msg,
	})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	dat, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(500)
		return
	}
	w.WriteHeader(code)
	w.Write(dat)
}
