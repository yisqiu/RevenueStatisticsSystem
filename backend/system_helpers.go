package main

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

func stamp() string { return time.Now().Format("2006-01-02 15:04:05") }

var lastAuditNano atomic.Int64

func auditID() string {
	for {
		previous := lastAuditNano.Load()
		next := time.Now().UnixNano()
		if next <= previous {
			next = previous + 1
		}
		if lastAuditNano.CompareAndSwap(previous, next) {
			return fmt.Sprintf("%019d-%s", next, random())
		}
	}
}
func (a *App) databaseError(w http.ResponseWriter, _ error) {
	fail(w, 503, "数据服务暂不可用，请稍后重试")
}
func localPermission(id Identity, permission string) bool {
	return id.Super || contains(id.Permissions, permission)
}
func moduleName(modules []Module, key string) string {
	for _, m := range modules {
		if m.Key == key {
			return m.Name
		}
	}
	return "营收日报"
}
