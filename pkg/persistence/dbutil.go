package persistence

import (
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"os"
	"strconv"
	"strings"

	"geoduels/pkg/entityid"
)

func getenvInt(name string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

type mapRow struct {
	Lat         float64
	Lng         float64
	Country     string
	PanoID      *string
	Heading     *float64
	Pitch       *float64
	LatE7       int32
	LngE7       int32
	HeadingCDeg *int16
	PitchCDeg   *int16
	RandKey     int32
}

func parseMapRows(b []byte) ([]mapRow, error) {
	var raw []map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		var envelope struct {
			CustomCoordinates []map[string]any `json:"customCoordinates"`
		}
		if err := json.Unmarshal(b, &envelope); err != nil {
			return nil, err
		}
		if envelope.CustomCoordinates == nil {
			return nil, errors.New("map JSON must be an array or include customCoordinates")
		}
		raw = envelope.CustomCoordinates
	}
	out := make([]mapRow, 0, len(raw))
	for _, it := range raw {
		lat, ok1 := asFloat(it["lat"])
		lng, ok2 := asFloat(it["lng"])
		if !ok1 || !ok2 {
			continue
		}
		if lat < -90 || lat > 90 || lng < -180 || lng > 180 {
			continue
		}
		row := compactMapRow(lat, lng)
		if country, ok := it["country"].(string); ok {
			row.Country = country
		} else if country, ok := it["countryCode"].(string); ok {
			row.Country = country
		}
		panoID, _ := it["panoId"].(string)
		if strings.TrimSpace(panoID) == "" {
			if extra, ok := it["extra"].(map[string]any); ok {
				panoID, _ = extra["panoId"].(string)
			}
		}
		if panoID = strings.TrimSpace(panoID); panoID != "" {
			row.PanoID = &panoID
		}
		if heading, ok := asFloat(it["heading"]); ok {
			row.Heading = &heading
			value := compactAngle(heading, false)
			row.HeadingCDeg = &value
		}
		if pitch, ok := asFloat(it["pitch"]); ok {
			row.Pitch = &pitch
			value := compactAngle(pitch, true)
			row.PitchCDeg = &value
		}
		out = append(out, row)
	}
	return out, nil
}

func stableRand(lat, lng float64) int32 {
	h := sha1.Sum([]byte(fmt.Sprintf("%.8f:%.8f", lat, lng)))
	v := int(h[0])<<16 | int(h[1])<<8 | int(h[2])
	return int32(v)
}

func compactMapRow(lat, lng float64) mapRow {
	return mapRow{
		Lat:     lat,
		Lng:     lng,
		LatE7:   int32(math.Round(lat * 10_000_000)),
		LngE7:   int32(math.Round(lng * 10_000_000)),
		RandKey: stableRand(lat, lng),
	}
}

func compactAngle(value float64, pitch bool) int16 {
	if pitch {
		value = math.Max(-90, math.Min(90, value))
	} else {
		value = math.Mod(value+180, 360)
		if value < 0 {
			value += 360
		}
		value -= 180
	}
	return int16(math.Round(value * 100))
}

func asFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	default:
		return 0, false
	}
}

func nullable(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func newUserID() string {
	return entityid.New()
}

func newDebugMatchID(index int) string {
	return entityid.New()
}

func normalizeDBURLForContainer(dsn string) string {
	if _, err := os.Stat("/.dockerenv"); err != nil {
		return dsn
	}
	u, err := url.Parse(dsn)
	if err != nil {
		return dsn
	}
	if u.Hostname() == "127.0.0.1" || u.Hostname() == "localhost" {
		port := u.Port()
		if port == "" {
			port = "5432"
		}
		u.Host = "host.docker.internal:" + port
		return u.String()
	}
	return dsn
}
