package postgres

import (
	"database/sql"
	"fmt"
	"log"
)

type ConnectionInfo struct {
	Host     string
	Port     int
	Username string
	DBName   string
	SSLMode  string
	Password string
}

func NewPostgresConnection(info ConnectionInfo) (*sql.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s dbname=%s sslmode=%s password=%s",
		info.Host,
		info.Port,
		info.Username,
		info.DBName,
		info.SSLMode,
		info.Password,
	)

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}

func Close(db *sql.DB) {
	if err := db.Close(); err != nil {
		log.Fatalf("Postgres connection error: %s", err)
	}
}
