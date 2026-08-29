package persistence

import (
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
)

func textVal(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

func uuidVal(value pgtype.UUID) string {
	if !value.Valid {
		return ""
	}
	return value.String()
}
