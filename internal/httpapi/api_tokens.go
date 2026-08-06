package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/paperwing/paperwing/internal/auth"
)

type createAPITokenInput struct {
	Name          string   `json:"name"`
	Scopes        []string `json:"scopes"`
	ExpiresInDays *int     `json:"expires_in_days"`
}

func (a *API) listAPITokens(w http.ResponseWriter, r *http.Request) {
	tokens, err := a.auth.ListAPITokens(r.Context())
	if err != nil {
		a.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": tokens})
}

func (a *API) createAPIToken(w http.ResponseWriter, r *http.Request) {
	var input createAPITokenInput
	if err := readJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	expiresInDays := 365
	if input.ExpiresInDays != nil {
		expiresInDays = *input.ExpiresInDays
	}
	if expiresInDays < 0 || expiresInDays > 3650 {
		writeError(w, http.StatusBadRequest, "expires_in_days must be between 0 and 3650")
		return
	}
	var expiresAt *time.Time
	if expiresInDays > 0 {
		value := time.Now().UTC().Add(time.Duration(expiresInDays) * 24 * time.Hour)
		expiresAt = &value
	}
	token, err := a.auth.CreateAPIToken(r.Context(), auth.NewAPIToken{
		Name: input.Name, Scopes: input.Scopes,
		ExpiresAt: expiresAt,
	})
	if errors.Is(err, auth.ErrInvalidInput) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		a.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, token)
}

func (a *API) revokeAPIToken(w http.ResponseWriter, r *http.Request) {
	if err := a.auth.RevokeAPIToken(r.Context(), r.PathValue("id")); err != nil {
		if errors.Is(err, auth.ErrAPITokenNotFound) {
			writeError(w, http.StatusNotFound, "API token not found")
			return
		}
		a.internalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
