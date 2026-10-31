package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/mservice/gitscanner/internal/assist"
)

// WithAssist enables turning a question in plain words into a findings filter.
// Left unset when no Claude API key is configured, so the route does not exist
// and the dashboard does not offer it.
func (s *Server) WithAssist(t assist.Translator, model string) *Server {
	s.assist, s.assistModel = t, model
	return s
}

func (s *Server) assistRoutes(mux *http.ServeMux) {
	if s.assist == nil {
		return
	}
	mux.HandleFunc("GET /v1/assist", s.handleAssistStatus)
	mux.HandleFunc("POST /v1/assist/filter", s.handleAssistFilter)
}

func (s *Server) handleAssistStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"enabled": true, "model": s.assistModel})
}

// handleAssistFilter answers with filter fields only. The question is not
// logged: it is free text from a box people paste into, and the outcome and
// timing are all an operator needs.
func (s *Server) handleAssistFilter(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Question string `json:"question"`
	}
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	slugs := make([]string, 0, len(s.providers))
	for _, p := range s.providers {
		slugs = append(slugs, p.Slug)
	}

	start := time.Now()
	f, err := s.assist.Translate(r.Context(), req.Question, slugs)
	outcome := "ok"
	switch {
	case errors.Is(err, assist.ErrEmpty), errors.Is(err, assist.ErrTooLong), errors.Is(err, assist.ErrCredentialLike):
		outcome = "rejected"
		s.writeError(w, http.StatusBadRequest, err)
	case errors.Is(err, assist.ErrRefused):
		outcome = "refused"
		s.writeError(w, http.StatusUnprocessableEntity, err)
	case err != nil:
		outcome = "failed"
		if s.log != nil {
			s.log.Warn("assist filter failed", "error", err)
		}
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "the assistant could not answer"})
	default:
		writeJSON(w, http.StatusOK, f)
	}
	if s.log != nil {
		s.log.Info("assist filter", "outcome", outcome, "ms", time.Since(start).Milliseconds())
	}
}
