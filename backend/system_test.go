package main

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestRevenueBusinessPermissionGate(t *testing.T) {
	a := App{}
	for _, tc := range []struct {
		method, path string
		id           Identity
		status       int
	}{
		{"POST", "/api/records", Identity{Permissions: []string{"Revenue.Read"}}, 403},
		{"POST", "/api/records", Identity{Permissions: []string{"Revenue.Read", "Revenue.Write"}}, 400},
		{"POST", "/api/import", Identity{Permissions: []string{"Revenue.Read"}}, 403},
		{"GET", "/api/integrations", Identity{Permissions: []string{"Revenue.Read", "Revenue.Write"}}, 403},
		{"POST", "/api/settings", Identity{Permissions: []string{"Revenue.Read", "Revenue.Write"}}, 403},
		{"GET", "/api/system/bootstrap", Identity{Permissions: []string{"Revenue.Read", "System.Manage"}}, 403},
	} {
		w := httptest.NewRecorder()
		a.api(w, httptest.NewRequest(tc.method, tc.path, strings.NewReader("invalid-json")), tc.id)
		if w.Code != tc.status {
			t.Fatalf("%s %s returned %d", tc.method, tc.path, w.Code)
		}
	}
}

func TestAuthorizedRechecksCentralAndLocalAccess(t *testing.T) {
	centralAllowed := true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/userinfo":
			jsonOut(w, 200, CentralUser{Sub: "stable-test", Preferred: "member"})
		case "/api/get-user":
			jsonOut(w, 200, map[string]any{"status": "ok", "data": CentralUser{ID: "stable-test", Owner: "sx", Name: "member"}})
		case "/api/get-roles":
			jsonOut(w, 200, map[string]any{"status": "ok", "data": []CentralRole{}})
		case "/api/get-permissions":
			jsonOut(w, 200, map[string]any{"status": "ok", "data": []Permission{{Owner: "sx", Name: "RevenueStatisticsSystem.Access", Users: []string{"sx/member"}, Resources: []string{"app-revenuestatistics"}, State: "Approved", Effect: "Allow", IsEnabled: centralAllowed}}})
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	t.Setenv("CASDOOR_INTERNAL", server.URL)
	db := sql.OpenDB(accessConnector{enabled: 1, roles: [][]driver.Value{{"查看员", `["Revenue.Read"]`}}})
	defer db.Close()
	a := App{db: db, client: server.Client(), sessions: map[string]Session{hashed("test-cookie"): {Sub: "stable-test", Token: "mock-token", Exp: time.Now().Add(time.Hour)}}}
	request := func(want int) {
		t.Helper()
		r := httptest.NewRequest("GET", "/api/auth/me", nil)
		r.AddCookie(&http.Cookie{Name: "revenue_session", Value: "test-cookie"})
		w := httptest.NewRecorder()
		a.authorized(a.api)(w, r)
		if w.Code != want {
			t.Fatalf("authorized=%d want=%d %s", w.Code, want, w.Body.String())
		}
	}
	request(200)
	centralAllowed = false
	request(403)
	if len(a.sessions) != 0 {
		t.Fatal("central revocation left session usable")
	}
	centralAllowed = true
	blocked := sql.OpenDB(accessConnector{enabled: 0})
	defer blocked.Close()
	a.db = blocked
	a.sessions[hashed("test-cookie")] = Session{Sub: "stable-test", Token: "mock-token", Exp: time.Now().Add(time.Hour)}
	request(403)
	if len(a.sessions) != 0 {
		t.Fatal("local disable left session usable")
	}
}

