package rkt

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func (c *RKT) getMigrationPath() (string, error) {
	migrationDir := filepath.Join(c.RootPath, "migrations")

	// Check if directory exists
	if _, err := os.Stat(migrationDir); os.IsNotExist(err) {
		return "", fmt.Errorf("migration directory does not exist: %s", migrationDir)
	}

	// Convert to forward slashes
	migrationDir = filepath.ToSlash(migrationDir)

	// Different handling for Windows vs Unix-like systems
	if runtime.GOOS == "windows" {
		// Windows: use file:// with forward slashes
		return "file://" + migrationDir, nil
	}

	// Mac/Linux: use file:// (path already starts with /)
	return "file://" + migrationDir, nil
}

func (c *RKT) MigrateUp(dsn string) error {
	migrationPath, err := c.getMigrationPath()
	if err != nil {
		return err
	}

	m, err := migrate.New(migrationPath, dsn)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Println("Error running migration:", err)
		return err
	}
	return nil
}

func (c *RKT) MigrateDownAll(dsn string) error {
	migrationPath, err := c.getMigrationPath()
	if err != nil {
		return err
	}

	m, err := migrate.New(migrationPath, dsn)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	defer m.Close()

	if err := m.Down(); err != nil {
		return err
	}
	return nil
}

func (c *RKT) Steps(n int, dsn string) error {
	migrationPath, err := c.getMigrationPath()
	if err != nil {
		return err
	}

	m, err := migrate.New(migrationPath, dsn)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	defer m.Close()

	if err := m.Steps(n); err != nil {
		return err
	}
	return nil
}

func (c *RKT) MigrateForce(dsn string) error {
	migrationPath, err := c.getMigrationPath()
	if err != nil {
		return err
	}

	m, err := migrate.New(migrationPath, dsn)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	defer m.Close()

	if err := m.Force(-1); err != nil {
		return err
	}
	return nil
}
