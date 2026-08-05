package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/paperwing/paperwing/internal/auth"
)

func (s *Store) AuthUser(ctx context.Context) (auth.User, error) {
	var user auth.User
	var created string
	err := s.db.QueryRowContext(ctx, `SELECT username,password_hash,created_at FROM auth_user WHERE id=1`).Scan(
		&user.Username, &user.PasswordHash, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return user, auth.ErrNotConfigured
	}
	if err != nil {
		return user, err
	}
	user.CreatedAt, err = parseTime(created)
	return user, err
}

func (s *Store) SetupAuth(ctx context.Context, user auth.User, session auth.Session) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var configured bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM auth_user WHERE id=1)`).Scan(&configured); err != nil {
		return err
	}
	if configured {
		return auth.ErrAlreadyConfigured
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO auth_user(id,username,password_hash,created_at) VALUES(1,?,?,?)`,
		user.Username, user.PasswordHash, formatTime(user.CreatedAt)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO auth_sessions(token_hash,created_at,expires_at) VALUES(?,?,?)`,
		session.TokenHash, formatTime(session.CreatedAt), formatTime(session.ExpiresAt)); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) SaveAuthSession(ctx context.Context, session auth.Session) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO auth_sessions(token_hash,created_at,expires_at) VALUES(?,?,?)`,
		session.TokenHash, formatTime(session.CreatedAt), formatTime(session.ExpiresAt))
	return err
}

func (s *Store) AuthSessionValid(ctx context.Context, tokenHash []byte, now time.Time) (bool, error) {
	var valid bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(
SELECT 1 FROM auth_sessions WHERE token_hash=? AND expires_at>?)`, tokenHash, formatTime(now)).Scan(&valid)
	return valid, err
}

func (s *Store) DeleteAuthSession(ctx context.Context, tokenHash []byte) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM auth_sessions WHERE token_hash=?`, tokenHash)
	return err
}

func (s *Store) DeleteExpiredAuthSessions(ctx context.Context, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM auth_sessions WHERE expires_at<=?`, formatTime(now))
	return err
}
