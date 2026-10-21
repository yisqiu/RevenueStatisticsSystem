package main

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Claims struct {
	SessionRevision uint64          `json:"sessionRevision"`
	Iss             string          `json:"iss"`
	Sub             string          `json:"sub"`
	Aud             json.RawMessage `json:"aud"`
	Azp             string          `json:"azp"`
	Exp             int64           `json:"exp"`
	Iat             int64           `json:"iat"`
	Nbf             int64           `json:"nbf"`
	Nonce           string          `json:"nonce"`
	Owner           string          `json:"owner"`
	Name            string          `json:"name"`
}
type CentralUser struct {
	ID          string `json:"id"`
	Preferred   string `json:"preferred_username"`
	Sub         string `json:"sub"`
	Owner       string `json:"owner"`
	Name        string `json:"name"`
	Display     string `json:"displayName"`
	IsForbidden bool   `json:"isForbidden"`
	IsDeleted   bool   `json:"isDeleted"`
}
type CentralRole struct {
	Owner     string   `json:"owner"`
	Name      string   `json:"name"`
	Users     []string `json:"users"`
	Roles     []string `json:"roles"`
	IsEnabled bool     `json:"isEnabled"`
}
type Permission struct {
	Owner     string   `json:"owner"`
	Name      string   `json:"name"`
	Users     []string `json:"users"`
	Roles     []string `json:"roles"`
	Resources []string `json:"resources"`
	State     string   `json:"state"`
	Effect    string   `json:"effect"`
	IsEnabled bool     `json:"isEnabled"`
	StartTime string   `json:"startTime"`
	EndTime   string   `json:"endTime"`
}

