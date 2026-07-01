package persistence

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *pgStore) CreateAuthSession(userID, refreshTokenHash string, expiresAt time.Time, params AuthSessionParams) (RefreshTokenRecord, error) {
	if userID == "" || refreshTokenHash == "" {
		return RefreshTokenRecord{}, errors.New("userID and refresh token hash required")
	}
	sessionID := newUserID()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return RefreshTokenRecord{}, err
	}
	defer tx.Rollback(ctx)
	row := tx.QueryRow(ctx, `
		insert into auth_sessions(
			id,
			user_id,
			refresh_token_hash,
			expires_at,
			created_at,
			last_used_at,
			user_agent,
			ip_address
		)
		values($1, $2, $3, $4, now(), now(), $5, $6)
		returning
			id,
			user_id,
			refresh_token_hash,
			expires_at,
			created_at,
			last_used_at,
			revoked_at,
			coalesce(user_agent, ''),
			coalesce(ip_address, '')
	`, sessionID, userID, refreshTokenHash, expiresAt, nullable(params.UserAgent), nullable(params.IPAddress))
	var rec RefreshTokenRecord
	if err := row.Scan(
		&rec.ID,
		&rec.UserID,
		&rec.RefreshTokenHash,
		&rec.ExpiresAt,
		&rec.CreatedAt,
		&rec.LastUsedAt,
		&rec.RevokedAt,
		&rec.UserAgent,
		&rec.IPAddress,
	); err != nil {
		return RefreshTokenRecord{}, err
	}
	if strings.TrimSpace(params.IPAddress) != "" {
		if _, err := tx.Exec(ctx, `
			update users
			set registration_ip_address = coalesce(nullif(trim(registration_ip_address), ''), $2)
			where id = $1
		`, userID, strings.TrimSpace(params.IPAddress)); err != nil {
			return RefreshTokenRecord{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return RefreshTokenRecord{}, err
	}
	return rec, nil
}

func (s *pgStore) GetAuthSessionByRefreshToken(hash string) (RefreshTokenRecord, bool, error) {
	if hash == "" {
		return RefreshTokenRecord{}, false, errors.New("hash required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	row := s.pool.QueryRow(ctx, `
		select
			id,
			user_id,
			refresh_token_hash,
			expires_at,
			created_at,
			last_used_at,
			revoked_at,
			coalesce(user_agent, ''),
			coalesce(ip_address, '')
		from auth_sessions
		where refresh_token_hash = $1
	`, hash)
	var rec RefreshTokenRecord
	if err := row.Scan(
		&rec.ID,
		&rec.UserID,
		&rec.RefreshTokenHash,
		&rec.ExpiresAt,
		&rec.CreatedAt,
		&rec.LastUsedAt,
		&rec.RevokedAt,
		&rec.UserAgent,
		&rec.IPAddress,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return RefreshTokenRecord{}, false, nil
		}
		return RefreshTokenRecord{}, false, err
	}
	return rec, true, nil
}

func (s *pgStore) RotateAuthSession(sessionID, currentHash, nextHash string, expiresAt time.Time, usedAt time.Time) (RefreshTokenRecord, bool, error) {
	if sessionID == "" || currentHash == "" || nextHash == "" {
		return RefreshTokenRecord{}, false, errors.New("session id and token hashes required")
	}
	if usedAt.IsZero() {
		usedAt = time.Now()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	row := s.pool.QueryRow(ctx, `
		update auth_sessions
		set refresh_token_hash = $3,
			expires_at = $4,
			last_used_at = $5
		where id = $1
		  and refresh_token_hash = $2
		  and revoked_at is null
		returning
			id,
			user_id,
			refresh_token_hash,
			expires_at,
			created_at,
			last_used_at,
			revoked_at,
			coalesce(user_agent, ''),
			coalesce(ip_address, '')
	`, sessionID, currentHash, nextHash, expiresAt, usedAt)
	var rec RefreshTokenRecord
	if err := row.Scan(
		&rec.ID,
		&rec.UserID,
		&rec.RefreshTokenHash,
		&rec.ExpiresAt,
		&rec.CreatedAt,
		&rec.LastUsedAt,
		&rec.RevokedAt,
		&rec.UserAgent,
		&rec.IPAddress,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return RefreshTokenRecord{}, false, nil
		}
		return RefreshTokenRecord{}, false, err
	}
	return rec, true, nil
}

func (s *pgStore) RevokeAuthSession(sessionID string) error {
	if sessionID == "" {
		return errors.New("session id required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	_, err := s.pool.Exec(ctx, `
		update auth_sessions
		set revoked_at = coalesce(revoked_at, now())
		where id = $1
	`, sessionID)
	return err
}

func (s *pgStore) RevokeAuthSessionsForUser(userID string) error {
	if userID == "" {
		return errors.New("userID required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	_, err := s.pool.Exec(ctx, `
		update auth_sessions
		set revoked_at = coalesce(revoked_at, now())
		where user_id = $1 and revoked_at is null
	`, userID)
	return err
}
