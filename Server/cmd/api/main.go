package main

import (
	"fmt"

	"github.com/G6kco/CyberSpace/cmd/router"
	"github.com/G6kco/CyberSpace/internal/config"
	"github.com/G6kco/CyberSpace/internal/database"
	"github.com/G6kco/CyberSpace/internal/types"
	"github.com/G6kco/CyberSpace/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func main() {
	cfs, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load configurations Error:%s\n", err)
		return
	}
	
	types.DBCONN, err = database.NewMySQL(cfs.DatabaseURL)
	if err != nil{
		fmt.Println("database connection failed")
		fmt.Printf("Error: %s", err)
		return
	}
	
	if cfs.AppEnv != ""{
		logger.LoadLogger(cfs.AppEnv)
	}else{
		_ = fmt.Errorf("loding logger failed")
		return 
	}
	types.LOG.Sync()
	
	server(cfs)
}

func server(cfs *config.Config){
	engine := gin.New()
	
	types.LOG.Info("Starting server......", zap.String("env->", cfs.AppEnv), zap.Uint("port->", cfs.ServerPort))
	
	router.LoadRouter(engine)
	types.LOG.Info("Router is loaded..")
	
	err := engine.Run(fmt.Sprintf(":%d",cfs.ServerPort))
	if err != nil{
		types.LOG.Fatal("Failed tp start server, ", zap.Error(err))
	}
}