package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/paperwing/paperwing/internal/accounts"
	"github.com/paperwing/paperwing/internal/auth"
	"github.com/paperwing/paperwing/internal/domain"
)

type AccountService interface {
	Test(context.Context, domain.NewAccount) error
	Create(context.Context, domain.NewAccount) (domain.Account, error)
	List(context.Context) ([]domain.Account, error)
	Delete(context.Context, string) error
	Sync(context.Context, string) error
}

type EmailRepository interface {
	ListEmails(context.Context, domain.ListEmailOptions) (domain.EmailPage, error)
	Email(context.Context, string) (domain.Email, error)
	Attachment(context.Context, string, string) (domain.Attachment, error)
}

type AuthService interface {
	Status(context.Context, string) (auth.Status, error)
	Setup(context.Context, string, string) (auth.SessionToken, error)
	Login(context.Context, string, string) (auth.SessionToken, error)
	Authenticate(context.Context, string) (bool, error)
	AuthenticateAPIToken(context.Context, string) (auth.APITokenAccess, bool, error)
	Logout(context.Context, string) error
	CreateAPIToken(context.Context, auth.NewAPIToken) (auth.IssuedAPIToken, error)
	ListAPITokens(context.Context) ([]auth.APIToken, error)
	RevokeAPIToken(context.Context, string) error
}

type API struct {
	accounts AccountService
	emails   EmailRepository
	auth     AuthService
	logger   *slog.Logger
	loginMu  sync.Mutex
	logins   map[string]loginAttempt
}

func New(accounts AccountService, emails EmailRepository, authentication AuthService, logger *slog.Logger) http.Handler {
	api := &API{accounts: accounts, emails: emails, auth: authentication, logger: logger,
		logins: make(map[string]loginAttempt)}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", api.health)
	mux.HandleFunc("GET /auth/status", api.authStatus)
	mux.HandleFunc("POST /auth/setup", api.setupAuth)
	mux.HandleFunc("POST /auth/login", api.login)
	mux.HandleFunc("POST /auth/logout", api.logout)
	mux.Handle("GET /api-tokens", api.requireSession(http.HandlerFunc(api.listAPITokens)))
	mux.Handle("POST /api-tokens", api.requireSession(http.HandlerFunc(api.createAPIToken)))
	mux.Handle("DELETE /api-tokens/{id}", api.requireSession(http.HandlerFunc(api.revokeAPIToken)))
	mux.Handle("POST /accounts/test", api.requireScope(auth.ScopeAccountsWrite, http.HandlerFunc(api.testAccount)))
	mux.Handle("POST /accounts", api.requireScope(auth.ScopeAccountsWrite, http.HandlerFunc(api.createAccount)))
	mux.Handle("GET /accounts", api.requireScope(auth.ScopeAccountsRead, http.HandlerFunc(api.listAccounts)))
	mux.Handle("DELETE /accounts/{id}", api.requireSession(http.HandlerFunc(api.deleteAccount)))
	mux.Handle("POST /accounts/{id}/sync", api.requireScope(auth.ScopeSyncWrite, http.HandlerFunc(api.syncAccount)))
	mux.Handle("GET /emails", api.requireScope(auth.ScopeMailRead, http.HandlerFunc(api.listEmails)))
	mux.Handle("GET /emails/{id}", api.requireScope(auth.ScopeMailRead, http.HandlerFunc(api.getEmail)))
	mux.Handle("GET /emails/{id}/attachments/{attachmentId}", api.requireScope(auth.ScopeMailRead, http.HandlerFunc(api.downloadAttachment)))
	return api.logRequests(api.recoverPanic(mux))
}

func (a *API) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) testAccount(w http.ResponseWriter, r *http.Request) {
	var input domain.NewAccount
	if err := readJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.accounts.Test(r.Context(), input); err != nil {
		if errors.Is(err, accounts.ErrInvalidInput) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *API) createAccount(w http.ResponseWriter, r *http.Request) {
	var input domain.NewAccount
	if err := readJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	account, err := a.accounts.Create(r.Context(), input)
	if err != nil {
		if errors.Is(err, accounts.ErrInvalidInput) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, account)
}

func (a *API) listAccounts(w http.ResponseWriter, r *http.Request) {
	accounts, err := a.accounts.List(r.Context())
	if err != nil {
		a.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": accounts})
}

func (a *API) deleteAccount(w http.ResponseWriter, r *http.Request) {
	err := a.accounts.Delete(r.Context(), r.PathValue("id"))
	if errors.Is(err, domain.ErrNotFound) {
		writeError(w, http.StatusNotFound, "account not found")
		return
	}
	if err != nil {
		a.internalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) syncAccount(w http.ResponseWriter, r *http.Request) {
	if err := a.accounts.Sync(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *API) listEmails(w http.ResponseWriter, r *http.Request) {
	page, err := queryInt(r, "page", 1, 1, 1_000_000)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	pageSize, err := queryInt(r, "page_size", 50, 1, 200)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := a.emails.ListEmails(r.Context(), domain.ListEmailOptions{
		AccountID: r.URL.Query().Get("account_id"), Page: page, PageSize: pageSize,
	})
	if err != nil {
		a.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) getEmail(w http.ResponseWriter, r *http.Request) {
	email, err := a.emails.Email(r.Context(), r.PathValue("id"))
	if errors.Is(err, domain.ErrNotFound) {
		writeError(w, http.StatusNotFound, "email not found")
		return
	}
	if err != nil {
		a.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, email)
}

func (a *API) downloadAttachment(w http.ResponseWriter, r *http.Request) {
	attachment, err := a.emails.Attachment(r.Context(), r.PathValue("id"), r.PathValue("attachmentId"))
	if errors.Is(err, domain.ErrNotFound) {
		writeError(w, http.StatusNotFound, "attachment not found")
		return
	}
	if err != nil {
		a.internalError(w, err)
		return
	}
	file, err := os.Open(attachment.Path)
	if errors.Is(err, os.ErrNotExist) {
		writeError(w, http.StatusNotFound, "attachment file not found")
		return
	}
	if err != nil {
		a.internalError(w, err)
		return
	}
	defer file.Close()
	w.Header().Set("Content-Type", attachment.ContentType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": attachment.Filename}))
	w.Header().Set("Content-Length", strconv.FormatInt(attachment.Size, 10))
	_, _ = io.Copy(w, file)
}

func (a *API) internalError(w http.ResponseWriter, err error) {
	a.logger.Error("request failed", "error", err)
	writeError(w, http.StatusInternalServerError, "internal server error")
}

func (a *API) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		a.logger.Info("http request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(start))
	})
}

func (a *API) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				a.logger.Error("request panic", "error", recovered)
				writeError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func readJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("request body must contain one JSON object")
	}
	return nil
}

func queryInt(r *http.Request, name string, fallback, min, max int) (int, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < min || value > max {
		return 0, fmt.Errorf("%s must be between %d and %d", name, min, max)
	}
	return value, nil
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"message": message}})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
