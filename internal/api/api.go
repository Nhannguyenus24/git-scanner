// Package api is the REST surface (plan.md §8) and, until it is split out, the
// provider registry endpoints (plan.md §5.5).
//
// Handlers live here, not in cmd/api, so they can be tested with httptest and
// so cmd/api stays a declaration of what the binary is.
package api

import (
	"encoding/json"
	"net/http"

	"github.com/mservice/gitscanner/libs/buildinfo"
)

// NewRouter returns every route the api serves besides the probes.
func NewRouter() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/version", handleVersion)
	return mux
}

func handleVersion(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, struct {
		Service string `json:"service"`
		buildinfo.Info
	}{Service: "api", Info: buildinfo.Get()})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
