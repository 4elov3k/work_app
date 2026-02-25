package database

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

// DB представляет подключение к базе данных
type DB struct {
	*sql.DB
}

// NewDB создает новое подключение к базе данных
func NewDB(connectionString string) (*DB, error) {
	db, err := sql.Open("postgres", connectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Проверка подключения
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Настройка пула соединений
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	return &DB{db}, nil
}
