package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/G6kco/CyberSpace/internal/config"
	"github.com/G6kco/CyberSpace/internal/database"
	"github.com/G6kco/CyberSpace/migrations"
	"github.com/go-sql-driver/mysql"
	migrate "github.com/golang-migrate/migrate/v4"
	migratemysql "github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "migration failed: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 1 || args[0] != "up" {
		return errors.New("usage: cyberspace-migrate up")
	}

	appConfig, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	dsn, err := migrationDSN(appConfig.DatabaseURL)
	if err != nil {
		return fmt.Errorf("parse database DSN: %w", err)
	}

	db, err := database.NewMySQL(dsn)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}

	sourceDriver, err := iofs.New(migrations.FS, ".")
	if err != nil {
		_ = db.Close()
		return fmt.Errorf("load migrations: %w", err)
	}

	databaseDriver, err := migratemysql.WithInstance(
		db,
		&migratemysql.Config{},
	)
	if err != nil {
		_ = db.Close()
		return fmt.Errorf("create migration database driver: %w", err)
	}

	migrator, err := migrate.NewWithInstance(
		"iofs",
		sourceDriver,
		"mysql",
		databaseDriver,
	)
	if err != nil {
		_ = sourceDriver.Close()
		_ = databaseDriver.Close()
		return fmt.Errorf("create migrator: %w", err)
	}

	defer migrator.Close()

	if err := migrator.Up(); err != nil &&
		!errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}

	fmt.Println("database migrations are up to date")
	return nil
}

func migrationDSN(rawDSN string) (string, error) {
	dsnConfig, err := mysql.ParseDSN(rawDSN)
	if err != nil {
		return "", err
	}

	dsnConfig.MultiStatements = true
	return dsnConfig.FormatDSN(), nil
}
