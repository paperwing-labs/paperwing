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
