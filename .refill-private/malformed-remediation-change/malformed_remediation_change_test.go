package malformedremediationchange

import (
	"stage-rigging-release/internal/app"
	"stage-rigging-release/internal/rigging"
	"stage-rigging-release/internal/store"
	"testing"
	"time"
)

func TestMalformedRemediationChangeRejected(t *testing.T) {
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
	s.AddPoint(rigging.Point{ID: "p1", CaseID: c.ID, Label: "吊点1", RatedLoadKg: 100, PlannedStaticLoadKg: 10, SlingAngleDegrees: 60, DynamicFactor: 1})
	s.AddFinding(rigging.Finding{ID: "finding-1", CaseID: c.ID, PointID: "p1", ObservationType: "位移", Status: "open", Description: "位移超限"})
	_, err = a.CreateRemediation(c.ID, "finding-1", "降载", "operator", []rigging.StructuredChange{{Kind: "参数调整", TargetID: "p1", Field: "plannedStaticLoadKg", Before: "10", After: "not-a-number"}})
	if err == nil {
		t.Fatal("整改变更的数值无法解析时应返回错误")
	}
}