func TestSystemRoleValidation(t *testing.T) {
	base := func() SystemRole {
		return SystemRole{Code: "operations", Name: "运营角色", Description: "负责资源运营", Enabled: true, Permissions: []string{"Revenue.Read", "Revenue.Write"}}
	}
	r := base()
	if err := validateSystemRole(&r); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*SystemRole){
		func(r *SystemRole) { r.Code = "super_admin" }, func(r *SystemRole) { r.Code = "role';DROP" }, func(r *SystemRole) { r.Name = "" },
		func(r *SystemRole) { r.Description = strings.Repeat("字", 301) }, func(r *SystemRole) { r.Permissions = []string{"*"} },
		func(r *SystemRole) { r.Permissions = []string{"Revenue.Read", "System.Manage"} }, func(r *SystemRole) { r.Permissions = []string{"Revenue.Read", "Resource.Review"} },
		func(r *SystemRole) { r.Permissions = []string{"Revenue.Read", "Revenue.Read"} }, func(r *SystemRole) { r.Permissions = []string{"Revenue.Write"} },
	} {
		r = base()
		change(&r)
		if validateSystemRole(&r) == nil {
			t.Fatalf("invalid role accepted: %+v", r)
		}
	}
}
func TestSystemRoleUnionAndDisabled(t *testing.T) {
	permissions, names := rolePermissions([]SystemRole{
		{Name: "查看", Enabled: true, Permissions: []string{"Revenue.Read"}},
		{Name: "编辑", Enabled: true, Permissions: []string{"Revenue.Read", "Revenue.Write"}},
		{Name: "停用接口", Enabled: false, Permissions: []string{"Integration.Manage"}},
		{Name: "伪造超级", Enabled: true, IsSuperAdmin: true, Permissions: []string{"*"}},
		{Name: "未知权限", Enabled: true, Permissions: []string{"System.Manage"}},
	})
	id := Identity{Permissions: permissions}
	if !localPermission(id, "Revenue.Read") || !localPermission(id, "Revenue.Write") || localPermission(id, "review") || localPermission(id, "delete") || localPermission(id, "Integration.Manage") {
		t.Fatalf("permission union incorrect: %v", permissions)
	}
	if contains(names, "停用接口") || contains(names, "伪造超级") || contains(permissions, "System.Manage") || len(permissions) != 2 {
		t.Fatal("disabled or protected permissions leaked")
	}
}
func TestSystemManagementRejectsOrdinaryUsers(t *testing.T) {
	a := App{}
	for _, path := range [][]string{{"bootstrap"}, {"sync-users"}, {"users"}, {"roles"}, {"logs"}, {"logs", "export"}} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/api/system/"+strings.Join(path, "/"), nil)
		a.system(w, r, Identity{Role: "超级管理员", Permissions: []string{"System.Manage"}}, path)
		if w.Code != 403 {
			t.Fatalf("ordinary user accessed %v", path)
		}
	}
}
func TestSystemSessionRevocationIsScoped(t *testing.T) {
	a := App{sessions: map[string]Session{"a": {Sub: "one"}, "b": {Sub: "one"}, "c": {Sub: "other"}}}
	a.invalidateUsers([]string{"one"})
	if len(a.sessions) != 1 || a.sessions["c"].Sub != "other" {
		t.Fatal("session revocation crossed user boundary")
	}
}
func TestSystemLogFilters(t *testing.T) {
	where, args, err := systemLogFilter(url.Values{"keyword": {"%_';DROP"}, "module": {"用户管理"}, "from": {"2026-10-01"}, "to": {"2026-10-22"}})
	if err != nil || strings.Contains(where, "DROP") || len(args) != 8 || args[7] != "2026-10-23 00:00:00" || args[0] != "%\\%\\_';drop%" {
		t.Fatalf("invalid log query: %s %v %v", where, args, err)
	}
	for _, q := range []url.Values{{"from": {"invalid"}}, {"from": {"2026-10-23"}, "to": {"2026-10-22"}}, {"keyword": {strings.Repeat("字", 201)}}} {
		if _, _, err = systemLogFilter(q); err == nil {
			t.Fatal("invalid log filter accepted")
		}
	}
	for _, value := range []string{"=1+1", " +cmd", "\t@evil", "-1"} {
		if !strings.HasPrefix(csvSafe(value), "'") {
			t.Fatal("unsafe CSV cell")
		}
	}
	if csvSafe("正常操作") != "正常操作" {
		t.Fatal("CSV text changed")
	}
}
func TestSystemDirectoryAndSuperProtection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Error("system management attempted a central write")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/get-users":
			json.NewEncoder(w).Encode(map[string]any{"status": "ok", "data": []DirectoryUser{
				{CentralUser: CentralUser{ID: "admin-id", Owner: "sx", Name: "manager", Display: "管理人员"}},
				{CentralUser: CentralUser{ID: "user-id", Owner: "sx", Name: "member", Display: "普通用户"}},
				{CentralUser: CentralUser{ID: "other-id", Owner: "sx", Name: "other"}},
			}})
		case "/api/get-roles":
			json.NewEncoder(w).Encode(map[string]any{"status": "ok", "data": []CentralRole{{Owner: "sx", Name: "child", Users: []string{"sx/member"}, IsEnabled: true}, {Owner: "sx", Name: "parent", Roles: []string{"sx/child"}, IsEnabled: true}}})
		case "/api/get-permissions":
			json.NewEncoder(w).Encode(map[string]any{"status": "ok", "data": []Permission{
				{Owner: "sx", Name: "RevenueStatisticsSystem.SuperAdmin", Users: []string{"sx/manager"}, Resources: []string{"app-revenuestatistics"}, State: "Approved", Effect: "Allow", IsEnabled: true},
				{Owner: "sx", Name: "RevenueStatisticsSystem.Access", Roles: []string{"sx/parent"}, Resources: []string{"app-revenuestatistics"}, State: "Approved", Effect: "Allow", IsEnabled: true},
			}})
		default:
			t.Errorf("unexpected central request: %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	t.Setenv("CASDOOR_INTERNAL", server.URL)
	a := App{client: server.Client()}
	directory, err := a.systemDirectory()
	if err != nil || len(directory) != 2 || directory[0].Subject != "admin-id" || !directory[0].IsSuperAdmin || directory[1].IsSuperAdmin {
		t.Fatalf("directory application scope: %+v %v", directory, err)
	}
	for _, statusOnly := range []bool{false, true} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("PUT", "/api/system/users/admin-id", strings.NewReader(`{"roles":["viewer"],"enabled":false,"version":1}`))
		a.saveSystemUser(w, r, Identity{Sub: "another-admin", Super: true}, "admin-id", statusOnly)
		if w.Code != 403 {
			t.Fatal("central super user was locally editable")
		}
	}
	for _, auditFails := range []bool{false, true} {
		t.Run(map[bool]string{false: "new_binding", true: "audit_failure_rollback"}[auditFails], func(t *testing.T) {
			committed, rolledBack := false, false
			writes := []string{}
			db := sql.OpenDB(accessConnector{transaction: &accessTx{committed: &committed, rolledBack: &rolledBack}, query: func(query string, args []driver.NamedValue) (driver.Rows, error) {
				switch {
				case strings.Contains(query, "SELECT USERNAME"):
					if args[0].Value != "user-id" {
						t.Fatal("binding used mutable username instead of subject")
					}
					return &accessRows{columns: []string{"USERNAME"}, values: [][]driver.Value{{"member"}}}, nil
				case strings.Contains(query, "SELECT VERSION,ENABLED"):
					return &accessRows{columns: []string{"VERSION", "ENABLED"}}, nil
				case strings.Contains(query, "SELECT ENABLED FROM"):
					return &accessRows{columns: []string{"ENABLED"}, values: [][]driver.Value{{int64(1)}}}, nil
				default:
					return nil, errors.New("unexpected binding query")
				}
			}, exec: func(query string, args []driver.NamedValue) (driver.Result, error) {
				writes = append(writes, query)
				if strings.Contains(query, "INSERT INTO "+schema+"SYSTEM_USER_ROLES") && (args[0].Value != "user-id" || args[1].Value != "viewer") {
					t.Fatal("incorrect binding persisted")
				}
				if auditFails && strings.Contains(query, "SYSTEM_LOGS") {
					return nil, errors.New("audit unavailable")
				}
				return driver.RowsAffected(1), nil
			}})
			defer db.Close()
			a.db = db
			a.sessions = map[string]Session{"old": {Sub: "user-id"}, "other": {Sub: "another-admin"}}
			w := httptest.NewRecorder()
			r := httptest.NewRequest("POST", "/api/system/users", strings.NewReader(`{"subject":"user-id","roles":["viewer"],"version":0}`))
			a.saveSystemUser(w, r, Identity{Sub: "another-admin", Name: "operator", Super: true}, "", false)
			if auditFails {
				if w.Code != 503 || committed || !rolledBack || len(a.sessions) != 2 {
					t.Fatal("failed audited write was committed or revoked sessions")
				}
			} else {
				if w.Code != 200 || !committed || len(a.sessions) != 1 || len(writes) != 6 {
					t.Fatalf("new binding failed: status=%d writes=%d committed=%v", w.Code, len(writes), committed)
				}
			}
		})
	}
}

