package migrate

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pressly/goose/v3"
)

func Up(db *sql.DB, migrationsDir string) error {
	if migrationsDir == "" {
		migrationsDir = defaultMigrationsDir()
	}
	if err := goose.SetDialect("sqlite3"); err != nil {
		return err
	}
	if err := goose.Up(db, migrationsDir); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}

func defaultMigrationsDir() string {
	candidates := []string{
		"server/db/migrations",
		"db/migrations",
		filepath.Join("..", "db", "migrations"),
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			return p
		}
	}
	return "db/migrations"
}
