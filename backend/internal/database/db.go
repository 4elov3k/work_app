package database

import (
	"database/sql"
	"errors"
	"fmt"

	_ "github.com/lib/pq"
)

// ErrNotFound is wrapped into every "X not found" error this package
// returns for a missing row, so callers (handlers.isRecordNotFoundError) can
// classify 404-vs-500 with errors.Is instead of matching an error message
// string convention across a package boundary — a message reword here would
// silently break that classification otherwise.
var ErrNotFound = errors.New("not found")

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
