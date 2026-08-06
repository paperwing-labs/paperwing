package httpapi

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/paperwing/paperwing/internal/auth"
)

const sessionCookieName = "paperwing_session"

type authInput struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginAttempt struct {
	Failures     int
	WindowStart  time.Time
	BlockedUntil time.Time
}

func (a *API) authStatus(w http.ResponseWriter, r *http.Request) {
	status, err := a.auth.Status(r.Context(), sessionToken(r))
	if err != nil {
		a.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"configured": status.Configured, "authenticated": status.Authenticated, "username": status.Username,
	})
}

func (a *API) setupAuth(w http.ResponseWriter, r *http.Request) {
	var input authInput
	if err := readJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	session, err := a.auth.Setup(r.Context(), input.Username, input.Password)
	if errors.Is(err, auth.ErrInvalidInput) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, auth.ErrAlreadyConfigured) {
		writeError(w, http.StatusConflict, "authentication is already configured")
		return
	}
	if err != nil {
		a.internalError(w, err)
		return
	}
	setSessionCookie(w, r, session)
	writeJSON(w, http.StatusCreated, map[string]any{
		"configured": true, "authenticated": true, "username": strings.TrimSpace(input.Username),
	})
}

func (a *API) login(w http.ResponseWriter, r *http.Request) {
	key := loginKey(r)
	if retryAfter, allowed := a.loginAllowed(key); !allowed {
		w.Header().Set("Retry-After", retryAfter)
		writeError(w, http.StatusTooManyRequests, "too many login attempts; try again shortly")
		return
	}
	var input authInput
	if err := readJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	session, err := a.auth.Login(r.Context(), input.Username, input.Password)
	if errors.Is(err, auth.ErrInvalidCredentials) {
		a.recordLoginFailure(key)
		writeError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}
	if err != nil {
		a.internalError(w, err)
		return
	}
	a.clearLoginFailures(key)
	setSessionCookie(w, r, session)
	writeJSON(w, http.StatusOK, map[string]any{
		"configured": true, "authenticated": true, "username": strings.TrimSpace(input.Username),
	})
}

func (a *API) logout(w http.ResponseWriter, r *http.Request) {
	if err := a.auth.Logout(r.Context(), sessionToken(r)); err != nil {
		a.internalError(w, err)
		return
	}
	clearSessionCookie(w, r)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *API) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		valid, err := a.auth.Authenticate(r.Context(), sessionToken(r))
		if err != nil {
			a.internalError(w, err)
			return
		}
		if !valid {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *API) requireScope(scope string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authorization := strings.TrimSpace(r.Header.Get("Authorization")); authorization != "" {
			token, ok := bearerToken(authorization)
			if !ok {
				w.Header().Set("WWW-Authenticate", "Bearer")
				writeError(w, http.StatusUnauthorized, "invalid bearer authorization")
				return
			}
			access, valid, err := a.auth.AuthenticateAPIToken(r.Context(), token)
			if err != nil {
				a.internalError(w, err)
				return
			}
			if !valid {
				w.Header().Set("WWW-Authenticate", "Bearer")
				writeError(w, http.StatusUnauthorized, "invalid or expired API token")
				return
			}
			if !access.Allows(scope) {
				writeError(w, http.StatusForbidden, "API token does not grant "+scope)
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		valid, err := a.auth.Authenticate(r.Context(), sessionToken(r))
		if err != nil {
			a.internalError(w, err)
			return
		}
		if !valid {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func bearerToken(authorization string) (string, bool) {
	parts := strings.Fields(authorization)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}

func sessionToken(r *http.Request) string {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, session auth.SessionToken) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: session.Token, Path: "/", Expires: session.ExpiresAt,
		MaxAge: int(time.Until(session.ExpiresAt).Seconds()), HttpOnly: true, Secure: requestIsHTTPS(r),
		SameSite: http.SameSiteStrictMode,
	})
}

func clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: "", Path: "/", MaxAge: -1, Expires: time.Unix(1, 0),
		HttpOnly: true, Secure: requestIsHTTPS(r), SameSite: http.SameSiteStrictMode,
	})
}

func requestIsHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	forwarded := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0])
	return strings.EqualFold(forwarded, "https")
}

func loginKey(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func (a *API) loginAllowed(key string) (string, bool) {
	a.loginMu.Lock()
	defer a.loginMu.Unlock()
	now := time.Now()
	attempt := a.logins[key]
	if now.Before(attempt.BlockedUntil) {
		seconds := int(time.Until(attempt.BlockedUntil).Seconds()) + 1
		return stringInt(seconds), false
	}
	if !attempt.WindowStart.IsZero() && now.Sub(attempt.WindowStart) > 5*time.Minute {
		delete(a.logins, key)
	}
	return "", true
}

func (a *API) recordLoginFailure(key string) {
	a.loginMu.Lock()
	defer a.loginMu.Unlock()
	now := time.Now()
	attempt := a.logins[key]
	if attempt.WindowStart.IsZero() || now.Sub(attempt.WindowStart) > 5*time.Minute {
		attempt = loginAttempt{WindowStart: now}
	}
	attempt.Failures++
	if attempt.Failures >= 5 {
		attempt.BlockedUntil = now.Add(time.Minute)
	}
	a.logins[key] = attempt
}

func (a *API) clearLoginFailures(key string) {
	a.loginMu.Lock()
	delete(a.logins, key)
	a.loginMu.Unlock()
}

func stringInt(value int) string {
	if value <= 0 {
		return "1"
	}
	return fmt.Sprintf("%d", value)
}
