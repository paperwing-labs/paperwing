package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	ScopeMailRead      = "mail:read"
	ScopeAccountsRead  = "accounts:read"
	ScopeAccountsWrite = "accounts:write"
	ScopeSyncWrite     = "sync:write"
)

var (
	ErrInvalidAPIToken  = errors.New("invalid API token")
	ErrAPITokenNotFound = errors.New("API token not found")

	validAPITokenScopes = map[string]struct{}{
		ScopeMailRead:      {},
		ScopeAccountsRead:  {},
		ScopeAccountsWrite: {},
		ScopeSyncWrite:     {},
	}
)

type APIToken struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	TokenPrefix string     `json:"token_prefix"`
	Scopes      []string   `json:"scopes"`
	CreatedAt   time.Time  `json:"created_at"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

type APITokenRecord struct {
	APIToken
	TokenHash []byte
}

type IssuedAPIToken struct {
	APIToken
	Token string `json:"token"`
}

type NewAPIToken struct {
	Name      string
	Scopes    []string
	ExpiresAt *time.Time
}

type APITokenAccess struct {
	ID     string
	Scopes []string
}

func (a APITokenAccess) Allows(scope string) bool {
	for _, candidate := range a.Scopes {
		if candidate == scope {
			return true
		}
	}
	return false
}

func (s *Service) CreateAPIToken(ctx context.Context, input NewAPIToken) (IssuedAPIToken, error) {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" || utf8.RuneCountInString(input.Name) > 64 {
		return IssuedAPIToken{}, fmt.Errorf("%w: token name must contain between 1 and 64 characters", ErrInvalidInput)
	}
	now := s.now()
	if input.ExpiresAt != nil {
		expiresAt := input.ExpiresAt.UTC()
		if !expiresAt.After(now) || expiresAt.After(now.Add(3650*24*time.Hour)) {
			return IssuedAPIToken{}, fmt.Errorf("%w: token expiration must be within the next 3650 days", ErrInvalidInput)
		}
		input.ExpiresAt = &expiresAt
	}
	scopes, err := normalizeScopes(input.Scopes)
	if err != nil {
		return IssuedAPIToken{}, err
	}

	idBytes := make([]byte, 16)
	secret := make([]byte, 32)
	if _, err := rand.Read(idBytes); err != nil {
		return IssuedAPIToken{}, err
	}
	if _, err := rand.Read(secret); err != nil {
		return IssuedAPIToken{}, err
	}
	token := "pw_" + base64.RawURLEncoding.EncodeToString(secret)
	metadata := APIToken{
		ID:          "tok_" + hex.EncodeToString(idBytes),
		Name:        input.Name,
		TokenPrefix: token[:11],
		Scopes:      scopes,
		CreatedAt:   now,
		ExpiresAt:   input.ExpiresAt,
	}
	record := APITokenRecord{APIToken: metadata, TokenHash: tokenHash(token)}
	if err := s.repo.SaveAPIToken(ctx, record); err != nil {
		return IssuedAPIToken{}, err
	}
	return IssuedAPIToken{APIToken: metadata, Token: token}, nil
}

func (s *Service) ListAPITokens(ctx context.Context) ([]APIToken, error) {
	return s.repo.APITokens(ctx)
}

func (s *Service) RevokeAPIToken(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return ErrAPITokenNotFound
	}
	return s.repo.DeleteAPIToken(ctx, id)
}

func (s *Service) AuthenticateAPIToken(ctx context.Context, token string) (APITokenAccess, bool, error) {
	token = strings.TrimSpace(token)
	if !strings.HasPrefix(token, "pw_") || len(token) < 32 {
		return APITokenAccess{}, false, nil
	}
	now := s.now()
	metadata, err := s.repo.APITokenByHash(ctx, tokenHash(token), now)
	if errors.Is(err, ErrInvalidAPIToken) {
		return APITokenAccess{}, false, nil
	}
	if err != nil {
		return APITokenAccess{}, false, err
	}
	_ = s.repo.TouchAPIToken(ctx, metadata.ID, now)
	return APITokenAccess{ID: metadata.ID, Scopes: metadata.Scopes}, true, nil
}

func normalizeScopes(input []string) ([]string, error) {
	if len(input) == 0 {
		return nil, fmt.Errorf("%w: select at least one API token scope", ErrInvalidInput)
	}
	unique := make(map[string]struct{}, len(input))
	for _, raw := range input {
		scope := strings.TrimSpace(raw)
		if _, ok := validAPITokenScopes[scope]; !ok {
			return nil, fmt.Errorf("%w: unknown API token scope %q", ErrInvalidInput, scope)
		}
		unique[scope] = struct{}{}
	}
	scopes := make([]string, 0, len(unique))
	for scope := range unique {
		scopes = append(scopes, scope)
	}
	sort.Strings(scopes)
	return scopes, nil
}
