package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	_ "modernc.org/sqlite"
	"stage-rigging-release/internal/credential"
	"stage-rigging-release/internal/rigging"
	"sync"
	"time"
)

type Store struct {
	db           *sql.DB
	mu           sync.Mutex
	cases        map[string]rigging.Case
	points       map[string][]rigging.Point
	equipment    map[string][]rigging.Equipment
	evals        map[string]rigging.Evaluation
	findings     map[string][]rigging.Finding
	rems         map[string][]rigging.Remediation
	reviews      map[string]rigging.Review
	reviewRounds map[string][]rigging.ReviewRound
	assignments  map[string][]rigging.Assignment
	scenarios    map[string][]rigging.LoadScenario
	observations map[string][]rigging.Observation
	plans        map[string][]rigging.RemediationPlan
	creds        map[string]credential.Credential
	caseCreds    map[string][]string
	frozen       map[string]credential.Evidence
	idempotency  map[string]json.RawMessage
	leasing      map[string]rigging.Lease
	audit        map[string][]string
	auditRecords map[string][]rigging.AuditRecord
	nextAuditSeq int64
}

type diskState struct {
	Cases           map[string]rigging.Case              `json:"cases"`
	Points          map[string][]rigging.Point           `json:"points"`
	Equipment       map[string][]rigging.Equipment       `json:"equipment"`
	Evaluations     map[string]rigging.Evaluation        `json:"evaluations"`
	Findings        map[string][]rigging.Finding         `json:"findings"`
	Remediations    map[string][]rigging.Remediation     `json:"remediations"`
	Reviews         map[string]rigging.Review            `json:"reviews"`
	ReviewRounds    map[string][]rigging.ReviewRound     `json:"reviewRounds"`
	Assignments     map[string][]rigging.Assignment      `json:"assignments"`
	Scenarios       map[string][]rigging.LoadScenario    `json:"scenarios"`
	Observations    map[string][]rigging.Observation     `json:"observations"`
	Plans           map[string][]rigging.RemediationPlan `json:"plans"`
	Credentials     map[string]credential.Credential     `json:"credentials"`
	CaseCredentials map[string][]string                  `json:"caseCredentials"`
	Frozen          map[string]credential.Evidence       `json:"frozen"`
	Idempotency     map[string]json.RawMessage           `json:"idempotency"`
	Leasing         map[string]rigging.Lease             `json:"leasing"`
	Audit           map[string][]string                  `json:"audit"`
	AuditRecords    map[string][]rigging.AuditRecord     `json:"auditRecords"`
	NextAuditSeq    int64                                `json:"nextAuditSeq"`
}

func Open(path string) (*Store, error) {
	db, e := sql.Open("sqlite", path)
	if e != nil {
		return nil, e
	}
	if _, e = db.Exec("PRAGMA journal_mode=WAL"); e != nil {
		return nil, e
	}
	if _, e = db.Exec(`CREATE TABLE IF NOT EXISTS audit_events (seq INTEGER PRIMARY KEY AUTOINCREMENT, case_id TEXT NOT NULL, message TEXT NOT NULL, created_at TEXT NOT NULL); CREATE TABLE IF NOT EXISTS app_state (id INTEGER PRIMARY KEY CHECK (id=1), data BLOB NOT NULL); PRAGMA user_version=1`); e != nil {
		return nil, e
	}
	if e = migrate(db); e != nil {
		return nil, e
	}
	s := &Store{db: db}
	s.initMaps()
	var raw []byte
	if err := db.QueryRow(`SELECT data FROM app_state WHERE id=1`).Scan(&raw); err == nil {
		var state diskState
		if err := json.Unmarshal(raw, &state); err != nil {
			db.Close()
			return nil, err
		}
		s.restore(state)
	} else if !errors.Is(err, sql.ErrNoRows) {
		db.Close()
		return nil, err
	}
	return s, nil
}
func (s *Store) Close() error { return s.db.Close() }

