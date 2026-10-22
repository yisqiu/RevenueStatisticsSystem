package main

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const centralSuperRole = "super_admin"

type PermissionOption struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

var businessPermissions = []PermissionOption{
	{"Revenue.Read", "查看统计及导出"},
	{"Revenue.Write", "日报新增编辑及Excel导入"},
	{"Integration.Manage", "系统接口管理"},
}

type SystemRole struct {
	Code         string   `json:"code"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Permissions  []string `json:"permissions"`
	Enabled      bool     `json:"enabled"`
	BuiltIn      bool     `json:"builtIn"`
	IsSuperAdmin bool     `json:"isSuperAdmin"`
	Version      int64    `json:"version"`
	Members      int      `json:"members"`
}
type SystemUser struct {
	Subject        string   `json:"subject"`
	Username       string   `json:"username"`
	DisplayName    string   `json:"displayName"`
	Phone          string   `json:"phone"`
	LastLogin      string   `json:"lastLogin"`
	Roles          []string `json:"roles"`
	Enabled        bool     `json:"enabled"`
	Version        int64    `json:"version"`
	IsSuperAdmin   bool     `json:"isSuperAdmin"`
	CentralAllowed bool     `json:"centralAllowed"`
	SyncedAt       string   `json:"syncedAt"`
}
type DirectoryUser struct {
	CentralUser
	Phone string `json:"phone"`
}
type systemError struct {
	status  int
	message string
}

func (e systemError) Error() string { return e.message }
func (a *App) systemError(w http.ResponseWriter, err error) {
	var public systemError
	if errors.As(err, &public) {
		fail(w, public.status, public.message)
	} else {
		a.databaseError(w, err)
	}
}

var errLocalAccess = errors.New("本系统用户已停用或尚未分配有效角色，请联系管理员")

func rolePermissions(roles []SystemRole) ([]string, []string) {
	permissions, names := []string{}, []string{}
	for _, role := range roles {
		if !role.Enabled || role.IsSuperAdmin {
			continue
		}
		names = append(names, role.Name)
		for _, option := range businessPermissions {
			if contains(role.Permissions, option.Key) && !contains(permissions, option.Key) {
				permissions = append(permissions, option.Key)
			}
		}
	}
	return permissions, names
}
func (a *App) localRole(ctx context.Context, id Identity) (Identity, error) {
	if id.Super {
		return id, nil
	}
	var enabled int
	err := a.db.QueryRowContext(ctx, "SELECT ENABLED FROM "+schema+"SYSTEM_USERS WHERE SUBJECT=?", id.Sub).Scan(&enabled)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && enabled != 1) {
		return id, errLocalAccess
	}
	if err != nil {
		return id, err
	}
	rows, err := a.db.QueryContext(ctx, "SELECT R.NAME,R.PERMISSIONS FROM "+schema+"SYSTEM_ROLES R JOIN "+schema+"SYSTEM_USER_ROLES U ON U.ROLE_CODE=R.CODE WHERE U.SUBJECT=? AND R.ENABLED=1 ORDER BY R.CODE", id.Sub)
	if err != nil {
		return id, err
	}
	defer rows.Close()
	roles := []SystemRole{}
	for rows.Next() {
		role := SystemRole{Enabled: true}
		var raw string
		if err = rows.Scan(&role.Name, &raw); err != nil {
			return id, err
		}
		if err = json.Unmarshal([]byte(raw), &role.Permissions); err != nil {
			return id, err
		}
		roles = append(roles, role)
	}
	if err = rows.Err(); err != nil {
		return id, err
	}
	id.Permissions, id.RoleNames = rolePermissions(roles)
	if !contains(id.Permissions, "Revenue.Read") {
		return id, errLocalAccess
	}
	id.Role = strings.Join(id.RoleNames, "、")
	return id, nil
}
func (a *App) invalidateUsers(subjects []string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for key, session := range a.sessions {
		if contains(subjects, session.Sub) {
			delete(a.sessions, key)
		}
	}
}
func centralRolesFor(user string, roles []CentralRole) map[string]bool {
	matched := map[string]bool{}
	owner := strings.SplitN(user, "/", 2)[0]
	for changed := true; changed; {
		changed = false
		for _, role := range roles {
			key := role.Owner + "/" + role.Name
			if matched[key] || !role.IsEnabled || role.Owner != owner {
				continue
			}
			member := contains(role.Users, user)
			for _, child := range role.Roles {
				if matched[child] {
					member = true
				}
			}
			if member {
				matched[key] = true
				changed = true
			}
		}
	}
	return matched
}

// Read the current application-authorized directory; never write central accounts or permissions.
func (a *App) systemDirectory() ([]SystemUser, error) {
	org := env("OIDC_ORGANIZATION", "sx")
	var users struct {
		Status string          `json:"status"`
		Data   []DirectoryUser `json:"data"`
	}
	var roles struct {
		Status string        `json:"status"`
		Data   []CentralRole `json:"data"`
	}
	var permissions struct {
		Status string       `json:"status"`
		Data   []Permission `json:"data"`
	}
	for _, item := range []struct {
		path   string
		out    any
		status *string
	}{
		{"/api/get-users?owner=" + url.QueryEscape(org), &users, &users.Status},
		{"/api/get-roles?owner=" + url.QueryEscape(org), &roles, &roles.Status},
		{"/api/get-permissions?owner=" + url.QueryEscape(org), &permissions, &permissions.Status},
	} {
		if err := a.centralJSON(item.path, "", item.out); err != nil || *item.status != "ok" {
			return nil, systemError{503, "统一账号目录暂不可用，请稍后重试"}
		}
	}
	result := []SystemUser{}
	for _, u := range users.Data {
		if u.ID == "" || u.Owner != org || u.IsDeleted || u.IsForbidden {
			continue
		}
		access, super, denied := evaluatePermissions(permissions.Data, org+"/"+u.Name, centralRolesFor(org+"/"+u.Name, roles.Data), time.Now())
		if denied || (!access && !super) {
			continue
		}
		display := u.Display
		if display == "" {
			display = u.Name
		}
		result = append(result, SystemUser{Subject: u.ID, Username: u.Name, DisplayName: display, Phone: u.Phone, Roles: []string{}, IsSuperAdmin: super, CentralAllowed: !u.IsForbidden})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Username < result[j].Username })
	return result, nil
}
func (a *App) system(w http.ResponseWriter, r *http.Request, actor Identity, path []string) {
	if !actor.Super {
		fail(w, 403, "系统管理需要本系统超级管理员权限")
		return
	}
	if len(path) == 1 {
		switch path[0] {
		case "bootstrap":
			if r.Method == "GET" {
				a.systemBootstrap(w, r)
				return
			}
		case "sync-users":
			if r.Method == "POST" {
				a.syncSystemUsers(w, r, actor)
				return
			}
		case "users":
			if r.Method == "POST" {
				a.saveSystemUser(w, r, actor, "", false)
				return
			}
		case "roles":
			if r.Method == "POST" {
				a.saveSystemRole(w, r, actor, "")
				return
			}
		case "logs":
			if r.Method == "GET" {
				a.systemLogs(w, r, false)
				return
			}
		}
	}
	if len(path) == 2 {
		if path[0] == "roles" && (r.Method == "PUT" || r.Method == "DELETE") {
			a.saveSystemRole(w, r, actor, path[1])
			return
		}
		if path[0] == "users" && r.Method == "PUT" {
			a.saveSystemUser(w, r, actor, path[1], false)
			return
		}
		if path[0] == "logs" && path[1] == "export" && r.Method == "GET" {
			a.systemLogs(w, r, true)
			return
		}
	}
	if len(path) == 3 && path[0] == "users" && path[2] == "status" && r.Method == "PUT" {
		a.saveSystemUser(w, r, actor, path[1], true)
		return
	}
	fail(w, 404, "接口不存在")
}
func (a *App) systemBootstrap(w http.ResponseWriter, r *http.Request) {
	directory, err := a.systemDirectory()
	if err != nil {
		a.systemError(w, err)
		return
	}
	current := map[string]SystemUser{}
	for _, user := range directory {
		current[user.Subject] = user
	}
	roles := []SystemRole{{Code: centralSuperRole, Name: "超级管理员", Description: "由统一身份认证指定，管理本系统全部功能", Enabled: true, BuiltIn: true, IsSuperAdmin: true, Permissions: []string{"*"}}}
	rows, err := a.db.QueryContext(r.Context(), "SELECT CODE,NAME,DESCRIPTION,PERMISSIONS,ENABLED,BUILT_IN,VERSION FROM "+schema+"SYSTEM_ROLES ORDER BY BUILT_IN DESC,CODE")
	if err != nil {
		a.databaseError(w, err)
		return
	}
	for rows.Next() {
		var role SystemRole
		var raw string
		var enabled, builtIn int
		if err = rows.Scan(&role.Code, &role.Name, &role.Description, &raw, &enabled, &builtIn, &role.Version); err == nil {
			err = json.Unmarshal([]byte(raw), &role.Permissions)
		}
		if err != nil {
			rows.Close()
			a.databaseError(w, err)
			return
		}
		role.Enabled = enabled == 1
		role.BuiltIn = builtIn == 1
		roles = append(roles, role)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		a.databaseError(w, err)
		return
	}
	bindings := map[string][]string{}
	rows, err = a.db.QueryContext(r.Context(), "SELECT SUBJECT,ROLE_CODE FROM "+schema+"SYSTEM_USER_ROLES ORDER BY ROLE_CODE")
	if err != nil {
		a.databaseError(w, err)
		return
	}
	for rows.Next() {
		var sub, code string
		if err = rows.Scan(&sub, &code); err != nil {
			break
		}
		bindings[sub] = append(bindings[sub], code)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		a.databaseError(w, err)
		return
	}
	rows, err = a.db.QueryContext(r.Context(), "SELECT I.SUBJECT,I.USERNAME,COALESCE(I.DISPLAY_NAME,''),COALESCE(TO_CHAR(I.LAST_LOGIN,'YYYY-MM-DD HH24:MI:SS'),''),COALESCE(U.ENABLED,1),COALESCE(U.VERSION,0),COALESCE(U.PHONE,''),COALESCE(U.SYNCED_AT,'') FROM "+schema+"IDENTITIES I LEFT JOIN "+schema+"SYSTEM_USERS U ON U.SUBJECT=I.SUBJECT ORDER BY I.USERNAME")
	if err != nil {
		a.databaseError(w, err)
		return
	}
	users := []SystemUser{}
	for rows.Next() {
		var user SystemUser
		var enabled int
		if err = rows.Scan(&user.Subject, &user.Username, &user.DisplayName, &user.LastLogin, &enabled, &user.Version, &user.Phone, &user.SyncedAt); err != nil {
			break
		}
		user.Enabled = enabled == 1
		user.Roles = append([]string{}, bindings[user.Subject]...)
		central := current[user.Subject]
		if central.Subject != "" {
			user.Phone = central.Phone
		}
		user.IsSuperAdmin = central.IsSuperAdmin
		user.CentralAllowed = central.CentralAllowed
		if user.IsSuperAdmin {
			user.Roles = []string{centralSuperRole}
			user.Enabled = true
		}
		for i := range roles {
			if contains(user.Roles, roles[i].Code) {
				roles[i].Members++
			}
		}
		users = append(users, user)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		a.databaseError(w, err)
		return
	}
	jsonOut(w, 200, map[string]any{"users": users, "roles": roles, "directory": directory, "permissions": businessPermissions})
}
func systemAudit(ctx context.Context, tx *sql.Tx, module, action, record, detail string, actor Identity) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO "+schema+"SYSTEM_LOGS(ID,MODULE,ACTION,RECORD_ID,ACTOR,DETAIL,CREATED_AT) VALUES(?,?,?,?,?,?,?)", auditID(), module, action, record, actor.Name, detail, stamp())
	return err
}
func upsertDirectoryIdentity(ctx context.Context, tx *sql.Tx, user SystemUser) error {
	_, err := tx.ExecContext(ctx, "MERGE INTO "+schema+"IDENTITIES T USING (SELECT ? AS SUBJECT FROM DUAL) S ON (T.SUBJECT=S.SUBJECT) WHEN MATCHED THEN UPDATE SET T.USERNAME=?,T.DISPLAY_NAME=? WHEN NOT MATCHED THEN INSERT(SUBJECT,ISSUER,ORGANIZATION,USERNAME,DISPLAY_NAME,LAST_LOGIN) VALUES(?,?,?,?,?,NULL)", user.Subject, user.Username, user.DisplayName, user.Subject, issuer(), env("OIDC_ORGANIZATION", "sx"), user.Username, user.DisplayName)
	return err
}
func (a *App) syncSystemUsers(w http.ResponseWriter, r *http.Request, actor Identity) {
	directory, err := a.systemDirectory()
	if err != nil {
		a.systemError(w, err)
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		a.databaseError(w, err)
		return
	}
	defer tx.Rollback()
	for _, user := range directory {
		if err = upsertDirectoryIdentity(r.Context(), tx, user); err != nil {
			a.databaseError(w, err)
			return
		}
		_, err = tx.ExecContext(r.Context(), "MERGE INTO "+schema+"SYSTEM_USERS T USING (SELECT ? AS SUBJECT FROM DUAL) S ON (T.SUBJECT=S.SUBJECT) WHEN MATCHED THEN UPDATE SET T.PHONE=?,T.SYNCED_AT=? WHEN NOT MATCHED THEN INSERT(SUBJECT,ENABLED,VERSION,PHONE,SYNCED_AT) VALUES(?,1,1,?,?)", user.Subject, user.Phone, stamp(), user.Subject, user.Phone, stamp())
		if err != nil {
			a.databaseError(w, err)
			return
		}
	}
	if err = systemAudit(r.Context(), tx, "用户管理", "同步用户", "directory", fmt.Sprintf("同步 %d 个具有本应用许可的统一账号；保留本地状态和角色", len(directory)), actor); err == nil {
		err = tx.Commit()
	}
	if err != nil {
		a.databaseError(w, err)
		return
	}
	jsonOut(w, 200, map[string]any{"count": len(directory), "message": "用户目录已同步"})
}

type saveUserRequest struct {
	Subject string   `json:"subject"`
	Roles   []string `json:"roles"`
	Enabled bool     `json:"enabled"`
	Version int64    `json:"version"`
}

func (a *App) saveSystemUser(w http.ResponseWriter, r *http.Request, actor Identity, subject string, statusOnly bool) {
	var input saveUserRequest
	if decode(w, r, &input) != nil {
		fail(w, 400, "请求格式无效")
		return
	}
	creating := subject == ""
	if creating {
		subject = input.Subject
	}
	if subject == "" || len(subject) > 200 || input.Version < 0 || len(input.Roles) > 50 || contains(input.Roles, centralSuperRole) {
		fail(w, 400, "用户或业务角色无效")
		return
	}
	if creating && len(input.Roles) == 0 {
		fail(w, 400, "请选择至少一个本系统角色")
		return
	}
	directory, err := a.systemDirectory()
	if err != nil {
		a.systemError(w, err)
		return
	}
	var target *SystemUser
	for i := range directory {
		if directory[i].Subject == subject {
			target = &directory[i]
			break
		}
	}
	if subject == actor.Sub || (target != nil && target.IsSuperAdmin) {
		fail(w, 403, "超级管理员由统一身份认证指定")
		return
	}
	if creating && (target == nil || !target.CentralAllowed) {
		fail(w, 400, "请选择具有本应用许可的有效统一账号")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		a.databaseError(w, err)
		return
	}
	defer tx.Rollback()
	if creating {
		if err = upsertDirectoryIdentity(r.Context(), tx, *target); err != nil {
			a.databaseError(w, err)
			return
		}
	}
	var username string
	err = tx.QueryRowContext(r.Context(), "SELECT USERNAME FROM "+schema+"IDENTITIES WHERE SUBJECT=?", subject).Scan(&username)
	if errors.Is(err, sql.ErrNoRows) {
		fail(w, 404, "用户不存在，请先同步用户")
		return
	}
	if err != nil {
		a.databaseError(w, err)
		return
	}
	var version int64
	var enabled int
	err = tx.QueryRowContext(r.Context(), "SELECT VERSION,ENABLED FROM "+schema+"SYSTEM_USERS WHERE SUBJECT=? FOR UPDATE", subject).Scan(&version, &enabled)
	if errors.Is(err, sql.ErrNoRows) && input.Version == 0 {
		version = 0
		enabled = 1
		_, err = tx.ExecContext(r.Context(), "INSERT INTO "+schema+"SYSTEM_USERS(SUBJECT,ENABLED,VERSION) VALUES(?,1,0)", subject)
	}
	if err != nil {
		a.databaseError(w, err)
		return
	}
	if input.Version != version {
		fail(w, 409, "用户已被其他人修改，请刷新后重试")
		return
	}
	action, detail := "分配角色", username+" → "+strings.Join(input.Roles, "、")
	if statusOnly {
		enabled = 0
		action = "停用用户"
		if input.Enabled {
			enabled = 1
			action = "启用用户"
		}
		detail = username
	} else {
		codes := append([]string{}, input.Roles...)
		sort.Strings(codes)
		for i, code := range codes {
			if i > 0 && code == codes[i-1] {
				fail(w, 400, "业务角色重复")
				return
			}
			var active int
			err = tx.QueryRowContext(r.Context(), "SELECT ENABLED FROM "+schema+"SYSTEM_ROLES WHERE CODE=? FOR UPDATE", code).Scan(&active)
			if errors.Is(err, sql.ErrNoRows) || (err == nil && active != 1) {
				fail(w, 400, "所选角色不存在或已停用")
				return
			}
			if err != nil {
				a.databaseError(w, err)
				return
			}
		}
		if _, err = tx.ExecContext(r.Context(), "DELETE FROM "+schema+"SYSTEM_USER_ROLES WHERE SUBJECT=?", subject); err != nil {
			a.databaseError(w, err)
			return
		}
		for _, code := range codes {
			if _, err = tx.ExecContext(r.Context(), "INSERT INTO "+schema+"SYSTEM_USER_ROLES(SUBJECT,ROLE_CODE) VALUES(?,?)", subject, code); err != nil {
				a.databaseError(w, err)
				return
			}
		}
		if creating {
			action = "新增系统用户"
		}
	}
	_, err = tx.ExecContext(r.Context(), "UPDATE "+schema+"SYSTEM_USERS SET ENABLED=?,VERSION=VERSION+1 WHERE SUBJECT=? AND VERSION=?", enabled, subject, version)
	if err == nil {
		err = systemAudit(r.Context(), tx, "用户管理", action, subject, detail, actor)
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		a.databaseError(w, err)
		return
	}
	a.invalidateUsers([]string{subject})
	jsonOut(w, 200, map[string]any{"version": version + 1, "message": "用户设置已保存"})
}

var roleCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,49}$`)

