package stalerevisionlostupdate

import (
	"stage-rigging-release/internal/app"
	"stage-rigging-release/internal/rigging"
	"stage-rigging-release/internal/store"
	"testing"
	"time"
)

func TestStaleCaseRevisionLostUpdate(t *testing.T) {
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := app.New(s)
	c, err := a.CreateCase("演出", "主舞台", time.Now(), time.Now().Add(time.Hour), "author")
	if err != nil {
		t.Fatal(err)
	}
	base := c
	if err := a.AddPoint(base, rigging.Point{ID: "p1", Label: "吊点1", RatedLoadKg: 100, PlannedStaticLoadKg: 10, SlingAngleDegrees: 60, DynamicFactor: 1}); err != nil {
		t.Fatal(err)
	}
	if err := a.AddPoint(base, rigging.Point{ID: "p2", Label: "吊点2", RatedLoadKg: 100, PlannedStaticLoadKg: 10, SlingAngleDegrees: 60, DynamicFactor: 1}); err != nil {
		t.Fatal(err)
	}
	current, _ := s.GetCase(c.ID)
	if current.Revision != 3 {
		t.Fatalf("连续写入应产生两个修订，实际为 %d", current.Revision)
	}
}
