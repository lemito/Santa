package migrations

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

func Run(ctx context.Context, db *sql.DB, dir string) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("database migrations not found in %s", dir)
	}
	sort.Strings(files)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS "SantaGoSchemaMigrations" ("Name" text PRIMARY KEY, "Checksum" text NOT NULL, "AppliedAt" timestamptz NOT NULL DEFAULT NOW())`); err != nil {
		return err
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		checksum := hex.EncodeToString(sum[:])
		name := filepath.Base(file)
		var applied string
		err = tx.QueryRowContext(ctx, `SELECT "Checksum" FROM "SantaGoSchemaMigrations" WHERE "Name"=$1`, name).Scan(&applied)
		switch {
		case err == nil:
			if applied != checksum {
				return fmt.Errorf("migration checksum changed: %s", name)
			}
			continue
		case errors.Is(err, sql.ErrNoRows):
		default:
			return err
		}
		if _, err := tx.ExecContext(ctx, string(data)); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO "SantaGoSchemaMigrations" ("Name","Checksum") VALUES ($1,$2)`, name, checksum); err != nil {
			return fmt.Errorf("record %s: %w", name, err)
		}
	}
	return tx.Commit()
}
