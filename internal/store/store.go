package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/paperwing/paperwing/internal/domain"
	"github.com/paperwing/paperwing/internal/mailaddr"
	"github.com/paperwing/paperwing/internal/secure"
	_ "modernc.org/sqlite"
)

type Store struct {
	db     *sql.DB
	cipher *secure.Cipher
}

const schema = `
PRAGMA foreign_keys = ON;
PRAGMA journal_mode = WAL;
PRAGMA busy_timeout = 5000;

CREATE TABLE IF NOT EXISTS accounts (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  host TEXT NOT NULL,
  port INTEGER NOT NULL,
  tls INTEGER NOT NULL,
  username TEXT NOT NULL,
  password_encrypted BLOB NOT NULL,
  uid_validity INTEGER NOT NULL DEFAULT 0,
  last_uid INTEGER NOT NULL DEFAULT 0,
  monitor_status TEXT NOT NULL DEFAULT 'starting',
  latest_connection_error TEXT NOT NULL DEFAULT '',
  last_successful_sync_at TEXT,
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS emails (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  uid_validity INTEGER NOT NULL,
  imap_uid INTEGER NOT NULL,
  message_id TEXT NOT NULL DEFAULT '',
  subject TEXT NOT NULL DEFAULT '',
  from_json TEXT NOT NULL DEFAULT '[]',
  to_json TEXT NOT NULL DEFAULT '[]',
  cc_json TEXT NOT NULL DEFAULT '[]',
  headers_json TEXT NOT NULL DEFAULT '{}',
  text_body TEXT NOT NULL DEFAULT '',
  html_body TEXT NOT NULL DEFAULT '',
  sent_at TEXT,
  received_at TEXT NOT NULL,
  size INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  UNIQUE(account_id, uid_validity, imap_uid)
);

CREATE INDEX IF NOT EXISTS emails_received_idx ON emails(received_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS emails_account_received_idx ON emails(account_id, received_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS attachments (
  id TEXT PRIMARY KEY,
  email_id TEXT NOT NULL REFERENCES emails(id) ON DELETE CASCADE,
  filename TEXT NOT NULL,
  content_type TEXT NOT NULL,
  content_id TEXT NOT NULL DEFAULT '',
  size INTEGER NOT NULL,
  path TEXT NOT NULL,
  UNIQUE(email_id, id)
);

CREATE TABLE IF NOT EXISTS auth_user (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  username TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS auth_sessions (
  token_hash BLOB PRIMARY KEY,
  created_at TEXT NOT NULL,
  expires_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS auth_sessions_expires_idx ON auth_sessions(expires_at);
`

func Open(path string, cipher *secure.Cipher) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("initialize database: %w", err)
	}
	return &Store{db: db, cipher: cipher}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) CreateAccount(ctx context.Context, account domain.AccountSecret) error {
	encrypted, err := s.cipher.Encrypt(account.Password)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO accounts(id,name,host,port,tls,username,password_encrypted,monitor_status,created_at)
VALUES(?,?,?,?,?,?,?,?,?)`, account.ID, account.Name, account.Host, account.Port, account.TLS,
		account.Username, encrypted, account.MonitorStatus, formatTime(account.CreatedAt))
	return err
}

func (s *Store) ListAccounts(ctx context.Context) ([]domain.Account, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,host,port,tls,username,monitor_status,
latest_connection_error,last_successful_sync_at,created_at FROM accounts ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	accounts := make([]domain.Account, 0)
	for rows.Next() {
		account, err := scanAccount(rows)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}
	return accounts, rows.Err()
}

func (s *Store) AccountSecret(ctx context.Context, id string) (domain.AccountSecret, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,name,host,port,tls,username,monitor_status,
latest_connection_error,last_successful_sync_at,created_at,password_encrypted,uid_validity,last_uid
FROM accounts WHERE id=?`, id)
	var account domain.AccountSecret
	var tls bool
	var lastSync sql.NullString
	var created string
	var encrypted []byte
	err := row.Scan(&account.ID, &account.Name, &account.Host, &account.Port, &tls, &account.Username,
		&account.MonitorStatus, &account.LatestConnectionError, &lastSync, &created, &encrypted,
		&account.UIDValidity, &account.LastUID)
	if errors.Is(err, sql.ErrNoRows) {
		return account, domain.ErrNotFound
	}
	if err != nil {
		return account, err
	}
	account.TLS = tls
	account.CreatedAt, err = parseTime(created)
	if err != nil {
		return account, err
	}
	if lastSync.Valid {
		t, parseErr := parseTime(lastSync.String)
		if parseErr != nil {
			return account, parseErr
		}
		account.LastSuccessfulSyncAt = &t
	}
	account.Password, err = s.cipher.Decrypt(encrypted)
	return account, err
}

