package main

import (
	"bytes"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"github.com/xuri/excelize/v2"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func reviewDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DM_TEST_DSN")
	if dsn == "" {
		t.Skip("DM_TEST_DSN 未设置")
	}
	db, e := sql.Open("dm", dsn)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close() })
	return db
}
func TestReviewChineseRecordAndOptimisticLock(t *testing.T) {
	db := reviewDB(t)
	a := &App{db: db}
	actor := "review-" + random()
	t.Cleanup(func() {
		db.Exec("DELETE FROM "+schema+"AUDIT_LOG WHERE ACTOR=?", actor)
		db.Exec("DELETE FROM "+schema+"REVENUE_RECORDS WHERE CREATED_BY=?", actor)
	})
	v := Record{Module: "ticket", Date: "2099-01-02", Category: strings.Repeat("测", 100), Quantity: 2, Amount: "60.01", Visitors: 2, Source: strings.Repeat("来", 100), Note: strings.Repeat("注", 500)}
	if e := a.saveRecords([]Record{v}, actor); e != nil {
		t.Fatalf("valid Chinese input failed: %v", e)
	}
	if e := db.QueryRow("SELECT ID,VERSION FROM "+schema+"REVENUE_RECORDS WHERE CREATED_BY=?", actor).Scan(&v.ID, &v.Version); e != nil {
		t.Fatal(e)
	}
	v.Amount = "80.02"
	if e := a.saveRecords([]Record{v}, actor); e != nil {
		t.Fatal(e)
	}
	v.Amount = "10.00"
	if e := a.saveRecords([]Record{v}, actor); e == nil {
		t.Fatal("stale edit accepted")
	}
	var amount int64
	db.QueryRow("SELECT AMOUNT_CENTS FROM "+schema+"REVENUE_RECORDS WHERE ID=?", v.ID).Scan(&amount)
	if amount != 8002 {
		t.Fatalf("stale edit changed amount: %d", amount)
	}
	v.ID = ""
	v.Category = "invalid-batch"
	bad := v
	bad.Amount = "1.001"
	if e := a.saveRecords([]Record{v, bad}, actor); e == nil {
		t.Fatal("invalid batch accepted")
	}
	var count int
	db.QueryRow("SELECT COUNT(*) FROM "+schema+"REVENUE_RECORDS WHERE CREATED_BY=?", actor).Scan(&count)
	if count != 1 {
		t.Fatal("batch partially persisted")
	}
}
func TestReviewAllCSVAndMissingIntegration(t *testing.T) {
	db := reviewDB(t)
	a := &App{db: db}
	w := httptest.NewRecorder()
	a.export(w, httptest.NewRequest("GET", "/api/export?date=2026-10-21&range=day&all=1&format=csv", nil))
	if w.Code != 200 {
		t.Fatalf("export %d", w.Code)
	}
	rows, e := csv.NewReader(strings.NewReader(strings.TrimPrefix(w.Body.String(), "\ufeff"))).ReadAll()
	if e != nil {
		t.Fatal(e)
	}
	var count int
	db.QueryRow("SELECT COUNT(*) FROM " + schema + "REVENUE_RECORDS WHERE BUSINESS_DATE>='2000-01-01' AND BUSINESS_DATE<'2101-01-01'").Scan(&count)
	if len(rows) != count+1 {
		t.Fatalf("all CSV exported %d of %d", len(rows)-1, count)
	}
	w = httptest.NewRecorder()
	a.integrations(w, httptest.NewRequest("POST", "/api/integrations", strings.NewReader(`{"id":"missing-review-record","name":"测试","module":"ticket","enabled":1}`)), Identity{Super: true})
	if w.Code != 404 {
		t.Fatalf("missing integration status %d", w.Code)
	}
}
func TestReviewInvalidExcelRollsBack(t *testing.T) {
	db := reviewDB(t)
	a := &App{db: db}
	f := excelize.NewFile()
	defer f.Close()
	for j, s := range exportHeaders {
		cell, _ := excelize.CoordinatesToCellName(j+1, 1)
		f.SetCellStr("Sheet1", cell, s)
	}
	rows := [][]string{{"2099-01-02", "ticket", "整批回滚验收", "1", "10.00", "1", "回归测试", "有效首行"}, {"2099-01-02", "ticket", "整批回滚验收", "1", "-10.00", "1", "回归测试", "无效末行"}}
	for i, row := range rows {
		for j, s := range row {
			cell, _ := excelize.CoordinatesToCellName(j+1, i+2)
			f.SetCellStr("Sheet1", cell, s)
		}
	}
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, _ := mw.CreateFormFile("file", "invalid.xlsx")
	f.Write(part)
	mw.Close()
	actor := "review-" + random()
	r := httptest.NewRequest("POST", "/api/import", &body)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	a.importExcel(w, r, Identity{Sub: actor})
	if w.Code != 400 {
		t.Fatalf("invalid workbook status %d", w.Code)
	}
	var count int
	db.QueryRow("SELECT COUNT(*) FROM "+schema+"REVENUE_RECORDS WHERE CREATED_BY=?", actor).Scan(&count)
	if count != 0 {
		t.Fatalf("partial import: %d", count)
	}
}
func TestReviewEmptyPeriodGrowth(t *testing.T) {
	db := reviewDB(t)
	a := &App{db: db}
	w := httptest.NewRecorder()
	a.summary(w, httptest.NewRequest("GET", "/api/summary?date=2098-01-01&range=month", nil))
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	var out struct{ Modules []Module }
	if e := json.Unmarshal(w.Body.Bytes(), &out); e != nil {
		t.Fatal(e)
	}
	for _, m := range out.Modules {
		if m.Mom != nil {
			t.Fatal("missing comparison reported as growth")
		}
	}
}

func TestReviewStructureAndVisitorUnits(t *testing.T) {
	db := reviewDB(t)
	a := &App{db: db}
	w := httptest.NewRecorder()
	a.summary(w, httptest.NewRequest("GET", "/api/summary?date=2026-10-21&range=month", nil))
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	var out struct {
		Modules  []Module
		Visitors int
	}
	if e := json.Unmarshal(w.Body.Bytes(), &out); e != nil {
		t.Fatal(e)
	}
	var expected int
	if e := db.QueryRow("SELECT SUM(VISITORS) FROM " + schema + "REVENUE_RECORDS WHERE BUSINESS_DATE>='2026-10-01' AND BUSINESS_DATE<'2026-11-01' AND MODULE_KEY NOT IN ('park','online')").Scan(&expected); e != nil {
		t.Fatal(e)
	}
	if out.Visitors != expected {
		t.Fatalf("people metric mixed non-person units: %d != %d", out.Visitors, expected)
	}
	for _, m := range out.Modules {
		sum := 0.0
		for _, value := range m.Struct {
			sum += value
		}
		if sum < 99.999999 || sum > 100.000001 {
			t.Fatalf("%s structure sum %f", m.Key, sum)
		}
		c := catalog[m.Key]
		for i, label := range c.Groups {
			if m.StructLabels[i] != label {
				t.Fatal("prototype structure order changed")
			}
		}
		for i, cat := range c.Categories {
			if m.Rows[i][0] != cat.Name {
				t.Fatalf("%s detail order differs", m.Key)
			}
		}
	}
}