// In-memory rows exercise local access without opening a production database.
type accessConnector struct {
	enabled     int64
	roles       [][]driver.Value
	missing     bool
	query       func(string, []driver.NamedValue) (driver.Rows, error)
	exec        func(string, []driver.NamedValue) (driver.Result, error)
	transaction driver.Tx
}

func (c accessConnector) Connect(context.Context) (driver.Conn, error) { return accessConn{c}, nil }
func (c accessConnector) Driver() driver.Driver                        { return accessDriver{} }

type accessDriver struct{}

func (accessDriver) Open(string) (driver.Conn, error) { return nil, errors.New("use connector") }

type accessConn struct{ config accessConnector }

func (accessConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("unexpected prepare") }
func (accessConn) Close() error                        { return nil }
func (c accessConn) Begin() (driver.Tx, error) {
	if c.config.transaction != nil {
		return c.config.transaction, nil
	}
	return nil, errors.New("unexpected transaction")
}
func (c accessConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if c.config.exec != nil {
		return c.config.exec(query, args)
	}
	return nil, errors.New("unexpected write")
}
func (c accessConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if c.config.query != nil {
		return c.config.query(query, args)
	}
	if strings.Contains(query, "SELECT ENABLED FROM") {
		if c.config.missing {
			return &accessRows{columns: []string{"ENABLED"}}, nil
		}
		return &accessRows{columns: []string{"ENABLED"}, values: [][]driver.Value{{c.config.enabled}}}, nil
	}
	return &accessRows{columns: []string{"NAME", "PERMISSIONS"}, values: c.config.roles}, nil
}

