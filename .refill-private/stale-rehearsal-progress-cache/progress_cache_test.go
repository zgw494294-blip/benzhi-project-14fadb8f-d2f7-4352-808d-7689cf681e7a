package stale_rehearsal_progress_cache_test

import (
	"stage-rigging-release/internal/app"
	"stage-rigging-release/internal/rigging"
	"stage-rigging-release/internal/store"
	"testing"
	"time"
)

func TestRehearsalProgressRefreshesAfterObservation(t *testing.T) {
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	a := app.New(s)
	start := time.Date(2032, 4, 10, 19, 30, 0, 0, time.UTC)
	c, err := a.CreateCase("缓存复现演出", "主舞台", start, start.Add(2*time.Hour), "mechanist-1")
	if err != nil {
		t.Fatal(err)
	}
	p := rigging.Point{
		ID: "point-cache", CaseID: c.ID, Label: "缓存吊点",
		RatedLoadKg: 500, PlannedStaticLoadKg: 100,
		SlingAngleDegrees: 60, DynamicFactor: 1.1,
	}
	s.AddPoint(p)
	s.PutEvaluation(rigging.Evaluation{ID: "evaluation-cache", CaseID: c.ID, CaseRevision: c.Revision, Outcome: "通过"})

	before, err := a.RehearsalProgress(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if before.Unchecked != 4 || before.Passed != 0 {
		t.Fatalf("首次清单不符合前置条件: unchecked=%d passed=%d", before.Unchecked, before.Passed)
	}

	_, finding, err := a.RecordObservation(c.ID, "mechanist-1", rigging.Observation{
		PointID: p.ID,
		Type:    "位移",
		Measurements: []rigging.Measurement{{
			Value: 1.0, MeasuredAt: start.Add(time.Minute),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if finding != nil {
		t.Fatalf("合格观察不应产生发现: %+v", finding)
	}

	after, err := a.RehearsalProgress(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Unchecked != 3 || after.Passed != 1 || after.Percent != 25 {
		t.Fatalf("提交观察后仍返回陈旧彩排进度: unchecked=%d passed=%d percent=%d", after.Unchecked, after.Passed, after.Percent)
	}

	current, ok := s.GetCase(c.ID)
	if !ok {
		t.Fatal("提交观察后案卷消失")
	}
	secondPoint := rigging.Point{
		ID: "point-cache-second", Label: "新增吊点",
		RatedLoadKg: 500, PlannedStaticLoadKg: 80,
		SlingAngleDegrees: 55, DynamicFactor: 1.1,
	}
	if err := a.AddPoint(current, secondPoint); err != nil {
		t.Fatal(err)
	}

	withSecondPoint, err := a.RehearsalProgress(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if withSecondPoint.Total != 8 || withSecondPoint.Unchecked != 7 || withSecondPoint.Passed != 1 {
		t.Fatalf("新增吊点后仍返回旧彩排范围: total=%d unchecked=%d passed=%d", withSecondPoint.Total, withSecondPoint.Unchecked, withSecondPoint.Passed)
	}
}
