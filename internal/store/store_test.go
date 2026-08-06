package store

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/paperwing/paperwing/internal/auth"
	"github.com/paperwing/paperwing/internal/domain"
	"github.com/paperwing/paperwing/internal/secure"
)

func TestOpenMigratesAPITokenExpirationMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "paperwing.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE api_tokens (
id TEXT PRIMARY KEY,
name TEXT NOT NULL,
token_prefix TEXT NOT NULL,
token_hash BLOB NOT NULL UNIQUE,
scopes_json TEXT NOT NULL,
created_at TEXT NOT NULL,
last_used_at TEXT,
expires_at TEXT NOT NULL
)`)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	cipher, err := secure.New(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	store, err := Open(path, cipher)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.db.Exec(`INSERT INTO api_tokens(
id,name,token_prefix,token_hash,scopes_json,created_at,expires_at,never_expires)
VALUES('tok_legacy','Legacy','pw_legacy',X'01','["mail:read"]','2026-01-01T00:00:00Z','2027-01-01T00:00:00Z',0)`); err != nil {
		t.Fatalf("new expiration mode column was not added: %v", err)
	}
}

func testStore(t *testing.T) *Store {
	t.Helper()
	cipher, err := secure.New(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	store, err := Open(filepath.Join(t.TempDir(), "paperwing.db"), cipher)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestStoreAuthentication(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	user := auth.User{Username: "admin", PasswordHash: "hash", CreatedAt: now}
	session := auth.Session{TokenHash: []byte("token-hash"), CreatedAt: now, ExpiresAt: now.Add(time.Hour)}
	if err := store.SetupAuth(ctx, user, session); err != nil {
		t.Fatal(err)
	}
	got, err := store.AuthUser(ctx)
	if err != nil || got.Username != user.Username || got.PasswordHash != user.PasswordHash {
		t.Fatalf("user=%#v err=%v", got, err)
	}
	valid, err := store.AuthSessionValid(ctx, session.TokenHash, now)
	if err != nil || !valid {
		t.Fatalf("valid=%v err=%v", valid, err)
	}
	if err := store.DeleteAuthSession(ctx, session.TokenHash); err != nil {
		t.Fatal(err)
	}
	valid, err = store.AuthSessionValid(ctx, session.TokenHash, now)
	if err != nil || valid {
		t.Fatalf("valid=%v err=%v after delete", valid, err)
	}
	if err := store.SetupAuth(ctx, user, session); !errors.Is(err, auth.ErrAlreadyConfigured) {
		t.Fatalf("duplicate setup error=%v", err)
	}

	tokenExpiresAt := now.Add(24 * time.Hour)
	token := auth.APITokenRecord{
		APIToken: auth.APIToken{ID: "tok_1", Name: "Assistant", TokenPrefix: "pw_example",
			Scopes: []string{auth.ScopeAccountsRead, auth.ScopeMailRead}, CreatedAt: now,
			ExpiresAt: &tokenExpiresAt},
		TokenHash: []byte("api-token-hash"),
	}
	if err := store.SaveAPIToken(ctx, token); err != nil {
		t.Fatal(err)
	}
	gotToken, err := store.APITokenByHash(ctx, token.TokenHash, now)
	if err != nil || gotToken.ID != token.ID || len(gotToken.Scopes) != 2 {
		t.Fatalf("token=%#v err=%v", gotToken, err)
	}
	usedAt := now.Add(time.Minute)
	if err := store.TouchAPIToken(ctx, token.ID, usedAt); err != nil {
		t.Fatal(err)
	}
	tokens, err := store.APITokens(ctx)
	if err != nil || len(tokens) != 1 || tokens[0].LastUsedAt == nil || !tokens[0].LastUsedAt.Equal(usedAt) {
		t.Fatalf("tokens=%#v err=%v", tokens, err)
	}
	if _, err := store.APITokenByHash(ctx, token.TokenHash, *token.ExpiresAt); !errors.Is(err, auth.ErrInvalidAPIToken) {
		t.Fatalf("expired token error=%v", err)
	}
	longLived := auth.APITokenRecord{
		APIToken: auth.APIToken{ID: "tok_2", Name: "Long lived", TokenPrefix: "pw_forever",
			Scopes: []string{auth.ScopeMailRead}, CreatedAt: now},
		TokenHash: []byte("long-lived-token-hash"),
	}
	if err := store.SaveAPIToken(ctx, longLived); err != nil {
		t.Fatal(err)
	}
	gotLongLived, err := store.APITokenByHash(ctx, longLived.TokenHash, now.Add(20*365*24*time.Hour))
	if err != nil || gotLongLived.ExpiresAt != nil {
		t.Fatalf("long-lived token=%#v err=%v", gotLongLived, err)
	}
	if err := store.DeleteAPIToken(ctx, token.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteAPIToken(ctx, token.ID); !errors.Is(err, auth.ErrAPITokenNotFound) {
		t.Fatalf("missing token error=%v", err)
	}
}

func TestStoreAccountEmailAndDeduplication(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	account := domain.AccountSecret{
		Account: domain.Account{ID: "ac_1", Name: "Inbox", Host: "imap.example.com", Port: 993,
			TLS: true, Username: "me@example.com", MonitorStatus: "starting", CreatedAt: now},
		Password: "not-plaintext",
	}
	if err := store.CreateAccount(ctx, account); err != nil {
		t.Fatal(err)
	}
	secret, err := store.AccountSecret(ctx, account.ID)
	if err != nil || secret.Password != account.Password {
		t.Fatalf("secret round trip: secret=%#v err=%v", secret, err)
	}

	email := domain.Email{
		ID: "em_1", AccountID: account.ID, UIDValidity: 10, IMAPUID: 20,
		MessageID: "one@example.com", Subject: "hello", From: []string{"a@example.com"},
		To: []string{"b@example.com"}, Headers: map[string][]string{"X-Test": {"yes"}},
		TextBody: "body", ReceivedAt: now, CreatedAt: now, Size: 123,
		Attachments: []domain.Attachment{{ID: "em_1-a001", Filename: "a.txt", ContentType: "text/plain", Size: 4, Path: "/tmp/a"}},
	}
	inserted, err := store.SaveEmail(ctx, email)
	if err != nil || !inserted {
		t.Fatalf("first insert: inserted=%v err=%v", inserted, err)
	}
	duplicate := email
	duplicate.ID = "em_different"
	inserted, err = store.SaveEmail(ctx, duplicate)
	if err != nil || inserted {
		t.Fatalf("duplicate insert: inserted=%v err=%v", inserted, err)
	}
	page, err := store.ListEmails(ctx, domain.ListEmailOptions{Page: 1, PageSize: 10})
	if err != nil || page.Total != 1 || len(page.Items) != 1 || page.Items[0].AttachmentCount != 1 {
		t.Fatalf("page=%#v err=%v", page, err)
	}
	got, err := store.Email(ctx, email.ID)
	if err != nil || got.TextBody != "body" || len(got.Attachments) != 1 {
		t.Fatalf("email=%#v err=%v", got, err)
	}
	if _, err := store.db.ExecContext(ctx, `UPDATE emails SET from_json=?,to_json=? WHERE id=?`,
		`["\"=?utf-8?B?6IW+6K6v5LqR?=\" <cloud_noreply@tencent.com>"]`,
		`["\"=?utf-8?B?eWFuZy5saXU=?=\" <yang.liu@example.com>"]`, email.ID); err != nil {
		t.Fatal(err)
	}
	page, err = store.ListEmails(ctx, domain.ListEmailOptions{Page: 1, PageSize: 10})
	if err != nil || page.Items[0].From[0] != "腾讯云 <cloud_noreply@tencent.com>" {
		t.Fatalf("legacy list address=%#v err=%v", page.Items[0].From, err)
	}
	got, err = store.Email(ctx, email.ID)
	if err != nil || got.To[0] != "yang.liu <yang.liu@example.com>" {
		t.Fatalf("legacy email address=%#v err=%v", got.To, err)
	}
	if err := store.DeleteAccount(ctx, account.ID); err != nil {
		t.Fatal(err)
	}
	_, err = store.Email(ctx, email.ID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected cascade delete, got %v", err)
	}
}