type accessTx struct{ committed, rolledBack *bool }

func (t *accessTx) Commit() error   { *t.committed = true; return nil }
func (t *accessTx) Rollback() error { *t.rolledBack = true; return nil }

type accessRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r *accessRows) Columns() []string { return r.columns }
func (r *accessRows) Close() error      { return nil }
func (r *accessRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}
func TestLocalAccessRequiresEnabledMembership(t *testing.T) {
	for _, tc := range []struct {
		name    string
		c       accessConnector
		allowed bool
	}{
		{"missing", accessConnector{missing: true}, false},
		{"disabled", accessConnector{enabled: 0, roles: [][]driver.Value{{"编辑员", `["Revenue.Read","Revenue.Write"]`}}}, false},
		{"unbound", accessConnector{enabled: 1}, false},
		{"bound", accessConnector{enabled: 1, roles: [][]driver.Value{{"查看员", `["Revenue.Read"]`}, {"编辑员", `["Revenue.Read","Revenue.Write"]`}}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := sql.OpenDB(tc.c)
			defer db.Close()
			a := App{db: db}
			id, err := a.localRole(context.Background(), Identity{Sub: "subject"})
			if (err == nil) != tc.allowed {
				t.Fatalf("unexpected access: %v", err)
			}
			if tc.allowed && (!localPermission(id, "Revenue.Write") || localPermission(id, "review")) {
				t.Fatal("effective permissions incorrect")
			}
		})
	}
	a := App{}
	if _, err := a.localRole(context.Background(), Identity{Super: true}); err != nil {
		t.Fatal("central super should remain independent of local membership")
	}
}
func TestCentralRoleClosureRejectsDisabledAndCycles(t *testing.T) {
	roles := centralRolesFor("sx/u", []CentralRole{{Owner: "sx", Name: "a", Users: []string{"sx/u"}, IsEnabled: true}, {Owner: "sx", Name: "b", Roles: []string{"sx/a"}, IsEnabled: true}, {Owner: "sx", Name: "off", Users: []string{"sx/u"}}, {Owner: "sx", Name: "cycle", Roles: []string{"sx/cycle"}, IsEnabled: true}})
	if len(roles) != 2 || !roles["sx/b"] {
		t.Fatalf("role closure invalid: %v", roles)
	}
	if !activeTime("", time.Now().Add(time.Hour).Format(time.RFC3339), time.Now()) {
		t.Fatal("active time failed")
	}
}
