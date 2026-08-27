package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"stage-rigging-release/internal/credential"
	"stage-rigging-release/internal/rigging"
	"strings"
	"time"
)

func (a *App) AcquireLease(caseID, holder string) (rigging.Lease, error) {
	c, ok := a.Store.GetCase(caseID)
	if !ok {
		return rigging.Lease{}, errors.New("案卷不存在")
	}
	if holder == "" {
		return rigging.Lease{}, errors.New("租约持有人不能为空")
	}
	l, ok := a.Store.AcquireLease(c, holder)
	if !ok {
		return l, errors.New("案卷正在由其他操作者编修")
	}
	return l, nil
}

func (a *App) PreflightPointBatch(caseID string, baseRevision int, points []rigging.Point) (rigging.BatchPointResult, error) {
	c, ok := a.Store.GetCase(caseID)
	if !ok {
		return rigging.BatchPointResult{}, errors.New("案卷不存在")
	}
	if c.Status != rigging.StatusDraft {
		return rigging.BatchPointResult{}, errors.New("只有草稿状态允许批量编修")
	}
	r := rigging.NormalizeAndValidatePoints(a.Store.Points(caseID), points, caseID, baseRevision)
	if c.Revision != baseRevision {
		r.Valid = false
		r.Problems = append(r.Problems, rigging.RowProblem{Row: -1, Field: "baseRevision", Code: "stale_revision", Message: fmt.Sprintf("当前修订为%d，请重新预检", c.Revision)})
	}
	return r, nil
}
func (a *App) CommitPointBatch(caseID, leaseID, requestID, actor string, baseRevision int, points []rigging.Point) (rigging.BatchPointResult, error) {
	var prior rigging.BatchPointResult
	if a.Store.Idempotent(caseID, "points-batch", requestID, &prior) {
		return prior, nil
	}
	r, err := a.PreflightPointBatch(caseID, baseRevision, points)
	if err != nil {
		return r, err
	}
	if !r.Valid {
		return r, errors.New("批次预检未通过")
	}
	r.RequestID = requestID
	return a.Store.CommitPointBatch(caseID, leaseID, requestID, baseRevision, r, actor)
}

func (a *App) Coverage(caseID string) (rigging.CoverageMatrix, error) {
	c, ok := a.Store.GetCase(caseID)
	if !ok {
		return rigging.CoverageMatrix{}, errors.New("案卷不存在")
	}
	return rigging.BuildCoverage(c, a.Store.Points(caseID), a.Store.Equipment(caseID), a.Store.Assignments(caseID)), nil
}
func (a *App) SaveAssignments(caseID, actor string, baseRevision int, as []rigging.Assignment) (rigging.CoverageMatrix, error) {
	c, ok := a.Store.GetCase(caseID)
	if !ok {
		return rigging.CoverageMatrix{}, errors.New("案卷不存在")
	}
	for i := range as {
		as[i].CaseID = caseID
		if as[i].ID == "" {
			as[i].ID = fmt.Sprintf("assignment-%d-%d", time.Now().UnixNano(), i)
		}
	}
	m := rigging.BuildCoverage(c, a.Store.Points(caseID), a.Store.Equipment(caseID), as)
	if len(m.Problems) > 0 {
		return m, errors.New("配装覆盖校验未通过")
	}
	return m, a.Store.SetAssignments(caseID, baseRevision, as, m, actor)
}

func evaluateInputs(c rigging.Case, points []rigging.Point, equipment []rigging.Equipment, as []rigging.Assignment) (rigging.Evaluation, rigging.CoverageMatrix, error) {
	for _, e := range equipment {
		if err := rigging.ValidateEquipment(e, c.PerformanceStartsAt); err != nil {
			return rigging.Evaluation{}, rigging.CoverageMatrix{}, err
		}
	}
	m := rigging.BuildCoverage(c, points, equipment, as)
	if m.Status != "完整覆盖" {
		return rigging.Evaluation{}, m, errors.New("器材与吊点配装存在阻断项")
	}
	ev := rigging.EvaluateWithCoverage(points, m)
	if ev.Outcome != "通过" {
		return ev, m, errors.New("载荷评估超限")
	}
	return ev, m, nil
}

