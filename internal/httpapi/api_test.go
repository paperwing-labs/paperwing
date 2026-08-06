package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/paperwing/paperwing/internal/auth"
	"github.com/paperwing/paperwing/internal/domain"
)

type accountStub struct{}

func (accountStub) Test(context.Context, domain.NewAccount) error { return nil }
func (accountStub) Create(_ context.Context, in domain.NewAccount) (domain.Account, error) {
	return domain.Account{ID: "ac_1", Name: in.Name, Host: in.Host, Port: in.Port, TLS: in.TLS,
		Username: in.Username, MonitorStatus: "starting"}, nil
}
func (accountStub) List(context.Context) ([]domain.Account, error) { return []domain.Account{}, nil }
func (accountStub) Delete(context.Context, string) error           { return nil }
func (accountStub) Sync(context.Context, string) error             { return nil }

type emailStub struct{}

func (emailStub) ListEmails(_ context.Context, opts domain.ListEmailOptions) (domain.EmailPage, error) {
	return domain.EmailPage{Items: []domain.EmailSummary{}, Page: opts.Page, PageSize: opts.PageSize}, nil
}
func (emailStub) Email(context.Context, string) (domain.Email, error) {
	return domain.Email{}, domain.ErrNotFound
}
func (emailStub) Attachment(context.Context, string, string) (domain.Attachment, error) {
	return domain.Attachment{}, domain.ErrNotFound
}

type authStub struct {
	authenticated bool
	tokenScopes   []string
}

func (s authStub) Status(context.Context, string) (auth.Status, error) {
	return auth.Status{Configured: true, Authenticated: s.authenticated, Username: "admin"}, nil
}
func (authStub) Setup(context.Context, string, string) (auth.SessionToken, error) {
	return auth.SessionToken{Token: "setup-token", ExpiresAt: time.Now().Add(time.Hour)}, nil
}
func (authStub) Login(context.Context, string, string) (auth.SessionToken, error) {
	return auth.SessionToken{Token: "login-token", ExpiresAt: time.Now().Add(time.Hour)}, nil
}
func (s authStub) Authenticate(context.Context, string) (bool, error) {
	return s.authenticated, nil
}
func (s authStub) AuthenticateAPIToken(_ context.Context, token string) (auth.APITokenAccess, bool, error) {
	if token != "valid-token" {
		return auth.APITokenAccess{}, false, nil
	}
	return auth.APITokenAccess{ID: "tok_1", Scopes: s.tokenScopes}, true, nil
}
func (authStub) Logout(context.Context, string) error { return nil }
func (authStub) CreateAPIToken(_ context.Context, input auth.NewAPIToken) (auth.IssuedAPIToken, error) {
	return auth.IssuedAPIToken{APIToken: auth.APIToken{
		ID: "tok_1", Name: input.Name, TokenPrefix: "pw_example", Scopes: input.Scopes,
		CreatedAt: time.Now(), ExpiresAt: input.ExpiresAt,
	}, Token: "pw_secret"}, nil
}
func (authStub) ListAPITokens(context.Context) ([]auth.APIToken, error) {
	return []auth.APIToken{}, nil
}
func (authStub) RevokeAPIToken(context.Context, string) error { return nil }

func testHandler() http.Handler {
	return New(accountStub{}, emailStub{}, authStub{authenticated: true}, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestCreateRejectsUnknownJSONField(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/accounts", strings.NewReader(`{
"name":"x","host":"h","port":993,"tls":true,"username":"u","password":"p","extra":true}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	testHandler().ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestListEmailsValidatesPagination(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/emails?page_size=201", nil)
	response := httptest.NewRecorder()
	testHandler().ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestMissingEmailIsNotFound(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/emails/missing", nil)
	response := httptest.NewRecorder()
	testHandler().ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestEmailAPIRequiresAuthentication(t *testing.T) {
	handler := New(accountStub{}, emailStub{}, authStub{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	request := httptest.NewRequest(http.MethodGet, "/emails", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestLoginSetsHTTPOnlySessionCookie(t *testing.T) {
	handler := New(accountStub{}, emailStub{}, authStub{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	request := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"username":"admin","password":"a password"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != sessionCookieName || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("unexpected cookies: %#v", cookies)
	}
}

func TestBearerTokenCanReadMailWithScope(t *testing.T) {
	handler := New(accountStub{}, emailStub{}, authStub{tokenScopes: []string{auth.ScopeMailRead}}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	request := httptest.NewRequest(http.MethodGet, "/emails", nil)
	request.Header.Set("Authorization", "Bearer valid-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestBearerTokenIsForbiddenWithoutRequiredScope(t *testing.T) {
	handler := New(accountStub{}, emailStub{}, authStub{authenticated: true, tokenScopes: []string{auth.ScopeMailRead}}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	request := httptest.NewRequest(http.MethodPost, "/accounts/test", strings.NewReader(`{
"name":"x","host":"h","port":993,"tls":true,"username":"u","password":"p"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer valid-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestAPITokenManagementRequiresBrowserSession(t *testing.T) {
	handler := New(accountStub{}, emailStub{}, authStub{tokenScopes: []string{auth.ScopeMailRead}}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	request := httptest.NewRequest(http.MethodGet, "/api-tokens", nil)
	request.Header.Set("Authorization", "Bearer valid-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestBearerTokenCannotDeleteAccount(t *testing.T) {
	handler := New(accountStub{}, emailStub{}, authStub{tokenScopes: []string{auth.ScopeAccountsWrite}}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	request := httptest.NewRequest(http.MethodDelete, "/accounts/ac_1", nil)
	request.Header.Set("Authorization", "Bearer valid-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestBearerTokenAccountAndSyncScopes(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   string
		scope  string
		status int
	}{
		{name: "list accounts", method: http.MethodGet, path: "/accounts", scope: auth.ScopeAccountsRead, status: http.StatusOK},
		{name: "test account", method: http.MethodPost, path: "/accounts/test", body: `{
"name":"x","host":"h","port":993,"tls":true,"username":"u","password":"p"}`, scope: auth.ScopeAccountsWrite, status: http.StatusOK},
		{name: "sync account", method: http.MethodPost, path: "/accounts/ac_1/sync", scope: auth.ScopeSyncWrite, status: http.StatusOK},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := New(accountStub{}, emailStub{}, authStub{tokenScopes: []string{test.scope}}, slog.New(slog.NewTextHandler(io.Discard, nil)))
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			request.Header.Set("Authorization", "Bearer valid-token")
			if test.body != "" {
				request.Header.Set("Content-Type", "application/json")
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

func TestCreateAPITokenUsesBrowserSession(t *testing.T) {
	handler := New(accountStub{}, emailStub{}, authStub{authenticated: true}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	request := httptest.NewRequest(http.MethodPost, "/api-tokens", strings.NewReader(`{
"name":"Assistant","scopes":["mail:read"],"expires_in_days":365}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated || !strings.Contains(response.Body.String(), `"token":"pw_secret"`) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
