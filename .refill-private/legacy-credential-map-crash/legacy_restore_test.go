package legacycredentialmapcrash_test

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"stage-rigging-release/internal/app"
	"stage-rigging-release/internal/rigging"
	"stage-rigging-release/internal/store"
)

func TestFreezeAfterLegacyStateRestoreDoesNotPanic(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "legacy-state.db")
	initial, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("create database: %v", err)
	}
	if err := initial.Close(); err != nil {
		t.Fatalf("close initial store: %v", err)
	}

	start := time.Date(2032, 5, 10, 19, 30, 0, 0, time.UTC)
	c := rigging.Case{
		ID:                  "case-legacy",
		ShowName:            "旧快照恢复演出",
		VenueZone:           "主舞台",
		PerformanceStartsAt: start,
		PerformanceEndsAt:   start.Add(2 * time.Hour),
		Status:              rigging.StatusApproved,
		Revision:            7,
		AuthorID:            "mechanist-a",
		CreatedAt:           start.Add(-24 * time.Hour),
		UpdatedAt:           start.Add(-time.Hour),
	}
	ev := rigging.Evaluation{
		ID:                   "eval-legacy",
		CaseID:               c.ID,
		CaseRevision:         c.Revision,
		InputDigest:          "legacy-input",
		MinimumMarginPercent: 25,
		Outcome:              "通过",
		EvaluatedAt:          start.Add(-2 * time.Hour),
	}
	legacyState, err := json.Marshal(map[string]any{
		"cases":        map[string]rigging.Case{c.ID: c},
		"evaluations":  map[string]rigging.Evaluation{c.ID: ev},
		"nextAuditSeq": 12,
	})
	if err != nil {
		t.Fatalf("marshal legacy state: %v", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open database for fixture: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO app_state(id,data) VALUES(1,?) ON CONFLICT(id) DO UPDATE SET data=excluded.data`, legacyState); err != nil {
		db.Close()
		t.Fatalf("write legacy state: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close fixture database: %v", err)
	}

	reopened, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("reopen legacy state: %v", err)
	}
	defer reopened.Close()

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("TestFreezeAfterLegacyStateRestoreDoesNotPanic: Freeze panicked after restore: %v", recovered)
		}
	}()
	issued, err := app.New(reopened).Freeze(c, "technical-lead")
	if err != nil {
		t.Fatalf("freeze restored case: %v", err)
	}
	if stored, ok := reopened.Credential(issued.ID); !ok || stored.ID != issued.ID {
		t.Fatalf("issued credential was not retained after restored-state freeze")
	}
}