func (a *App) CreateScenario(caseID, actor string, baseRevision int, adjustments []rigging.ScenarioAdjustment) (rigging.LoadScenario, error) {
	c, ok := a.Store.GetCase(caseID)
	if !ok {
		return rigging.LoadScenario{}, errors.New("案卷不存在")
	}
	if c.Revision != baseRevision {
		return rigging.LoadScenario{}, errors.New("情景基础修订陈旧")
	}
	points := a.Store.Points(caseID)
	byID := map[string]int{}
	for i, p := range points {
		byID[p.ID] = i
	}
	sort.Slice(adjustments, func(i, j int) bool { return adjustments[i].PointID < adjustments[j].PointID })
	for _, x := range adjustments {
		i, ok := byID[x.PointID]
		if !ok {
			return rigging.LoadScenario{}, fmt.Errorf("吊点%s不存在", x.PointID)
		}
		p := points[i]
		if x.PlannedStaticLoadKg != nil {
			p.PlannedStaticLoadKg = *x.PlannedStaticLoadKg
		}
		if x.SlingAngleDegrees != nil {
			p.SlingAngleDegrees = *x.SlingAngleDegrees
		}
		if x.DynamicFactor != nil {
			p.DynamicFactor = *x.DynamicFactor
		}
		if x.LoadSharePercent != nil {
			p.PlannedStaticLoadKg = p.PlannedStaticLoadKg * *x.LoadSharePercent / 100
		}
		if err := rigging.ValidatePoint(p); err != nil {
			return rigging.LoadScenario{}, fmt.Errorf("吊点%s：%w", p.ID, err)
		}
		points[i] = p
	}
	ev, m, err := evaluateInputs(c, points, a.Store.Equipment(caseID), a.Store.Assignments(caseID))
	if err != nil && m.Status != "完整覆盖" {
		return rigging.LoadScenario{}, err
	}
	formal, _ := a.Store.Evaluation(caseID)
	formalBy := map[string]rigging.PointResult{}
	for _, r := range formal.PointResults {
		formalBy[r.PointID] = r
	}
	cmp := []rigging.ScenarioComparison{}
	for _, r := range ev.PointResults {
		old := formalBy[r.PointID]
		transition := "保持"
		if old.OverLimit && !r.OverLimit {
			transition = "由超限转为通过"
		}
		if !old.OverLimit && r.OverLimit {
			transition = "由通过转为超限"
		}
		cmp = append(cmp, rigging.ScenarioComparison{PointID: r.PointID, FormalEffectiveLoadKg: old.EffectiveLoadKg, ScenarioEffectiveLoadKg: r.EffectiveLoadKg, MarginDeltaPercent: r.MarginPercent - old.MarginPercent, Transition: transition})
	}
	raw, _ := json.Marshal(struct {
		Revision    int
		Adjustments []rigging.ScenarioAdjustment
	}{baseRevision, adjustments})
	h := sha256.Sum256(raw)
	digest := hex.EncodeToString(h[:])
	ev.ID = "scenario-eval-" + digest[:12]
	ev.CaseID = caseID
	ev.CaseRevision = baseRevision
	s := rigging.LoadScenario{ID: "scenario-" + digest[:12], CaseID: caseID, InputDigest: digest, BaseRevision: baseRevision, Adjustments: adjustments, Result: ev, Comparison: cmp, CreatedBy: actor, CreatedAt: time.Now()}
	return s, a.Store.AddScenario(s)
}
func (a *App) AdoptScenario(caseID, scenarioID, requestID, actor string) (rigging.Evaluation, error) {
	if requestID == "" {
		return rigging.Evaluation{}, errors.New("请求标识不能为空")
	}
	s, ok := a.Store.Scenario(caseID, scenarioID)
	if !ok {
		return rigging.Evaluation{}, errors.New("试算情景不存在")
	}
	c, _ := a.Store.GetCase(caseID)
	if c.Revision != s.BaseRevision {
		return rigging.Evaluation{}, errors.New("情景基础修订陈旧")
	}
	m, _ := a.Coverage(caseID)
	if m.Status != "完整覆盖" {
		return rigging.Evaluation{}, errors.New("器材配装已不满足采用条件")
	}
	ev := s.Result
	ev.ID = fmt.Sprintf("eval-%d", time.Now().UnixNano())
	ev.EvaluatedAt = time.Now()
	return a.Store.AdoptScenario(caseID, scenarioID, requestID, ev, actor)
}

