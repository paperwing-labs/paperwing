package auth

import (
	"context"
	"errors"
	"testing"
	"time"
)

type memoryRepository struct {
	user     *User
	sessions map[string]Session
	tokens   map[string]APITokenRecord
}

func (r *memoryRepository) AuthUser(context.Context) (User, error) {
	if r.user == nil {
		return User{}, ErrNotConfigured
	}
	return *r.user, nil
}

func (r *memoryRepository) SetupAuth(_ context.Context, user User, session Session) error {
	if r.user != nil {
		return ErrAlreadyConfigured
	}
	r.user = &user
	return r.SaveAuthSession(context.Background(), session)
}

func (r *memoryRepository) SaveAuthSession(_ context.Context, session Session) error {
	if r.sessions == nil {
		r.sessions = make(map[string]Session)
	}
	r.sessions[string(session.TokenHash)] = session
	return nil
}

func (r *memoryRepository) AuthSessionValid(_ context.Context, hash []byte, now time.Time) (bool, error) {
	session, ok := r.sessions[string(hash)]
	return ok && session.ExpiresAt.After(now), nil
}

func (r *memoryRepository) DeleteAuthSession(_ context.Context, hash []byte) error {
	delete(r.sessions, string(hash))
	return nil
}

func (r *memoryRepository) DeleteExpiredAuthSessions(_ context.Context, now time.Time) error {
	for hash, session := range r.sessions {
		if !session.ExpiresAt.After(now) {
			delete(r.sessions, hash)
		}
	}
	return nil
}

func (r *memoryRepository) SaveAPIToken(_ context.Context, token APITokenRecord) error {
	if r.tokens == nil {
		r.tokens = make(map[string]APITokenRecord)
	}
	r.tokens[string(token.TokenHash)] = token
	return nil
}

func (r *memoryRepository) APITokens(context.Context) ([]APIToken, error) {
	tokens := make([]APIToken, 0, len(r.tokens))
	for _, token := range r.tokens {
		tokens = append(tokens, token.APIToken)
	}
	return tokens, nil
}

func (r *memoryRepository) APITokenByHash(_ context.Context, hash []byte, now time.Time) (APIToken, error) {
	token, ok := r.tokens[string(hash)]
	if !ok || (token.ExpiresAt != nil && !token.ExpiresAt.After(now)) {
		return APIToken{}, ErrInvalidAPIToken
	}
	return token.APIToken, nil
}

func (r *memoryRepository) TouchAPIToken(_ context.Context, id string, usedAt time.Time) error {
	for hash, token := range r.tokens {
		if token.ID == id {
			token.LastUsedAt = &usedAt
			r.tokens[hash] = token
		}
	}
	return nil
}

func (r *memoryRepository) DeleteAPIToken(_ context.Context, id string) error {
	for hash, token := range r.tokens {
		if token.ID == id {
			delete(r.tokens, hash)
			return nil
		}
	}
	return ErrAPITokenNotFound
}

func TestSetupLoginAndLogout(t *testing.T) {
	repository := &memoryRepository{}
	service := New(repository)
	now := time.Date(2026, 8, 5, 10, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	status, err := service.Status(context.Background(), "")
	if err != nil || status.Configured {
		t.Fatalf("initial status=%#v err=%v", status, err)
	}
	token, err := service.Setup(context.Background(), "admin", "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	status, err = service.Status(context.Background(), token.Token)
	if err != nil || !status.Configured || !status.Authenticated || status.Username != "admin" {
		t.Fatalf("configured status=%#v err=%v", status, err)
	}
	if _, err := service.Login(context.Background(), "admin", "wrong password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong password error=%v", err)
	}
	login, err := service.Login(context.Background(), "admin", "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Logout(context.Background(), login.Token); err != nil {
		t.Fatal(err)
	}
	authenticated, err := service.Authenticate(context.Background(), login.Token)
	if err != nil || authenticated {
		t.Fatalf("authenticated=%v err=%v after logout", authenticated, err)
	}
}

func TestPasswordValidation(t *testing.T) {
	service := New(&memoryRepository{})
	if _, err := service.Setup(context.Background(), "admin", "too-short"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("short password error=%v", err)
	}
}

func TestAPITokenLifecycleAndScopes(t *testing.T) {
	repository := &memoryRepository{}
	service := New(repository)
	now := time.Date(2026, 8, 6, 10, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	expiresAt := now.Add(24 * time.Hour)
	issued, err := service.CreateAPIToken(context.Background(), NewAPIToken{
		Name: " Personal assistant ", Scopes: []string{ScopeMailRead, ScopeAccountsRead, ScopeMailRead},
		ExpiresAt: &expiresAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	if issued.Name != "Personal assistant" || issued.Token == "" || issued.TokenPrefix == "" || len(issued.Scopes) != 2 {
		t.Fatalf("issued token=%#v", issued)
	}
	access, valid, err := service.AuthenticateAPIToken(context.Background(), issued.Token)
	if err != nil || !valid || !access.Allows(ScopeMailRead) || access.Allows(ScopeAccountsWrite) {
		t.Fatalf("access=%#v valid=%v err=%v", access, valid, err)
	}
	tokens, err := service.ListAPITokens(context.Background())
	if err != nil || len(tokens) != 1 || tokens[0].LastUsedAt == nil {
		t.Fatalf("tokens=%#v err=%v", tokens, err)
	}
	if err := service.RevokeAPIToken(context.Background(), issued.ID); err != nil {
		t.Fatal(err)
	}
	_, valid, err = service.AuthenticateAPIToken(context.Background(), issued.Token)
	if err != nil || valid {
		t.Fatalf("valid=%v err=%v after revoke", valid, err)
	}
}

func TestAPITokenValidationAndExpiration(t *testing.T) {
	repository := &memoryRepository{}
	service := New(repository)
	now := time.Date(2026, 8, 6, 10, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	expiresAt := now.Add(time.Hour)
	if _, err := service.CreateAPIToken(context.Background(), NewAPIToken{
		Name: "bad", Scopes: []string{"everything"}, ExpiresAt: &expiresAt,
	}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("unknown scope error=%v", err)
	}
	issued, err := service.CreateAPIToken(context.Background(), NewAPIToken{
		Name: "short lived", Scopes: []string{ScopeMailRead}, ExpiresAt: &expiresAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Hour)
	_, valid, err := service.AuthenticateAPIToken(context.Background(), issued.Token)
	if err != nil || valid {
		t.Fatalf("expired valid=%v err=%v", valid, err)
	}

	longLived, err := service.CreateAPIToken(context.Background(), NewAPIToken{
		Name: "long lived", Scopes: []string{ScopeMailRead},
	})
	if err != nil || longLived.ExpiresAt != nil {
		t.Fatalf("long-lived token=%#v err=%v", longLived, err)
	}
	now = now.Add(20 * 365 * 24 * time.Hour)
	_, valid, err = service.AuthenticateAPIToken(context.Background(), longLived.Token)
	if err != nil || !valid {
		t.Fatalf("long-lived valid=%v err=%v", valid, err)
	}
}
