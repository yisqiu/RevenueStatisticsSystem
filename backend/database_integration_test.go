package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"github.com/xuri/excelize/v2"
	"net/http/httptest"
	"os"
	"testing"
)

func TestDatabaseSummaryReconciles(t *testing.T) {
	dsn := os.Getenv("DM_TEST_DSN")
	if dsn == "" {
		t.Skip("DM_TEST_DSN 未设置")
	}
	db, e := sql.Open("dm", dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	a := &App{db: db}
	for _, period := range []string{"day", "month", "year"} {
		w := httptest.NewRecorder()
		a.summary(w, httptest.NewRequest("GET", "/api/summary?date=2026-10-21&range="+period, nil))
		if w.Code != 200 {
			t.Fatalf("%s: %d %s", period, w.Code, w.Body.String())
		}
		var got struct {
			Modules  []Module `json:"modules"`
			Total    float64  `json:"total"`
			Visitors int      `json:"visitors"`
		}
		if e = json.Unmarshal(w.Body.Bytes(), &got); e != nil {
			t.Fatal(e)
		}
		if len(got.Modules) != 9 || got.Total <= 0 || got.Visitors <= 0 {
			t.Fatalf("missing data in %s", period)
		}
		sum := float64(0)
		for _, m := range got.Modules {
			sum += m.Value
		}
		if sum-got.Total > 0.000001 || got.Total-sum > 0.000001 {
			t.Fatal("totals do not reconcile")
		}
	}
}

func TestDatabaseExportValidWorkbook(t *testing.T) {
	dsn := os.Getenv("DM_TEST_DSN")
	if dsn == "" {
		t.Skip("DM_TEST_DSN 未设置")
	}
	db, e := sql.Open("dm", dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	a := &App{db: db}
	w := httptest.NewRecorder()
	a.export(w, httptest.NewRequest("GET", "/api/export?date=2026-10-21&range=month&format=xlsx", nil))
	if w.Code != 200 {
		t.Fatalf("export status %d", w.Code)
	}
	f, e := excelize.OpenReader(bytes.NewReader(w.Body.Bytes()))
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	rows, e := f.GetRows("营收明细")
	if e != nil || len(rows) < 1000 || rows[0][4] != "营收(元)" {
		t.Fatalf("invalid workbook rows %d err %v", len(rows), e)
	}
	var total int64
	for _, row := range rows[1:] {
		n, e := cents(row[4])
		if e != nil {
			t.Fatal(e)
		}
		total += n
	}
	var expected int64
	e = db.QueryRow("SELECT SUM(AMOUNT_CENTS) FROM " + schema + "REVENUE_RECORDS WHERE BUSINESS_DATE>='2026-10-01' AND BUSINESS_DATE<'2026-11-01'").Scan(&expected)
	if e != nil || expected != total {
		t.Fatalf("export total %d != database %d, %v", total, expected, e)
	}
}
