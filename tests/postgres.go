package tests

import (
	"context"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/require"
	logger "github.com/textileio/go-log/v2"
)

var (
	log = logger.Logger("tests")
)

// PostgresURL gets a Postgres database URL for test. It always creates a new
// database on the server so tests won't clash with each other. It connects to
// the server specified by PG_URL envvar if present, or starts a new Postgres
// docker container which stops automatically after 10 minutes.
func PostgresURL(t *testing.T) string {
	ctx := context.Background()
	pgURL := initURL(t)

	// use pgxpool to allow concurrent access.
	conn, err := pgxpool.Connect(ctx, pgURL)
	require.NoError(t, err, "connecting to postgres")
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
	require.NoError(t, err, "creating database")

	u, err := url.Parse(pgURL)
	require.NoError(t, err, "parsing postgres url")
	u.Path = dbName

	return u.String()
}

func initURL(t *testing.T) string {
	pgURL := os.Getenv("PG_URL")
	if pgURL != "" {
		return pgURL
	}
	var pool *dockertest.Pool
	var container *dockertest.Resource
	pool, err := dockertest.NewPool("")
	require.NoError(t, err, "creating dockertest pool")

	container, err = pool.Run("postgres", "14.1", []string{"POSTGRES_USER=test", "POSTGRES_PASSWORD=test"})
	require.NoError(t, err, err, "failed to start postgres docker container")
	if err = container.Expire(600); err != nil {
		log.Warnf("failed to expire postgres docker container, continuing: %w", err)
	}

	pgURL = fmt.Sprintf("postgres://test:test@localhost:%s?sslmode=disable&timezone=UTC", container.GetPort("5432/tcp"))
	err = pool.Retry(func() error {
		ctx := context.Background()
		conn, err := pgx.Connect(ctx, pgURL)
		if err != nil {
			log.Warnf("postgres container is not up yet: %w", err)
			return fmt.Errorf("connecting to the database: %s", err)
		}
		_ = conn.Close(ctx)
		return nil
	})
	require.NoError(t, err, "starting postgres")

	t.Cleanup(func() {
		_ = pool.Purge(container)
	})

	return pgURL
}