func validateSystemRole(role *SystemRole) error {
	role.Name = strings.TrimSpace(role.Name)
	role.Description = strings.TrimSpace(role.Description)
	if !roleCodePattern.MatchString(role.Code) || role.Code == centralSuperRole {
		return systemError{400, "角色编码须为小写字母开头的字母、数字或下划线，最多50位"}
	}
	if role.Name == "" || utf8.RuneCountInString(role.Name) > 50 || role.Description == "" || utf8.RuneCountInString(role.Description) > 300 {
		return systemError{400, "请填写角色名称（最多50字）和描述（最多300字）"}
	}
	if len(role.Permissions) == 0 || len(role.Permissions) > len(businessPermissions) {
		return systemError{400, "请选择有效的业务权限"}
	}
	seen := map[string]bool{}
	for _, key := range role.Permissions {
		known := false
		for _, option := range businessPermissions {
			if option.Key == key {
				known = true
			}
		}
		if !known || seen[key] {
			return systemError{400, "权限包含重复或不可分配的项目"}
		}
		seen[key] = true
	}
	if !seen["Revenue.Read"] {
		return systemError{400, "业务角色需保留营收查看权限"}
	}
	return nil
}
func (a *App) saveSystemRole(w http.ResponseWriter, r *http.Request, actor Identity, code string) {
	var role SystemRole
	deleting := r.Method == "DELETE"
	creating := code == ""
	if deleting {
		role.Version, _ = strconv.ParseInt(r.URL.Query().Get("version"), 10, 64)
		role.Code = code
	} else {
		if decode(w, r, &role) != nil {
			fail(w, 400, "请求格式无效")
			return
		}
		if err := validateSystemRole(&role); err != nil {
			a.systemError(w, err)
			return
		}
		if role.IsSuperAdmin || role.BuiltIn || role.Members != 0 {
			fail(w, 400, "请仅提交可编辑的角色字段")
			return
		}
		if !creating && code != role.Code {
			fail(w, 400, "角色编码不可修改")
			return
		}
		code = role.Code
	}
	if code == centralSuperRole {
		fail(w, 403, "超级管理员由统一身份认证指定")
		return
	}
	if !roleCodePattern.MatchString(code) || (!creating && role.Version < 1) {
		fail(w, 400, "缺少有效角色或版本")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		a.databaseError(w, err)
		return
	}
	defer tx.Rollback()
	var version int64
	var builtIn int
	var previousName, previousDescription, previousPermissions string
	var previousEnabled int
	err = tx.QueryRowContext(r.Context(), "SELECT VERSION,BUILT_IN,NAME,DESCRIPTION,PERMISSIONS,ENABLED FROM "+schema+"SYSTEM_ROLES WHERE CODE=? FOR UPDATE", code).Scan(&version, &builtIn, &previousName, &previousDescription, &previousPermissions, &previousEnabled)
	if creating {
		if err == nil {
			fail(w, 409, "角色编码已存在")
			return
		}
		if !errors.Is(err, sql.ErrNoRows) {
			a.databaseError(w, err)
			return
		}
	} else {
		if errors.Is(err, sql.ErrNoRows) {
			fail(w, 404, "角色已删除或不存在")
			return
		}
		if err != nil {
			a.databaseError(w, err)
			return
		}
		if version != role.Version {
			fail(w, 409, "角色已被其他人修改，请刷新后重试")
			return
		}
	}
	subjects := []string{}
	rows, err := tx.QueryContext(r.Context(), "SELECT SUBJECT FROM "+schema+"SYSTEM_USER_ROLES WHERE ROLE_CODE=?", code)
	if err != nil {
		a.databaseError(w, err)
		return
	}
	for rows.Next() {
		var subject string
		if err = rows.Scan(&subject); err != nil {
			break
		}
		subjects = append(subjects, subject)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		a.databaseError(w, err)
		return
	}
	action := "编辑角色"
	if deleting {
		if builtIn == 1 {
			fail(w, 409, "内置角色保留使用，可编辑权限或停用")
			return
		}
		if len(subjects) > 0 {
			fail(w, 409, "角色仍有绑定用户，请先解除绑定")
			return
		}
		_, err = tx.ExecContext(r.Context(), "DELETE FROM "+schema+"SYSTEM_ROLES WHERE CODE=? AND VERSION=?", code, version)
		action = "删除角色"
	} else {
		raw, _ := json.Marshal(role.Permissions)
		enabled := 0
		if role.Enabled {
			enabled = 1
		}
		if creating {
			_, err = tx.ExecContext(r.Context(), "INSERT INTO "+schema+"SYSTEM_ROLES(CODE,NAME,DESCRIPTION,PERMISSIONS,ENABLED,BUILT_IN,VERSION,UPDATED_AT) VALUES(?,?,?,?,?,0,1,?)", code, role.Name, role.Description, string(raw), enabled, stamp())
			action = "新增角色"
		} else {
			_, err = tx.ExecContext(r.Context(), "UPDATE "+schema+"SYSTEM_ROLES SET NAME=?,DESCRIPTION=?,PERMISSIONS=?,ENABLED=?,VERSION=VERSION+1,UPDATED_AT=? WHERE CODE=? AND VERSION=?", role.Name, role.Description, string(raw), enabled, stamp(), code, version)
		}
	}
	if err == nil {
		detail := fmt.Sprintf("%s；原名称：%s；原描述：%s；原状态：%d；原权限：%s", code, previousName, previousDescription, previousEnabled, previousPermissions)
		if !deleting {
			detail += fmt.Sprintf("；现名称：%s；描述：%s；启用：%t；权限：%s", role.Name, role.Description, role.Enabled, strings.Join(role.Permissions, "、"))
		}
		err = systemAudit(r.Context(), tx, "角色管理", action, code, detail, actor)
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		a.databaseError(w, err)
		return
	}
	a.invalidateUsers(subjects)
	jsonOut(w, 200, map[string]any{"code": code, "version": version + 1, "message": "角色设置已保存"})
}

