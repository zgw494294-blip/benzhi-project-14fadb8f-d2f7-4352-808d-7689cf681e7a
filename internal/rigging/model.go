package rigging

import (
	"errors"
	"fmt"
	"math"
	"time"
)

type Status string

const (
	StatusDraft       Status = "draft"
	StatusEvaluated   Status = "evaluated"
	StatusRehearsal   Status = "rehearsal"
	StatusRemediation Status = "remediation"
	StatusReview      Status = "review"
	StatusApproved    Status = "approved"
	StatusFrozen      Status = "frozen"
)

type Case struct {
	ID, ShowName, VenueZone                string
	PerformanceStartsAt, PerformanceEndsAt time.Time
	Status                                 Status
	Revision                               int
	AuthorID, EditLeaseID                  string
	CreatedAt, UpdatedAt                   time.Time
}
type Point struct {
	ID, CaseID, Label                                                                              string
	XMillimeters, YMillimeters, RatedLoadKg, PlannedStaticLoadKg, SlingAngleDegrees, DynamicFactor float64
	RedundancyGroup                                                                                string
}
type Equipment struct {
	ID, CaseID, EquipmentType, SerialNumber, CertificateRef, InspectionResult, InspectorID string
	RatedLoadKg                                                                            float64
	CertificateExpiresOn                                                                   time.Time
	InspectedAt                                                                            time.Time
}
type PointResult struct {
	PointID                                    string
	EffectiveLoadKg, CapacityKg, MarginPercent float64
	OverLimit                                  bool
}
type Evaluation struct {
	ID, CaseID, InputDigest string
	CaseRevision            int
	PointResults            []PointResult
	MinimumMarginPercent    float64
	OverLimitPointIDs       []string
	Outcome                 string
	EvaluatedAt             time.Time
}
type Finding struct {
	ID, CaseID, PointID, ObservationType, Severity, Description, Status string
	MeasuredValue                                                       float64
	OpenedAt, ClosedAt                                                  time.Time
}
type Remediation struct {
	ID, CaseID, FindingID, ActionType, BeforeSnapshot, AfterSnapshot, SubmittedBy string
	AffectedPointIDs, AffectedEquipmentIDs                                        []string
	RetestResult                                                                  string
	SubmittedAt                                                                   time.Time
}
type Review struct {
	ID, CaseID, ReviewerID, Decision, Reason string
	CreatedAt                                time.Time
}
type Credential struct {
	ID, CaseID, EvidenceDigest    string
	FrozenRevision                int
	ValidFrom, ValidUntil         time.Time
	Conditions                    []string
	IssuedBy, IssuedAt, RevokedAt string
}

func ValidateCase(c Case) error {
	if c.ShowName == "" || c.VenueZone == "" {
		return errors.New("演出名称和舞台区域不能为空")
	}
	if c.PerformanceEndsAt.Before(c.PerformanceStartsAt) {
		return errors.New("演出结束时间必须晚于开始时间")
	}
	return nil
}
func ValidatePoint(p Point) error {
	if p.Label == "" {
		return errors.New("吊点标签不能为空")
	}
	if math.IsNaN(p.RatedLoadKg) || math.IsInf(p.RatedLoadKg, 0) || p.RatedLoadKg <= 0 {
		return errors.New("额定承载必须为有效正数")
	}
	if math.IsNaN(p.PlannedStaticLoadKg) || math.IsInf(p.PlannedStaticLoadKg, 0) || p.PlannedStaticLoadKg <= 0 {
		return errors.New("预定静载必须为有效正数")
	}
	if math.IsNaN(p.SlingAngleDegrees) || math.IsInf(p.SlingAngleDegrees, 0) || p.SlingAngleDegrees <= 0 || p.SlingAngleDegrees >= 90 {
		return errors.New("吊索角度必须在0到90度之间")
	}
	if math.IsNaN(p.DynamicFactor) || math.IsInf(p.DynamicFactor, 0) || p.DynamicFactor < 1 {
		return errors.New("动态系数不能小于1")
	}
	return nil
}
func ValidateEquipment(e Equipment, now time.Time) error {
	if e.EquipmentType == "" || e.SerialNumber == "" || e.RatedLoadKg <= 0 {
		return errors.New("器材身份和额定载荷不能为空")
	}
	if e.CertificateRef == "" {
		return errors.New("缺少合格证")
	}
	if e.CertificateExpiresOn.Before(now) {
		return fmt.Errorf("器材%s合格证已过期", e.SerialNumber)
	}
	if e.InspectionResult != "合格" {
		return errors.New("现场检查未通过")
	}
	return nil
}
func Evaluate(points []Point) Evaluation {
	e := Evaluation{EvaluatedAt: time.Now(), Outcome: "通过", MinimumMarginPercent: math.MaxFloat64}
	anyFiniteMargin := false
	for _, p := range points {
		eff := p.PlannedStaticLoadKg * p.DynamicFactor / math.Sin(p.SlingAngleDegrees*math.Pi/180)
		margin := (p.RatedLoadKg - eff) / p.RatedLoadKg * 100
		invalid := math.IsNaN(eff) || math.IsInf(eff, 0) || math.IsNaN(margin) || math.IsInf(margin, 0)
		overLimit := invalid || margin < 0
		r := PointResult{PointID: p.ID, EffectiveLoadKg: eff, CapacityKg: p.RatedLoadKg, MarginPercent: margin, OverLimit: overLimit}
		e.PointResults = append(e.PointResults, r)
		if !invalid {
			anyFiniteMargin = true
			if margin < e.MinimumMarginPercent {
				e.MinimumMarginPercent = margin
			}
		}
		if overLimit {
			e.Outcome = "阻断"
			e.OverLimitPointIDs = append(e.OverLimitPointIDs, p.ID)
		}
	}
	if len(points) == 0 || !anyFiniteMargin {
		e.MinimumMarginPercent = 0
		e.Outcome = "阻断"
	}
	return e
}
func CanTransition(from, to Status, findings []Finding, eval Evaluation) error {
	switch to {
	case StatusEvaluated:
		if eval.Outcome != "通过" {
			return errors.New("载荷评估未通过")
		}
	case StatusRehearsal:
		if from != StatusEvaluated {
			return errors.New("请先完成载荷评估")
		}
	case StatusReview:
		for _, f := range findings {
			if f.Status != "closed" {
				return errors.New("存在未裁决彩排发现")
			}
		}
	case StatusApproved:
		if from != StatusReview {
			return errors.New("尚未进入复核")
		}
	case StatusFrozen:
		if from != StatusApproved {
			return errors.New("复核尚未批准")
		}
	default:
		return nil
	}
	return nil
}
