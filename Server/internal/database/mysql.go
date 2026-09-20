package database

import (
	"database/sql"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/G6kco/CyberSpace/internal/types"
)

func NewMySQL(dsn string) (*sql.DB, error) {

	if dsn == "" {
		return nil, types.DataBaseStrEmptyError
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(2 * time.Minute)

	return db, nil
}
