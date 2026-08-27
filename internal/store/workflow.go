package store

import (
	"encoding/json"
	"errors"
	"sort"
	"stage-rigging-release/internal/credential"
	"stage-rigging-release/internal/rigging"
	"strconv"
	"time"
)

func idemKey(caseID, action, requestID string) string {
	return caseID + "\x00" + action + "\x00" + requestID
}
func (s *Store) Idempotent(caseID, action, requestID string, out any) bool {
	if requestID == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, ok := s.idempotency[idemKey(caseID, action, requestID)]
	if ok {
		_ = json.Unmarshal(raw, out)
	}
	return ok
}
func (s *Store) rememberLocked(caseID, action, requestID string, v any) {
	if requestID == "" {
		return
	}
	raw, _ := json.Marshal(v)
	s.idempotency[idemKey(caseID, action, requestID)] = raw
}

func (s *Store) CommitPointBatch(caseID, leaseID, requestID string, baseRevision int, result rigging.BatchPointResult, actor string) (rigging.BatchPointResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if raw, ok := s.idempotency[idemKey(caseID, "points-batch", requestID)]; ok {
		var prior rigging.BatchPointResult
		_ = json.Unmarshal(raw, &prior)
		return prior, nil
	}
	c, ok := s.cases[caseID]
	if !ok {
		return result, errors.New("案卷不存在")
	}
	if c.Status != rigging.StatusDraft {
		return result, errors.New("只有草稿状态允许批量编修")
	}
	l, ok := s.leasing[caseID]
	if !ok || l.ID != leaseID || !l.Active(time.Now()) {
		return result, errors.New("编辑租约不存在或已过期")
	}
	if c.Revision != baseRevision {
		return result, errors.New("基础修订陈旧")
	}
	if requestID == "" {
		return result, errors.New("请求标识不能为空")
	}
	if !result.Valid {
		return result, errors.New("批次包含阻断问题")
	}
	points := append([]rigging.Point{}, s.points[caseID]...)
	index := map[string]int{}
	for i, p := range points {
		index[p.ID] = i
	}
	for _, change := range result.Changes {
		if i, exists := index[change.After.ID]; exists {
			points[i] = change.After
		} else {
			index[change.After.ID] = len(points)
			points = append(points, change.After)
		}
	}
	sort.Slice(points, func(i, j int) bool { return points[i].ID < points[j].ID })
	now := time.Now()
	c.Revision++
	c.UpdatedAt = now
	s.points[caseID] = points
	s.cases[caseID] = c
	result.AppliedRevision = c.Revision
	result.ExpectedRevision = c.Revision
	result.CommittedAt = &now
	s.rememberLocked(caseID, "points-batch", requestID, result)
	s.auditLocked(caseID, "points", "", "", actor, "原子提交吊点批次", c.Revision)
	if err := s.persistLocked(); err != nil {
		return result, err
	}
	return result, nil
}

func (s *Store) SetAssignments(caseID string, baseRevision int, assignments []rigging.Assignment, matrix rigging.CoverageMatrix, actor string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.cases[caseID]
	if !ok {
		return errors.New("案卷不存在")
	}
	if c.Revision != baseRevision {
		return errors.New("基础修订陈旧")
	}
	if len(matrix.Problems) > 0 {
		return errors.New("配装存在阻断原因")
	}
	s.assignments[caseID] = append([]rigging.Assignment{}, assignments...)
	c.Revision++
	c.UpdatedAt = time.Now()
	s.cases[caseID] = c
	s.auditLocked(caseID, "equipment", "", "", actor, "保存吊点配装关系", c.Revision)
	return s.persistLocked()
}
func (s *Store) Assignments(caseID string) []rigging.Assignment {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]rigging.Assignment{}, s.assignments[caseID]...)
}

