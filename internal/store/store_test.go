package store

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/paperwing/paperwing/internal/auth"
	"github.com/paperwing/paperwing/internal/domain"
	"github.com/paperwing/paperwing/internal/secure"
)

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
