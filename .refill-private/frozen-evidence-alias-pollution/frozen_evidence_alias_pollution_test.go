package frozen_evidence_alias_pollution_test

import (
	"stage-rigging-release/internal/app"
	"stage-rigging-release/internal/rigging"
	"stage-rigging-release/internal/store"
	"testing"
	"time"
)

func TestFrozenEvidenceCanonicalizationDoesNotMutateLiveState(t *testing.T) {
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	now := time.Now()
	c := rigging.Case{
		ID: "case-alias", ShowName: "别名边界测试", VenueZone: "主舞台",
		PerformanceStartsAt: now.Add(-time.Hour), PerformanceEndsAt: now.Add(time.Hour),
		Status: rigging.StatusApproved, Revision: 7, AuthorID: "author",
		CreatedAt: now, UpdatedAt: now,
	}
	s.PutCase(c)
	plan := rigging.RemediationPlan{
		ID: "plan-alias", CaseID: c.ID, FindingID: "finding-1", ActionType: "降载",
		Changes: []rigging.StructuredChange{
			{TargetID: "point-z", Field: "plannedStaticLoadKg", Before: "80", After: "70"},
			{TargetID: "point-a", Field: "plannedStaticLoadKg", Before: "70", After: "60"},
		},
		Items: []rigging.RetestItem{
			{ID: "retest-z", Kind: "吊点观察", Status: "通过"},
			{ID: "retest-a", Kind: "载荷复算", Status: "通过"},
		},
		SubmittedBy: "author", SubmittedAt: now,
	}
	if err := s.AddPlan(plan, "author"); err != nil {
		t.Fatal(err)
	}
	round := rigging.ReviewRound{
		ID: "review-round-1", CaseID: c.ID, ReviewerID: "reviewer", Number: 1, Revision: 8,
		Evidence: []rigging.EvidenceItem{
			{Category: "整改复验-z", ObjectID: "object-z", Status: "完整"},
			{Category: "整改复验-a", ObjectID: "object-a", Status: "完整"},
		},
		ReturnItems:  []rigging.ReviewReturnItem{{ID: "return-z"}, {ID: "return-a"}},
		Contributors: []string{"worker-z", "worker-a"}, SubmittedAt: now,
	}
	if err := s.AddReviewRound(round); err != nil {
		t.Fatal(err)
	}
	c, _ = s.GetCase(c.ID)
	c.Status = rigging.StatusApproved
	s.PutCase(c)
	s.PutEvaluation(rigging.Evaluation{
		ID: "eval-alias", CaseID: c.ID, CaseRevision: c.Revision,
		PointResults:         []rigging.PointResult{{PointID: "point-1", MarginPercent: 50}},
		MinimumMarginPercent: 50, Outcome: "通过", EvaluatedAt: now,
	})

	if _, err := app.New(s).Freeze(c, "issuer"); err != nil {
		t.Fatal(err)
	}

	gotPlan := s.Plans(c.ID)[0]
	gotRound := s.ReviewRounds(c.ID)[0]
	if gotPlan.Changes[0].TargetID != "point-z" || gotPlan.Items[0].ID != "retest-z" ||
		gotRound.Evidence[0].Category != "整改复验-z" || gotRound.ReturnItems[0].ID != "return-z" ||
		gotRound.Contributors[0] != "worker-z" {
		t.Fatalf("冻结摘要计算污染了在线状态：plan=%+v review=%+v", gotPlan, gotRound)
	}
}
