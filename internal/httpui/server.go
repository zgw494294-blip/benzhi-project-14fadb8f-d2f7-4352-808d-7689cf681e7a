package httpui

import (
	"encoding/json"
	"net/http"
	"stage-rigging-release/internal/app"
	"stage-rigging-release/internal/rigging"
	"strconv"
	"time"
)

type Server struct{ App *app.App }

func New(a *app.App) *Server { return &Server{App: a} }
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.index)
	mux.Handle("/static/", http.FileServer(http.FS(staticFS)))
	mux.HandleFunc("/api/cases", s.cases)
	mux.HandleFunc("/api/cases/", s.caseAction)
	mux.HandleFunc("/api/verify", s.verify)
	return security(mux)
}
func security(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	b, _ := staticFS.ReadFile("static/index.html")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(b)
}
func write(w http.ResponseWriter, v any, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
func problem(w http.ResponseWriter, code int, title string, err error, details any) {
	body := map[string]any{"type": "about:blank", "title": title, "status": code}
	if err != nil {
		body["detail"] = err.Error()
	}
	if details != nil {
		body["problems"] = details
	}
	write(w, body, code)
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(v); err != nil {
		problem(w, 400, "请求格式错误", err, nil)
		return false
	}
	return true
}
func (s *Server) cases(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		write(w, map[string]any{"message": "请通过POST创建案卷"}, 200)
		return
	}
	var in struct {
		ShowName, VenueZone, AuthorID          string
		PerformanceStartsAt, PerformanceEndsAt string
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in) != nil {
		write(w, map[string]string{"error": "请求格式错误"}, 400)
		return
	}
	st, _ := time.Parse(time.RFC3339, in.PerformanceStartsAt)
	et, _ := time.Parse(time.RFC3339, in.PerformanceEndsAt)
	c, e := s.App.CreateCase(in.ShowName, in.VenueZone, st, et, in.AuthorID)
	if e != nil {
		write(w, map[string]string{"error": e.Error()}, 400)
		return
	}
	write(w, c, 201)
}
func (s *Server) caseAction(w http.ResponseWriter, r *http.Request) {
	parts := splitPath(r.URL.Path)
	if len(parts) < 3 || parts[0] != "api" || parts[1] != "cases" {
		http.NotFound(w, r)
		return
	}
	id := parts[2]
	c, ok := s.App.Store.GetCase(id)
	if !ok {
		write(w, map[string]string{"error": "案卷不存在"}, 404)
		return
	}
	switch {
	case len(parts) == 3 && r.Method == "GET":
		coverage, _ := s.App.Coverage(id)
		progress, _ := s.App.RehearsalProgress(id)
		write(w, map[string]any{"case": c, "points": s.App.Store.Points(id), "equipment": s.App.Store.Equipment(id), "assignments": s.App.Store.Assignments(id), "coverage": coverage, "evaluation": s.App.Store.EvaluationOrNil(id), "scenarios": s.App.Store.Scenarios(id), "observations": s.App.Store.Observations(id), "rehearsalProgress": progress, "findings": s.App.Store.Findings(id), "remediations": s.App.Store.Plans(id), "reviews": s.App.Store.ReviewRounds(id), "credentials": s.App.Store.Credentials(id), "timeline": s.App.Store.AuditRecords(id, "", "")}, 200)
	case len(parts) == 4 && parts[3] == "lease" && r.Method == "POST":
		var in struct {
			Holder string `json:"holder"`
		}
		if !decode(w, r, &in) {
			return
		}
		v, e := s.App.AcquireLease(id, in.Holder)
		if e != nil {
			problem(w, 409, "无法取得编辑租约", e, nil)
			return
		}
		write(w, v, 200)
	case len(parts) == 5 && parts[3] == "points" && parts[4] == "preflight" && r.Method == "POST":
		var in struct {
			BaseRevision int             `json:"baseRevision"`
			Points       []rigging.Point `json:"points"`
		}
		if !decode(w, r, &in) {
			return
		}
		v, e := s.App.PreflightPointBatch(id, in.BaseRevision, in.Points)
		if e != nil {
			problem(w, 409, "批量预检失败", e, nil)
			return
		}
		write(w, v, 200)
	case len(parts) == 5 && parts[3] == "points" && parts[4] == "batch" && r.Method == "POST":
		var in struct {
			LeaseID      string          `json:"leaseId"`
			RequestID    string          `json:"requestId"`
			Actor        string          `json:"actor"`
			BaseRevision int             `json:"baseRevision"`
			Points       []rigging.Point `json:"points"`
		}
		if !decode(w, r, &in) {
			return
		}
		v, e := s.App.CommitPointBatch(id, in.LeaseID, in.RequestID, in.Actor, in.BaseRevision, in.Points)
		if e != nil {
			problem(w, 409, "批量提交被拒绝", e, v.Problems)
			return
		}
		write(w, v, 200)
	case len(parts) == 4 && parts[3] == "points" && r.Method == "POST":
		var p rigging.Point
		if !decode(w, r, &p) {
			return
		}
		if e := s.App.AddPoint(c, p); e != nil {
			write(w, map[string]string{"error": e.Error()}, 400)
			return
		}
		write(w, map[string]string{"status": "ok"}, 201)
	case len(parts) >= 4 && parts[3] == "equipment" && r.Method == "POST":
		var eq rigging.Equipment
		if !decode(w, r, &eq) {
			return
		}
		if e := s.App.AddEquipment(c, eq); e != nil {
			write(w, map[string]string{"error": e.Error()}, 400)
			return
		}
		write(w, map[string]string{"status": "ok"}, 201)
	case len(parts) == 4 && parts[3] == "assignments" && r.Method == "POST":
		var in struct {
			BaseRevision int                  `json:"baseRevision"`
			Actor        string               `json:"actor"`
			Assignments  []rigging.Assignment `json:"assignments"`
		}
		if !decode(w, r, &in) {
			return
		}
		m, e := s.App.SaveAssignments(id, in.Actor, in.BaseRevision, in.Assignments)
		if e != nil {
			problem(w, 400, "配装覆盖校验未通过", e, m.Problems)
			return
		}
		write(w, m, 200)
	case len(parts) == 4 && parts[3] == "coverage" && r.Method == "GET":
		m, e := s.App.Coverage(id)
		if e != nil {
			problem(w, 404, "查询失败", e, nil)
			return
		}
		write(w, m, 200)
	case len(parts) == 4 && parts[3] == "evaluate" && r.Method == "POST":
		ev, e := s.App.Evaluate(c)
		if e != nil {
			write(w, map[string]string{"error": e.Error()}, 400)
			return
		}
		write(w, ev, 200)
	case len(parts) == 5 && parts[3] == "evaluations" && parts[4] == "scenarios" && r.Method == "GET":
		write(w, s.App.Store.Scenarios(id), 200)
	case len(parts) == 5 && parts[3] == "evaluations" && parts[4] == "scenarios" && r.Method == "POST":
		var in struct {
			Actor        string                       `json:"actor"`
			BaseRevision int                          `json:"baseRevision"`
			Adjustments  []rigging.ScenarioAdjustment `json:"adjustments"`
		}
		if !decode(w, r, &in) {
			return
		}
		v, e := s.App.CreateScenario(id, in.Actor, in.BaseRevision, in.Adjustments)
		if e != nil {
			problem(w, 400, "试算失败", e, nil)
			return
		}
		write(w, v, 201)
	case len(parts) == 7 && parts[3] == "evaluations" && parts[4] == "scenarios" && parts[6] == "adopt" && r.Method == "POST":
		var in struct {
			Actor     string `json:"actor"`
			RequestID string `json:"requestId"`
		}
		if !decode(w, r, &in) {
			return
		}
		v, e := s.App.AdoptScenario(id, parts[5], in.RequestID, in.Actor)
		if e != nil {
			problem(w, 409, "无法采用情景", e, nil)
			return
		}
		write(w, v, 200)
	case len(parts) == 4 && parts[3] == "observations" && r.Method == "POST":
		var in struct {
			Actor         string                `json:"actor"`
			PointID       string                `json:"pointId"`
			Type          string                `json:"type"`
			Description   string                `json:"description"`
			SeverityBasis string                `json:"severityBasis"`
			Measurements  []rigging.Measurement `json:"measurements"`
		}
		if !decode(w, r, &in) {
			return
		}
		v, f, e := s.App.RecordObservationContext(r.Context(), id, in.Actor, rigging.Observation{PointID: in.PointID, Type: in.Type, Description: in.Description, SeverityBasis: in.SeverityBasis, Measurements: in.Measurements})
		if e != nil {
			problem(w, 400, "观察提交失败", e, nil)
			return
		}
		write(w, map[string]any{"observation": v, "finding": f}, 201)
	case len(parts) == 5 && parts[3] == "rehearsal" && parts[4] == "progress" && r.Method == "GET":
		v, e := s.App.RehearsalProgress(id)
		if e != nil {
			problem(w, 404, "查询失败", e, nil)
			return
		}
		write(w, v, 200)
	case len(parts) == 4 && parts[3] == "observe" && r.Method == "POST":
		var in struct {
			PointID, ObservationType, Description string
			MeasuredValue                         float64
			Pass                                  bool
		}
		if !decode(w, r, &in) {
			return
		}
		o := rigging.Observation{PointID: in.PointID, Type: in.ObservationType, Description: in.Description, SeverityBasis: "旧接口提交", Measurements: []rigging.Measurement{{Value: in.MeasuredValue, MeasuredAt: time.Now()}}}
		v, f, e := s.App.RecordObservationContext(r.Context(), id, c.AuthorID, o)
		if e != nil {
			problem(w, 400, "观察提交失败", e, nil)
			return
		}
		write(w, map[string]any{"observation": v, "finding": f}, 201)
	case len(parts) == 4 && parts[3] == "remediate" && r.Method == "POST":
		var in struct{ FindingID, ActionType string }
		if !decode(w, r, &in) {
			return
		}
		if e := s.App.Remediate(c, in.FindingID, in.ActionType); e != nil {
			write(w, map[string]string{"error": e.Error()}, 400)
			return
		}
		write(w, map[string]string{"status": "ok"}, 200)
	case len(parts) == 4 && parts[3] == "remediations" && r.Method == "GET":
		write(w, s.App.Store.Plans(id), 200)
	case len(parts) == 4 && parts[3] == "remediations" && r.Method == "POST":
		var in struct {
			FindingID  string                     `json:"findingId"`
			ActionType string                     `json:"actionType"`
			Actor      string                     `json:"actor"`
			Changes    []rigging.StructuredChange `json:"changes"`
		}
		if !decode(w, r, &in) {
			return
		}
		v, e := s.App.CreateRemediation(id, in.FindingID, in.ActionType, in.Actor, in.Changes)
		if e != nil {
			problem(w, 400, "整改提交失败", e, nil)
			return
		}
		write(w, v, 201)
	case len(parts) == 6 && parts[3] == "remediations" && parts[5] == "retests" && r.Method == "POST":
		var in struct {
			ItemID   string `json:"itemId"`
			Actor    string `json:"actor"`
			Status   string `json:"status"`
			Evidence string `json:"evidence"`
		}
		if !decode(w, r, &in) {
			return
		}
		v, e := s.App.RecordRetest(id, parts[4], in.ItemID, in.Actor, in.Status, in.Evidence)
		if e != nil {
			problem(w, 400, "复验提交失败", e, nil)
			return
		}
		write(w, v, 200)
	case len(parts) == 5 && parts[3] == "reviews" && parts[4] == "evidence" && r.Method == "GET":
		write(w, s.App.EvidenceChecklist(id), 200)
	case len(parts) == 4 && parts[3] == "reviews" && r.Method == "GET":
		write(w, s.App.Store.ReviewRounds(id), 200)
	case len(parts) == 4 && parts[3] == "reviews" && r.Method == "POST":
		var in struct {
			ReviewerID string `json:"reviewerId"`
		}
		if !decode(w, r, &in) {
			return
		}
		v, e := s.App.SubmitReviewRound(id, in.ReviewerID)
		if e != nil {
			problem(w, 409, "复核提交失败", e, nil)
			return
		}
		write(w, v, 201)
	case len(parts) == 6 && parts[3] == "reviews" && parts[5] == "decision" && r.Method == "POST":
		var in struct {
			ReviewerID  string                     `json:"reviewerId"`
			Decision    string                     `json:"decision"`
			Reason      string                     `json:"reason"`
			ReturnItems []rigging.ReviewReturnItem `json:"returnItems"`
		}
		if !decode(w, r, &in) {
			return
		}
		v, e := s.App.DecideReview(id, parts[4], in.ReviewerID, in.Decision, in.Reason, in.ReturnItems)
		if e != nil {
			problem(w, 400, "复核决定失败", e, nil)
			return
		}
		write(w, v, 200)
	case len(parts) == 6 && parts[3] == "reviews" && parts[5] == "responses" && r.Method == "POST":
		var in struct {
			ItemID      string `json:"itemId"`
			Response    string `json:"response"`
			EvidenceRef string `json:"evidenceRef"`
		}
		if !decode(w, r, &in) {
			return
		}
		v, e := s.App.RespondReview(id, parts[4], in.ItemID, in.Response, in.EvidenceRef)
		if e != nil {
			problem(w, 400, "退回项响应失败", e, nil)
			return
		}
		write(w, v, 200)
	case len(parts) == 4 && parts[3] == "review" && r.Method == "POST":
		var in struct{ ReviewerID, Decision, Reason string }
		if !decode(w, r, &in) {
			return
		}
		if e := s.App.SubmitReview(c, in.ReviewerID, in.Decision, in.Reason); e != nil {
			write(w, map[string]string{"error": e.Error()}, 400)
			return
		}
		write(w, map[string]string{"status": "ok"}, 200)
	case len(parts) == 4 && (parts[3] == "freeze" || parts[3] == "credentials") && r.Method == "POST":
		var in struct{ Issuer string }
		if !decode(w, r, &in) {
			return
		}
		cr, e := s.App.Freeze(c, in.Issuer)
		if e != nil {
			write(w, map[string]string{"error": e.Error()}, 400)
			return
		}
		write(w, cr, 200)
	case len(parts) == 4 && parts[3] == "credentials" && r.Method == "GET":
		write(w, s.App.Store.Credentials(id), 200)
	case len(parts) == 6 && parts[3] == "credentials" && parts[5] == "revoke" && r.Method == "POST":
		var in struct {
			Actor     string `json:"actor"`
			Role      string `json:"role"`
			Reason    string `json:"reason"`
			RequestID string `json:"requestId"`
		}
		if !decode(w, r, &in) {
			return
		}
		v, e := s.App.RevokeCredential(parts[4], in.Actor, in.Role, in.Reason, in.RequestID)
		if e != nil {
			problem(w, 403, "凭据撤销失败", e, nil)
			return
		}
		write(w, v, 200)
	case len(parts) == 4 && parts[3] == "timeline" && r.Method == "GET":
		write(w, s.App.Store.AuditRecords(id, r.URL.Query().Get("category"), r.URL.Query().Get("credentialId")), 200)
	default:
		http.NotFound(w, r)
	}
}
func (s *Server) verify(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("credentialId")
	if id == "" {
		id = r.URL.Query().Get("id")
	}
	if id == "" {
		id = r.URL.Query().Get("caseId")
	}
	d, e := s.App.DiagnoseCredential(id, time.Now())
	code := 200
	if d.Status == "格式错误" {
		code = 400
	}
	write(w, map[string]any{"diagnostic": d, "error": errString(e)}, code)
}
func errString(e error) string {
	if e == nil {
		return ""
	}
	return e.Error()
}
func splitPath(p string) []string {
	var out []string
	for _, x := range stringsSplit(p, "/") {
		if x != "" {
			out = append(out, x)
		}
	}
	return out
}
func stringsSplit(s, sep string) []string {
	var out []string
	for {
		i := indexOf(s, sep)
		if i < 0 {
			out = append(out, s)
			break
		}
		out = append(out, s[:i])
		s = s[i+len(sep):]
	}
	return out
}
func indexOf(s, t string) int {
	for i := 0; i+len(t) <= len(s); i++ {
		if s[i:i+len(t)] == t {
			return i
		}
	}
	return -1
}

var _ = strconv.IntSize
