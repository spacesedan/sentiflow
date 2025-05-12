package clients

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	postgresInstance PostgresClient
	postgresOnce     sync.Once
	initErr          error // To capture error from sync.Once
)

// PostgresClient holds the pgx connection pool.
type PostgresClient struct {
	Pool *pgxpool.Pool
}

// GetPostgresClient initializes and returns a singleton PostgresClient using pgxpool.
// It reads connection details from environment variables:
// DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME, POSTGRES_SSLMODE.
// Pool configuration from:
// POSTGRES_MAX_OPEN_CONNS, POSTGRES_MIN_CONNS (formerly MAX_IDLE), POSTGRES_CONN_MAX_LIFETIME_MINUTES.
func GetPostgresClient(ctx context.Context) (PostgresClient, error) {
	postgresOnce.Do(func() {
		host := os.Getenv("DB_HOST")
		port := os.Getenv("DB_PORT")
		user := os.Getenv("DB_USER")
		password := os.Getenv("DB_PASSWORD")
		dbname := os.Getenv("DB_NAME")
		sslmode := os.Getenv("POSTGRES_SSLMODE") // Kept as POSTGRES_SSLMODE

		// Check if the primary DB connection variables are set
		if host == "" || port == "" || user == "" || dbname == "" { // Password can sometimes be empty for local dev
			log.Println("Warning: One or more DB_HOST, DB_PORT, DB_USER, DB_NAME environment variables are not set. Attempting to use defaults for local development.")
			if host == "" {
				host = "localhost"
			}
			if port == "" {
				port = "5432"
			}
			if user == "" {
				user = "myuser" // Common default user
			}
			// Password check: warn if the DB_PASSWORD environment variable was empty.
			// The 'password' variable is already initialized from os.Getenv("DB_PASSWORD").
			if os.Getenv("DB_PASSWORD") == "" {
				// It's better to require a password, but for local dev, some setups might not have one.
				// Consider logging a more prominent warning or failing if critical.
				log.Println("Warning: DB_PASSWORD environment variable is not set.")
			}
			if dbname == "" {
				dbname = "postgres" // Common default db
			}
		} // Closes: if host == "" || port == "" || user == "" || dbname == ""

		// Default for sslmode if not set by environment variable.
		// The 'sslmode' variable is already initialized from os.Getenv("POSTGRES_SSLMODE") (line 38).
		// If the environment variable was empty, we set 'sslmode' to "disable" and log it.
		if os.Getenv("POSTGRES_SSLMODE") == "" {
			sslmode = "disable" // Ensure it's 'disable' if the env var was empty
			log.Println("Warning: POSTGRES_SSLMODE is not set. Defaulting to 'disable'.")
		}
		// If POSTGRES_SSLMODE was set in the environment, 'sslmode' already holds its value.


		connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
			host, port, user, password, dbname, sslmode)

		config, err := pgxpool.ParseConfig(connStr)
		if err != nil {
			initErr = fmt.Errorf("failed to parse postgres connection string: %w", err)
			log.Printf("%v\n", initErr)
			return
		}

		// Configure the pool
		maxConnsStr := os.Getenv("POSTGRES_MAX_OPEN_CONNS")
		minConnsStr := os.Getenv("POSTGRES_MIN_CONNS") // Changed from MAX_IDLE_CONNS
		connMaxLifetimeMinutesStr := os.Getenv("POSTGRES_CONN_MAX_LIFETIME_MINUTES")
		connMaxIdleTimeMinutesStr := os.Getenv("POSTGRES_CONN_MAX_IDLE_TIME_MINUTES")

		maxConns, err := strconv.Atoi(maxConnsStr)
		if err != nil || maxConns <= 0 {
			maxConns = 25 // Default value
			if maxConnsStr != "" {
				log.Printf("Warning: Invalid POSTGRES_MAX_OPEN_CONNS value '%s'. Using default: %d\n", maxConnsStr, maxConns)
			}
		}
		config.MaxConns = int32(maxConns)

		minConns, err := strconv.Atoi(minConnsStr)
		if err != nil || minConns < 0 { // minConns can be 0
			minConns = 5 // Default value
			if minConnsStr != "" {
				log.Printf("Warning: Invalid POSTGRES_MIN_CONNS value '%s'. Using default: %d\n", minConnsStr, minConns)
			}
		}
		config.MinConns = int32(minConns)

		connMaxLifetimeMinutes, err := strconv.Atoi(connMaxLifetimeMinutesStr)
		if err != nil || connMaxLifetimeMinutes <= 0 {
			connMaxLifetimeMinutes = 15 // Default value in minutes
			if connMaxLifetimeMinutesStr != "" {
				log.Printf("Warning: Invalid POSTGRES_CONN_MAX_LIFETIME_MINUTES value '%s'. Using default: %d minutes\n", connMaxLifetimeMinutesStr, connMaxLifetimeMinutes)
			}
		}
		config.MaxConnLifetime = time.Duration(connMaxLifetimeMinutes) * time.Minute

		connMaxIdleTimeMinutes, err := strconv.Atoi(connMaxIdleTimeMinutesStr)
		if err != nil || connMaxIdleTimeMinutes <= 0 {
			connMaxIdleTimeMinutes = 5 // Default value in minutes
			if connMaxIdleTimeMinutesStr != "" {
				log.Printf("Warning: Invalid POSTGRES_CONN_MAX_IDLE_TIME_MINUTES value '%s'. Using default: %d minutes\n", connMaxIdleTimeMinutesStr, connMaxIdleTimeMinutes)
			}
		}
		config.MaxConnIdleTime = time.Duration(connMaxIdleTimeMinutes) * time.Minute

		// config.HealthCheckPeriod = 1 * time.Minute // Optional: configure health check period

		pool, err := pgxpool.NewWithConfig(context.Background(), config) // Use background context for pool creation
		if err != nil {
			initErr = fmt.Errorf("failed to create postgres connection pool: %w", err)
			log.Printf("%v\n", initErr)
			return
		}

		// Ping the database to verify connection
		if err = pool.Ping(ctx); err != nil {
			initErr = fmt.Errorf("failed to ping postgres database: %w", err)
			log.Printf("%v\n", initErr)
			pool.Close() // Close the pool if ping fails
			return
		}

		log.Println("Successfully connected to PostgreSQL database using pgxpool.")
		postgresInstance = PostgresClient{Pool: pool}
	})

	if initErr != nil {
		return PostgresClient{}, initErr // Return the error captured during initialization
	}
	if postgresInstance.Pool == nil && initErr == nil {
		// This case should ideally not be reached if initErr is handled correctly.
		// It means sync.Once completed without setting initErr but Pool is still nil.
		return PostgresClient{}, fmt.Errorf("failed to initialize postgres client: connection pool is nil after sync.Once execution")
	}

	return postgresInstance, nil
}

// Close closes the pgx connection pool.
// It's good practice to defer this in the main application function.
func (pc *PostgresClient) Close() {
	if pc.Pool != nil {
		pc.Pool.Close()
		log.Println("PostgreSQL connection pool closed.")
	}
}
