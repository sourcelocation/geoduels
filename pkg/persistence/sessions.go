package persistence

import (
	"context"
	"errors"
	"strings"
	"time"

	db "geoduels/pkg/persistence/sqlc/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func sessionUUID(value string) (pgtype.UUID, error) {
	var id pgtype.UUID
	if err := id.Scan(value); err != nil {
		return id, err
	}
	return id, nil
}
func sessionText(value string) pgtype.Text {
	return pgtype.Text{String: value, Valid: strings.TrimSpace(value) != ""}
}
func sessionTime(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: true}
}
func sessionRecord(id, userID pgtype.UUID, hash string, expires, created, used pgtype.Timestamptz, revoked pgtype.Timestamptz, agent, ip string) RefreshTokenRecord {
	r := RefreshTokenRecord{RefreshTokenHash: hash, ExpiresAt: expires.Time, CreatedAt: created.Time, LastUsedAt: used.Time, UserAgent: agent, IPAddress: ip}
	r.ID = id.String()
	r.UserID = userID.String()
	if revoked.Valid {
		v := revoked.Time
		r.RevokedAt = &v
	}
	return r
}

func (s *DB) CreateAuthSession(userID, refreshTokenHash string, expiresAt time.Time, params AuthSessionParams) (RefreshTokenRecord, error) {
	if userID == "" || refreshTokenHash == "" {
		return RefreshTokenRecord{}, errors.New("userID and refresh token hash required")
	}
	id, err := sessionUUID(newUserID())
	if err != nil {
		return RefreshTokenRecord{}, err
	}
	uid, err := sessionUUID(userID)
	if err != nil {
		return RefreshTokenRecord{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	var result RefreshTokenRecord
	err = s.withTx(ctx, func(tx pgx.Tx) error {
		q := db.New(tx)
		row, queryErr := q.CreateAuthSession(ctx, db.CreateAuthSessionParams{ID: id, UserID: uid, RefreshTokenHash: refreshTokenHash, ExpiresAt: sessionTime(expiresAt), UserAgent: sessionText(params.UserAgent), IpAddress: sessionText(params.IPAddress)})
		if queryErr != nil {
			return queryErr
		}
		if strings.TrimSpace(params.IPAddress) != "" {
			if queryErr = q.SetRegistrationIP(ctx, db.SetRegistrationIPParams{RegistrationIpAddress: sessionText(strings.TrimSpace(params.IPAddress)), UserID: uid}); queryErr != nil {
				return queryErr
			}
		}
		result = sessionRecord(row.ID, row.UserID, row.RefreshTokenHash, row.ExpiresAt, row.CreatedAt, row.LastUsedAt, row.RevokedAt, row.UserAgent, row.IpAddress)
		return nil
	})
	return result, err
}
func (s *DB) GetAuthSessionByRefreshToken(hash string) (RefreshTokenRecord, bool, error) {
	if hash == "" {
		return RefreshTokenRecord{}, false, errors.New("hash required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	row, err := s.db.GetAuthSessionByRefreshToken(ctx, hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return RefreshTokenRecord{}, false, nil
	}
	if err != nil {
		return RefreshTokenRecord{}, false, err
	}
	return sessionRecord(row.ID, row.UserID, row.RefreshTokenHash, row.ExpiresAt, row.CreatedAt, row.LastUsedAt, row.RevokedAt, row.UserAgent, row.IpAddress), true, nil
}
func (s *DB) RotateAuthSession(sessionID, currentHash, nextHash string, expiresAt, usedAt time.Time) (RefreshTokenRecord, bool, error) {
	if sessionID == "" || currentHash == "" || nextHash == "" {
		return RefreshTokenRecord{}, false, errors.New("session id and token hashes required")
	}
	if usedAt.IsZero() {
		usedAt = time.Now()
	}
	id, err := sessionUUID(sessionID)
	if err != nil {
		return RefreshTokenRecord{}, false, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	row, err := s.db.RotateAuthSession(ctx, db.RotateAuthSessionParams{NextRefreshTokenHash: nextHash, ExpiresAt: sessionTime(expiresAt), LastUsedAt: sessionTime(usedAt), SessionID: id, CurrentRefreshTokenHash: currentHash})
	if errors.Is(err, pgx.ErrNoRows) {
		return RefreshTokenRecord{}, false, nil
	}
	if err != nil {
		return RefreshTokenRecord{}, false, err
	}
	return sessionRecord(row.ID, row.UserID, row.RefreshTokenHash, row.ExpiresAt, row.CreatedAt, row.LastUsedAt, row.RevokedAt, row.UserAgent, row.IpAddress), true, nil
}
func (s *DB) RevokeAuthSession(sessionID string) error {
	if sessionID == "" {
		return errors.New("session id required")
	}
	id, err := sessionUUID(sessionID)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	return s.db.RevokeAuthSession(ctx, id)
}
func (s *DB) RevokeAuthSessionsForUser(userID string) error {
	if userID == "" {
		return errors.New("userID required")
	}
	id, err := sessionUUID(userID)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	return s.db.RevokeAuthSessionsForUser(ctx, id)
}