type SystemLog struct {
	ID       string `json:"id"`
	Time     string `json:"time"`
	Actor    string `json:"actor"`
	Module   string `json:"module"`
	Action   string `json:"action"`
	RecordID string `json:"recordId"`
	Detail   string `json:"detail"`
}

const systemLogUnion = "(SELECT A.ID,TO_CHAR(A.CREATED_AT,'YYYY-MM-DD HH24:MI:SS') AS CREATED_AT,COALESCE(I.USERNAME,N.NAME,A.ACTOR,'') AS ACTOR,COALESCE(A.MODULE,M.NAME,'营收日报') AS MODULE,A.ACTION,A.TARGET_ID AS RECORD_ID,COALESCE(A.DETAIL,R.CATEGORY,'') AS DETAIL FROM " + schema + "AUDIT_LOG A LEFT JOIN " + schema + "IDENTITIES I ON I.SUBJECT=A.ACTOR LEFT JOIN " + schema + "INTEGRATIONS N ON A.ACTOR='integration:'||N.ID LEFT JOIN " + schema + "REVENUE_RECORDS R ON R.ID=A.TARGET_ID LEFT JOIN " + schema + "MODULES M ON M.MODULE_KEY=R.MODULE_KEY UNION ALL SELECT ID,CREATED_AT,ACTOR,MODULE,ACTION,RECORD_ID,DETAIL FROM " + schema + "SYSTEM_LOGS) L"

