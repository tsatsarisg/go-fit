package utils

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

type Envelope map[string]any

func WriteJson(w http.ResponseWriter, status int, data Envelope) error {
	w.Header().Set("Content-Type", "application/json")
	js, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		http.Error(w, "Error encoding JSON", http.StatusInternalServerError)
		return err
	}
	js = append(js, '\n')
	w.WriteHeader(status)
	w.Write(js)
	return nil
}

func ReadIdParam(r *http.Request, key string) (int64, error) {
	param := chi.URLParam(r, key)
	if param == "" {
		return 0, errors.New("missing or invalid ID parameter")
	}
	id, err := strconv.ParseInt(param, 10, 64)
	if err != nil {
		return 0, errors.New("invalid ID parameter")
	}
	return id, nil
}