func (a *App) RecordObservation(caseID, actor string, o rigging.Observation) (rigging.Observation, *rigging.Finding, error) {
	c, ok := a.Store.GetCase(caseID)
	if !ok {
		return o, nil, errors.New("案卷不存在")
	}
	if _, ok := a.Store.Evaluation(caseID); !ok {
		return o, nil, errors.New("请先完成正式载荷评估")
	}
	found := false
	for _, p := range a.Store.Points(caseID) {
		if p.ID == o.PointID {
			found = true
		}
	}
	if !found {
		return o, nil, errors.New("吊点不存在")
	}
	o.ID = fmt.Sprintf("observation-%d", time.Now().UnixNano())
	o.CaseID = caseID
	o.SubmittedBy = actor
	o.CreatedAt = time.Now()
	judged, f, err := rigging.JudgeObservation(o)
	if err != nil {
		return o, nil, err
	}
	a.Store.AddObservation(judged, f)
	_ = c
	return judged, f, nil
}
func (a *App) RehearsalProgress(caseID string) (rigging.RehearsalProgress, error) {
	if _, ok := a.Store.GetCase(caseID); !ok {
		return rigging.RehearsalProgress{}, errors.New("案卷不存在")
	}
	return rigging.Checklist(caseID, a.Store.Points(caseID), a.Store.Observations(caseID), a.Store.Findings(caseID)), nil
}

func (a *App) CreateRemediation(caseID, findingID, action, actor string, changes []rigging.StructuredChange) (rigging.RemediationPlan, error) {
	var finding rigging.Finding
	for _, f := range a.Store.Findings(caseID) {
		if f.ID == findingID {
			finding = f
		}
	}
	if finding.ID == "" {
		return rigging.RemediationPlan{}, errors.New("发现不存在")
	}
	if finding.Status == "closed" {
		return rigging.RemediationPlan{}, errors.New("发现已经关闭")
	}
	if action != "器材替换" && action != "降载" && action != "几何调整" {
		return rigging.RemediationPlan{}, errors.New("未知整改类型")
	}
	if len(changes) == 0 {
		return rigging.RemediationPlan{}, errors.New("整改必须包含结构化前后差异")
	}
	validTarget := false
	for _, ch := range changes {
		if strings.TrimSpace(ch.Before) == strings.TrimSpace(ch.After) {
			return rigging.RemediationPlan{}, errors.New("整改前后没有实际变化")
		}
		if ch.TargetID == finding.PointID {
			validTarget = true
		}
		for _, e := range a.Store.Assignments(caseID) {
			if e.PointID == finding.PointID && e.EquipmentID == ch.TargetID {
				validTarget = true
			}
		}
	}
	if !validTarget {
		return rigging.RemediationPlan{}, errors.New("整改目标与发现无关或不存在")
	}
	round := 1
	for _, p := range a.Store.Plans(caseID) {
		if p.FindingID == findingID {
			round++
		}
	}
	id := fmt.Sprintf("remediation-%d", time.Now().UnixNano())
	items := []rigging.RetestItem{}
	if action == "器材替换" {
		for _, ch := range changes {
			if ch.Kind == "器材替换" || ch.TargetID != finding.PointID {
				items = append(items, rigging.RetestItem{ID: id + "-equipment", Kind: "器材复查", TargetID: ch.TargetID, Reason: "替换器材需要重新核验证书与现场状态", Status: "待执行"})
				break
			}
		}
	}
	items = append(items, rigging.RetestItem{ID: id + "-observation", Kind: "吊点观察", TargetID: finding.PointID, Reason: "发现吊点需要定向负载观察", Status: "待执行"}, rigging.RetestItem{ID: id + "-load", Kind: "载荷复算", TargetID: finding.PointID, Reason: "整改影响吊点载荷依赖", Status: "待执行"})
	p := rigging.RemediationPlan{ID: id, CaseID: caseID, FindingID: findingID, ActionType: action, SubmittedBy: actor, Changes: changes, Items: items, Round: round, SubmittedAt: time.Now()}
	return p, a.Store.AddPlan(p, actor)
}
func (a *App) RecordRetest(caseID, planID, itemID, actor, status, evidence string) (rigging.RemediationPlan, error) {
	if status != "通过" && status != "失败" {
		return rigging.RemediationPlan{}, errors.New("复验结论必须是通过或失败")
	}
	if strings.TrimSpace(evidence) == "" {
		return rigging.RemediationPlan{}, errors.New("复验必须填写证据说明")
	}
	var plan rigging.RemediationPlan
	for _, p := range a.Store.Plans(caseID) {
		if p.ID == planID {
			plan = p
		}
	}
	if plan.ID == "" {
		return plan, errors.New("整改不存在")
	}
	found := false
	for i := range plan.Items {
		if plan.Items[i].ID == itemID {
			if plan.Items[i].Kind == "载荷复算" && status == "通过" {
				if _, ok := a.Store.Evaluation(caseID); !ok {
					return plan, errors.New("相关正式载荷尚未重算")
				}
			}
			found = true
			plan.Items[i].Status = status
			plan.Items[i].PerformedBy = actor
			plan.Items[i].Evidence = evidence
			plan.Items[i].PerformedAt = time.Now()
			if err := a.Store.PutRetest(caseID, planID, plan.Items[i]); err != nil {
				return plan, err
			}
		}
	}
	if !found {
		return plan, errors.New("复验项不存在")
	}
	all := true
	for _, x := range plan.Items {
		if x.Status != "通过" {
			all = false
		}
	}
	if all {
		now := time.Now()
		plan.CompletedAt = &now
		if err := a.Store.CompletePlan(caseID, planID, plan.FindingID, now); err != nil {
			return plan, err
		}
	}
	return plan, nil
}

