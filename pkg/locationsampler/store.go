package locationsampler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"geoduels/pkg/contracts"
)

type DBStore struct {
	pool   *pgxpool.Pool
	mapKey string
}

func NewDBStoreFromEnv() (*DBStore, error) {
	url := os.Getenv("POSTGRES_URL")
	if url == "" {
		return nil, errors.New("POSTGRES_URL is required for location sampler")
	}
	url = normalizeDBURLForContainer(url)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	mapKey := os.Getenv("LOCATION_MAP_KEY")
	if mapKey == "" {
		mapKey = "a-source-world"
	}
	return &DBStore{pool: pool, mapKey: mapKey}, nil
}

func (d *DBStore) Close() {
	if d.pool != nil {
		d.pool.Close()
	}
}

func (d *DBStore) ActiveCatalogID(ctx context.Context) (string, error) {
	var id string
	err := d.pool.QueryRow(ctx, `
		select ma.active_revision_id
		from map_aliases ma
		where ma.map_key = $1
		limit 1
	`, d.mapKey).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("no active map revision found for LOCATION_MAP_KEY=%q", d.mapKey)
		}
		return "", err
	}
	return id, nil
}

func (d *DBStore) FetchRows(ctx context.Context, catalogID string, after float64, limit int) ([]row, error) {
	q := `
		select id, lat, lng, country, pano_id, heading, pitch
		from locations
		where map_revision_id = $1
		  and rand_key >= $2
		  and pano_id is not null
		  and btrim(pano_id) <> ''
		order by rand_key asc
		limit $3
	`
	rows, err := d.pool.Query(ctx, q, catalogID, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]row, 0, limit)
	for rows.Next() {
		var r row
		if err := scanRow(rows, &r); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) >= limit {
		return out, nil
	}
	remaining := limit - len(out)
	rows2, err := d.pool.Query(ctx, `
		select id, lat, lng, country, pano_id, heading, pitch
		from locations
		where map_revision_id = $1
		  and rand_key < $2
		  and pano_id is not null
		  and btrim(pano_id) <> ''
		order by rand_key asc
		limit $3
	`, catalogID, after, remaining)
	if err != nil {
		return nil, err
	}
	defer rows2.Close()
	for rows2.Next() {
		var r row
		if err := scanRow(rows2, &r); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows2.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRow(src rowScanner, out *row) error {
	var panoID *string
	var heading *float64
	var pitch *float64
	if err := src.Scan(&out.ID, &out.Point.Lat, &out.Point.Lng, &out.Point.Country, &panoID, &heading, &pitch); err != nil {
		return err
	}
	out.Point.PanoID = panoID
	out.Point.Heading = heading
	out.Point.Pitch = pitch
	return nil
}

type RedisStateStore struct {
	rdb *redis.Client
	ttl time.Duration
}

func NewRedisStateStoreFromEnv(ttl time.Duration) stateStore {
	url := os.Getenv("REDIS_URL")
	if url == "" {
		return newMemoryStateStore(ttl)
	}
	opt, err := redis.ParseURL(url)
	if err != nil {
		return newMemoryStateStore(ttl)
	}
	rdb := redis.NewClient(opt)
	if rdb.Ping(context.Background()).Err() != nil {
		return newMemoryStateStore(ttl)
	}
	return &RedisStateStore{rdb: rdb, ttl: ttl}
}

func (s *RedisStateStore) GetRound(matchID string, roundIndex int) (contracts.LocationPoint, bool, error) {
	ctx := context.Background()
	b, err := s.rdb.Get(ctx, roundKey(matchID, roundIndex)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return contracts.LocationPoint{}, false, nil
		}
		return contracts.LocationPoint{}, false, err
	}
	var p contracts.LocationPoint
	if err := json.Unmarshal(b, &p); err != nil {
		return contracts.LocationPoint{}, false, err
	}
	return p, true, nil
}

func (s *RedisStateStore) SaveRound(matchID string, roundIndex int, p contracts.LocationPoint) error {
	ctx := context.Background()
	b, _ := json.Marshal(p)
	return s.rdb.Set(ctx, roundKey(matchID, roundIndex), b, s.ttl).Err()
}

func (s *RedisStateStore) MarkUsed(matchID string, locationID int64) (bool, error) {
	ctx := context.Background()
	key := usedKey(matchID)
	added, err := s.rdb.SAdd(ctx, key, locationID).Result()
	if err != nil {
		return false, err
	}
	_ = s.rdb.Expire(ctx, key, s.ttl).Err()
	return added == 1, nil
}

type memoryStateStore struct {
	mu    sync.Mutex
	round map[string]contracts.LocationPoint
	used  map[string]map[int64]struct{}
}

func newMemoryStateStore(_ time.Duration) stateStore {
	return &memoryStateStore{
		round: map[string]contracts.LocationPoint{},
		used:  map[string]map[int64]struct{}{},
	}
}

func (m *memoryStateStore) GetRound(matchID string, roundIndex int) (contracts.LocationPoint, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.round[roundKey(matchID, roundIndex)]
	return v, ok, nil
}

func (m *memoryStateStore) SaveRound(matchID string, roundIndex int, p contracts.LocationPoint) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.round[roundKey(matchID, roundIndex)] = p
	return nil
}

func (m *memoryStateStore) MarkUsed(matchID string, locationID int64) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := usedKey(matchID)
	if _, ok := m.used[key]; !ok {
		m.used[key] = map[int64]struct{}{}
	}
	if _, exists := m.used[key][locationID]; exists {
		return false, nil
	}
	m.used[key][locationID] = struct{}{}
	return true, nil
}

func roundKey(matchID string, roundIndex int) string {
	return "match:sampler:round:" + matchID + ":" + strconv.Itoa(roundIndex)
}

func usedKey(matchID string) string {
	return "match:sampler:used:" + matchID
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
