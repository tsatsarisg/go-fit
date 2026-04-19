package httpx

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"sync/atomic"

	"github.com/go-chi/chi/v5"
)

type Envelope map[string]any

// prettyJSON is toggled on at startup in development so responses are
// human-readable; in production we use compact json.Marshal for throughput.
var prettyJSON atomic.Bool

// SetPrettyJSON enables or disables indented JSON responses. Intended to be
// called once at startup from config wiring.
func SetPrettyJSON(pretty bool) { prettyJSON.Store(pretty) }

func WriteJson(w http.ResponseWriter, status int, data Envelope) error {
	w.Header().Set("Content-Type", "application/json")

	var (
		js  []byte
		err error
	)
	if prettyJSON.Load() {
		js, err = json.MarshalIndent(data, "", "  ")
	} else {
		js, err = json.Marshal(data)
	}
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
