package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Uses temporary local fixtures and a read-only mock central directory.
// No central account, permission or password is written.
func TestSystemManagementDatabaseLifecycle(t *testing.T) {
	db := reviewDB(t)
	suffix := strings.ReplaceAll(strings.ToLower(random()[:10]), "-", "_")
	subject, name, code := "sm-"+suffix, "sm_"+suffix, "sm_"+suffix
	display, phone := "管理验收账号", "13800000000"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Error("central write attempted")
			w.WriteHeader(405)
			return
		}
		switch r.URL.Path {
		case "/api/get-users":
			jsonOut(w, 200, map[string]any{"status": "ok", "data": []DirectoryUser{{CentralUser: CentralUser{ID: subject, Owner: "sx", Name: name, Display: display}, Phone: phone}, {CentralUser: CentralUser{ID: "sm-central-super", Owner: "sx", Name: "sm_super"}}}})
		case "/api/get-roles":
			jsonOut(w, 200, map[string]any{"status": "ok", "data": []CentralRole{}})
		case "/api/get-permissions":
			jsonOut(w, 200, map[string]any{"status": "ok", "data": []Permission{{Owner: "sx", Name: "RevenueStatisticsSystem.Access", Users: []string{"sx/" + name}, Resources: []string{"app-revenuestatistics"}, State: "Approved", Effect: "Allow", IsEnabled: true}, {Owner: "sx", Name: "RevenueStatisticsSystem.SuperAdmin", Users: []string{"sx/sm_super"}, Resources: []string{"app-revenuestatistics"}, State: "Approved", Effect: "Allow", IsEnabled: true}}})
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	t.Setenv("CASDOOR_INTERNAL", server.URL)
	a := App{db: db, client: server.Client(), sessions: map[string]Session{"target": {Sub: subject}, "other": {Sub: "sm-operator"}}}
	actor := Identity{Sub: "sm-operator", Name: "系统管理验收", Super: true}
	t.Cleanup(func() {
		// Retain operation logs; remove only this test's temporary local objects.
		db.Exec("DELETE FROM "+schema+"SYSTEM_USER_ROLES WHERE SUBJECT=?", subject)
		db.Exec("DELETE FROM "+schema+"SYSTEM_USERS WHERE SUBJECT=?", subject)
		db.Exec("DELETE FROM "+schema+"IDENTITIES WHERE SUBJECT=?", subject)
		db.Exec("DELETE FROM "+schema+"SYSTEM_ROLES WHERE CODE=?", code)
		db.Exec("DELETE FROM " + schema + "SYSTEM_USER_ROLES WHERE SUBJECT='sm-central-super'")
		db.Exec("DELETE FROM " + schema + "SYSTEM_USERS WHERE SUBJECT='sm-central-super'")
		db.Exec("DELETE FROM " + schema + "IDENTITIES WHERE SUBJECT='sm-central-super'")
	})
	request := func(method, path string, body any, want int) *httptest.ResponseRecorder {
		t.Helper()
		raw, _ := json.Marshal(body)
		w := httptest.NewRecorder()
		a.system(w, httptest.NewRequest(method, "/api/system/"+path, strings.NewReader(string(raw))), actor, strings.Split(strings.SplitN(path, "?", 2)[0], "/"))
		if w.Code != want {
			t.Fatalf("%s %s: status %d want %d: %s", method, path, w.Code, want, w.Body.String())
		}
		return w
	}
	role := SystemRole{Code: code, Name: "验收营收角色", Description: "数据库生命周期验收", Enabled: true, Permissions: []string{"Revenue.Read", "Revenue.Write"}}
	request("POST", "roles", role, 200)
	request("POST", "roles", role, 409)
	request("POST", "users", saveUserRequest{Subject: subject, Roles: []string{"viewer", code}}, 200)
	if len(a.sessions) != 1 || a.sessions["other"].Sub != "sm-operator" {
		t.Fatal("user session revocation escaped scope")
	}
	var lastLogin string
	if e := db.QueryRow("SELECT COALESCE(TO_CHAR(LAST_LOGIN,'YYYY-MM-DD HH24:MI:SS'),'') FROM "+schema+"IDENTITIES WHERE SUBJECT=?", subject).Scan(&lastLogin); e != nil || lastLogin != "" {
		t.Fatal("binding invented recent login", e, lastLogin)
	}
	_, e := db.Exec("UPDATE "+schema+"IDENTITIES SET LAST_LOGIN=TO_TIMESTAMP('2001-02-03 04:05:06','YYYY-MM-DD HH24:MI:SS') WHERE SUBJECT=?", subject)
	if e != nil {
		t.Fatal(e)
	}
	request("PUT", "users/"+subject+"/status", saveUserRequest{Version: 1, Enabled: false}, 200)
	display, phone = "同步后的名称", "13900000000"
	request("POST", "sync-users", nil, 200)
	var version, enabled, bindingCount int
	var gotDisplay, gotPhone, synced string
	e = db.QueryRow("SELECT U.VERSION,U.ENABLED,I.DISPLAY_NAME,U.PHONE,U.SYNCED_AT,TO_CHAR(I.LAST_LOGIN,'YYYY-MM-DD HH24:MI:SS') FROM "+schema+"SYSTEM_USERS U JOIN "+schema+"IDENTITIES I ON I.SUBJECT=U.SUBJECT WHERE U.SUBJECT=?", subject).Scan(&version, &enabled, &gotDisplay, &gotPhone, &synced, &lastLogin)
	if e != nil || version != 2 || enabled != 0 || gotDisplay != display || gotPhone != phone || synced == "" || lastLogin != "2001-02-03 04:05:06" {
		t.Fatalf("sync lost local data: %d %d %q %q %q %q %v", version, enabled, gotDisplay, gotPhone, synced, lastLogin, e)
	}
	db.QueryRow("SELECT COUNT(*) FROM "+schema+"SYSTEM_USER_ROLES WHERE SUBJECT=?", subject).Scan(&bindingCount)
	if bindingCount != 2 {
		t.Fatal("sync replaced role bindings")
	}
	request("PUT", "users/"+subject+"/status", saveUserRequest{Version: 1, Enabled: true}, 409)
	request("PUT", "users/"+subject+"/status", saveUserRequest{Version: 2, Enabled: true}, 200)
	id, e := a.localRole(context.Background(), Identity{Sub: subject})
	if e != nil || !localPermission(id, "Revenue.Write") {
		t.Fatal("enabled multi-role union failed", e)
	}
	role.Version = 1
	role.Enabled = false
	request("PUT", "roles/"+code, role, 200)
	id, e = a.localRole(context.Background(), Identity{Sub: subject})
	if e != nil || localPermission(id, "Revenue.Write") || !localPermission(id, "Revenue.Read") {
		t.Fatal("disabled role retained privileges", e)
	}
	request("PUT", "roles/"+code, role, 409)
	request("DELETE", "roles/"+code+"?version=2", nil, 409)
	request("DELETE", "roles/viewer?version=1", nil, 409)
	request("PUT", "users/sm-central-super/status", saveUserRequest{Version: 1, Enabled: false}, 403)
	request("PUT", "users/"+subject, saveUserRequest{Version: 3, Roles: []string{"super_admin"}}, 400)
	request("PUT", "users/"+subject, saveUserRequest{Version: 3, Roles: []string{"viewer", "viewer"}}, 400)
	role.Version = 2
	role.Enabled = true
	role.Name = "应被回滚的名称"
	actor.Name = strings.Repeat("x", 500)
	request("PUT", "roles/"+code, role, 503)
	actor.Name = "系统管理验收"
	var actualName string
	db.QueryRow("SELECT VERSION,NAME FROM "+schema+"SYSTEM_ROLES WHERE CODE=?", code).Scan(&version, &actualName)
	if version != 2 || actualName != "验收营收角色" {
		t.Fatal("audit failure did not roll back role change", version, actualName)
	}
	request("PUT", "users/"+subject, saveUserRequest{Version: 3, Roles: []string{}}, 200)
	if _, e = a.localRole(context.Background(), Identity{Sub: subject}); e == nil {
		t.Fatal("unbound ordinary user retained access")
	}
	request("DELETE", "roles/"+code+"?version=1", nil, 409)
	request("DELETE", "roles/"+code+"?version=2", nil, 200)
	request("DELETE", "roles/"+code+"?version=2", nil, 404)
	logs := request("GET", "logs?keyword="+code+"&size=2", nil, 200)
	var page struct {
		Items []SystemLog
		Total int
	}
	if json.Unmarshal(logs.Body.Bytes(), &page) != nil || len(page.Items) != 2 || page.Total < 3 {
		t.Fatal("log filter/pagination failed", logs.Body.String())
	}
	exported := request("GET", "logs/export?keyword="+code, nil, 200)
	csvRows, e := csv.NewReader(strings.NewReader(strings.TrimPrefix(exported.Body.String(), "\ufeff"))).ReadAll()
	if e != nil || len(csvRows) != page.Total+1 {
		t.Fatalf("filtered export mismatch: %d / %d %v", len(csvRows), page.Total, e)
	}
	last := request("GET", "logs?keyword="+code+"&module=角色管理&size=100", nil, 200)
	if !strings.Contains(last.Body.String(), "验收营收角色") || !strings.Contains(last.Body.String(), "删除角色") {
		t.Fatal("deleted role identity missing from logs")
	}
	request("GET", "logs?from=2026-10-23&to=2026-10-22", nil, 400)
	empty := request("GET", "logs?keyword=sm-no-result-"+suffix, nil, 200)
	if !strings.Contains(empty.Body.String(), `"total":0`) {
		t.Fatal("empty logs query failed")
	}
	t.Log(fmt.Sprintf("temporary subject %s lifecycle verified; management audit retained", subject))
}
