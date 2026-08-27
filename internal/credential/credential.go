package credential

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"stage-rigging-release/internal/rigging"
	"strings"
	"time"
)

type Evidence struct {
	Case             rigging.Case
	Points           []rigging.Point
	Equipment        []rigging.Equipment
	Evaluation       rigging.Evaluation
	Findings         []rigging.Finding
	Remediations     []rigging.Remediation
	Review           rigging.Review
	Assignments      []rigging.Assignment
	Observations     []rigging.Observation
	RemediationPlans []rigging.RemediationPlan
	ReviewRounds     []rigging.ReviewRound
}
type Credential struct {
	ID, CaseID, EvidenceDigest                        string
	PartitionDigests                                  map[string]string
	FrozenRevision                                    int
	ValidFrom, ValidUntil                             time.Time
	Conditions                                        []string
	IssuedBy                                          string
	IssuedAt                                          time.Time
	RevokedAt                                         *time.Time
	RevokedBy, RevocationReason, PreviousCredentialID string
	Signature                                         string
}

func Digest(e Evidence) string {
	p := PartitionDigests(e)
	b, _ := json.Marshal(p)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func PartitionDigests(e Evidence) map[string]string {
	points := append([]rigging.Point{}, e.Points...)
	sort.Slice(points, func(i, j int) bool { return points[i].ID < points[j].ID })
	equipment := append([]rigging.Equipment{}, e.Equipment...)
	sort.Slice(equipment, func(i, j int) bool { return equipment[i].ID < equipment[j].ID })
	assignments := append([]rigging.Assignment{}, e.Assignments...)
	sort.Slice(assignments, func(i, j int) bool {
		if assignments[i].PointID != assignments[j].PointID {
			return assignments[i].PointID < assignments[j].PointID
		}
		if assignments[i].Path != assignments[j].Path {
			return assignments[i].Path < assignments[j].Path
		}
		return assignments[i].EquipmentID < assignments[j].EquipmentID
	})
	observations := append([]rigging.Observation{}, e.Observations...)
	sort.Slice(observations, func(i, j int) bool { return observations[i].ID < observations[j].ID })
	findings := append([]rigging.Finding{}, e.Findings...)
	sort.Slice(findings, func(i, j int) bool { return findings[i].ID < findings[j].ID })
	legacy := append([]rigging.Remediation{}, e.Remediations...)
	sort.Slice(legacy, func(i, j int) bool { return legacy[i].ID < legacy[j].ID })
	plans := append([]rigging.RemediationPlan{}, e.RemediationPlans...)
	normalizePlanDetails(plans)
	sort.Slice(plans, func(i, j int) bool { return plans[i].ID < plans[j].ID })
	rounds := append([]rigging.ReviewRound{}, e.ReviewRounds...)
	normalizeReviewDetails(rounds)
	sort.Slice(rounds, func(i, j int) bool { return rounds[i].Number < rounds[j].Number })
	return map[string]string{
		"case": digestPart(e.Case), "points": digestPart(points), "equipment": digestPart(struct {
			Equipment   []rigging.Equipment
			Assignments []rigging.Assignment
		}{equipment, assignments}),
		"evaluation": digestPart(e.Evaluation), "rehearsal": digestPart(struct {
			Observations []rigging.Observation
			Findings     []rigging.Finding
		}{observations, findings}),
		"remediation": digestPart(struct {
			Legacy []rigging.Remediation
			Plans  []rigging.RemediationPlan
		}{legacy, plans}),
		"review": digestPart(struct {
			Latest rigging.Review
			Rounds []rigging.ReviewRound
		}{e.Review, rounds}),
	}
}

func normalizePlanDetails(plans []rigging.RemediationPlan) {
	for i := range plans {
		sort.Slice(plans[i].Changes, func(a, b int) bool {
			return plans[i].Changes[a].TargetID+"\x00"+plans[i].Changes[a].Field < plans[i].Changes[b].TargetID+"\x00"+plans[i].Changes[b].Field
		})
		sort.Slice(plans[i].Items, func(a, b int) bool { return plans[i].Items[a].ID < plans[i].Items[b].ID })
	}
}

func normalizeReviewDetails(rounds []rigging.ReviewRound) {
	for i := range rounds {
		sort.Slice(rounds[i].Evidence, func(a, b int) bool {
			return rounds[i].Evidence[a].Category+"\x00"+rounds[i].Evidence[a].ObjectID < rounds[i].Evidence[b].Category+"\x00"+rounds[i].Evidence[b].ObjectID
		})
		sort.Slice(rounds[i].ReturnItems, func(a, b int) bool { return rounds[i].ReturnItems[a].ID < rounds[i].ReturnItems[b].ID })
		sort.Strings(rounds[i].Contributors)
	}
}

func digestPart(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func Issue(e Evidence, by string, from, until time.Time, conditions []string) Credential {
	d := Digest(e)
	sig := sha256.Sum256([]byte(d + "|" + by))
	return Credential{ID: fmt.Sprintf("RC-%s-%d", d[:12], time.Now().UnixNano()), CaseID: e.Case.ID, EvidenceDigest: d, PartitionDigests: PartitionDigests(e), FrozenRevision: e.Case.Revision, ValidFrom: from, ValidUntil: until, Conditions: conditions, IssuedBy: by, IssuedAt: time.Now(), Signature: hex.EncodeToString(sig[:])}
}
func Verify(c Credential, e Evidence, now time.Time) (string, bool) {
	if c.RevokedAt != nil {
		return "已撤销", false
	}
	if now.Before(c.ValidFrom) {
		return "尚未生效", false
	}
	if now.After(c.ValidUntil) {
		return "已过期", false
	}
	h := sha256.Sum256([]byte(c.EvidenceDigest + "|" + c.IssuedBy))
	if hex.EncodeToString(h[:]) != c.Signature {
		return "签名损坏", false
	}
	if Digest(e) != c.EvidenceDigest {
		return "证据摘要不一致", false
	}
	return "有效", true
}
func (c Credential) String() string {
	return fmt.Sprintf("%s %s %s", c.ID, c.CaseID, strings.Join(c.Conditions, ";"))
}
