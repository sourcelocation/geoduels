package accounts

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"

	"github.com/jackc/pgx/v5"

	db "geoduels/pkg/persistence/sqlc/db"
)

// ErrGuestNickname is returned when resetting a guest's nickname; guests
// always play as "Guest".
var ErrGuestNickname = errors.New("guest nicknames cannot be reset")

// ResetNicknameTx replaces a registered player's public nickname with a
// neutral generated one inside the caller's transaction and returns the old
// and new nicknames. The account keeps its claimed nickname state, so the
// player can choose another nickname from their profile.
func ResetNicknameTx(ctx context.Context, tx pgx.Tx, userID string) (string, string, error) {
	uid, err := profileUUID(userID)
	if err != nil {
		return "", "", err
	}
	q := db.New(tx)
	current, err := q.LockNicknameForReset(ctx, uid)
	if err != nil {
		return "", "", err
	}
	if !current.Registered {
		return "", "", ErrGuestNickname
	}
	for range 8 {
		candidate, err := neutralNickname()
		if err != nil {
			return "", "", err
		}
		taken, err := q.NicknameTaken(ctx, db.NicknameTakenParams{ID: uid, Lower: candidate})
		if err != nil {
			return "", "", err
		}
		if taken {
			continue
		}
		if err := q.ResetNickname(ctx, db.ResetNicknameParams{ID: uid, DisplayName: candidate}); err != nil {
			return "", "", err
		}
		return current.DisplayName, candidate, nil
	}
	return "", "", errors.New("no neutral nickname available")
}

// neutralNickname generates a name independent of the player's previous one.
func neutralNickname() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(100_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Player%08d", n.Int64()), nil
}
