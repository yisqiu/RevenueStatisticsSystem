package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLogoutInvalidatesOnlyBusinessSession(t *testing.T) {
	a := &App{sessions: map[string]Session{hashed("local-cookie"): {Sub: "immutable-id", Exp: time.Now().Add(time.Hour)}}}
	r := httptest.NewRequest("POST", "/api/auth/logout", nil)
	r.Header.Set("Origin", "http://cd.libeasy.cn:8027")
	r.AddCookie(&http.Cookie{Name: "revenue_session", Value: "local-cookie"})
	w := httptest.NewRecorder()
	a.logout(w, r)
	if w.Code != 200 {
		t.Fatalf("logout status %d", w.Code)
	}
	if _, ok := a.getSession(r); ok {
		t.Fatal("old business session still valid")
	}
	if len(w.Result().Cookies()) != 1 || w.Result().Cookies()[0].MaxAge >= 0 {
		t.Fatal("session cookie was not cleared")
	}
}
func TestLogoutRejectsCrossOrigin(t *testing.T) {
	a := &App{sessions: map[string]Session{hashed("local-cookie"): {Sub: "immutable-id", Exp: time.Now().Add(time.Hour)}}}
	for _, method := range []string{"GET", "POST"} {
		r := httptest.NewRequest(method, "/api/auth/logout", nil)
		r.Header.Set("Origin", "https://other.example")
		r.AddCookie(&http.Cookie{Name: "revenue_session", Value: "local-cookie"})
		w := httptest.NewRecorder()
		a.logout(w, r)
		if w.Code != 403 && w.Code != 405 {
			t.Fatal("unsafe logout accepted")
		}
	}
	if len(a.sessions) != 1 {
		t.Fatal("rejected request changed session")
	}
}
