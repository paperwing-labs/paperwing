package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/paperwing/paperwing/internal/auth"
)

func (s *Store) SaveAPIToken(ctx context.Context, token auth.APITokenRecord) error {
	scopes, err := json.Marshal(token.Scopes)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO api_tokens(
id,name,token_prefix,token_hash,scopes_json,created_at,last_used_at,expires_at,never_expires)
VALUES(?,?,?,?,?,?,?,?,?)`, token.ID, token.Name, token.TokenPrefix, token.TokenHash, scopes,
		formatTime(token.CreatedAt), nullableTime(token.LastUsedAt), storedAPITokenExpiration(token), token.ExpiresAt == nil)
	return err
}

func (s *Store) APITokens(ctx context.Context) ([]auth.APIToken, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,token_prefix,scopes_json,created_at,last_used_at,expires_at,never_expires
FROM api_tokens ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tokens := make([]auth.APIToken, 0)
	for rows.Next() {
		token, err := scanAPIToken(rows)
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
	}
	return tokens, rows.Err()
}

func (s *Store) APITokenByHash(ctx context.Context, tokenHash []byte, now time.Time) (auth.APIToken, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,name,token_prefix,scopes_json,created_at,last_used_at,expires_at,never_expires
FROM api_tokens WHERE token_hash=? AND (never_expires=1 OR expires_at>?)`, tokenHash, formatTime(now))
	token, err := scanAPIToken(row)
	if errors.Is(err, sql.ErrNoRows) {
		return auth.APIToken{}, auth.ErrInvalidAPIToken
	}
	return token, err
}

func (s *Store) TouchAPIToken(ctx context.Context, id string, usedAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE api_tokens SET last_used_at=?
WHERE id=? AND (last_used_at IS NULL OR last_used_at<?)`, formatTime(usedAt), id,
		formatTime(usedAt.Add(-5*time.Minute)))
	return err
}

func (s *Store) DeleteAPIToken(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM api_tokens WHERE id=?`, id)
	if err != nil {
		return err
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if deleted == 0 {
		return auth.ErrAPITokenNotFound
	}
	return nil
}

type rowScanner interface {
	Scan(...any) error
}

func scanAPIToken(row rowScanner) (auth.APIToken, error) {
	var token auth.APIToken
	var scopes []byte
	var created string
	var lastUsed sql.NullString
	var expires string
	var neverExpires bool
	if err := row.Scan(&token.ID, &token.Name, &token.TokenPrefix, &scopes, &created, &lastUsed, &expires, &neverExpires); err != nil {
		return token, err
	}
	if err := json.Unmarshal(scopes, &token.Scopes); err != nil {
		return token, err
	}
	var err error
	token.CreatedAt, err = parseTime(created)
	if err != nil {
		return token, err
	}
	if !neverExpires {
		value, err := parseTime(expires)
		if err != nil {
			return token, err
		}
		token.ExpiresAt = &value
	}
	if lastUsed.Valid {
		value, err := parseTime(lastUsed.String)
		if err != nil {
			return token, err
		}
		token.LastUsedAt = &value
	}
	return token, nil
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return formatTime(*value)
}

func storedAPITokenExpiration(token auth.APITokenRecord) string {
	if token.ExpiresAt == nil {
		return formatTime(token.CreatedAt)
	}
	return formatTime(*token.ExpiresAt)
}

func ensureAPITokenNeverExpiresColumn(db *sql.DB) error {
	rows, err := db.Query(`PRAGMA table_info(api_tokens)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	found := false
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull, primaryKey int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return err
		}
		if name == "never_expires" {
			found = true
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if found {
		return nil
	}
	_, err = db.Exec(`ALTER TABLE api_tokens ADD COLUMN never_expires INTEGER NOT NULL DEFAULT 0`)
	return err
}
