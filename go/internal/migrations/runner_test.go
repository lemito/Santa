package migrations

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestRunIsRepeatableAndDetectsChecksumDrift(t *testing.T) {
	connection := os.Getenv("SANTA_TEST_DB")
	if connection == "" {
		t.Skip("SANTA_TEST_DB is not set")
	}
	db, err := sql.Open("pgx", connection)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS "SantaGoMigrationProbe" CASCADE; DROP TABLE IF EXISTS "SantaGoSchemaMigrations"`); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "0000_probe.sql"), []byte(`CREATE TABLE "SantaGoMigrationProbe" ("Id" integer PRIMARY KEY);`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "0001_probe_row.sql"), []byte(`INSERT INTO "SantaGoMigrationProbe" ("Id") VALUES (1);`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Run(ctx, db, dir); err != nil {
		t.Fatal(err)
	}
	if err := Run(ctx, db, dir); err != nil {
		t.Fatalf("second Run() error = %v", err)
	}
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM "SantaGoMigrationProbe"`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("probe rows=%d, want 1", count)
	}
	if err := os.WriteFile(filepath.Join(dir, "0001_probe_row.sql"), []byte(`INSERT INTO "SantaGoMigrationProbe" ("Id") VALUES (2);`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Run(ctx, db, dir); err == nil {
		t.Fatal("checksum drift was accepted")
	}
}