func issuer() string { return strings.TrimRight(env("OIDC_ISSUER", "http://cd.libeasy.cn:8012"), "/") }
func central() string {
	return strings.TrimRight(env("CASDOOR_INTERNAL", "http://127.0.0.1:8012"), "/")
}
func clientID() string { return secret("OIDC_CLIENT_ID") }
func (a *App) centralJSON(path, token string, out any) error {
	req, e := http.NewRequest("GET", central()+path, nil)
	if e != nil {
		return e
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	} else {
		q := req.URL.Query()
		q.Set("clientId", clientID())
		q.Set("clientSecret", secret("OIDC_CLIENT_SECRET"))
		req.URL.RawQuery = q.Encode()
	}
	resp, e := a.client.Do(req)
	if e != nil {
		return errors.New("统一认证暂不可用")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return errors.New("统一认证请求被拒绝")
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(out)
}
func (a *App) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		fail(w, 405, "方法不支持")
		return
	}
	if clientID() == "" || secret("OIDC_CLIENT_SECRET") == "" {
		fail(w, 503, "统一认证尚未配置")
		return
	}
	state, bind, verifier, nonce := random(), random(), random(), random()
	redirect := r.URL.Query().Get("redirect")
	if !strings.HasPrefix(redirect, "/") || strings.HasPrefix(redirect, "//") {
		redirect = "/dashboard"
	}
	a.mu.Lock()
	if len(a.flows) > 1000 {
		a.mu.Unlock()
		fail(w, 429, "登录请求过多，请稍后重试")
		return
	}
	a.flows[state] = Flow{Bind: hashed(bind), Verifier: verifier, Nonce: nonce, Redirect: redirect, Exp: time.Now().Add(5 * time.Minute)}
	a.mu.Unlock()
	cookie(w, "revenue_login", bind, 300)
	h := sha256.Sum256([]byte(verifier))
	q := url.Values{"client_id": {clientID()}, "response_type": {"code"}, "redirect_uri": {env("PUBLIC_URL", "http://cd.libeasy.cn:8027") + "/api/auth/callback"}, "scope": {"openid profile email"}, "state": {state}, "nonce": {nonce}, "code_challenge": {base64.RawURLEncoding.EncodeToString(h[:])}, "code_challenge_method": {"S256"}}
	http.Redirect(w, r, issuer()+"/login/oauth/authorize?"+q.Encode(), 302)
}
func (a *App) callback(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		fail(w, 405, "方法不支持")
		return
	}
	state := r.URL.Query().Get("state")
	a.mu.Lock()
	f, ok := a.flows[state]
	delete(a.flows, state)
	a.mu.Unlock()
	c, e := r.Cookie("revenue_login")
	cookie(w, "revenue_login", "", -1)
	if !ok || time.Now().After(f.Exp) || e != nil || hashed(c.Value) != f.Bind || r.URL.Query().Get("code") == "" || r.URL.Query().Get("error") != "" {
		a.authError(w, 400, "统一认证回调校验失败，请重新登录")
		return
	}
	form := url.Values{"client_id": {clientID()}, "client_secret": {secret("OIDC_CLIENT_SECRET")}, "grant_type": {"authorization_code"}, "code": {r.URL.Query().Get("code")}, "code_verifier": {f.Verifier}, "redirect_uri": {env("PUBLIC_URL", "http://cd.libeasy.cn:8027") + "/api/auth/callback"}}
	req, _ := http.NewRequest("POST", central()+"/api/login/oauth/access_token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, e := a.client.Do(req)
	if e != nil {
		a.authError(w, 503, "统一认证暂不可用")
		return
	}
	defer resp.Body.Close()
	var tokens struct {
		Access string `json:"access_token"`
		ID     string `json:"id_token"`
	}
	if resp.StatusCode != 200 || json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&tokens) != nil || tokens.Access == "" || tokens.ID == "" {
		a.authError(w, 401, "统一认证令牌交换失败")
		return
	}
	claims, e := a.verifyJWT(tokens.ID, f.Nonce)
	if e != nil {
		a.authError(w, 401, "统一认证身份凭证校验失败")
		return
	}
	access, e := a.verifyJWT(tokens.Access, "")
	if e != nil || access.Sub != claims.Sub {
		a.authError(w, 401, "统一认证访问凭证校验失败")
		return
	}
	s := Session{Token: tokens.Access, Sub: claims.Sub, Exp: time.Unix(access.Exp, 0), Revision: access.SessionRevision}
	if s.Exp.After(time.Now().Add(8 * time.Hour)) {
		s.Exp = time.Now().Add(8 * time.Hour)
	}
	id, e := a.identity(s)
	if e != nil {
		a.authError(w, 403, "此账号未获营收统计系统访问授权，请联系管理员")
		return
	}
	s.Name = id.Name
	s.Display = id.Display
	_, e = a.db.ExecContext(r.Context(), "MERGE INTO REVENUESTATISTICSSYSTEM.IDENTITIES T USING (SELECT ? AS SUBJECT FROM DUAL) S ON (T.SUBJECT=S.SUBJECT) WHEN MATCHED THEN UPDATE SET T.USERNAME=?,T.DISPLAY_NAME=?,T.LAST_LOGIN=CURRENT_TIMESTAMP WHEN NOT MATCHED THEN INSERT(SUBJECT,ISSUER,ORGANIZATION,USERNAME,DISPLAY_NAME) VALUES(?,?,?,?,?)", claims.Sub, id.Name, id.Display, claims.Sub, issuer(), env("OIDC_ORGANIZATION", "sx"), id.Name, id.Display)
	if e != nil {
		a.authError(w, 500, "本地身份保存失败")
		return
	}
	sid := random()
	a.mu.Lock()
	a.sessions[hashed(sid)] = s
	a.mu.Unlock()
	cookie(w, "revenue_session", sid, int(time.Until(s.Exp).Seconds()))
	http.Redirect(w, r, "/oidc-callback?success=1&redirect="+url.QueryEscape(f.Redirect), 303)
}
func (a *App) authError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	io.WriteString(w, "<!doctype html><html lang='zh'><meta charset='utf-8'><title>营收统计系统 · 登录</title><body style='font-family:system-ui;max-width:600px;margin:100px auto'><h1>统一身份认证</h1><p>"+msg+"</p><a href='/api/auth/login'>重新登录</a></body></html>")
}
func (a *App) session(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		fail(w, 405, "方法不支持")
		return
	}
	s, ok := a.getSession(r)
	if !ok {
		fail(w, 401, "尚未登录")
		return
	}
	id, e := a.identity(s)
	if e != nil {
		fail(w, 403, "应用授权无效")
		return
	}
	jsonOut(w, 200, map[string]any{"access_token": "session", "redirectPath": "/dashboard", "username": id.Name})
}
func (a *App) logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		fail(w, 405, "方法不支持")
		return
	}
	if !originOK(r) {
		fail(w, 403, "请求来源校验失败")
		return
	}
	a.mu.Lock()
	delete(a.sessions, hashedCookie(r))
	a.mu.Unlock()
	cookie(w, "revenue_session", "", -1)
	jsonOut(w, 200, map[string]bool{"success": true})
}
func (a *App) verifyJWT(token, nonce string) (Claims, error) {
	var c Claims
	p := strings.Split(token, ".")
	if len(p) != 3 {
		return c, errors.New("JWT 格式无效")
	}
	header, err := base64.RawURLEncoding.DecodeString(p[0])
	if err != nil {
		return c, err
	}
	var h struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if json.Unmarshal(header, &h) != nil || h.Alg != "RS256" || h.Kid == "" {
		return c, errors.New("不允许的签名算法")
	}
	var jwks struct {
		Keys []struct {
			Kty string `json:"kty"`
			Kid string `json:"kid"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	req, _ := http.NewRequest("GET", central()+"/.well-known/jwks", nil)
	resp, e := a.client.Do(req)
	if e != nil {
		return c, errors.New("签名公钥不可用")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 || json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&jwks) != nil {
		return c, errors.New("签名公钥无效")
	}
	sig, e := base64.RawURLEncoding.DecodeString(p[2])
	if e != nil {
		return c, e
	}
	sum := sha256.Sum256([]byte(p[0] + "." + p[1]))
	verified := false
	for _, k := range jwks.Keys {
		if k.Kid != h.Kid || k.Kty != "RSA" {
			continue
		}
		n, ne := base64.RawURLEncoding.DecodeString(k.N)
		eb, ee := base64.RawURLEncoding.DecodeString(k.E)
		if ne != nil || ee != nil || len(n) < 256 || len(eb) > 4 {
			continue
		}
		exponent := 0
		for _, b := range eb {
			exponent = exponent*256 + int(b)
		}
		if rsa.VerifyPKCS1v15(&rsa.PublicKey{N: new(big.Int).SetBytes(n), E: exponent}, crypto.SHA256, sum[:], sig) == nil {
			verified = true
			break
		}
	}
	if !verified {
		return c, errors.New("签名校验失败")
	}
	payload, e := base64.RawURLEncoding.DecodeString(p[1])
	if e != nil {
		return c, e
	}
	if json.Unmarshal(payload, &c) != nil {
		return c, errors.New("凭证内容无效")
	}
	if e = validateClaims(c, clientID(), issuer(), nonce, time.Now()); e != nil {
		return c, e
	}
	return c, nil
}
func validateClaims(c Claims, client, expectedIssuer, nonce string, now time.Time) error {
	if c.Iss != expectedIssuer || c.Sub == "" || c.Exp <= now.Unix() || c.Iat > now.Unix()+60 || c.Nbf > now.Unix()+60 {
		return errors.New("来源或有效期无效")
	}
	var aud string
	audOK := false
	multi := false
	if json.Unmarshal(c.Aud, &aud) == nil {
		audOK = aud == client
	} else {
		var list []string
		if json.Unmarshal(c.Aud, &list) == nil {
			audOK = contains(list, client)
			multi = len(list) > 1
		}
	}
	if !audOK || (c.Azp != "" && c.Azp != client) || (multi && c.Azp != client) {
		return errors.New("受众无效")
	}
	if nonce != "" && c.Nonce != nonce {
		return errors.New("nonce 校验失败")
	}
	return nil
}
func (a *App) identity(s Session) (Identity, error) {
	var id Identity
	var u CentralUser
	if e := a.centralJSON("/api/userinfo", s.Token, &u); e != nil {
		return id, e
	}
	org := env("OIDC_ORGANIZATION", "sx")
	if u.Preferred != "" {
		u.Name = u.Preferred
		u.Owner = org
	}
	if u.Owner != org || u.Name == "" || u.IsForbidden || u.IsDeleted {
		return id, errors.New("账号不可用")
	}
	if u.Sub != "" && u.Sub != s.Sub {
		return id, errors.New("身份不一致")
	}
	var userResult struct {
		Status string      `json:"status"`
		Data   CentralUser `json:"data"`
	}
	if e := a.centralJSON("/api/get-user?id="+url.QueryEscape(org+"/"+u.Name), "", &userResult); e != nil || userResult.Status != "ok" || userResult.Data.ID == "" || userResult.Data.IsForbidden || userResult.Data.IsDeleted {
		return id, errors.New("账号已停用")
	}
	if userResult.Data.ID != s.Sub || userResult.Data.Owner != org || userResult.Data.Name != u.Name {
		return id, errors.New("稳定身份不一致")
	}
	var rolesResult struct {
		Status string        `json:"status"`
		Data   []CentralRole `json:"data"`
	}
	if e := a.centralJSON("/api/get-roles?owner="+url.QueryEscape(org), "", &rolesResult); e != nil || rolesResult.Status != "ok" {
		return id, errors.New("角色目录不可用")
	}
	roles := map[string]bool{}
	for changed := true; changed; {
		changed = false
		for _, r := range rolesResult.Data {
			key := r.Owner + "/" + r.Name
			if roles[key] || !r.IsEnabled || r.Owner != org {
				continue
			}
			matches := contains(r.Users, org+"/"+u.Name)
			for _, child := range r.Roles {
				if roles[child] {
					matches = true
				}
			}
			if matches {
				roles[key] = true
				changed = true
			}
		}
	}
	var permissionsResult struct {
		Status string       `json:"status"`
		Data   []Permission `json:"data"`
	}
	if e := a.centralJSON("/api/get-permissions?owner="+url.QueryEscape(org), "", &permissionsResult); e != nil || permissionsResult.Status != "ok" {
		return id, errors.New("权限目录不可用")
	}
	access, super, denied := evaluatePermissions(permissionsResult.Data, org+"/"+u.Name, roles, time.Now())
	if denied || (!access && !super) {
		return id, errors.New("无应用许可")
	}
	display := userResult.Data.Display
	if display == "" {
		display = u.Name
	}
	role := "应用用户"
	if super {
		role = "超级用户"
	}
	return Identity{Sub: s.Sub, Name: u.Name, Display: display, Super: super, Role: role}, nil
}
func evaluatePermissions(ps []Permission, user string, roles map[string]bool, now time.Time) (access, super, denied bool) {
	for _, p := range ps {
		if !p.IsEnabled || p.Owner != "sx" || p.State != "Approved" || !contains(p.Resources, "app-revenuestatistics") {
			continue
		}
		if p.Name != "RevenueStatisticsSystem.Access" && p.Name != "RevenueStatisticsSystem.SuperAdmin" {
			continue
		}
		if !activeTime(p.StartTime, p.EndTime, now) {
			continue
		}
		matches := contains(p.Users, user)
		for _, role := range p.Roles {
			if roles[role] {
				matches = true
			}
		}
		if !matches {
			continue
		}
		if p.Effect == "Deny" {
			denied = true
			continue
		}
		if p.Effect != "Allow" {
			continue
		}
		if p.Name == "RevenueStatisticsSystem.Access" {
			access = true
		} else {
			super = true
		}
	}
	return
}
func activeTime(start, end string, now time.Time) bool {
	parse := func(s string) (time.Time, error) {
		if s == "" {
			return time.Time{}, nil
		}
		for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05-07:00", "2006-01-02 15:04:05"} {
			if t, e := time.Parse(layout, s); e == nil {
				return t, nil
			}
		}
		return time.Time{}, errors.New("授权日期无效")
	}
	s, e := parse(start)
	if e != nil {
		return false
	}
	t, e := parse(end)
	return e == nil && (s.IsZero() || !now.Before(s)) && (t.IsZero() || now.Before(t))
}
