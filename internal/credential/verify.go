package credential

import "time"

func IsWithin(c Credential, now time.Time) bool {
	return !now.Before(c.ValidFrom) && !now.After(c.ValidUntil) && c.RevokedAt == nil
}