func (s *Store) AddScenario(v rigging.LoadScenario) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scenarios[v.CaseID] = append(s.scenarios[v.CaseID], v)
	s.auditLocked(v.CaseID, "evaluation", v.ID, "", v.CreatedBy, "创建载荷试算情景", v.BaseRevision)
	return s.persistLocked()
}
func (s *Store) Scenarios(caseID string) []rigging.LoadScenario {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]rigging.LoadScenario{}, s.scenarios[caseID]...)
}
func (s *Store) Scenario(caseID, id string) (rigging.LoadScenario, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, v := range s.scenarios[caseID] {
		if v.ID == id {
			return v, true
		}
	}
	return rigging.LoadScenario{}, false
}
func (s *Store) AdoptScenario(caseID, id, requestID string, ev rigging.Evaluation, actor string) (rigging.Evaluation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if raw, ok := s.idempotency[idemKey(caseID, "scenario-adopt", requestID)]; ok {
		var prior rigging.Evaluation
		_ = json.Unmarshal(raw, &prior)
		return prior, nil
	}
	c := s.cases[caseID]
	var found bool
	for i := range s.scenarios[caseID] {
		if s.scenarios[caseID][i].ID == id {
			if s.scenarios[caseID][i].BaseRevision != c.Revision {
				return ev, errors.New("情景基础修订陈旧")
			}
			now := time.Now()
			s.scenarios[caseID][i].AdoptedAt = &now
			found = true
		}
	}
	if !found {
		return ev, errors.New("试算情景不存在")
	}
	c.Revision++
	c.Status = rigging.StatusEvaluated
	c.UpdatedAt = time.Now()
	ev.CaseID = caseID
	ev.CaseRevision = c.Revision
	s.evals[caseID] = ev
	s.cases[caseID] = c
	s.rememberLocked(caseID, "scenario-adopt", requestID, ev)
	s.auditLocked(caseID, "evaluation", ev.ID, "", actor, "采用试算并写入正式评估", c.Revision)
	return ev, s.persistLocked()
}

func (s *Store) AddObservation(o rigging.Observation, f *rigging.Finding) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.observations[o.CaseID] = append(s.observations[o.CaseID], o)
	if f != nil {
		s.findings[o.CaseID] = append(s.findings[o.CaseID], *f)
	}
	c := s.cases[o.CaseID]
	c.Status = rigging.StatusRehearsal
	c.Revision++
	c.UpdatedAt = time.Now()
	s.cases[o.CaseID] = c
	s.auditLocked(o.CaseID, "rehearsal", o.ID, "", o.SubmittedBy, "提交彩排清单观察", c.Revision)
	return s.persistLocked()
}
func (s *Store) Observations(caseID string) []rigging.Observation {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]rigging.Observation{}, s.observations[caseID]...)
}

func (s *Store) AddPlan(p rigging.RemediationPlan, actor string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, ch := range p.Changes {
		for i := range s.points[p.CaseID] {
			if s.points[p.CaseID][i].ID != ch.TargetID {
				continue
			}
			v, err := strconv.ParseFloat(ch.After, 64)
			if err == nil {
				switch ch.Field {
				case "plannedStaticLoadKg":
					s.points[p.CaseID][i].PlannedStaticLoadKg = v
				case "slingAngleDegrees":
					s.points[p.CaseID][i].SlingAngleDegrees = v
				case "dynamicFactor":
					s.points[p.CaseID][i].DynamicFactor = v
				}
			}
		}
		if p.ActionType == "器材替换" {
			for i := range s.assignments[p.CaseID] {
				if (s.assignments[p.CaseID][i].PointID == ch.TargetID && s.assignments[p.CaseID][i].EquipmentID == ch.Before) || s.assignments[p.CaseID][i].EquipmentID == ch.TargetID {
					s.assignments[p.CaseID][i].EquipmentID = ch.After
				}
			}
		}
	}
	s.plans[p.CaseID] = append(s.plans[p.CaseID], p)
	c := s.cases[p.CaseID]
	c.Status = rigging.StatusRemediation
	c.Revision++
	c.UpdatedAt = time.Now()
	s.cases[p.CaseID] = c
	delete(s.evals, p.CaseID)
	s.auditLocked(p.CaseID, "remediation", p.ID, "", actor, "提交结构化整改并生成定向复验", c.Revision)
	return s.persistLocked()
}
func (s *Store) Plans(caseID string) []rigging.RemediationPlan {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]rigging.RemediationPlan{}, s.plans[caseID]...)
}
func (s *Store) PutRetest(caseID, planID string, item rigging.RetestItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for pi := range s.plans[caseID] {
		if s.plans[caseID][pi].ID != planID {
			continue
		}
		for ii := range s.plans[caseID][pi].Items {
			if s.plans[caseID][pi].Items[ii].ID == item.ID {
				s.plans[caseID][pi].Items[ii] = item
				s.auditLocked(caseID, "remediation", item.ID, "", item.PerformedBy, "提交定向复验结果", s.cases[caseID].Revision)
				return s.persistLocked()
			}
		}
	}
	return errors.New("复验项不存在")
}
func (s *Store) CompletePlan(caseID, planID, findingID string, when time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.plans[caseID] {
		if s.plans[caseID][i].ID == planID {
			s.plans[caseID][i].CompletedAt = &when
		}
	}
	for i := range s.findings[caseID] {
		if s.findings[caseID][i].ID == findingID {
			s.findings[caseID][i].Status = "closed"
			s.findings[caseID][i].ClosedAt = when
		}
	}
	s.auditLocked(caseID, "remediation", planID, "", "", "全部定向复验通过，关闭原发现", s.cases[caseID].Revision)
	return s.persistLocked()
}

