package stale_release_digest_cache_test

import (
	"stage-rigging-release/internal/credential"
	"stage-rigging-release/internal/rigging"
	"testing"
	"time"
)

func TestReissuedCredentialUsesCurrentEvidence(t *testing.T) {
	checkedAt := time.Date(2032, time.June, 15, 19, 0, 0, 0, time.UTC)
	validFrom := checkedAt.Add(-time.Hour)
	validUntil := checkedAt.Add(time.Hour)
	firstEvidence := credential.Evidence{
		Case: rigging.Case{ID: "case-cache-reissue", Revision: 7},
		Points: []rigging.Point{{
			ID:                  "point-main",
			CaseID:              "case-cache-reissue",
			RatedLoadKg:         500,
			PlannedStaticLoadKg: 100,
			SlingAngleDegrees:   60,
			DynamicFactor:       1.1,
		}},
	}
	first := credential.Issue(firstEvidence, "tech-lead", validFrom, validUntil, nil)
	if status, valid := credential.Verify(first, firstEvidence, checkedAt); !valid {
		t.Fatalf("首张凭据应有效，实际状态为%s", status)
	}

	currentEvidence := firstEvidence
	currentEvidence.Case.Revision = 8
	currentEvidence.Points = append([]rigging.Point(nil), firstEvidence.Points...)
	currentEvidence.Points[0].PlannedStaticLoadKg = 160
	second := credential.Issue(currentEvidence, "tech-lead", validFrom, validUntil, nil)

	if second.FrozenRevision != currentEvidence.Case.Revision {
		t.Fatalf("第二张凭据修订号=%d，期望%d", second.FrozenRevision, currentEvidence.Case.Revision)
	}
	if status, valid := credential.Verify(second, currentEvidence, checkedAt); !valid {
		t.Fatalf("当前修订刚签发的凭据必须立即可验，实际状态为%s", status)
	}
}
