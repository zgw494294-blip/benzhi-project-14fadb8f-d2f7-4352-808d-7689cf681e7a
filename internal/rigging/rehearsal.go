package rigging

import (
	"fmt"
	"math"
	"sort"
	"time"
)

func JudgeObservation(o Observation) (Observation, *Finding, error) {
	if o.PointID == "" {
		return o, nil, fmt.Errorf("吊点不能为空")
	}
	if o.Type != "位移" && o.Type != "异响" && o.Type != "制动" && o.Type != "复位" {
		return o, nil, fmt.Errorf("未知检查项目")
	}
	if (o.Type == "位移" || o.Type == "复位") && len(o.Measurements) == 0 {
		return o, nil, fmt.Errorf("测量项目至少需要一个测量值")
	}
	if (o.Type == "异响" || o.Type == "制动") && (o.Description == "" || o.SeverityBasis == "") {
		return o, nil, fmt.Errorf("定性观察必须填写描述和严重程度依据")
	}
	o.WorstValue = 0
	for i := range o.Measurements {
		if o.Measurements[i].MeasuredAt.IsZero() {
			o.Measurements[i].MeasuredAt = time.Now()
		}
		if math.Abs(o.Measurements[i].Value) > math.Abs(o.WorstValue) {
			o.WorstValue = o.Measurements[i].Value
		}
	}
	o.Passed = true
	severity := ""
	if o.Type == "位移" || o.Type == "复位" {
		if math.Abs(o.WorstValue) > 5 {
			o.Passed = false
			severity = "high"
		} else if math.Abs(o.WorstValue) > 2 {
			o.Passed = false
			severity = "medium"
		}
	}
	if o.Type == "异响" && o.Description != "无" {
		o.Passed = false
		severity = "medium"
	}
	if o.Type == "制动" && o.Description != "正常" && o.Description != "合格" {
		o.Passed = false
		severity = "high"
	}
	if o.Passed {
		return o, nil, nil
	}
	f := &Finding{ID: fmt.Sprintf("finding-%d", time.Now().UnixNano()), CaseID: o.CaseID, PointID: o.PointID, ObservationType: o.Type, Severity: severity, Description: o.Description, MeasuredValue: o.WorstValue, Status: "open", OpenedAt: time.Now()}
	return o, f, nil
}

func Checklist(caseID string, points []Point, observations []Observation, findings []Finding) RehearsalProgress {
	p := RehearsalProgress{CaseID: caseID, Items: []ChecklistItem{}, BlockingReasons: []string{}}
	latest := map[string]Observation{}
	findingByKey := map[string]Finding{}
	for _, o := range observations {
		latest[o.PointID+"\x00"+o.Type] = o
	}
	for _, f := range findings {
		findingByKey[f.PointID+"\x00"+f.ObservationType] = f
	}
	for _, point := range points {
		for _, typ := range []string{"位移", "异响", "制动", "复位"} {
			item := ChecklistItem{PointID: point.ID, Type: typ, Status: "未检查"}
			key := point.ID + "\x00" + typ
			if o, ok := latest[key]; ok {
				item.WorstValue = o.WorstValue
				item.Status = "已通过"
				if !o.Passed {
					item.Status = "已产生发现"
				}
			}
			if f, ok := findingByKey[key]; ok && f.Status != "closed" {
				item.FindingID = f.ID
				item.Status = "已产生发现"
			}
			p.Items = append(p.Items, item)
			p.Total++
			switch item.Status {
			case "未检查":
				p.Unchecked++
				p.BlockingReasons = append(p.BlockingReasons, point.ID+"缺少"+typ+"检查")
			case "已通过":
				p.Passed++
			default:
				p.Findings++
				p.BlockingReasons = append(p.BlockingReasons, point.ID+"存在未裁决发现"+item.FindingID)
			}
		}
	}
	if p.Total > 0 {
		p.Percent = (p.Total - p.Unchecked) * 100 / p.Total
	}
	sort.Slice(p.Items, func(i, j int) bool {
		if p.Items[i].PointID == p.Items[j].PointID {
			return p.Items[i].Type < p.Items[j].Type
		}
		return p.Items[i].PointID < p.Items[j].PointID
	})
	return p
}