func (s *Store) AddReviewRound(r rigging.ReviewRound) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reviewRounds[r.CaseID] = append(s.reviewRounds[r.CaseID], r)
	c := s.cases[r.CaseID]
	c.Status = rigging.StatusReview
	s.cases[r.CaseID] = c
	s.auditLocked(r.CaseID, "review", r.ID, "", r.ReviewerID, "提交独立复核轮次", c.Revision)
	return s.persistLocked()
}
func (s *Store) ReviewRounds(caseID string) []rigging.ReviewRound {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]rigging.ReviewRound{}, s.reviewRounds[caseID]...)
}
func (s *Store) UpdateReviewRound(caseID string, round rigging.ReviewRound, newStatus rigging.Status) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.reviewRounds[caseID] {
		if s.reviewRounds[caseID][i].ID == round.ID {
			s.reviewRounds[caseID][i] = round
			c := s.cases[caseID]
			c.Status = newStatus
			if newStatus == rigging.StatusDraft {
				c.Revision++
			}
			c.UpdatedAt = time.Now()
			s.cases[caseID] = c
			s.auditLocked(caseID, "review", round.ID, "", round.ReviewerID, "记录复核决定："+round.Decision, c.Revision)
			return s.persistLocked()
		}
	}
	return errors.New("复核轮次不存在")
}

func (s *Store) FreezeCredential(caseID string, c credential.Credential, e credential.Evidence) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.creds[c.ID] = c
	s.caseCreds[caseID] = append(s.caseCreds[caseID], c.ID)
	s.frozen[c.ID] = e
	cs := s.cases[caseID]
	cs.Status = rigging.StatusFrozen
	cs.Revision++
	cs.UpdatedAt = time.Now()
	s.cases[caseID] = cs
	s.auditLocked(caseID, "credential", c.ID, c.ID, c.IssuedBy, "冻结证据并签发凭据", cs.Revision)
	return s.persistLocked()
}
func (s *Store) FrozenEvidence(credentialID string) (credential.Evidence, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.frozen[credentialID]
	return e, ok
}
func (s *Store) Credentials(caseID string) []credential.Credential {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []credential.Credential{}
	for _, id := range s.caseCreds[caseID] {
		out = append(out, s.creds[id])
	}
	return out
}
func (s *Store) RevokeCredential(id, actor, reason, requestID string) (credential.Credential, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.creds[id]
	if !ok {
		return c, errors.New("凭据不存在")
	}
	if raw, ok := s.idempotency[idemKey(c.CaseID, "credential-revoke", requestID)]; ok {
		var prior credential.Credential
		_ = json.Unmarshal(raw, &prior)
		return prior, nil
	}
	if c.RevokedAt != nil {
		return c, nil
	}
	now := time.Now()
	c.RevokedAt = &now
	c.RevokedBy = actor
	c.RevocationReason = reason
	s.creds[id] = c
	cs := s.cases[c.CaseID]
	cs.Status = rigging.StatusDraft
	cs.Revision++
	cs.UpdatedAt = now
	s.cases[c.CaseID] = cs
	s.rememberLocked(c.CaseID, "credential-revoke", requestID, c)
	s.auditLocked(c.CaseID, "credential", id, id, actor, "撤销凭据："+reason, cs.Revision)
	return c, s.persistLocked()
}

func (s *Store) AuditRecords(caseID, category, credentialID string) []rigging.AuditRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []rigging.AuditRecord{}
	for _, a := range s.auditRecords[caseID] {
		if category != "" && a.Category != category {
			continue
		}
		if credentialID != "" && a.CredentialID != credentialID {
			continue
		}
		out = append(out, a)
	}
	return out
}
func (s *Store) RecordVerification(caseID, credentialID, status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.auditLocked(caseID, "verification", credentialID, credentialID, "", "验真结论："+status, s.cases[caseID].Revision)
	_ = s.persistLocked()
}
