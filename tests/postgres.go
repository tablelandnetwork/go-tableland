package tests

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/require"
	logger "github.com/textileio/go-log/v2"
)

var (
	log = logger.Logger("tests")
)

// PostgresURLWithImage gets a Postgres database URL for test such as
// PostgresURL(), but with a custom docker image name and tag.
func PostgresURLWithImage(t *testing.T, image string, tag string, dbName string) string {
	ctx := context.Background()
	pgURL := initURL(t, image, tag)

	if dbName != "" {
		pgURLSplitted := strings.Split(pgURL, "?")
		pgURL = fmt.Sprintf("%s/%s?%s", pgURLSplitted[0], dbName, pgURLSplitted[1])
	}

	u, err := url.Parse(pgURL)
	require.NoError(t, err, "parsing postgres url")

	// use pgxpool to allow concurrent access.
	conn, err := pgxpool.Connect(ctx, pgURL)
	require.NoError(t, err, "connecting to postgres")
	defer conn.Close()

	return u.String()
}

// PostgresURL gets a Postgres database URL for test. It always creates a new
// database on the server so tests won't clash with each other. It connects to
// the server specified by PG_URL envvar if present, or starts a new Postgres
// docker container which stops automatically after 10 minutes.
func PostgresURL(t *testing.T) string {
	return PostgresURLWithImage(t, "postgres", "14.1", "")
}

func initURL(t *testing.T, image string, tag string) string {
	pgURL := os.Getenv("PG_URL")
	if pgURL != "" {
		return pgURL
	}
	var pool *dockertest.Pool
	var container *dockertest.Resource
	pool, err := dockertest.NewPool("")
	require.NoError(t, err, "creating dockertest pool")

	container, err = pool.Run(image, tag, []string{"POSTGRES_USER=admin", "POSTGRES_PASSWORD=admin"})
	require.NoError(t, err, err, "failed to start postgres docker container")
	if err = container.Expire(600); err != nil {
		log.Warnf("failed to expire postgres docker container, continuing: %w", err)
	}
	pgURL = fmt.Sprintf("postgres://admin:admin@localhost:%s?sslmode=disable&timezone=UTC", container.GetPort("5432/tcp"))
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
