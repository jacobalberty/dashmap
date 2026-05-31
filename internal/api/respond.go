package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// Error codes used in the response envelope.
const (
	codeBadRequest  = "bad_request"
	codeNotFound    = "not_found"
	codeCarNotFound = "car_not_found"
	codeInternal    = "internal"
)

type errBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// envelope is the uniform JSON response shape: { data, meta?, error }.
type envelope struct {
	Data  any      `json:"data"`
	Meta  any      `json:"meta,omitempty"`
	Error *errBody `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, data, meta any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(envelope{Data: data, Meta: meta}); err != nil {
		slog.Error("api: encode response", "err", err)
	}
}

func writeErr(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(envelope{Error: &errBody{Code: code, Message: msg}}); err != nil {
		slog.Error("api: encode error response", "err", err)
	}
}
