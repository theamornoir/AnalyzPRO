package main

// dbcheck - минимальная проверка подключения к БД из окружения бота.
// Собирает конфиг из .env (как бот) и пытается открыть/пропинговать БД
// (SQLite или PostgreSQL/Yandex Cloud в зависимости от DB_DRIVER), затем
// делает SELECT 1. Использование:
//
//	DB_DRIVER=postgres DB_DSN="host=... dbname=prisma sslmode=verify-full sslrootcert=/etc/prisma/ssl/yb-ca.pem" \
//	  go run ./cmd/dbcheck

import (
	"fmt"
	"os"

	"github.com/theamornoir/analyzpro/internal/config"
	"github.com/theamornoir/analyzpro/internal/db"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		os.Exit(1)
	}
	conn, err := db.OpenConfig(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "OPEN FAIL:", err)
		os.Exit(1)
	}
	defer conn.Close()

	var ok int
	if err := conn.QueryRow("SELECT 1").Scan(&ok); err != nil {
		fmt.Fprintln(os.Stderr, "PING FAIL:", err)
		os.Exit(1)
	}
	fmt.Printf("OK: %s reachable, SELECT 1 = %d\n", cfg.DBDriver, ok)
}