func (s *Store) DeleteAccount(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM accounts WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *Store) UpdateMonitorState(ctx context.Context, id, status, message string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE accounts SET monitor_status=?, latest_connection_error=? WHERE id=?`, status, message, id)
	return err
}

func (s *Store) ResetUIDValidity(ctx context.Context, id string, uidValidity uint32) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM emails WHERE account_id=?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE accounts SET uid_validity=?,last_uid=0 WHERE id=?`, uidValidity, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) CompleteSync(ctx context.Context, id string, uidValidity, lastUID uint32) error {
	_, err := s.db.ExecContext(ctx, `UPDATE accounts SET uid_validity=?,last_uid=?,last_successful_sync_at=?,latest_connection_error='' WHERE id=?`,
		uidValidity, lastUID, formatTime(time.Now().UTC()), id)
	return err
}

func (s *Store) SaveEmail(ctx context.Context, email domain.Email) (bool, error) {
	email.From = mailaddr.NormalizeList(email.From)
	email.To = mailaddr.NormalizeList(email.To)
	email.Cc = mailaddr.NormalizeList(email.Cc)
	fromJSON, _ := json.Marshal(email.From)
	toJSON, _ := json.Marshal(email.To)
	ccJSON, _ := json.Marshal(email.Cc)
	headersJSON, _ := json.Marshal(email.Headers)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO emails(
id,account_id,uid_validity,imap_uid,message_id,subject,from_json,to_json,cc_json,headers_json,
text_body,html_body,sent_at,received_at,size,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		email.ID, email.AccountID, email.UIDValidity, email.IMAPUID, email.MessageID, email.Subject,
		string(fromJSON), string(toJSON), string(ccJSON), string(headersJSON), email.TextBody, email.HTMLBody,
		nullTime(email.SentAt), formatTime(email.ReceivedAt), email.Size, formatTime(email.CreatedAt))
	if err != nil {
		return false, err
	}
	inserted, _ := result.RowsAffected()
	if inserted == 0 {
		return false, tx.Commit()
	}
	for _, attachment := range email.Attachments {
		if _, err := tx.ExecContext(ctx, `INSERT INTO attachments(id,email_id,filename,content_type,content_id,size,path)
VALUES(?,?,?,?,?,?,?)`, attachment.ID, email.ID, attachment.Filename, attachment.ContentType,
			attachment.ContentID, attachment.Size, attachment.Path); err != nil {
			return false, err
		}
	}
	return true, tx.Commit()
}