func (s *Store) initMaps() {
	s.cases = map[string]rigging.Case{}
	s.points = map[string][]rigging.Point{}
	s.equipment = map[string][]rigging.Equipment{}
	s.evals = map[string]rigging.Evaluation{}
	s.findings = map[string][]rigging.Finding{}
	s.rems = map[string][]rigging.Remediation{}
	s.reviews = map[string]rigging.Review{}
	s.reviewRounds = map[string][]rigging.ReviewRound{}
	s.assignments = map[string][]rigging.Assignment{}
	s.scenarios = map[string][]rigging.LoadScenario{}
	s.observations = map[string][]rigging.Observation{}
	s.plans = map[string][]rigging.RemediationPlan{}
	s.creds = map[string]credential.Credential{}
	s.caseCreds = map[string][]string{}
	s.frozen = map[string]credential.Evidence{}
	s.idempotency = map[string]json.RawMessage{}
	s.leasing = map[string]rigging.Lease{}
	s.audit = map[string][]string{}
	s.auditRecords = map[string][]rigging.AuditRecord{}
}
func (s *Store) restore(d diskState) {
	if d.Cases != nil {
		s.cases = d.Cases
	}
	if d.Points != nil {
		s.points = d.Points
	}
	if d.Equipment != nil {
		s.equipment = d.Equipment
	}
	if d.Evaluations != nil {
		s.evals = d.Evaluations
	}
	if d.Findings != nil {
		s.findings = d.Findings
	}
	if d.Remediations != nil {
		s.rems = d.Remediations
	}
	if d.Reviews != nil {
		s.reviews = d.Reviews
	}
	if d.ReviewRounds != nil {
		s.reviewRounds = d.ReviewRounds
	}
	if d.Assignments != nil {
		s.assignments = d.Assignments
	}
	if d.Scenarios != nil {
		s.scenarios = d.Scenarios
	}
	if d.Observations != nil {
		s.observations = d.Observations
	}
	if d.Plans != nil {
		s.plans = d.Plans
	}
	if d.Credentials != nil {
		s.creds = d.Credentials
	}
	if d.CaseCredentials != nil {
		s.caseCreds = d.CaseCredentials
	}
	if d.Frozen != nil {
		s.frozen = d.Frozen
	}
	if d.Idempotency != nil {
		s.idempotency = d.Idempotency
	}
	if d.Leasing != nil {
		s.leasing = d.Leasing
	}
	if d.Audit != nil {
		s.audit = d.Audit
	}
	if d.AuditRecords != nil {
		s.auditRecords = d.AuditRecords
	}
	s.nextAuditSeq = d.NextAuditSeq
}
func (s *Store) persistLocked() error {
	return s.persistLockedContext(context.Background())
}
func (s *Store) persistLockedContext(ctx context.Context) error {
	d := diskState{Cases: s.cases, Points: s.points, Equipment: s.equipment, Evaluations: s.evals, Findings: s.findings, Remediations: s.rems, Reviews: s.reviews, ReviewRounds: s.reviewRounds, Assignments: s.assignments, Scenarios: s.scenarios, Observations: s.observations, Plans: s.plans, Credentials: s.creds, CaseCredentials: s.caseCreds, Frozen: s.frozen, Idempotency: s.idempotency, Leasing: s.leasing, Audit: s.audit, AuditRecords: s.auditRecords, NextAuditSeq: s.nextAuditSeq}
	b, err := json.Marshal(d)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO app_state(id,data) VALUES(1,?) ON CONFLICT(id) DO UPDATE SET data=excluded.data`, b)
	return err
}
func (s *Store) auditLocked(caseID, category, objectID, credentialID, actor, message string, revision int) {
	s.nextAuditSeq++
	now := time.Now()
	s.audit[caseID] = append(s.audit[caseID], message)
	s.auditRecords[caseID] = append(s.auditRecords[caseID], rigging.AuditRecord{Seq: s.nextAuditSeq, CaseID: caseID, Category: category, ObjectID: objectID, CredentialID: credentialID, Actor: actor, Message: message, Revision: revision, CreatedAt: now})
}
func (s *Store) PutCase(c rigging.Case) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cases[c.ID] = c
	s.auditLocked(c.ID, "case", c.ID, "", "", "案卷已更新", c.Revision)
	_ = s.persistLocked()
}
func (s *Store) GetCase(id string) (rigging.Case, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.cases[id]
	return c, ok
}
func (s *Store) AddPoint(p rigging.Point) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.points[p.CaseID] = append(s.points[p.CaseID], p)
	s.auditLocked(p.CaseID, "points", p.ID, "", "", "登记吊点 "+p.Label, s.cases[p.CaseID].Revision)
	_ = s.persistLocked()
}
func (s *Store) Points(id string) []rigging.Point {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]rigging.Point{}, s.points[id]...)
}
func (s *Store) AddEquipment(e rigging.Equipment) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.equipment[e.CaseID] = append(s.equipment[e.CaseID], e)
	s.auditLocked(e.CaseID, "equipment", e.ID, "", "", "核验器材 "+e.SerialNumber, s.cases[e.CaseID].Revision)
	_ = s.persistLocked()
}
func (s *Store) Equipment(id string) []rigging.Equipment {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]rigging.Equipment{}, s.equipment[id]...)
}
func (s *Store) PutEvaluation(e rigging.Evaluation) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evals[e.CaseID] = e
	s.auditLocked(e.CaseID, "evaluation", e.ID, "", "", "完成载荷评估", e.CaseRevision)
	_ = s.persistLocked()
}
func (s *Store) Evaluation(id string) (rigging.Evaluation, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.evals[id]
	return e, ok
}
func (s *Store) AddFinding(f rigging.Finding) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.findings[f.CaseID] = append(s.findings[f.CaseID], f)
	s.auditLocked(f.CaseID, "rehearsal", f.ID, "", "", "记录彩排观察", 0)
	_ = s.persistLocked()
}
func (s *Store) Findings(id string) []rigging.Finding {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]rigging.Finding{}, s.findings[id]...)
}
func (s *Store) UpdateFinding(caseID, id, status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.findings[caseID] {
		if s.findings[caseID][i].ID == id {
			s.findings[caseID][i].Status = status
			s.findings[caseID][i].ClosedAt = time.Now()
		}
	}
	_ = s.persistLocked()
}
func (s *Store) AddRemediation(r rigging.Remediation) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rems[r.CaseID] = append(s.rems[r.CaseID], r)
	s.auditLocked(r.CaseID, "remediation", r.ID, "", r.SubmittedBy, "提交整改 "+r.ActionType, 0)
	_ = s.persistLocked()
}
func (s *Store) Remediations(id string) []rigging.Remediation {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]rigging.Remediation{}, s.rems[id]...)
}
func (s *Store) PutReview(r rigging.Review) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reviews[r.CaseID] = r
	s.auditLocked(r.CaseID, "review", r.ID, "", r.ReviewerID, "完成独立复核", 0)
	_ = s.persistLocked()
}
func (s *Store) Review(id string) (rigging.Review, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.reviews[id]
	return r, ok
}
func (s *Store) PutCredential(id string, c credential.Credential) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.creds[c.ID] = c
	s.caseCreds[id] = append(s.caseCreds[id], c.ID)
	s.auditLocked(id, "credential", c.ID, c.ID, c.IssuedBy, "签发开演放行凭据", c.FrozenRevision)
	_ = s.persistLocked()
}
func (s *Store) Credential(id string) (credential.Credential, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.creds[id]
	if !ok {
		ids := s.caseCreds[id]
		if len(ids) > 0 {
			c, ok = s.creds[ids[len(ids)-1]]
		}
	}
	return c, ok
}
func (s *Store) Timeline(id string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string{}, s.audit[id]...)
}
func (s *Store) Snapshot(id string) (credential.Evidence, error) {
	c, ok := s.GetCase(id)
	if !ok {
		return credential.Evidence{}, errors.New("案卷不存在")
	}
	e, ok := s.Evaluation(id)
	if !ok {
		return credential.Evidence{}, errors.New("缺少载荷评估")
	}
	r, _ := s.Review(id)
	return credential.Evidence{Case: c, Points: s.Points(id), Equipment: s.Equipment(id), Evaluation: e, Findings: s.Findings(id), Remediations: s.Remediations(id), Review: r, Assignments: s.Assignments(id), Observations: s.Observations(id), RemediationPlans: s.Plans(id), ReviewRounds: s.ReviewRounds(id)}, nil
}
func (s *Store) Health(ctx context.Context) error { return s.db.PingContext(ctx) }
func marshal(v any) string                        { b, _ := json.Marshal(v); return string(b) }
