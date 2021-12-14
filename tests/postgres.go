package tests

import (
	"context"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/ory/dockertest/v3"
	logger "github.com/textileio/go-log/v2"
)

var (
	log               = logger.Logger("tests")
	storedPGURL       atomic.Value // string
	startPostgresOnce sync.Once
)

// PostgresURL gets a Postgres database URL for test. It always creates a new
// database on the server so tests won't clash with each other. It connects to
// the server specified by PG_URL envvar if present, or starts a new Postgres
// docker container which stops automatically after 10 minutes.
func PostgresURL() (string, error) {
	ctx := context.Background()
	if storedPGURL.Load() == nil {
		if err := initURL(); err != nil {
			return "", fmt.Errorf("initialize postgres: %w", err)
		}
	}
	// use pgxpool to allow concurrent access.
	pgURL := storedPGURL.Load().(string)
	conn, err := pgxpool.Connect(ctx, pgURL)
	if err != nil {
		return "", fmt.Errorf("connect postgres: %w", err)
	}
	defer conn.Close()

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	var dbName string
	for i := 0; i < 10; i++ {
		dbName = fmt.Sprintf("db%d", r.Uint64())
		_, err = conn.Exec(ctx, "CREATE DATABASE "+dbName+";")
		if err == nil {
			break
		}
	}
	if err != nil {
		return "", fmt.Errorf("create database: %w", err)
	}

	u, err := url.Parse(pgURL)
	if err != nil {
		return "", err
	}
	u.Path = dbName
	return u.String(), nil
}

func initURL() (err error) {
	startPostgresOnce.Do(func() {
		pgURL := os.Getenv("PG_URL")
		if pgURL != "" {
			storedPGURL.Store(pgURL)
			return
		}
		var pool *dockertest.Pool
		var container *dockertest.Resource
		pool, err = dockertest.NewPool("")
		if err != nil {
			return
		}
		container, err = pool.Run("postgres", "latest", []string{"POSTGRES_USER=test", "POSTGRES_PASSWORD=test"})
		if err != nil {
			log.Errorf("failed to start postgres docker container: %w", err)
			return
		}
		if err = container.Expire(600); err != nil {
			log.Warnf("failed to expire postgres docker container, continuing: %w", err)
		}

		pgURL = fmt.Sprintf("postgres://test:test@localhost:%s?sslmode=disable&timezone=UTC", container.GetPort("5432/tcp"))
		err = pool.Retry(func() error {
			ctx := context.Background()
			conn, err := pgx.Connect(ctx, pgURL)
			if err != nil {
				log.Warnf("postgres container is not up yet: %w", err)
				return err
			}
			_ = conn.Close(ctx)
			return nil
		})
		if err == nil {
			storedPGURL.Store(pgURL)
		}
	})
	return
}
