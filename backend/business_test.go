package main

import (
	"net/http/httptest"
	"testing"
)

func TestMoneyExactAndInvalid(t *testing.T) {
	for s, want := range map[string]int64{"0": 0, "0.01": 1, "123.4": 12340, "9999999999.99": 999999999999} {
		got, e := cents(s)
		if e != nil || got != want {
			t.Fatalf("%s: got %d %v", s, got, e)
		}
	}
	for _, s := range []string{"-1", "NaN", "1e3", "1.001", "1.", ".1", "1,000", "", "1..2", "10000000000"} {
		if _, e := cents(s); e == nil {
			t.Errorf("accepted invalid amount %q", s)
		}
	}
}
func TestDateBounds(t *testing.T) {
	for _, v := range []struct{ q, start, end, prev string }{{"date=2024-02-29&range=day", "2024-02-29", "2024-03-01", "2024-02-28"}, {"date=2026-01-21&range=month", "2026-01-01", "2026-02-01", "2025-12-01"}, {"date=2026-10-21&range=year", "2026-01-01", "2027-01-01", "2025-01-01"}} {
		s, e, p, _, err := bounds(httptest.NewRequest("GET", "/?"+v.q, nil))
		if err != nil || s != v.start || e != v.end || p != v.prev {
			t.Fatalf("%s got %s %s %s %v", v.q, s, e, p, err)
		}
	}
	for _, q := range []string{"date=2026-02-30", "date=2026-10-21&range=bad"} {
		if _, _, _, _, e := bounds(httptest.NewRequest("GET", "/?"+q, nil)); e == nil {
			t.Fatal("invalid period accepted")
		}
	}
}
func TestRecordValidation(t *testing.T) {
	good := Record{Date: "2026-10-21", Category: "门票", Quantity: 3, Visitors: 3, Amount: "90.01"}
	if validateRecord(&good) != nil || good.Cents != 9001 {
		t.Fatal("valid record rejected")
	}
	bad := good
	bad.Quantity = -1
	if validateRecord(&bad) == nil {
		t.Fatal("negative quantity accepted")
	}
	bad = good
	bad.Category = " "
	if validateRecord(&bad) == nil {
		t.Fatal("blank category accepted")
	}
	bad = good
	bad.Date = "2026-02-29"
	if validateRecord(&bad) == nil {
		t.Fatal("invalid date accepted")
	}
}
func TestIngestRequiresCredential(t *testing.T) {
	a := &App{}
	w := httptest.NewRecorder()
	a.ingest(w, httptest.NewRequest("POST", "/api/ingest", nil))
	if w.Code != 401 {
		t.Fatalf("got %d", w.Code)
	}
}
func TestSettingsAndIntegrationRequireSuper(t *testing.T) {
	a := &App{}
	for _, endpoint := range []string{"settings", "integrations"} {
		w := httptest.NewRecorder()
		a.api(w, httptest.NewRequest("POST", "/api/"+endpoint, nil), Identity{Sub: "ordinary"})
		if w.Code != 403 {
			t.Fatalf("%s got %d", endpoint, w.Code)
		}
	}
}
