package app

import (
	"errors"
	"fmt"
	"stage-rigging-release/internal/credential"
	"stage-rigging-release/internal/rigging"
	"stage-rigging-release/internal/store"
	"time"
)

type App struct{ Store *store.Store }

func New(s *store.Store) *App { return &App{Store: s} }
func (a *App) CreateCase(show, zone string, start, end time.Time, author string) (rigging.Case, error) {
	c := rigging.Case{ID: fmt.Sprintf("case-%d", time.Now().UnixNano()), ShowName: show, VenueZone: zone, PerformanceStartsAt: start, PerformanceEndsAt: end, Status: rigging.StatusDraft, Revision: 1, AuthorID: author, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if e := rigging.ValidateCase(c); e != nil {
		return c, e
	}
	a.Store.PutCase(c)
	return c, nil
}
func (a *App) AddPoint(c rigging.Case, p rigging.Point) error {
	if p.CaseID == "" {
		p.CaseID = c.ID
	}
	if err := rigging.ValidatePoint(p); err != nil {
		return err
	}
	current, ok := a.Store.GetCase(c.ID)
	if !ok {
		return errors.New("案卷不存在")
	}
	a.Store.AddPoint(p)
	current.Revision++
	current.UpdatedAt = time.Now()
	a.Store.PutCase(current)
	return nil
}
func (a *App) AddEquipment(c rigging.Case, e rigging.Equipment) error {
	e.CaseID = c.ID
	if err := rigging.ValidateEquipment(e, time.Now()); err != nil {
		return err
	}
	current, ok := a.Store.GetCase(c.ID)
	if !ok {
		return errors.New("案卷不存在")
	}
	a.Store.AddEquipment(e)
	current.Revision++
	current.UpdatedAt = time.Now()
	a.Store.PutCase(current)
	return nil
}
func (a *App) Evaluate(c rigging.Case) (rigging.Evaluation, error) {
	ev, _, err := evaluateInputs(c, a.Store.Points(c.ID), a.Store.Equipment(c.ID), a.Store.Assignments(c.ID))
	if err != nil {
		return ev, err
	}
	current, ok := a.Store.GetCase(c.ID)
	if !ok {
		return ev, errors.New("案卷不存在")
	}
	ev.CaseID = c.ID
	ev.ID = fmt.Sprintf("eval-%d", time.Now().UnixNano())
	current.Status = rigging.StatusEvaluated
	current.Revision++
	ev.CaseRevision = current.Revision
	a.Store.PutEvaluation(ev)
	current.UpdatedAt = time.Now()
	a.Store.PutCase(current)
	return ev, nil
}
func (a *App) Observe(c rigging.Case, pointID, typ, desc string, value float64, pass bool) rigging.Finding {
	f := rigging.FindingFromObservation(c.ID, pointID, typ, desc, value, pass)
	a.Store.AddFinding(f)
	current, ok := a.Store.GetCase(c.ID)
	if !ok {
		return f
	}
	current.Status = rigging.StatusRehearsal
	current.Revision++
	current.UpdatedAt = time.Now()
	a.Store.PutCase(current)
	return f
}
func (a *App) Remediate(c rigging.Case, findingID, action string) error {
	fs := a.Store.Findings(c.ID)
	var f rigging.Finding
	for _, x := range fs {
		if x.ID == findingID {
			f = x
		}
	}
	if f.ID == "" {
		return errors.New("发现不存在")
	}
	r := rigging.RemediationScope(f, action, a.Store.Points(c.ID), a.Store.Equipment(c.ID))
	r.SubmittedBy = c.AuthorID
	r.RetestResult = "待复验"
	a.Store.AddRemediation(r)
	a.Store.UpdateFinding(c.ID, findingID, "closed")
	current, ok := a.Store.GetCase(c.ID)
	if !ok {
		return errors.New("案卷不存在")
	}
	current.Status = rigging.StatusRemediation
	current.Revision++
	current.UpdatedAt = time.Now()
	a.Store.PutCase(current)
	return nil
}
func (a *App) SubmitReview(c rigging.Case, reviewer, decision, reason string) error {
	if reviewer == c.AuthorID {
		return errors.New("复核员必须与编制人不同")
	}
	fs := a.Store.Findings(c.ID)
	ev, _ := a.Store.Evaluation(c.ID)
	if err := rigging.CanTransition(c.Status, rigging.StatusReview, fs, ev); err != nil {
		return err
	}
	r := rigging.Review{ID: fmt.Sprintf("review-%d", time.Now().UnixNano()), CaseID: c.ID, ReviewerID: reviewer, Decision: decision, Reason: reason, CreatedAt: time.Now()}
	a.Store.PutReview(r)
	current, ok := a.Store.GetCase(c.ID)
	if !ok {
		return errors.New("案卷不存在")
	}
	if decision == "批准" {
		current.Status = rigging.StatusApproved
	} else {
		current.Status = rigging.StatusDraft
	}
	current.Revision++
	current.UpdatedAt = time.Now()
	a.Store.PutCase(current)
	return nil
}
func (a *App) Freeze(c rigging.Case, issuer string) (credential.Credential, error) {
	if c.Status != rigging.StatusApproved {
		return credential.Credential{}, errors.New("案卷尚未批准")
	}
	ev, e := a.Store.Snapshot(c.ID)
	if e != nil {
		return credential.Credential{}, e
	}
	if c.PerformanceEndsAt.Before(c.PerformanceStartsAt) {
		return credential.Credential{}, errors.New("凭据有效时段与演出时段不一致")
	}
	current, ok := a.Store.GetCase(c.ID)
	if !ok {
		return credential.Credential{}, errors.New("案卷不存在")
	}
	ev.Case.Status = rigging.StatusFrozen
	ev.Case.Revision = current.Revision + 1
	ev.Case.UpdatedAt = time.Now()
	cr := credential.Issue(ev, issuer, c.PerformanceStartsAt, c.PerformanceEndsAt, []string{"按冻结方案执行", "开演前完成现场复核"})
	chain := a.Store.Credentials(c.ID)
	if len(chain) > 0 {
		prior := chain[len(chain)-1]
		if prior.RevokedAt == nil {
			return credential.Credential{}, errors.New("当前凭据仍有效，不能签发替代凭据")
		}
		cr.PreviousCredentialID = prior.ID
	}
	if err := a.Store.FreezeCredential(c.ID, cr, ev); err != nil {
		return credential.Credential{}, err
	}
	return cr, nil
}
func (a *App) Verify(id string) (string, bool, error) {
	d, err := a.DiagnoseCredential(id, time.Now())
	return d.Status, d.Valid, err
}
