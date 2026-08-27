package rigging

import "time"

func NewCase(id, show, zone, author string, start, end time.Time) Case {
	return Case{ID: id, ShowName: show, VenueZone: zone, AuthorID: author, PerformanceStartsAt: start, PerformanceEndsAt: end, Status: StatusDraft, Revision: 1, CreatedAt: time.Now(), UpdatedAt: time.Now()}
}