func (a *App) EvidenceChecklist(caseID string) []rigging.EvidenceItem {
	items := []rigging.EvidenceItem{}
	m, _ := a.Coverage(caseID)
	status := "完整"
	msg := "器材覆盖完整"
	if m.Status != "完整覆盖" {
		status = "缺失"
		msg = "器材覆盖存在阻断"
	}
	items = append(items, rigging.EvidenceItem{Category: "器材覆盖", ObjectID: caseID, Status: status, Message: msg})
	_, ok := a.Store.Evaluation(caseID)
	status = "完整"
	if !ok {
		status = "缺失"
	}
	items = append(items, rigging.EvidenceItem{Category: "正式评估", ObjectID: caseID, Status: status})
	p, _ := a.RehearsalProgress(caseID)
	status = "完整"
	if p.Unchecked > 0 {
		status = "缺失"
	}
	items = append(items, rigging.EvidenceItem{Category: "彩排清单", ObjectID: caseID, Status: status, Message: strings.Join(p.BlockingReasons, "；")})
	open := rigging.OpenFindings(a.Store.Findings(caseID))
	status = "完整"
	if len(open) > 0 {
		status = "缺失"
	}
	items = append(items, rigging.EvidenceItem{Category: "发现闭环", ObjectID: caseID, Status: status})
	for _, plan := range a.Store.Plans(caseID) {
		for _, it := range plan.Items {
			if it.Status != "通过" {
				items = append(items, rigging.EvidenceItem{Category: "整改复验", ObjectID: it.ID, Status: "缺失", Message: it.Reason})
			}
		}
	}
	return items
}
func (a *App) SubmitReviewRound(caseID, reviewer string) (rigging.ReviewRound, error) {
	c, ok := a.Store.GetCase(caseID)
	if !ok {
		return rigging.ReviewRound{}, errors.New("案卷不存在")
	}
	evidence := a.EvidenceChecklist(caseID)
	for _, e := range evidence {
		if e.Status != "完整" {
			return rigging.ReviewRound{}, fmt.Errorf("证据清单不完整：%s %s", e.Category, e.ObjectID)
		}
	}
	contributors := []string{c.AuthorID}
	for _, p := range a.Store.Plans(caseID) {
		if p.SubmittedBy != "" {
			contributors = append(contributors, p.SubmittedBy)
		}
	}
	for _, audit := range a.Store.AuditRecords(caseID, "", "") {
		if audit.Revision == c.Revision && audit.Actor != "" {
			contributors = append(contributors, audit.Actor)
		}
	}
	for _, x := range contributors {
		if reviewer == x {
			return rigging.ReviewRound{}, fmt.Errorf("职责冲突：复核员%s参与了当前修订编修或整改", reviewer)
		}
	}
	rounds := a.Store.ReviewRounds(caseID)
	if len(rounds) > 0 {
		last := rounds[len(rounds)-1]
		if last.Decision == "退回" {
			for _, it := range last.ReturnItems {
				if it.Response == "" || it.EvidenceRef == "" {
					return rigging.ReviewRound{}, errors.New("上一轮退回意见尚未全部响应")
				}
			}
		}
	}
	r := rigging.ReviewRound{ID: fmt.Sprintf("review-round-%d", len(rounds)+1), CaseID: caseID, ReviewerID: reviewer, Number: len(rounds) + 1, Revision: c.Revision, Evidence: evidence, Contributors: contributors, SubmittedAt: time.Now()}
	return r, a.Store.AddReviewRound(r)
}
func (a *App) DecideReview(caseID, reviewID, reviewer, decision, reason string, returns []rigging.ReviewReturnItem) (rigging.ReviewRound, error) {
	rounds := a.Store.ReviewRounds(caseID)
	if len(rounds) == 0 {
		return rigging.ReviewRound{}, errors.New("没有待决定复核轮次")
	}
	r := rounds[len(rounds)-1]
	if r.ID != reviewID {
		return r, errors.New("只能决定最新复核轮次")
	}
	if r.ReviewerID != reviewer {
		return r, errors.New("只能由该轮复核员提交决定")
	}
	if decision != "批准" && decision != "退回" {
		return r, errors.New("复核决定必须是批准或退回")
	}
	if decision == "退回" && len(returns) == 0 {
		return r, errors.New("退回至少需要一条具体意见")
	}
	for i := range returns {
		if returns[i].Category == "" || returns[i].ObjectID == "" || returns[i].Comment == "" {
			return r, errors.New("退回意见必须指向具体证据类别和对象")
		}
		if returns[i].ID == "" {
			returns[i].ID = fmt.Sprintf("return-%d-%d", r.Number, i+1)
		}
	}
	now := time.Now()
	r.Decision = decision
	r.Reason = reason
	r.ReturnItems = returns
	r.DecidedAt = &now
	status := rigging.StatusApproved
	if decision == "退回" {
		status = rigging.StatusDraft
	}
	return r, a.Store.UpdateReviewRound(caseID, r, status)
}
func (a *App) RespondReview(caseID, reviewID, itemID, response, evidence string) (rigging.ReviewRound, error) {
	rounds := a.Store.ReviewRounds(caseID)
	var r rigging.ReviewRound
	for _, x := range rounds {
		if x.ID == reviewID {
			r = x
		}
	}
	if r.ID == "" {
		return r, errors.New("复核轮次不存在")
	}
	found := false
	for i := range r.ReturnItems {
		if r.ReturnItems[i].ID == itemID {
			if response == "" || evidence == "" {
				return r, errors.New("响应和新修订证据不能为空")
			}
			now := time.Now()
			r.ReturnItems[i].Response = response
			r.ReturnItems[i].EvidenceRef = evidence
			r.ReturnItems[i].RespondedAt = &now
			found = true
		}
	}
	if !found {
		return r, errors.New("退回项不存在")
	}
	return r, a.Store.UpdateReviewRound(caseID, r, rigging.StatusDraft)
}

