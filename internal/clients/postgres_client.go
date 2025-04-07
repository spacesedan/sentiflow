package clients

import (
	"context"
	"log"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	postgresInstance Postgres
	postgresOnce     sync.Once
)

type Postgres struct {
	DB *pgxpool.Pool
}

func GetPostgresClient(ctx context.Context, dsn string) Postgres {
	postgresOnce.Do(func() {
		pool, err := pgxpool.New(ctx, dsn)
		if err != nil {
			log.Fatalf("failed to create postgreSQL client: %v", err)
		}

		err = pool.Ping(ctx)
		if err != nil {
			log.Fatalf("[PostgresClient] Failed to ping PostgreSQL: %v", err)
		}

		postgresInstance = Postgres{
			DB: pool,
		}
	})

	return postgresInstance
}

func (p Postgres) Close() {
	if p.DB != nil {
		p.DB.Close()
	}
}
