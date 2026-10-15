package database

import (
	"database/sql"
	"time"

	"github.com/go-sql-driver/mysql"

	"github.com/G6kco/CyberSpace/internal/types"
)

func NewMySQL(dsn string) (*sql.DB, error) {

	if dsn == "" {
		return nil, types.DataBaseStrEmptyError
	}

	// Attempt deadlines and cooldowns are compared as time.Time, so DATETIME
	// columns must scan into time.Time and be read and written as UTC,
	// whatever the configured DSN says.
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, err
	}
	cfg.ParseTime = true
	cfg.Loc = time.UTC

	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(2 * time.Minute)

	return db, nil
}