func (a *App) RevokeCredential(id, actor, role, reason, requestID string) (credential.Credential, error) {
	if role != "演出技术负责人" {
		return credential.Credential{}, errors.New("只有演出技术负责人可以撤销凭据")
	}
	if strings.TrimSpace(reason) == "" {
		return credential.Credential{}, errors.New("撤销原因不能为空")
	}
	if requestID == "" {
		return credential.Credential{}, errors.New("请求标识不能为空")
	}
	return a.Store.RevokeCredential(id, actor, reason, requestID)
}
func (a *App) DiagnoseCredential(id string, now time.Time) (credential.Diagnostic, error) {
	if !strings.HasPrefix(id, "RC-") {
		return credential.Diagnostic{CredentialID: id, Status: "格式错误", CheckedAt: now}, errors.New("凭据格式错误")
	}
	cr, ok := a.Store.Credential(id)
	if !ok {
		return credential.Diagnostic{CredentialID: id, Status: "凭据不存在", CheckedAt: now}, nil
	}
	if _, ok := a.Store.GetCase(cr.CaseID); !ok {
		return credential.Diagnostic{CredentialID: id, CaseID: cr.CaseID, Status: "案卷不存在", CheckedAt: now}, nil
	}
	if cr.RevokedAt != nil {
		currentCase, _ := a.Store.GetCase(cr.CaseID)
		d := credential.Diagnose(cr, credential.Evidence{Case: currentCase}, nil, now)
		a.Store.RecordVerification(cr.CaseID, id, d.Status)
		return d, nil
	}
	if _, ok := a.Store.FrozenEvidence(id); !ok {
		return credential.Diagnostic{CredentialID: id, CaseID: cr.CaseID, Status: "冻结证据不存在", CheckedAt: now}, nil
	}
	current, err := a.Store.Snapshot(cr.CaseID)
	if err != nil {
		return credential.Diagnostic{}, err
	}
	d := credential.Diagnose(cr, current, a.Store.AuditRecords(cr.CaseID, "", ""), now)
	a.Store.RecordVerification(cr.CaseID, id, d.Status)
	return d, nil
}
