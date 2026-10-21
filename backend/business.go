package main

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/xuri/excelize/v2"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const schema = "REVENUESTATISTICSSYSTEM."

type Record struct {
	ID       string `json:"id"`
	Module   string `json:"module"`
	Date     string `json:"date"`
	Category string `json:"category"`
	Quantity int    `json:"quantity"`
	Amount   string `json:"amount"`
	Visitors int    `json:"visitors"`
	Source   string `json:"source"`
	Note     string `json:"note"`
	Version  int    `json:"version"`
	Cents    int64  `json:"-"`
}
type Module struct {
	Key          string             `json:"key"`
	Name         string             `json:"name"`
	Icon         string             `json:"icon"`
	Desc         string             `json:"desc"`
	Cols         []string           `json:"cols"`
	Enabled      int                `json:"enabled"`
	Value        float64            `json:"value"`
	Mom          any                `json:"mom"`
	Visitors     int                `json:"visitors"`
	Trend        []float64          `json:"trend"`
	Rows         [][]any            `json:"rows"`
	Struct       map[string]float64 `json:"struct"`
	StructLabels []string           `json:"structLabels"`
}

func cents(s string) (int64, error) {
	p := strings.Split(s, ".")
	if len(p) > 2 || len(p[0]) == 0 || len(p[0]) > 10 {
		return 0, errors.New("金额格式无效")
	}
	for _, c := range s {
		if (c < '0' || c > '9') && c != '.' {
			return 0, errors.New("金额须为非负数")
		}
	}
	n, e := strconv.ParseInt(p[0], 10, 64)
	if e != nil {
		return 0, e
	}
	f := int64(0)
	if len(p) == 2 {
		if len(p[1]) == 0 || len(p[1]) > 2 {
			return 0, errors.New("金额最多两位小数")
		}
		f, e = strconv.ParseInt((p[1] + "00")[:2], 10, 64)
	}
	return n*100 + f, e
}
func money(n int64) string { return fmt.Sprintf("%d.%02d", n/100, n%100) }
func validateRecord(v *Record) error {
	d, e := time.Parse("2006-01-02", v.Date)
	if e != nil || d.Year() < 2000 || d.Year() > 2100 {
		return errors.New("日期格式无效")
	}
	v.Category = strings.TrimSpace(v.Category)
	if len([]rune(v.Category)) == 0 || len([]rune(v.Category)) > 100 {
		return errors.New("品类须为 1-100 字")
	}
	if v.Quantity < 0 || v.Quantity > 100000000 || v.Visitors < 0 || v.Visitors > 100000000 {
		return errors.New("数量超出范围")
	}
	if len([]rune(v.Note)) > 500 || len([]rune(v.Source)) > 100 {
		return errors.New("备注或来源过长")
	}
	v.Cents, e = cents(v.Amount)
	return e
}
func bounds(r *http.Request) (string, string, string, string, error) {
	date := r.URL.Query().Get("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	d, e := time.Parse("2006-01-02", date)
	if e != nil || d.Year() < 2000 || d.Year() > 2100 {
		return "", "", "", "", errors.New("统计日期须在2000至2100年之间")
	}
	start := d
	end := d.AddDate(0, 0, 1)
	prev := d.AddDate(0, 0, -1)
	switch r.URL.Query().Get("range") {
	case "day":
	case "year":
		start = time.Date(d.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
		end = start.AddDate(1, 0, 0)
		prev = start.AddDate(-1, 0, 0)
	case "", "month":
		start = time.Date(d.Year(), d.Month(), 1, 0, 0, 0, 0, time.UTC)
		end = start.AddDate(0, 1, 0)
		prev = start.AddDate(0, -1, 0)
	default:
		return "", "", "", "", errors.New("统计周期无效")
	}
	return start.Format("2006-01-02"), end.Format("2006-01-02"), prev.Format("2006-01-02"), date, nil
}
func (a *App) modules() ([]Module, error) {
	rows, e := a.db.Query("SELECT MODULE_KEY,NAME,ICON,DESCRIPTION,COLUMNS_JSON,ENABLED FROM " + schema + "MODULES ORDER BY SORT_ORDER")
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Module{}
	for rows.Next() {
		var m Module
		var c string
		if e = rows.Scan(&m.Key, &m.Name, &m.Icon, &m.Desc, &c, &m.Enabled); e != nil {
			return nil, e
		}
		if e = json.Unmarshal([]byte(c), &m.Cols); e != nil {
			return nil, e
		}
		m.Trend = make([]float64, 12)
		m.Rows = [][]any{}
		m.Struct = map[string]float64{}
		m.StructLabels = append([]string{}, catalog[m.Key].Groups...)
		out = append(out, m)
	}
	return out, rows.Err()
}
func (a *App) records(start, end, module string) ([]Record, error) {
	q := "SELECT ID,MODULE_KEY,BUSINESS_DATE,CATEGORY,QUANTITY,AMOUNT_CENTS,VISITORS,SOURCE,NOTE,VERSION FROM " + schema + "REVENUE_RECORDS WHERE BUSINESS_DATE>=? AND BUSINESS_DATE<?"
	args := []any{start, end}
	if module != "" {
		q += " AND MODULE_KEY=?"
		args = append(args, module)
	}
	q += " ORDER BY BUSINESS_DATE DESC,MODULE_KEY,CATEGORY,ID"
	rows, e := a.db.Query(q, args...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Record{}
	for rows.Next() {
		var v Record
		var source, note sql.NullString
		if e = rows.Scan(&v.ID, &v.Module, &v.Date, &v.Category, &v.Quantity, &v.Cents, &v.Visitors, &source, &note, &v.Version); e != nil {
			return nil, e
		}
		v.Source = source.String
		v.Note = note.String
		v.Amount = money(v.Cents)
		out = append(out, v)
	}
	return out, rows.Err()
}
func (a *App) summary(w http.ResponseWriter, r *http.Request) {
	start, end, prev, date, e := bounds(r)
	if e != nil {
		fail(w, 400, e.Error())
		return
	}
	mods, e := a.modules()
	if e != nil {
		fail(w, 500, "读取业态失败")
		return
	}
	year := date[:4]
	d, _ := time.Parse("2006-01-02", date)
	trendEnd := time.Date(d.Year(), d.Month()+1, 1, 0, 0, 0, 0, time.UTC)
	trendStart := trendEnd.AddDate(-1, 0, 0)
	months := make([]string, 12)
	monthIndexes := map[string]int{}
	for i := range months {
		month := trendStart.AddDate(0, i, 0)
		months[i] = fmt.Sprintf("%d月", month.Month())
		monthIndexes[month.Format("2006-01")] = i
	}
	all, e := a.records(trendStart.Format("2006-01-02"), trendEnd.Format("2006-01-02"), "")
	if e != nil {
		fail(w, 500, "读取营收失败")
		return
	}
	previous, e := a.records(prev, start, "")
	if e != nil {
		fail(w, 500, "读取对比期失败")
		return
	}
	selected, e := a.records(start, end, "")
	if e != nil {
		fail(w, 500, "读取明细失败")
		return
	}
	total := int64(0)
	visitors := 0
	previousTotal, previousTicket, currentTicket := int64(0), int64(0), int64(0)
	previousVisitors := 0
	week := make([]float64, 7)
	weekVisitors := make([]int, 7)
	weekStart := d.AddDate(0, 0, -(int(d.Weekday())+6)%7)
	weekly, e := a.records(weekStart.Format("2006-01-02"), weekStart.AddDate(0, 0, 7).Format("2006-01-02"), "")
	if e != nil {
		fail(w, 500, "读取周报失败")
		return
	}
	for _, v := range weekly {
		vd, _ := time.Parse("2006-01-02", v.Date)
		i := int(vd.Sub(weekStart).Hours() / 24)
		week[i] += float64(v.Cents) / 1e6
		if humanVisits(v.Module) {
			weekVisitors[i] += v.Visitors
		}
	}
	for i := range mods {
		m := &mods[i]
		sum, old := int64(0), int64(0)
		cats := map[string]*Record{}
		order := []string{}
		for _, v := range all {
			if v.Module == m.Key {
				if month, ok := monthIndexes[v.Date[:7]]; ok {
					m.Trend[month] += float64(v.Cents) / 1e6
				}
			}
		}
		for _, v := range previous {
			if v.Module == m.Key {
				old += v.Cents
				if m.Enabled == 1 && humanVisits(m.Key) {
					previousVisitors += v.Visitors
				}
			}
		}
		for _, v := range selected {
			if v.Module != m.Key {
				continue
			}
			sum += v.Cents
			m.Visitors += v.Visitors
			if cats[v.Category] == nil {
				cats[v.Category] = &Record{}
				order = append(order, v.Category)
			}
			c := cats[v.Category]
			c.Quantity += v.Quantity
			c.Cents += v.Cents
		}
		m.Value = float64(sum) / 1e6
		m.Mom = growth(float64(sum), float64(old))
		orderCategories(m.Key, order)
		for _, group := range m.StructLabels {
			m.Struct[group] = 0
		}
		for _, name := range order {
			c := cats[name]
			pct := float64(0)
			if sum > 0 {
				pct = float64(c.Cents) / float64(sum) * 100
			}
			unit := float64(0)
			if c.Quantity > 0 {
				unit = float64(c.Cents) / 100 / float64(c.Quantity)
			}
			group := categoryGroup(m.Key, name)
			if _, ok := m.Struct[group]; !ok {
				m.StructLabels = append(m.StructLabels, group)
			}
			m.Struct[group] += pct
			m.Rows = append(m.Rows, []any{name, c.Quantity, money(c.Cents), fmt.Sprintf("%.2f", unit), fmt.Sprintf("%.1f%%", pct)})
		}
		if m.Enabled == 1 {
			total += sum
			if humanVisits(m.Key) {
				visitors += m.Visitors
			}
			previousTotal += old
			if m.Key == "ticket" {
				currentTicket = sum
				previousTicket = old
			}
		}
	}
	currentARPU, previousARPU := float64(0), float64(0)
	if visitors > 0 {
		currentARPU = float64(total) / float64(visitors)
	}
	if previousVisitors > 0 {
		previousARPU = float64(previousTotal) / float64(previousVisitors)
	}
	mom := map[string]any{"total": growth(float64(total), float64(previousTotal)), "second": growth(float64(total-currentTicket), float64(previousTotal-previousTicket)), "visitors": growth(float64(visitors), float64(previousVisitors)), "arpu": growth(currentARPU, previousARPU)}
	jsonOut(w, 200, map[string]any{"modules": mods, "total": float64(total) / 1e6, "visitors": visitors, "week": week, "weekVisitors": weekVisitors, "date": date, "from": start, "to": end, "year": year, "months": months, "trendFrom": trendStart.Format("2006-01"), "trendTo": d.Format("2006-01"), "mom": mom})
}
func growth(current, previous float64) any {
	if previous == 0 {
		return nil
	}
	return (current - previous) / previous * 100
}
func (a *App) saveRecords(vs []Record, actor string) error {
	if len(vs) == 0 || len(vs) > 3000 {
		return errors.New("每批提交 1-3000 条")
	}
	mods, e := a.modules()
	if e != nil {
		return e
	}
	allowed := map[string]bool{}
	for _, m := range mods {
		allowed[m.Key] = m.Enabled == 1
	}
	for i := range vs {
		if !allowed[vs[i].Module] {
			return fmt.Errorf("第 %d 行业态无效或停用", i+1)
		}
		if e = validateRecord(&vs[i]); e != nil {
			return fmt.Errorf("第 %d 行：%s", i+1, e)
		}
	}
	tx, e := a.db.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	for _, v := range vs {
		if v.ID == "" {
			v.ID = random()
			_, e = tx.Exec("INSERT INTO "+schema+"REVENUE_RECORDS(ID,MODULE_KEY,BUSINESS_DATE,CATEGORY,QUANTITY,AMOUNT_CENTS,VISITORS,SOURCE,NOTE,CREATED_BY) VALUES(?,?,?,?,?,?,?,?,?,?)", v.ID, v.Module, v.Date, v.Category, v.Quantity, v.Cents, v.Visitors, v.Source, v.Note, actor)
		} else {
			var res sql.Result
			res, e = tx.Exec("UPDATE "+schema+"REVENUE_RECORDS SET MODULE_KEY=?,BUSINESS_DATE=?,CATEGORY=?,QUANTITY=?,AMOUNT_CENTS=?,VISITORS=?,SOURCE=?,NOTE=?,VERSION=VERSION+1,UPDATED_AT=CURRENT_TIMESTAMP WHERE ID=? AND VERSION=?", v.Module, v.Date, v.Category, v.Quantity, v.Cents, v.Visitors, v.Source, v.Note, v.ID, v.Version)
			if e == nil {
				n, _ := res.RowsAffected()
				if n != 1 {
					e = errors.New("记录已被修改，请刷新后重试")
				}
			}
		}
		if e != nil {
			return e
		}
		_, e = tx.Exec("INSERT INTO "+schema+"AUDIT_LOG(ID,ACTOR,ACTION,TARGET_ID) VALUES(?,?,?,?)", random(), actor, "保存日报", v.ID)
		if e != nil {
			return e
		}
	}
	return tx.Commit()
}
func (a *App) api(w http.ResponseWriter, r *http.Request, id Identity) {
	path := strings.TrimPrefix(r.URL.Path, "/api/")
	if path == "auth/me" && r.Method == "GET" {
		roles := []string{"revenue-user"}
		if id.Super {
			roles = append(roles, "casdoor-super-admin")
		}
		jsonOut(w, 200, map[string]any{"id": id.Sub, "userId": id.Sub, "username": id.Name, "realName": id.Display, "homePath": "/dashboard", "roles": roles, "role": id.Role, "super": id.Super, "permissions": []string{"Revenue.Read"}})
		return
	}
	switch path {
	case "summary":
		if r.Method == "GET" {
			a.summary(w, r)
			return
		}
	case "records":
		if r.Method == "GET" {
			s, e, _, _, err := bounds(r)
			if err != nil {
				fail(w, 400, err.Error())
				return
			}
			vs, err := a.records(s, e, r.URL.Query().Get("module"))
			if err != nil {
				fail(w, 500, "查询失败")
				return
			}
			jsonOut(w, 200, vs)
			return
		}
		if r.Method == "POST" {
			var v Record
			if decode(w, r, &v) != nil {
				fail(w, 400, "日报格式无效")
				return
			}
			if e := a.saveRecords([]Record{v}, id.Sub); e != nil {
				fail(w, 400, e.Error())
				return
			}
			jsonOut(w, 200, map[string]bool{"saved": true})
			return
		}
	case "export":
		if r.Method == "GET" {
			a.export(w, r)
			return
		}
	case "import":
		if r.Method == "POST" {
			a.importExcel(w, r, id)
			return
		}
	case "settings":
		a.settings(w, r, id)
		return
	case "integrations":
		a.integrations(w, r, id)
		return
	case "integration-test":
		if r.Method == "GET" {
			jsonOut(w, 200, map[string]any{"mode": "test", "record": Record{Module: "ticket", Date: time.Now().Format("2006-01-02"), Category: "接口测试门票", Quantity: 3, Amount: "90.00", Visitors: 3, Source: "测试接口"}, "endpoint": "/api/ingest", "method": "POST", "authentication": "Authorization: Bearer <接口密钥>", "idempotency": "使用 sourceId 去重"})
			return
		}
	}
	fail(w, 404, "接口或方法不存在")
}
func (a *App) settings(w http.ResponseWriter, r *http.Request, id Identity) {
	if r.Method == "GET" {
		var name, level, dept string
		var version int
		e := a.db.QueryRow("SELECT SCENIC_NAME,SCENIC_LEVEL,ADMIN_DEPARTMENT,VERSION FROM "+schema+"SETTINGS WHERE ID=1").Scan(&name, &level, &dept, &version)
		if e != nil {
			fail(w, 500, "读取设置失败")
			return
		}
		jsonOut(w, 200, map[string]any{"name": name, "level": level, "department": dept, "version": version})
		return
	}
	if r.Method != "POST" {
		fail(w, 405, "方法不支持")
		return
	}
	if !id.Super {
		fail(w, 403, "需要本系统超级用户权限")
		return
	}
	var v struct {
		Name       string `json:"name"`
		Level      string `json:"level"`
		Department string `json:"department"`
		Version    int    `json:"version"`
	}
	if decode(w, r, &v) != nil || strings.TrimSpace(v.Name) == "" || len([]rune(v.Name)) > 200 || len([]rune(v.Level)) > 300 || len([]rune(v.Department)) > 100 {
		fail(w, 400, "设置内容无效")
		return
	}
	res, e := a.db.Exec("UPDATE "+schema+"SETTINGS SET SCENIC_NAME=?,SCENIC_LEVEL=?,ADMIN_DEPARTMENT=?,VERSION=VERSION+1 WHERE ID=1 AND VERSION=?", v.Name, v.Level, v.Department, v.Version)
	if e != nil {
		fail(w, 500, "保存失败")
		return
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		fail(w, 409, "设置已更新，请刷新")
		return
	}
	jsonOut(w, 200, map[string]bool{"saved": true})
}
func (a *App) integrations(w http.ResponseWriter, r *http.Request, id Identity) {
	if !id.Super {
		fail(w, 403, "需要本系统超级用户权限")
		return
	}
	if r.Method == "GET" {
		rows, e := a.db.Query("SELECT ID,NAME,MODULE_KEY,ENABLED FROM " + schema + "INTEGRATIONS ORDER BY CREATED_AT")
		if e != nil {
			fail(w, 500, "读取接口配置失败")
			return
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var key, name, module string
			var enabled int
			if rows.Scan(&key, &name, &module, &enabled) != nil {
				fail(w, 500, "读取失败")
				return
			}
			out = append(out, map[string]any{"id": key, "name": name, "module": module, "enabled": enabled})
		}
		jsonOut(w, 200, out)
		return
	}
	if r.Method == "POST" {
		var v struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Module  string `json:"module"`
			Enabled int    `json:"enabled"`
		}
		if decode(w, r, &v) != nil || strings.TrimSpace(v.Name) == "" || len([]rune(v.Name)) > 100 || (v.Enabled != 0 && v.Enabled != 1) {
			fail(w, 400, "接口配置无效")
			return
		}
		var e error
		token := ""
		if v.ID == "" {
			token = random()
			v.ID = random()
			_, e = a.db.Exec("INSERT INTO "+schema+"INTEGRATIONS(ID,NAME,MODULE_KEY,TOKEN_HASH,ENABLED) VALUES(?,?,?,?,?)", v.ID, v.Name, v.Module, hashed(token), v.Enabled)
		} else {
			var res sql.Result
			res, e = a.db.Exec("UPDATE "+schema+"INTEGRATIONS SET NAME=?,ENABLED=? WHERE ID=?", v.Name, v.Enabled, v.ID)
			if e == nil {
				n, err := res.RowsAffected()
				if err != nil || n != 1 {
					fail(w, 404, "接口配置不存在，请刷新")
					return
				}
			}
		}
		if e != nil {
			fail(w, 400, "保存失败，请核对业态")
			return
		}
		jsonOut(w, 200, map[string]string{"token": token, "id": v.ID})
		return
	}
	fail(w, 405, "方法不支持")
}
func (a *App) ingest(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		fail(w, 405, "使用 POST")
		return
	}
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if len(token) < 20 {
		fail(w, 401, "接口凭据无效")
		return
	}
	var key, module string
	e := a.db.QueryRow("SELECT ID,MODULE_KEY FROM "+schema+"INTEGRATIONS WHERE TOKEN_HASH=? AND ENABLED=1", hashed(token)).Scan(&key, &module)
	if e != nil {
		fail(w, 401, "接口凭据无效或停用")
		return
	}
	var v struct {
		SourceID string `json:"sourceId"`
		Record   Record `json:"record"`
	}
	if decode(w, r, &v) != nil || len(v.SourceID) < 1 || len(v.SourceID) > 100 || v.Record.ID != "" {
		fail(w, 400, "提交格式或 sourceId 无效")
		return
	}
	v.Record.Module = module
	v.Record.Source = "接口接入"
	if validateRecord(&v.Record) != nil {
		fail(w, 400, "日报字段无效")
		return
	}
	rid := hashed(key + ":" + v.SourceID)
	v.Record.ID = rid
	tx, e := a.db.Begin()
	if e != nil {
		fail(w, 500, "数据库暂不可用")
		return
	}
	defer tx.Rollback()
	var n int
	e = tx.QueryRow("SELECT COUNT(*) FROM "+schema+"REVENUE_RECORDS WHERE ID=?", rid).Scan(&n)
	if e != nil {
		fail(w, 500, "读取失败")
		return
	}
	if n > 0 {
		jsonOut(w, 200, map[string]any{"id": rid, "duplicate": true})
		return
	}
	q := v.Record
	_, e = tx.Exec("INSERT INTO "+schema+"REVENUE_RECORDS(ID,MODULE_KEY,BUSINESS_DATE,CATEGORY,QUANTITY,AMOUNT_CENTS,VISITORS,SOURCE,NOTE,CREATED_BY) VALUES(?,?,?,?,?,?,?,?,?,?)", rid, module, q.Date, q.Category, q.Quantity, q.Cents, q.Visitors, q.Source, q.Note, "integration:"+key)
	if e != nil {
		fail(w, 409, "写入冲突，请使用相同 sourceId 重试")
		return
	}
	if tx.Commit() != nil {
		fail(w, 500, "提交结果待核实，请使用相同 sourceId 重试")
		return
	}
	jsonOut(w, 200, map[string]any{"id": rid, "duplicate": false})
}

var exportHeaders = []string{"日期", "业态代码", "品类", "数量", "营收(元)", "接待量", "来源", "备注"}

func (a *App) export(w http.ResponseWriter, r *http.Request) {
	start, end, _, _, e := bounds(r)
	if e != nil {
		fail(w, 400, "日期无效")
		return
	}
	if r.URL.Query().Get("all") == "1" {
		start, end = "2000-01-01", "2101-01-01"
	}
	vs, e := a.records(start, end, r.URL.Query().Get("module"))
	if e != nil {
		fail(w, 500, "导出查询失败")
		return
	}
	rows := [][]string{exportHeaders}
	if r.URL.Query().Get("template") == "1" {
		vs = []Record{{Date: time.Now().Format("2006-01-02"), Module: "ticket", Category: "门票", Quantity: 1, Amount: "30.00", Visitors: 1, Source: "手工导入", Note: "示例行，请替换后导入"}}
	}
	for _, v := range vs {
		rows = append(rows, []string{v.Date, v.Module, v.Category, strconv.Itoa(v.Quantity), v.Amount, strconv.Itoa(v.Visitors), v.Source, v.Note})
	}
	if r.URL.Query().Get("format") == "csv" {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=revenue.csv")
		w.Write([]byte{0xef, 0xbb, 0xbf})
		c := csv.NewWriter(w)
		for _, row := range rows {
			for i, s := range row {
				if strings.ContainsAny(stringPrefix(s), "=+-@\t\r") {
					row[i] = "'" + s
				}
			}
			c.Write(row)
		}
		c.Flush()
		return
	}
	f := excelize.NewFile()
	defer f.Close()
	f.SetSheetName("Sheet1", "营收明细")
	for i, row := range rows {
		for j, value := range row {
			cell, _ := excelize.CoordinatesToCellName(j+1, i+1)
			f.SetCellStr("营收明细", cell, value)
		}
	}
	style, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "FFFFFF"}, Fill: excelize.Fill{Type: "pattern", Color: []string{"A63A2B"}, Pattern: 1}})
	f.SetCellStyle("营收明细", "A1", "H1", style)
	f.SetColWidth("营收明细", "A", "H", 22)
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", "attachment; filename=revenue.xlsx")
	f.Write(w)
}
func stringPrefix(s string) string {
	if len(s) == 0 {
		return ""
	}
	return s[:1]
}
func (a *App) importExcel(w http.ResponseWriter, r *http.Request, id Identity) {
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	if r.ParseMultipartForm(10<<20) != nil {
		fail(w, 400, "文件须小于 10 MB")
		return
	}
	defer r.MultipartForm.RemoveAll()
	file, _, e := r.FormFile("file")
	if e != nil {
		fail(w, 400, "请选择 Excel 文件")
		return
	}
	defer file.Close()
	f, e := excelize.OpenReader(file, excelize.Options{UnzipSizeLimit: 50 << 20})
	if e != nil {
		fail(w, 400, "Excel 文件无效")
		return
	}
	defer f.Close()
	rows, e := f.GetRows(f.GetSheetName(0))
	if e != nil || len(rows) < 2 || len(rows) > 3001 {
		fail(w, 400, "须包含表头和 1-3000 条记录")
		return
	}
	if strings.Join(rows[0], "|") != strings.Join(exportHeaders, "|") {
		fail(w, 400, "表头与导入模板不一致，请下载模板")
		return
	}
	vs := []Record{}
	for i, row := range rows[1:] {
		if len(row) < 6 {
			fail(w, 400, fmt.Sprintf("第 %d 行字段不足", i+2))
			return
		}
		for len(row) < 8 {
			row = append(row, "")
		}
		q, e1 := strconv.Atoi(row[3])
		v, e2 := strconv.Atoi(row[5])
		if e1 != nil || e2 != nil {
			fail(w, 400, fmt.Sprintf("第 %d 行数量须为整数", i+2))
			return
		}
		vs = append(vs, Record{Date: row[0], Module: row[1], Category: row[2], Quantity: q, Amount: row[4], Visitors: v, Source: row[6], Note: row[7]})
	}
	if e = a.saveRecords(vs, id.Sub); e != nil {
		fail(w, 400, e.Error())
		return
	}
	jsonOut(w, 200, map[string]int{"imported": len(vs)})
}
