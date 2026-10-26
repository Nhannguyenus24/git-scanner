package api

import (
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/mservice/gitscanner/internal/secrets"
	"github.com/mservice/gitscanner/internal/store"
)

// WithSealer lets the findings routes open credentials kept for unmask-listed
// owners. Without it nothing is ever shown unmasked, whatever the switches say.
func (s *Server) WithSealer(sl *secrets.Sealer) *Server {
	s.sealer = sl
	return s
}

type settingsResponse struct {
	NotificationsEnabled bool     `json:"notifications_enabled"`
	MaskingEnabled       bool     `json:"masking_enabled"`
	UnmaskOwners         []string `json:"unmask_owners"`
	// SealingAvailable is false when SECRET_SEAL_KEY is unset, in which case
	// the unmask list and the masking switch have nothing to act on.
	SealingAvailable bool `json:"sealing_available"`
}

func (s *Server) settingsBody(st store.Settings) settingsResponse {
	return settingsResponse{
		NotificationsEnabled: st.NotificationsEnabled,
		MaskingEnabled:       st.MaskingEnabled,
		UnmaskOwners:         nonNil(st.UnmaskOwners),
		SealingAvailable:     s.sealer != nil,
	}
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	st, err := s.store.GetSettings(r.Context())
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, s.settingsBody(st))
}

// handlePutSettings changes the switches the body names. Each is a pointer so an
// omitted field is left alone, and an empty or mistyped body changes nothing.
func (s *Server) handlePutSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		NotificationsEnabled *bool `json:"notifications_enabled"`
		MaskingEnabled       *bool `json:"masking_enabled"`
	}
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.NotificationsEnabled == nil && req.MaskingEnabled == nil {
		s.writeError(w, http.StatusBadRequest, errors.New("name at least one setting"))
		return
	}
	ctx := r.Context()
	if v := req.NotificationsEnabled; v != nil {
		if err := s.store.SetNotificationsEnabled(ctx, *v); err != nil {
			s.writeError(w, http.StatusInternalServerError, err)
			return
		}
		s.log.Info("notifications switched", "enabled", *v)
	}
	if v := req.MaskingEnabled; v != nil {
		if err := s.store.SetMaskingEnabled(ctx, *v); err != nil {
			s.writeError(w, http.StatusInternalServerError, err)
			return
		}
		// Warn, not info: turning masking off is the one switch that puts live
		// credentials on screen.
		s.log.Warn("secret masking switched", "enabled", *v)
	}
	s.handleGetSettings(w, r)
}

// githubLogin is GitHub's own rule for user and organisation names.
var githubLogin = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,37}[A-Za-z0-9])?$`)

func (s *Server) handleAddUnmaskOwner(w http.ResponseWriter, r *http.Request) {
	owner := strings.TrimSpace(r.PathValue("owner"))
	if !githubLogin.MatchString(owner) {
		s.writeError(w, http.StatusBadRequest, errors.New("not a GitHub user or organisation name"))
		return
	}
	if err := s.store.AddUnmaskOwner(r.Context(), owner); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.log.Warn("unmask owner added", "owner", strings.ToLower(owner))
	s.handleGetSettings(w, r)
}

// handleRemoveUnmaskOwner also destroys the values kept for that owner; see
// store.RemoveUnmaskOwner.
func (s *Server) handleRemoveUnmaskOwner(w http.ResponseWriter, r *http.Request) {
	owner := strings.TrimSpace(r.PathValue("owner"))
	if !githubLogin.MatchString(owner) {
		s.writeError(w, http.StatusBadRequest, errors.New("not a GitHub user or organisation name"))
		return
	}
	if err := s.store.RemoveUnmaskOwner(r.Context(), owner); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.log.Info("unmask owner removed", "owner", strings.ToLower(owner))
	s.handleGetSettings(w, r)
}