func systemLogFilter(q url.Values) (string, []any, error) {
	clauses := []string{"1=1"}
	args := []any{}
	keyword := strings.TrimSpace(q.Get("keyword"))
	if utf8.RuneCountInString(keyword) > 200 {
		return "", nil, systemError{400, "搜索关键词最多200字"}
	}
	if keyword != "" {
		escaped := "%" + strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(strings.ToLower(keyword)) + "%"
		clauses = append(clauses, "(LOWER(ACTOR) LIKE ? ESCAPE '\\' OR LOWER(MODULE) LIKE ? ESCAPE '\\' OR LOWER(ACTION) LIKE ? ESCAPE '\\' OR LOWER(RECORD_ID) LIKE ? ESCAPE '\\' OR LOWER(DETAIL) LIKE ? ESCAPE '\\')")
		for i := 0; i < 5; i++ {
			args = append(args, escaped)
		}
	}
	if module := q.Get("module"); module != "" {
		if utf8.RuneCountInString(module) > 30 {
			return "", nil, systemError{400, "模块筛选无效"}
		}
		clauses = append(clauses, "MODULE=?")
		args = append(args, module)
	}
	for _, key := range []string{"from", "to"} {
		if value := q.Get(key); value != "" {
			day, err := time.Parse("2006-01-02", value)
			if err != nil {
				return "", nil, systemError{400, "日期格式无效"}
			}
			if key == "from" {
				clauses = append(clauses, "CREATED_AT>=?")
				args = append(args, day.Format("2006-01-02")+" 00:00:00")
			} else {
				clauses = append(clauses, "CREATED_AT<?")
				args = append(args, day.AddDate(0, 0, 1).Format("2006-01-02")+" 00:00:00")
			}
		}
	}
	if q.Get("from") != "" && q.Get("to") != "" && q.Get("from") > q.Get("to") {
		return "", nil, systemError{400, "开始日期应早于结束日期"}
	}
	return " WHERE " + strings.Join(clauses, " AND "), args, nil
}
func csvSafe(value string) string {
	trimmed := strings.TrimLeft(value, " \t\r\n")
	if trimmed != "" && strings.ContainsAny(trimmed[:1], "=+-@") {
		return "'" + value
	}
	return value
}
func (a *App) systemLogs(w http.ResponseWriter, r *http.Request, export bool) {
	where, args, err := systemLogFilter(r.URL.Query())
	if err != nil {
		a.systemError(w, err)
		return
	}
	query := "SELECT ID,CREATED_AT,ACTOR,MODULE,ACTION,RECORD_ID,DETAIL FROM " + systemLogUnion + where + " ORDER BY CREATED_AT DESC,ID DESC"
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	if page > 1000000 {
		fail(w, 400, "页码无效")
		return
	}
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))
	if size < 1 {
		size = 15
	}
	if size > 100 {
		size = 100
	}
	total := 0
	if !export {
		if err = a.db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM "+systemLogUnion+where, args...).Scan(&total); err != nil {
			a.databaseError(w, err)
			return
		}
		query += " OFFSET ? ROWS FETCH NEXT ? ROWS ONLY"
		args = append(args, (page-1)*size, size)
	}
	rows, err := a.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		a.databaseError(w, err)
		return
	}
	defer rows.Close()
	items := []SystemLog{}
	for rows.Next() {
		var item SystemLog
		if err = rows.Scan(&item.ID, &item.Time, &item.Actor, &item.Module, &item.Action, &item.RecordID, &item.Detail); err != nil {
			a.databaseError(w, err)
			return
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		a.databaseError(w, err)
		return
	}
	if export {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="system-operation-logs.csv"`)
		w.Header().Set("Cache-Control", "no-store")
		fmt.Fprint(w, "\ufeff")
		out := csv.NewWriter(w)
		_ = out.Write([]string{"操作时间", "操作人", "模块", "操作", "记录编号", "详情"})
		for _, item := range items {
			values := []string{item.Time, item.Actor, item.Module, item.Action, item.RecordID, item.Detail}
			for i := range values {
				values[i] = csvSafe(values[i])
			}
			_ = out.Write(values)
		}
		out.Flush()
		return
	}
	jsonOut(w, 200, map[string]any{"items": items, "total": total, "page": page, "size": size})
}
