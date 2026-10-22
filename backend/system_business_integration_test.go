package main

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRevenueLocalPermissionsAndAuditedInterface(t *testing.T) {
	db := reviewDB(t)
	a := App{db: db}
	actor := "sm-business-" + random()[:10]
	id := Identity{Sub: actor, Name: "权限验收", Permissions: []string{"Revenue.Read", "Revenue.Write", "Integration.Manage"}}
	var integrationID, recordID string
	t.Cleanup(func() {
		db.Exec("DELETE FROM "+schema+"REVENUE_RECORDS WHERE CREATED_BY=? OR ID=?", actor, recordID)
		db.Exec("DELETE FROM "+schema+"INTEGRATIONS WHERE ID=?", integrationID)
		// Audit rows stay available even after their temporary business objects are removed.
	})
	request := func(path, body string, identity Identity, want int) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		a.api(w, httptest.NewRequest("POST", "/api/"+path, strings.NewReader(body)), identity)
		if w.Code != want {
			t.Fatalf("%s returned %d want %d", path, w.Code, want)
		}
		return w
	}
	payload := `{"name":"管理权限验收接口","module":"ticket","enabled":1}`
	request("integrations", payload, Identity{Permissions: []string{"Revenue.Read", "Revenue.Write"}}, 403)
	out := request("integrations", payload, id, 200)
	var config struct{ ID, Token string }
	if json.Unmarshal(out.Body.Bytes(), &config) != nil || config.Token == "" {
		t.Fatal("granted interface management failed")
	}
	integrationID = config.ID
	for n := 0; n < 2; n++ {
		r := httptest.NewRequest("POST", "/api/ingest", strings.NewReader(`{"sourceId":"management-test","record":{"date":"2100-01-02","category":"管理接口验收","quantity":3,"amount":"0.03","visitors":3}}`))
		r.Header.Set("Authorization", "Bearer "+config.Token)
		w := httptest.NewRecorder()
		a.ingest(w, r)
		if w.Code != 200 {
			t.Fatal("ingest failed", w.Code)
		}
		var saved struct {
			ID        string
			Duplicate bool
		}
		json.Unmarshal(w.Body.Bytes(), &saved)
		recordID = saved.ID
		if saved.Duplicate != (n == 1) {
			t.Fatal("ingest deduplication failed")
		}
	}
	var count int
	db.QueryRow("SELECT COUNT(*) FROM "+schema+"SYSTEM_LOGS WHERE RECORD_ID=? AND ACTION='接口写入日报'", recordID).Scan(&count)
	if count != 1 {
		t.Fatal("interface write/audit inconsistent", count)
	}
	record := `{"module":"ticket","date":"2100-01-02","category":"管理权限日报验收","quantity":1,"amount":"0.01","visitors":1,"source":"` + actor + `"}`
	request("records", record, Identity{Permissions: []string{"Revenue.Read"}}, 403)
	request("records", record, id, 200)
	var savedID string
	db.QueryRow("SELECT ID FROM "+schema+"REVENUE_RECORDS WHERE CREATED_BY=?", actor).Scan(&savedID)
	var detail, module string
	var actorName string
	e := db.QueryRow("SELECT DETAIL,MODULE,ACTOR FROM "+schema+"AUDIT_LOG WHERE TARGET_ID=?", savedID).Scan(&detail, &module, &actorName)
	if e != nil || !strings.Contains(detail, "管理权限日报验收") || module != "票务数据" || actorName != actor {
		t.Fatal("business audit metadata missing", e)
	}
	w := httptest.NewRecorder()
	a.systemLogs(w, httptest.NewRequest("GET", "/api/system/logs?keyword="+savedID, nil), false)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "管理权限日报验收") {
		t.Fatal("business log aggregation failed", w.Code)
	}
}
