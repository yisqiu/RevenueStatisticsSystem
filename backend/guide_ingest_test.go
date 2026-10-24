package main

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func guideFixture() guidePushRow {
	return guidePushRow{SourceID: "gm-order-test", OrderID: "test", OrderNo: "TEST", BusinessDate: "2100-01-02", ReceptionType: "游客接待", Status: "completed", VisitorCount: json.Number("3"), PayAmount: "160.00", PricePending: "0", UpdatedAt: "2026-10-22 12:00:00.123456"}
}

func TestGuideImportRevenueAndReversal(t *testing.T) {
	for _, state := range []string{"pending", "in_progress", "completed", "cancelled", "refunded"} {
		row := guideFixture()
		row.Status = state
		got, err := normalizeGuide(row)
		if err != nil || got.Active != (state == "completed") || got.Record.Cents != 16000 || got.Record.Visitors != 3 {
			t.Fatalf("state %s: %+v %v", state, got, err)
		}
	}
	row := guideFixture()
	row.PricePending = json.Number("1")
	got, err := normalizeGuide(row)
	if err != nil || got.Active {
		t.Fatal("未核价订单计入收入", err)
	}
}

func TestGuideImportStaleDuplicateAndConflictingVersions(t *testing.T) {
	current, _ := normalizeGuide(guideFixture())
	if guideVersionAction(current.UpdatedAt, current.Hash, current) != "duplicate" {
		t.Fatal("重复版本应幂等")
	}
	older := current
	older.UpdatedAt = "2026-10-22 11:59:59.999999"
	if guideVersionAction(current.UpdatedAt, current.Hash, older) != "stale" {
		t.Fatal("迟到旧版本应忽略")
	}
	changed := guideFixture()
	changed.PayAmount = "170.00"
	conflict, _ := normalizeGuide(changed)
	if guideVersionAction(current.UpdatedAt, current.Hash, conflict) != "conflict" {
		t.Fatal("同版本金额不同应拒绝")
	}
	conflict.UpdatedAt = "2026-10-22 12:00:01.000000"
	if guideVersionAction(current.UpdatedAt, current.Hash, conflict) != "apply" {
		t.Fatal("较新金额应更新")
	}
}

func TestGuideImportRejectsInvalidRows(t *testing.T) {
	for _, mutate := range []func(*guidePushRow){
		func(r *guidePushRow) { r.SourceID = "another-order" },
		func(r *guidePushRow) { r.Status = "paid" },
		func(r *guidePushRow) { r.VisitorCount = "-1" },
		func(r *guidePushRow) { r.PricePending = "2" },
		func(r *guidePushRow) { r.PayAmount = "-10.00" },
		func(r *guidePushRow) { r.BusinessDate = "invalid" },
		func(r *guidePushRow) { r.UpdatedAt = "invalid" },
	} {
		row := guideFixture()
		mutate(&row)
		if _, err := normalizeGuide(row); err == nil {
			t.Fatal("非法来源行被接受")
		}
	}
}

func TestGuideIngestRequiresBearerBeforeDatabase(t *testing.T) {
	a := App{}
	for _, auth := range []string{"", "Bearer short", "not-a-bearer-credential-123456789"} {
		r := httptest.NewRequest("POST", "/api/ingest/guide", strings.NewReader(`{}`))
		r.Header.Set("Authorization", auth)
		w := httptest.NewRecorder()
		a.guideIngest(w, r)
		if w.Code != 401 {
			t.Fatal("无效凭据未拒绝", w.Code)
		}
	}
}
