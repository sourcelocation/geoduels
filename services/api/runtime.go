package main

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"

	"geoduels/pkg/matchlaunch"
)

func redisFromEnv() (*redis.Client, func(), error) {
	url := getenv("REDIS_URL", "")
	if url == "" {
		return nil, nil, errors.New("REDIS_URL is required")
	}
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, nil, err
	}
	rdb := redis.NewClient(opt)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, nil, err
	}
	return rdb, func() { _ = rdb.Close() }, nil
}

func (a *api) close() {
	if a.db != nil {
		a.db.Close()
	}
	if a.redis != nil {
		_ = a.redis.Close()
	}
}

func (a *api) launcher() matchlaunch.Launcher {
	return matchlaunch.Launcher{
		Coord:          a.coord,
		Persist:        a.runtimeStore,
		HTTPClient:     a.httpClient,
		TicketSecret:   a.ticketAuth,
		InternalSecret: a.internalSecret,
	}
}