func (s *Store) ListEmails(ctx context.Context, opts domain.ListEmailOptions) (domain.EmailPage, error) {
	page := domain.EmailPage{Items: make([]domain.EmailSummary, 0), Page: opts.Page, PageSize: opts.PageSize}
	where, args := "", []any{}
	if opts.AccountID != "" {
		where, args = " WHERE e.account_id=?", append(args, opts.AccountID)
	}
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM emails e"+where, args...).Scan(&page.Total); err != nil {
		return page, err
	}
	args = append(args, opts.PageSize, (opts.Page-1)*opts.PageSize)
	rows, err := s.db.QueryContext(ctx, `SELECT e.id,e.account_id,e.message_id,e.subject,e.from_json,e.to_json,
e.sent_at,e.received_at,e.size,(SELECT COUNT(*) FROM attachments a WHERE a.email_id=e.id)
FROM emails e`+where+` ORDER BY e.received_at DESC,e.id DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var item domain.EmailSummary
		var fromJSON, toJSON, received string
		var sent sql.NullString
		if err := rows.Scan(&item.ID, &item.AccountID, &item.MessageID, &item.Subject, &fromJSON,
			&toJSON, &sent, &received, &item.Size, &item.AttachmentCount); err != nil {
			return page, err
		}
		_ = json.Unmarshal([]byte(fromJSON), &item.From)
		_ = json.Unmarshal([]byte(toJSON), &item.To)
		item.From = mailaddr.NormalizeList(item.From)
		item.To = mailaddr.NormalizeList(item.To)
		item.ReceivedAt, err = parseTime(received)
		if err != nil {
			return page, err
		}
		if sent.Valid {
			t, err := parseTime(sent.String)
			if err != nil {
				return page, err
			}
			item.SentAt = &t
		}
		page.Items = append(page.Items, item)
	}
	return page, rows.Err()
}

func (s *Store) Email(ctx context.Context, id string) (domain.Email, error) {
	var email domain.Email
	var fromJSON, toJSON, ccJSON, headersJSON, sent, received, created sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT id,account_id,message_id,subject,from_json,to_json,cc_json,headers_json,
text_body,html_body,sent_at,received_at,size,created_at FROM emails WHERE id=?`, id).Scan(
		&email.ID, &email.AccountID, &email.MessageID, &email.Subject, &fromJSON, &toJSON, &ccJSON,
		&headersJSON, &email.TextBody, &email.HTMLBody, &sent, &received, &email.Size, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return email, domain.ErrNotFound
	}
	if err != nil {
		return email, err
	}
	_ = json.Unmarshal([]byte(fromJSON.String), &email.From)
	_ = json.Unmarshal([]byte(toJSON.String), &email.To)
	_ = json.Unmarshal([]byte(ccJSON.String), &email.Cc)
	_ = json.Unmarshal([]byte(headersJSON.String), &email.Headers)
	email.From = mailaddr.NormalizeList(email.From)
	email.To = mailaddr.NormalizeList(email.To)
	email.Cc = mailaddr.NormalizeList(email.Cc)
	if email.Headers == nil {
		email.Headers = map[string][]string{}
	}
	email.ReceivedAt, err = parseTime(received.String)
	if err != nil {
		return email, err
	}
	email.CreatedAt, err = parseTime(created.String)
	if err != nil {
		return email, err
	}
	if sent.Valid {
		t, parseErr := parseTime(sent.String)
		if parseErr != nil {
			return email, parseErr
		}
		email.SentAt = &t
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,email_id,filename,content_type,content_id,size,path FROM attachments WHERE email_id=? ORDER BY id`, id)
	if err != nil {
		return email, err
	}
	defer rows.Close()
	email.Attachments = make([]domain.Attachment, 0)
	for rows.Next() {
		var a domain.Attachment
		if err := rows.Scan(&a.ID, &a.EmailID, &a.Filename, &a.ContentType, &a.ContentID, &a.Size, &a.Path); err != nil {
			return email, err
		}
		email.Attachments = append(email.Attachments, a)
	}
	return email, rows.Err()
}

func (s *Store) Attachment(ctx context.Context, emailID, attachmentID string) (domain.Attachment, error) {
	var a domain.Attachment
	err := s.db.QueryRowContext(ctx, `SELECT id,email_id,filename,content_type,content_id,size,path
FROM attachments WHERE id=? AND email_id=?`, attachmentID, emailID).Scan(
		&a.ID, &a.EmailID, &a.Filename, &a.ContentType, &a.ContentID, &a.Size, &a.Path)
	if errors.Is(err, sql.ErrNoRows) {
		return a, domain.ErrNotFound
	}
	return a, err
}

type scanner interface{ Scan(...any) error }

func scanAccount(row scanner) (domain.Account, error) {
	var account domain.Account
	var tls bool
	var lastSync sql.NullString
	var created string
	err := row.Scan(&account.ID, &account.Name, &account.Host, &account.Port, &tls, &account.Username,
		&account.MonitorStatus, &account.LatestConnectionError, &lastSync, &created)
	if err != nil {
		return account, err
	}
	account.TLS = tls
	account.CreatedAt, err = parseTime(created)
	if err != nil {
		return account, err
	}
	if lastSync.Valid {
		t, parseErr := parseTime(lastSync.String)
		if parseErr != nil {
			return account, parseErr
		}
		account.LastSuccessfulSyncAt = &t
	}
	return account, nil
}

func formatTime(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func parseTime(value string) (time.Time, error) { return time.Parse(time.RFC3339Nano, value) }

func nullTime(t *time.Time) any {
	if t == nil || t.IsZero() {
		return nil
	}
	return formatTime(*t)
}
