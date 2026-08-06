// Package auth implements Paperwing's single-user password and session authentication.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

var (
	ErrNotConfigured      = errors.New("authentication is not configured")
	ErrAlreadyConfigured  = errors.New("authentication is already configured")
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrInvalidInput       = errors.New("invalid authentication input")
)

const (
	sessionDuration = 30 * 24 * time.Hour
	argonMemory     = 19 * 1024
	argonIterations = 2
	argonParallel   = 1
	argonKeyLength  = 32
)

type User struct {
	Username     string
	PasswordHash string
	CreatedAt    time.Time
}

type Session struct {
	TokenHash []byte
	CreatedAt time.Time
	ExpiresAt time.Time
}

type SessionToken struct {
	Token     string
	ExpiresAt time.Time
}

type Status struct {
	Configured    bool
	Authenticated bool
	Username      string
}

type Repository interface {
	AuthUser(context.Context) (User, error)
	SetupAuth(context.Context, User, Session) error
	SaveAuthSession(context.Context, Session) error
	AuthSessionValid(context.Context, []byte, time.Time) (bool, error)
	DeleteAuthSession(context.Context, []byte) error
	DeleteExpiredAuthSessions(context.Context, time.Time) error
	SaveAPIToken(context.Context, APITokenRecord) error
	APITokens(context.Context) ([]APIToken, error)
	APITokenByHash(context.Context, []byte, time.Time) (APIToken, error)
	TouchAPIToken(context.Context, string, time.Time) error
	DeleteAPIToken(context.Context, string) error
}

type Service struct {
	repo Repository
	now  func() time.Time
}

func New(repo Repository) *Service {
	return &Service{repo: repo, now: func() time.Time { return time.Now().UTC() }}
}

func (s *Service) Status(ctx context.Context, token string) (Status, error) {
	user, err := s.repo.AuthUser(ctx)
	if errors.Is(err, ErrNotConfigured) {
		return Status{}, nil
	}
	if err != nil {
		return Status{}, err
	}
	status := Status{Configured: true}
	if token == "" {
		return status, nil
	}
	valid, err := s.repo.AuthSessionValid(ctx, tokenHash(token), s.now())
	if err != nil {
		return Status{}, err
	}
	if valid {
		status.Authenticated = true
		status.Username = user.Username
	}
	return status, nil
}

func (s *Service) Authenticate(ctx context.Context, token string) (bool, error) {
	if token == "" {
		return false, nil
	}
	return s.repo.AuthSessionValid(ctx, tokenHash(token), s.now())
}

func (s *Service) Setup(ctx context.Context, username, password string) (SessionToken, error) {
	username = strings.TrimSpace(username)
	if err := validateCredentials(username, password); err != nil {
		return SessionToken{}, err
	}
	passwordHash, err := hashPassword(password)
	if err != nil {
		return SessionToken{}, err
	}
	now := s.now()
	token, session, err := newSession(now)
	if err != nil {
		return SessionToken{}, err
	}
	if err := s.repo.SetupAuth(ctx, User{Username: username, PasswordHash: passwordHash, CreatedAt: now}, session); err != nil {
		return SessionToken{}, err
	}
	return token, nil
}

func (s *Service) Login(ctx context.Context, username, password string) (SessionToken, error) {
	user, err := s.repo.AuthUser(ctx)
	if err != nil {
		if errors.Is(err, ErrNotConfigured) {
			return SessionToken{}, ErrInvalidCredentials
		}
		return SessionToken{}, err
	}
	usernameValid := subtle.ConstantTimeCompare([]byte(strings.TrimSpace(username)), []byte(user.Username)) == 1
	passwordValid := verifyPassword(user.PasswordHash, password)
	if !usernameValid || !passwordValid {
		return SessionToken{}, ErrInvalidCredentials
	}
	now := s.now()
	token, session, err := newSession(now)
	if err != nil {
		return SessionToken{}, err
	}
	if err := s.repo.SaveAuthSession(ctx, session); err != nil {
		return SessionToken{}, err
	}
	_ = s.repo.DeleteExpiredAuthSessions(ctx, now)
	return token, nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.repo.DeleteAuthSession(ctx, tokenHash(token))
}

func validateCredentials(username, password string) error {
	if username == "" || utf8.RuneCountInString(username) > 64 {
		return fmt.Errorf("%w: username must contain between 1 and 64 characters", ErrInvalidInput)
	}
	passwordLength := utf8.RuneCountInString(password)
	if passwordLength < 12 || len(password) > 1024 {
		return fmt.Errorf("%w: password must contain at least 12 characters", ErrInvalidInput)
	}
	return nil
}

func newSession(now time.Time) (SessionToken, Session, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return SessionToken{}, Session{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	expiresAt := now.Add(sessionDuration)
	return SessionToken{Token: token, ExpiresAt: expiresAt}, Session{
		TokenHash: tokenHash(token), CreatedAt: now, ExpiresAt: expiresAt,
	}, nil
}

func tokenHash(token string) []byte {
	hash := sha256.Sum256([]byte(token))
	return hash[:]
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, argonIterations, argonMemory, argonParallel, argonKeyLength)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, argonMemory,
		argonIterations, argonParallel, base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash)), nil
}

func verifyPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	version, err := strconv.Atoi(strings.TrimPrefix(parts[2], "v="))
	if err != nil || version != argon2.Version {
		return false
	}
	var memory, iterations uint32
	var parallel uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallel); err != nil {
		return false
	}
	if memory < 8*1024 || memory > 256*1024 || iterations < 1 || iterations > 10 || parallel < 1 || parallel > 8 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) < 8 || len(salt) > 64 {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(want) < 16 || len(want) > 64 {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, iterations, memory, parallel, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}
