package authsession

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"geoduels/internal/storekit"
	"geoduels/pkg/entityid"
	db "geoduels/pkg/persistence/sqlc/db"
)

// PGStore owns PostgreSQL access for the auth-session feature: refresh-token
// sessions, rotation, and revocation.
type PGStore struct {
	pool *pgxpool.Pool
}

func NewPGStore(pool *pgxpool.Pool) *PGStore { return &PGStore{pool: pool} }

func (s *PGStore) q() *db.Queries { return db.New(s.pool) }

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
	return storekit.Timestamptz(value)
}

func sessionRecord(id, userID pgtype.UUID, hash string, expires, created, used, revoked pgtype.Timestamptz, agent, ip string) Record {
	r := Record{RefreshTokenHash: hash, ExpiresAt: expires.Time, CreatedAt: created.Time, LastUsedAt: used.Time, UserAgent: agent, IPAddress: ip}
	r.ID = id.String()
	r.UserID = userID.String()
	if revoked.Valid {
		v := revoked.Time
		r.RevokedAt = &v
	}
	return r
}

func (s *PGStore) CreateAuthSession(userID, refreshTokenHash string, expiresAt time.Time, params Params) (Record, error) {
	if userID == "" || refreshTokenHash == "" {
		return Record{}, errors.New("userID and refresh token hash required")
	}
	id, err := sessionUUID(entityid.New())
	if err != nil {
		return Record{}, err
	}
	uid, err := sessionUUID(userID)
	if err != nil {
		return Record{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Record{}, err
	}
	defer tx.Rollback(ctx)
	q := db.New(tx)
	row, queryErr := q.CreateAuthSession(ctx, db.CreateAuthSessionParams{ID: id, UserID: uid, RefreshTokenHash: refreshTokenHash, ExpiresAt: sessionTime(expiresAt), UserAgent: sessionText(params.UserAgent), IpAddress: sessionText(params.IPAddress)})
	if queryErr != nil {
		return Record{}, queryErr
	}
	if strings.TrimSpace(params.IPAddress) != "" {
		if queryErr = q.SetRegistrationIP(ctx, db.SetRegistrationIPParams{RegistrationIpAddress: sessionText(strings.TrimSpace(params.IPAddress)), UserID: uid}); queryErr != nil {
			return Record{}, queryErr
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Record{}, err
	}
	return sessionRecord(row.ID, row.UserID, row.RefreshTokenHash, row.ExpiresAt, row.CreatedAt, row.LastUsedAt, row.RevokedAt, row.UserAgent, row.IpAddress), nil
}

func (s *PGStore) GetAuthSessionByRefreshToken(hash string) (Record, bool, error) {
	if hash == "" {
		return Record{}, false, errors.New("hash required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	row, err := s.q().GetAuthSessionByRefreshToken(ctx, hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return Record{}, false, nil
	}
	if err != nil {
		return Record{}, false, err
	}
	return sessionRecord(row.ID, row.UserID, row.RefreshTokenHash, row.ExpiresAt, row.CreatedAt, row.LastUsedAt, row.RevokedAt, row.UserAgent, row.IpAddress), true, nil
}

func (s *PGStore) RotateAuthSession(sessionID, currentHash, nextHash string, expiresAt, usedAt time.Time) (Record, bool, error) {
	if sessionID == "" || currentHash == "" || nextHash == "" {
		return Record{}, false, errors.New("session id and token hashes required")
	}
	if usedAt.IsZero() {
		usedAt = time.Now()
	}
	id, err := sessionUUID(sessionID)
	if err != nil {
		return Record{}, false, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	row, err := s.q().RotateAuthSession(ctx, db.RotateAuthSessionParams{NextRefreshTokenHash: nextHash, ExpiresAt: sessionTime(expiresAt), LastUsedAt: sessionTime(usedAt), SessionID: id, CurrentRefreshTokenHash: currentHash})
	if errors.Is(err, pgx.ErrNoRows) {
		return Record{}, false, nil
	}
	if err != nil {
		return Record{}, false, err
	}
	return sessionRecord(row.ID, row.UserID, row.RefreshTokenHash, row.ExpiresAt, row.CreatedAt, row.LastUsedAt, row.RevokedAt, row.UserAgent, row.IpAddress), true, nil
}

func (s *PGStore) RevokeAuthSession(sessionID string) error {
	if sessionID == "" {
		return errors.New("session id required")
	}
	id, err := sessionUUID(sessionID)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	return s.q().RevokeAuthSession(ctx, id)
}

func (s *PGStore) RevokeAuthSessionsForUser(userID string) error {
	if userID == "" {
		return errors.New("userID required")
	}
	id, err := sessionUUID(userID)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	return s.q().RevokeAuthSessionsForUser(ctx, id)
}
