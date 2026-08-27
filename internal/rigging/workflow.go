package rigging

import "time"

// RowProblem keeps validation failures attached to the submitted row.  Code is
// stable for API clients while Message is deliberately Chinese for operators.
type RowProblem struct {
	Row     int    `json:"row"`
	PointID string `json:"pointId,omitempty"`
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type PointChange struct {
	Row    int    `json:"row"`
	Action string `json:"action"`
	Before *Point `json:"before,omitempty"`
	After  Point  `json:"after"`
}

type BatchPointResult struct {
	CaseID           string        `json:"caseId"`
	RequestID        string        `json:"requestId,omitempty"`
	BaseRevision     int           `json:"baseRevision"`
	ExpectedRevision int           `json:"expectedRevision"`
	AppliedRevision  int           `json:"appliedRevision,omitempty"`
	Changes          []PointChange `json:"changes"`
	Problems         []RowProblem  `json:"problems"`
	Valid            bool          `json:"valid"`
	CommittedAt      *time.Time    `json:"committedAt,omitempty"`
}

type Assignment struct {
	ID, CaseID, PointID, Path, EquipmentID string
	Position                               string
}

type CoverageProblem struct {
	PointID, Path, EquipmentID, Code, Message string
}

type PathCoverage struct {
	PointID, Path, ShortfallEquipmentID  string
	RequiredLoadKg, CapacityKg, MarginKg float64
	CertificateValidUntil                time.Time
	EquipmentTypes, MissingTypes         []string
	Complete                             bool
}

type CoverageMatrix struct {
	CaseID, Status string
	Rows           []PathCoverage
	Problems       []CoverageProblem
}

type ScenarioAdjustment struct {
	PointID             string   `json:"pointId"`
	PlannedStaticLoadKg *float64 `json:"plannedStaticLoadKg,omitempty"`
	SlingAngleDegrees   *float64 `json:"slingAngleDegrees,omitempty"`
	DynamicFactor       *float64 `json:"dynamicFactor,omitempty"`
	LoadSharePercent    *float64 `json:"loadSharePercent,omitempty"`
}

type ScenarioComparison struct {
	PointID                 string
	FormalEffectiveLoadKg   float64
	ScenarioEffectiveLoadKg float64
	MarginDeltaPercent      float64
	Transition              string
}

type LoadScenario struct {
	ID, CaseID, InputDigest string
	BaseRevision            int
	Adjustments             []ScenarioAdjustment
	Result                  Evaluation
	Comparison              []ScenarioComparison
	CreatedBy               string
	CreatedAt               time.Time
	AdoptedAt               *time.Time
}

type Measurement struct {
	Value      float64   `json:"value"`
	MeasuredAt time.Time `json:"measuredAt"`
}

type Observation struct {
	ID, CaseID, PointID, Type, Description, SeverityBasis, SubmittedBy string
	Measurements                                                       []Measurement
	WorstValue                                                         float64
	Passed                                                             bool
	CreatedAt                                                          time.Time
}

type ChecklistItem struct {
	PointID, Type, Status, FindingID string
	WorstValue                       float64
}

type RehearsalProgress struct {
	CaseID                             string
	Total, Unchecked, Passed, Findings int
	Percent                            int
	Items                              []ChecklistItem
	BlockingReasons                    []string
}

type StructuredChange struct {
	Kind, TargetID, Field, Before, After string
}

type RetestItem struct {
	ID, Kind, TargetID, Reason, Status, PerformedBy, Evidence string
	PerformedAt                                               time.Time
}

type RemediationPlan struct {
	ID, CaseID, FindingID, ActionType, SubmittedBy string
	Changes                                        []StructuredChange
	Items                                          []RetestItem
	Round                                          int
	SubmittedAt                                    time.Time
	CompletedAt                                    *time.Time
}

type EvidenceItem struct {
	Category, ObjectID, Status, Message string
}

type ReviewReturnItem struct {
	ID, Category, ObjectID, Comment, Response, EvidenceRef string
	RespondedAt                                            *time.Time
}

type ReviewRound struct {
	ID, CaseID, ReviewerID, Decision, Reason string
	Number, Revision                         int
	Evidence                                 []EvidenceItem
	ReturnItems                              []ReviewReturnItem
	Contributors                             []string
	SubmittedAt                              time.Time
	DecidedAt                                *time.Time
}

type AuditRecord struct {
	Seq                                                      int64
	CaseID, Category, ObjectID, CredentialID, Actor, Message string
	Revision                                                 int
	CreatedAt                                                time.Time
}
