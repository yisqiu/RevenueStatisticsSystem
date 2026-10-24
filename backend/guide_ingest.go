package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type guidePushRow struct {
	SourceID      string `json:"sourceId"`
	OrderID       string `json:"orderId"`
	OrderNo       string `json:"orderNo"`
	BusinessDate  string `json:"businessDate"`
	ReceptionType string `json:"receptionType"`
	Status        string `json:"status"`
	VisitorCount  any    `json:"visitorCount"`
	PayAmount     string `json:"payAmount"`
	PricePending  any    `json:"pricePending"`
	UpdatedAt     string `json:"updatedAt"`
}

type guideImport struct {
	SourceID, UpdatedAt, Hash string
	Record                    Record
	Active                    bool
}

func normalizeGuide(row guidePushRow) (guideImport, error) {
	var out guideImport
	if strings.TrimSpace(row.ReceptionType) == "" {
		return out, errors.New("接待类型不能为空")
	}
	if row.OrderID == "" || row.SourceID != "gm-order-"+row.OrderID || len(row.SourceID) > 100 || len([]rune(row.OrderNo)) > 100 || len([]rune(row.ReceptionType)) > 100 {
		return out, errors.New("来源订单编号或接待类型无效")
	}
	if !contains([]string{"pending", "in_progress", "completed", "cancelled", "refunded"}, row.Status) {
		return out, errors.New("订单状态无效")
	}
	visitors, err := strconv.Atoi(fmt.Sprint(row.VisitorCount))
	if err != nil {
		return out, errors.New("接待人数无效")
	}
	pending, err := strconv.Atoi(fmt.Sprint(row.PricePending))
	if err != nil || (pending != 0 && pending != 1) {
		return out, errors.New("核价状态无效")
	}
	updated, err := time.Parse("2006-01-02 15:04:05.999999", row.UpdatedAt)
	if err != nil || updated.Year() < 2000 || updated.Year() > 2100 {
		return out, errors.New("源数据更新时间无效")
	}
	record := Record{Module: "guide", Date: row.BusinessDate, Category: row.ReceptionType, Quantity: 1, Amount: row.PayAmount, Visitors: visitors, Source: "数据中台同步", Note: "来源订单：" + row.OrderNo + "；接待类型：" + row.ReceptionType}
	if err = validateRecord(&record); err != nil {
		return out, err
	}
	record.Amount = money(record.Cents)
	out = guideImport{SourceID: row.SourceID, UpdatedAt: updated.Format("2006-01-02 15:04:05.000000"), Record: record, Active: row.Status == "completed" && pending == 0}
	// Include the source status so a same-version state change is rejected, not silently lost.
	// Preserve existing source hashes when only the local category mapping changes.
	// Reception type remains covered by Note in both hash formats.
	hashRecord := record
	hashRecord.Category = "讲解员接待"
	b, _ := json.Marshal(struct {
		Record  Record
		Status  string
		Pending int
	}{hashRecord, row.Status, pending})
	out.Hash = hashed(string(b))
	return out, nil
}

func guideVersionAction(oldTime, oldHash string, item guideImport) string {
	if item.UpdatedAt < oldTime {
		return "stale"
	}
	if item.UpdatedAt == oldTime {
		if item.Hash == oldHash {
			return "duplicate"
		}
		return "conflict"
	}
	return "apply"
}

