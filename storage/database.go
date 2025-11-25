// Package storage creates a database connection and runs the migrations located in the `db/migrations` directory.
package storage

import (
	"context"
	"errors"
	"fmt"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/golang-migrate/migrate/v4/source/github"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kozalosev/goSadTgBot/logconst"
	log "log/slog"
	"os"
	"strconv"
)

const (
	migrationsPath             = "db/migrations"
	duplicateConstraintSQLCode = "23505"
)

type DatabaseConfig struct {
	host     string
	port     string
	user     string
	password string
	dbName   string
}

func NewDatabaseConfig(host, port, username, password, dbName string) *DatabaseConfig {
	return &DatabaseConfig{
		host:     host,
		port:     port,
		user:     username,
		password: password,
		dbName:   dbName,
	}
}

// ConnectToDatabase returns a connection pool, which can be used to execute queries to the database.
func ConnectToDatabase(ctx context.Context, config *DatabaseConfig) *pgxpool.Pool {
	logger := log.With(logconst.FieldFunc, "ConnectToDatabase")

	intPort, err := strconv.ParseInt(config.port, 10, strconv.IntSize)
	if err != nil {
		logger.Error("Failed to parse a database port number",
			logconst.FieldCalledFunc, "ParseInt",
			logconst.FieldError, err)
		os.Exit(1)
	}

	connURL := fmt.Sprintf("postgres://%s:%s@%s:%d/%s",
		config.user, config.password, config.host, intPort, config.dbName)
	conn, err := pgxpool.New(ctx, connURL)
	if err != nil {
		logger.Error("Failed to create a database connection pool",
			logconst.FieldCalledObject, "Pool",
			logconst.FieldCalledMethod, "New",
			logconst.FieldError, err)
		os.Exit(1)
	}

	if err := conn.Ping(ctx); err != nil {
		logger.Error("Failed to ping the database",
			logconst.FieldCalledObject, "Pool",
			logconst.FieldCalledMethod, "Ping",
			logconst.FieldError, err)
		os.Exit(1)
	}
	return conn
}

// RunMigrations either from source code on a local machine if available (for developers) or from a GitHub repository (for production).
func RunMigrations(config *DatabaseConfig, migrationsRepo string) {
	logger := log.With(logconst.FieldFunc, "RunMigrations")

	var sourceURL string
	if _, err := os.Stat(migrationsPath); err == nil {
		sourceURL = "file://" + migrationsPath
	} else if _, err := os.Stat("../" + migrationsPath); err == nil {
		sourceURL = "file://../" + migrationsPath
	} else if _, err := os.Stat("../../" + migrationsPath); err == nil {
		sourceURL = "file://../../" + migrationsPath
	} else {
		logger.Warn("Run migrations from the repository")
		sourceURL = "github://" + migrationsRepo + "/" + migrationsPath
	}
	databaseURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		config.user, config.password, config.host, config.port, config.dbName)

	m, err := migrate.New(sourceURL, databaseURL)
	if err != nil {
		log.Error("migrate.New failed",
			logconst.FieldCalledFunc, "migrate.New",
			logconst.FieldError, err)
		os.Exit(1)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Error("Failed to apply migrations",
			logconst.FieldCalledObject, "Migrate",
			logconst.FieldCalledMethod, "Up",
			logconst.FieldError, err)
		os.Exit(1)
	}
}

// DuplicateConstraintViolation returns true if the errors represents a duplicate constraint violation (SQLSTATE 23505)
// occurred in the database.
func DuplicateConstraintViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == duplicateConstraintSQLCode
}
