package handlers

import (
	"context"
	"database/sql"
	"testing"

	"github.com/sms-relay/server/internal/db"
	"github.com/sms-relay/server/internal/migrate"
	_ "github.com/mattn/go-sqlite3"
)

func TestEnsureDeviceRecordReusesSameNameDevice(t *testing.T) {
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	if err := migrate.Up(sqlDB, "../../db/migrations"); err != nil {
		t.Fatal(err)
	}

	queries := db.New(sqlDB)
	ctx := context.Background()

	_, err = queries.CreateUser(ctx, db.CreateUserParams{
		ID:                  "user-1",
		PasswordHash:        "hash",
		PasswordFingerprint: "fp-1",
	})
	if err != nil {
		t.Fatal(err)
	}

	h := &Handler{queries: queries}

	firstID, err := h.ensureDeviceRecord(ctx, "user-1", "Pixel 8", "client-a")
	if err != nil {
		t.Fatal(err)
	}

	secondID, err := h.ensureDeviceRecord(ctx, "user-1", "Pixel 8", "client-b")
	if err != nil {
		t.Fatal(err)
	}
	if firstID != secondID {
		t.Fatalf("expected same device id, got %q and %q", firstID, secondID)
	}

	devices, err := queries.ListDevicesByUser(ctx, "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) != 1 {
		t.Fatalf("expected 1 device, got %d", len(devices))
	}
	if !devices[0].ClientID.Valid || devices[0].ClientID.String != "client-b" {
		t.Fatalf("expected client_id client-b, got %+v", devices[0].ClientID)
	}
}
