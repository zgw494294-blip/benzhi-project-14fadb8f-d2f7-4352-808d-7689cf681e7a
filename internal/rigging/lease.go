package rigging

import "time"

type Lease struct {
	ID, CaseID, Holder string
	ExpiresAt          time.Time
}

func (l Lease) Active(now time.Time) bool { return l.ID != "" && now.Before(l.ExpiresAt) }
