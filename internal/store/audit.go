package store

import "time"

type AuditEvent struct {
	Seq             int64
	CaseID, Message string
	CreatedAt       time.Time
}

func (s *Store) RecordAudit(caseID, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.auditLocked(caseID, "case", "", "", "", message, s.cases[caseID].Revision)
	_ = s.persistLocked()
}