func (a *App) guideIngest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		fail(w, 405, "使用 POST")
		return
	}
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") || len(strings.TrimPrefix(header, "Bearer ")) < 20 {
		fail(w, 401, "接口凭据无效")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		a.databaseError(w, err)
		return
	}
	defer tx.Rollback()
	var key, module, integrationName string
	// Serialize batches for this integration; token revocation and all row changes share this transaction.
	err = tx.QueryRowContext(r.Context(), "SELECT ID,MODULE_KEY,NAME FROM "+schema+"INTEGRATIONS WHERE TOKEN_HASH=? AND ENABLED=1 FOR UPDATE", hashed(strings.TrimPrefix(header, "Bearer "))).Scan(&key, &module, &integrationName)
	if errors.Is(err, sql.ErrNoRows) {
		fail(w, 401, "接口凭据无效或停用")
		return
	}
	if err != nil {
		a.databaseError(w, err)
		return
	}
	if module != "guide" {
		fail(w, 403, "接口未绑定讲解员业态")
		return
	}
	var input struct {
		ObjectName string         `json:"objectName"`
		DataList   []guidePushRow `json:"dataList"`
	}
	if decode(w, r, &input) != nil || input.ObjectName != "guide_service_orders" || len(input.DataList) == 0 || len(input.DataList) > 500 {
		fail(w, 400, "讲解员推送格式无效，每批应为1至500条")
		return
	}
	items := make([]guideImport, 0, len(input.DataList))
	seen := map[string]bool{}
	for _, row := range input.DataList {
		item, e := normalizeGuide(row)
		if e != nil || seen[row.SourceID] {
			fail(w, 400, "订单字段无效或同批存在重复流水")
			return
		}
		seen[row.SourceID] = true
		items = append(items, item)
	}
	counts := map[string]int{"received": len(items), "upserted": 0, "removed": 0, "duplicate": 0, "stale": 0}
	for _, item := range items {
		rid := hashed(key + ":" + item.SourceID)
		var oldTime, oldHash string
		err = tx.QueryRowContext(r.Context(), "SELECT SOURCE_UPDATED_AT,PAYLOAD_HASH FROM "+schema+"GUIDE_IMPORT_STATE WHERE ID=?", rid).Scan(&oldTime, &oldHash)
		exists := err == nil
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			a.databaseError(w, err)
			return
		}
		switch guideVersionAction(oldTime, oldHash, item) {
		case "stale":
			counts["stale"]++
			continue
		case "duplicate":
			counts["duplicate"]++
			continue
		case "conflict":
			fail(w, 409, "同一源版本数据不同，请核对源更新时间")
			return
		}
		actor := "integration:" + key
		q := item.Record
		if item.Active {
			res, e := tx.ExecContext(r.Context(), "UPDATE "+schema+"REVENUE_RECORDS SET BUSINESS_DATE=?,CATEGORY=?,QUANTITY=?,AMOUNT_CENTS=?,VISITORS=?,SOURCE=?,NOTE=?,VERSION=VERSION+1,UPDATED_AT=CURRENT_TIMESTAMP WHERE ID=? AND MODULE_KEY='guide' AND CREATED_BY=?", q.Date, q.Category, q.Quantity, q.Cents, q.Visitors, q.Source, q.Note, rid, actor)
			if e != nil {
				a.databaseError(w, e)
				return
			}
			n, e := res.RowsAffected()
			if e == nil && n == 0 {
				_, e = tx.ExecContext(r.Context(), "INSERT INTO "+schema+"REVENUE_RECORDS(ID,MODULE_KEY,BUSINESS_DATE,CATEGORY,QUANTITY,AMOUNT_CENTS,VISITORS,SOURCE,NOTE,CREATED_BY) VALUES(?,'guide',?,?,?,?,?,?,?,?)", rid, q.Date, q.Category, q.Quantity, q.Cents, q.Visitors, q.Source, q.Note, actor)
			}
			if e != nil {
				a.databaseError(w, e)
				return
			}
			counts["upserted"]++
		} else {
			res, e := tx.ExecContext(r.Context(), "DELETE FROM "+schema+"REVENUE_RECORDS WHERE ID=? AND MODULE_KEY='guide' AND CREATED_BY=?", rid, actor)
			if e != nil {
				a.databaseError(w, e)
				return
			}
			n, e := res.RowsAffected()
			if e != nil {
				a.databaseError(w, e)
				return
			}
			counts["removed"] += int(n)
		}
		if exists {
			_, err = tx.ExecContext(r.Context(), "UPDATE "+schema+"GUIDE_IMPORT_STATE SET SOURCE_UPDATED_AT=?,PAYLOAD_HASH=?,IMPORTED_AT=CURRENT_TIMESTAMP WHERE ID=?", item.UpdatedAt, item.Hash, rid)
		} else {
			_, err = tx.ExecContext(r.Context(), "INSERT INTO "+schema+"GUIDE_IMPORT_STATE(ID,INTEGRATION_ID,SOURCE_ID,SOURCE_UPDATED_AT,PAYLOAD_HASH) VALUES(?,?,?,?,?)", rid, key, item.SourceID, item.UpdatedAt, item.Hash)
		}
		if err != nil {
			a.databaseError(w, err)
			return
		}
	}
	if counts["upserted"] > 0 || counts["removed"] > 0 {
		err = systemAudit(r.Context(), tx, "系统接口", "讲解员数据同步", key, fmt.Sprintf("%s；接收%d；写入更新%d；撤回%d；重复%d；旧版本%d", integrationName, counts["received"], counts["upserted"], counts["removed"], counts["duplicate"], counts["stale"]), Identity{Name: integrationName})
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		a.databaseError(w, err)
		return
	}
	jsonOut(w, 200, counts)
}
