package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClaims(t *testing.T) {
	now := time.Now()
	base := Claims{Iss: "issuer", Sub: "immutable-id", Aud: json.RawMessage(`"client"`), Exp: now.Unix() + 60, Nonce: "expected"}
	if validateClaims(base, "client", "issuer", "expected", now) != nil {
		t.Fatal("合法凭证被拒绝")
	}
	for _, change := range []func(*Claims){func(c *Claims) { c.Iss = "attacker" }, func(c *Claims) { c.Aud = json.RawMessage(`"other-app"`) }, func(c *Claims) { c.Exp = now.Unix() - 1 }, func(c *Claims) { c.Nonce = "replay" }, func(c *Claims) { c.Sub = "" }, func(c *Claims) { c.Azp = "other-app" }, func(c *Claims) { c.Aud = json.RawMessage(`["client","other"]`) }} {
		c := base
		change(&c)
		if validateClaims(c, "client", "issuer", "expected", now) == nil {
			t.Fatal("非法凭证被接受")
		}
	}
}
func TestApplicationPermissions(t *testing.T) {
	p := Permission{Owner: "sx", Name: "RevenueStatisticsSystem.SuperAdmin", Users: []string{"sx/user"}, Resources: []string{"app-revenuestatistics"}, State: "Approved", Effect: "Allow", IsEnabled: true}
	now := time.Now()
	_, super, denied := evaluatePermissions([]Permission{p}, "sx/user", nil, now)
	if !super || denied {
		t.Fatal("明确授权失败")
	}
	for _, change := range []func(*Permission){func(p *Permission) { p.Name = "SuperAdmin" }, func(p *Permission) { p.Resources = []string{"app-vben"} }, func(p *Permission) { p.IsEnabled = false }, func(p *Permission) { p.EndTime = now.Add(-time.Hour).Format(time.RFC3339) }, func(p *Permission) { p.Users = []string{"sx/admin"} }, func(p *Permission) { p.Owner = "other" }} {
		bad := p
		change(&bad)
		a, s, _ := evaluatePermissions([]Permission{bad}, "sx/user", nil, now)
		if a || s {
			t.Fatal("跨应用或失效授权被接受")
		}
	}
	deny := p
	deny.Effect = "Deny"
	_, _, denied = evaluatePermissions([]Permission{p, deny}, "sx/user", nil, now)
	if !denied {
		t.Fatal("显式拒绝未生效")
	}
}
func TestNoUsernameElevation(t *testing.T) {
	p := Permission{Owner: "sx", Name: "RevenueStatisticsSystem.Access", Users: []string{"sx/admin"}, Resources: []string{"app-revenuestatistics"}, State: "Approved", Effect: "Allow", IsEnabled: true}
	access, super, _ := evaluatePermissions([]Permission{p}, "sx/admin", nil, time.Now())
	if !access || super {
		t.Fatal("同名管理员不应提权")
	}
}
func TestCallbackConsumesState(t *testing.T) {
	a := &App{flows: map[string]Flow{"state": {Bind: hashed("browser"), Exp: time.Now().Add(time.Minute)}}}
	r := httptest.NewRequest("GET", "/api/auth/callback?state=state", nil)
	w := httptest.NewRecorder()
	a.callback(w, r)
	if w.Code != 400 {
		t.Fatal("无浏览器绑定应拒绝")
	}
	if _, ok := a.flows["state"]; ok {
		t.Fatal("state 必须原子消费")
	}
}
func TestStandardOIDCUserinfoStableIdentity(t *testing.T) {
	stableID := "immutable-id"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/userinfo":
			json.NewEncoder(w).Encode(map[string]any{"sub": "immutable-id", "preferred_username": "user", "name": "Display Name"})
		case "/api/get-user":
			json.NewEncoder(w).Encode(map[string]any{"status": "ok", "data": CentralUser{ID: stableID, Owner: "sx", Name: "user", Display: "Display Name"}})
		case "/api/get-roles":
			json.NewEncoder(w).Encode(map[string]any{"status": "ok", "data": []CentralRole{}})
		case "/api/get-permissions":
			json.NewEncoder(w).Encode(map[string]any{"status": "ok", "data": []Permission{{Owner: "sx", Name: "RevenueStatisticsSystem.Access", Users: []string{"sx/user"}, Resources: []string{"app-revenuestatistics"}, State: "Approved", Effect: "Allow", IsEnabled: true}}})
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	t.Setenv("CASDOOR_INTERNAL", server.URL)
	t.Setenv("OIDC_ORGANIZATION", "sx")
	app := App{client: server.Client()}
	id, err := app.identity(Session{Token: "test", Sub: "immutable-id"})
	if err != nil || id.Name != "user" || id.Display != "Display Name" || id.Super {
		t.Fatalf("standard OIDC identity rejected: %v", err)
	}
	stableID = "replacement-account-id"
	if _, err = app.identity(Session{Token: "test", Sub: "immutable-id"}); err == nil {
		t.Fatal("recreated username accepted as old identity")
	}
}
