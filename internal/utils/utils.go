package utils

import (
	"encoding/json"
	"net/http"
)

type Envelope map[string]interface{}

func WriteJson(w http.ResponseWriter, status int, data Envelope) error {
	js, err := json.MarshalIndent(data, "", " ")
	if err != nil {
		return nil
	}

	w.Header().Set("Content-type", "application/json")
	w.WriteHeader(status)
	w.Write(js)
	return nil
}