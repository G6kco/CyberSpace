package main

import (
	"fmt"

	"github.com/G6kco/CyberSpace/internal/config"
	"github.com/G6kco/CyberSpace/internal/database"
	"github.com/G6kco/CyberSpace/internal/types"
	"github.com/G6kco/CyberSpace/logger"
)

func main() {
	cfs, err := config.Load()
	if err != nil {
		fmt.Println("Failed to load configurations")
		fmt.Errorf("Error: %w", err)
		return
	}
	
	types.DBCONN, err = database.NewMySQL(cfs.DatabaseURL)
	if err != nil{
		fmt.Println("database connection failed")
		fmt.Errorf("Error: %w", err)
		return
	}
	
	if cfs.AppEnv != ""{
		logger.LoadLogger(cfs.AppEnv)
	}else{
		fmt.Errorf("loding logger failed")
		return 
	}
	types.LOG.Sync()
}