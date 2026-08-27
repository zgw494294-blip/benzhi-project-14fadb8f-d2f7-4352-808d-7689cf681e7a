package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"stage-rigging-release/internal/app"
	"stage-rigging-release/internal/httpui"
	"stage-rigging-release/internal/store"
	"syscall"
	"time"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:19091", "监听地址")
	self := flag.Bool("selfcheck", false, "执行自检")
	dbpath := flag.String("db", "rigging.db", "数据库路径")
	flag.Parse()
	if p := os.Getenv("PORT"); p != "" {
		*addr = "127.0.0.1:" + p
	}
	if !validAddr(*addr) {
		fmt.Fprintln(os.Stderr, "监听地址必须是127.0.0.1:<port>")
		os.Exit(2)
	}
	st, e := store.Open(*dbpath)
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	defer st.Close()
	a := app.New(st)
	srv := &http.Server{Addr: *addr, Handler: httpui.New(a).Handler(), ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second}
	if *self {
		go srv.ListenAndServe()
		time.Sleep(100 * time.Millisecond)
		if e := runSelfcheck(*addr); e != nil {
			fmt.Fprintln(os.Stderr, e)
			os.Exit(1)
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		srv.Shutdown(ctx)
		return
	}
	go func() {
		if e := srv.ListenAndServe(); e != nil && e != http.ErrServerClosed {
			fmt.Fprintln(os.Stderr, e)
		}
	}()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}
func runSelfcheck(addr string) error {
	client := &http.Client{Timeout: 3 * time.Second}
	payload := map[string]string{"showName": "自检演出", "venueZone": "主舞台", "performanceStartsAt": time.Now().UTC().Format(time.RFC3339), "performanceEndsAt": time.Now().Add(time.Hour).UTC().Format(time.RFC3339), "authorId": "mechanic"}
	b, _ := json.Marshal(payload)
	r, e := client.Post("http://"+addr+"/api/cases", "application/json", bytes.NewReader(b))
	if e != nil {
		return e
	}
	defer r.Body.Close()
	if r.StatusCode != 201 {
		return fmt.Errorf("创建案卷状态%d", r.StatusCode)
	}
	raw, _ := io.ReadAll(r.Body)
	var c struct {
		ID string `json:"ID"`
	}
	json.Unmarshal(raw, &c)
	if c.ID == "" {
		var alt map[string]any
		json.Unmarshal(raw, &alt)
		if id, ok := alt["ID"].(string); ok {
			c.ID = id
		}
	}
	base := "http://" + addr + "/api/cases/" + c.ID
	point := map[string]any{"id": "p1", "label": "主吊点", "ratedLoadKg": 1000, "plannedStaticLoadKg": 80, "slingAngleDegrees": 60, "dynamicFactor": 1.2}
	if e = postJSON(client, base+"/points", point, 201); e != nil {
		return e
	}
	types := []string{"葫芦", "钢索", "卸扣", "安全绳"}
	assignments := []map[string]any{}
	for i, typ := range types {
		for _, path := range []string{"主路径", "冗余路径"} {
			equipmentID := fmt.Sprintf("e%d-%s", i, path)
			eq := map[string]any{"id": equipmentID, "equipmentType": typ, "serialNumber": fmt.Sprintf("SELF-%d-%s", i, path), "ratedLoadKg": 1000, "certificateRef": "CERT-1", "certificateExpiresOn": time.Now().Add(24 * time.Hour).Format(time.RFC3339), "inspectionResult": "合格"}
			if e = postJSON(client, base+"/equipment", eq, 201); e != nil {
				return e
			}
			assignments = append(assignments, map[string]any{"pointId": "p1", "path": path, "equipmentId": equipmentID})
		}
	}
	if e = postJSON(client, base+"/assignments", map[string]any{"baseRevision": 10, "actor": "mechanic", "assignments": assignments}, 200); e != nil {
		return e
	}
	if e = postJSON(client, base+"/evaluate", map[string]any{}, 200); e != nil {
		return e
	}
	observations := []map[string]any{
		{"actor": "mechanic", "pointId": "p1", "type": "位移", "measurements": []map[string]any{{"value": 0.2, "measuredAt": time.Now().Format(time.RFC3339)}}, "description": "位移正常"},
		{"actor": "mechanic", "pointId": "p1", "type": "复位", "measurements": []map[string]any{{"value": 0.1, "measuredAt": time.Now().Format(time.RFC3339)}}, "description": "复位正常"},
		{"actor": "mechanic", "pointId": "p1", "type": "异响", "description": "无", "severityBasis": "现场监听无异常"},
		{"actor": "mechanic", "pointId": "p1", "type": "制动", "description": "正常", "severityBasis": "制动保持测试通过"},
	}
	for _, observation := range observations {
		if e = postJSON(client, base+"/observations", observation, 201); e != nil {
			return e
		}
	}
	var review struct{ ID string }
	if e = postJSONDecode(client, base+"/reviews", map[string]any{"reviewerId": "reviewer"}, 201, &review); e != nil {
		return e
	}
	if e = postJSON(client, base+"/reviews/"+review.ID+"/decision", map[string]any{"reviewerId": "reviewer", "decision": "批准", "reason": "证据完整"}, 200); e != nil {
		return e
	}
	var cr struct {
		ID string `json:"ID"`
	}
	if e = postJSONDecode(client, base+"/freeze", map[string]any{"issuer": "tech-lead"}, 200, &cr); e != nil {
		return e
	}
	vr, e := client.Get("http://" + addr + "/api/verify?credentialId=" + cr.ID)
	if e != nil || vr.StatusCode != 200 {
		return fmt.Errorf("凭据验真失败")
	}
	return nil
}

func postJSON(client *http.Client, url string, value any, expected int) error {
	return postJSONDecode(client, url, value, expected, nil)
}
func postJSONDecode(client *http.Client, url string, value any, expected int, out any) error {
	b, _ := json.Marshal(value)
	r, e := client.Post(url, "application/json", bytes.NewReader(b))
	if e != nil {
		return e
	}
	defer r.Body.Close()
	if r.StatusCode != expected {
		body, _ := io.ReadAll(r.Body)
		return fmt.Errorf("请求%s状态%d: %s", url, r.StatusCode, string(body))
	}
	if out != nil {
		return json.NewDecoder(r.Body).Decode(out)
	}
	return nil
}
