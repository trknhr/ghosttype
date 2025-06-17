package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/trknhr/ghosttype/cmd"
	"github.com/trknhr/ghosttype/internal/logger"
	"github.com/trknhr/ghosttype/internal/store"
	_ "github.com/tursodatabase/go-libsql"
)

func main() {
	db, err := store.OpenDefaultDB()
	initLogger()

	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}

	if err := store.Migrate(db); err != nil {
		log.Fatalf("failed to migrate schema: %v", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	if err := cmd.Execute(db); err != nil {
		log.Fatal(err)
	}
}

func initLogger() {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		log.Printf("failed to get cache dir: %v", err)
		return
	}
	logPath := filepath.Join(cacheDir, "ghosttype", "ghosttype.log")
	level := os.Getenv("GHOSTTYPE_LOG_LEVEL")
	if level == "" {
		level = "warn"
	}
	if err := logger.Init(logPath, level); err != nil {
		log.Printf("logger init failed: %v", err)
	}
}
