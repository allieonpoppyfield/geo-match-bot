package db

import (
	"database/sql"
	"fmt"
	"geo_match_bot/internal/config"

	_ "github.com/jackc/pgx/v4/stdlib"
)

type DB struct {
	Conn *sql.DB
}

func NewPostgresDB(cfg *config.Config) (*DB, error) {
	dsn := fmt.Sprintf("user=%s password=%s dbname=%s host=%s sslmode=disable",
		cfg.PostgresUser, cfg.PostgresPass, cfg.PostgresDB, cfg.PostgresHost)
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}

	return &DB{Conn: db}, nil
}
