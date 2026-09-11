package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/G6kco/CyberSpace.git/internal/types"
)

func NewMySQL(dsn string) error {
	var err error
	if dsn == "" {
		return types.DataBaseStrEmptyError
	}
	
	types.DBCONN , err = sql.Open("mysql", dsn)
	if err != nil{
		return err
	}
	
	types.DBCONN.SetMaxOpenConns(25)
	types.DBCONN.SetMaxIdleConns(10)
	types.DBCONN.SetConnMaxLifetime(5 * time.Minute)
	types.DBCONN.SetConnMaxIdleTime(2 * time.Minute)
	
	ctx, cancel := context.WithTimeout(context.Background(), 10 * time.Second)
	defer cancel()
	if err := types.DBCONN.PingContext(ctx); err != nil{
		return err
	}

	return nil
}