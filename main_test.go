package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMessageLifecycle(t *testing.T) {
	app := newServer().routes()
	request := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		w := httptest.NewRecorder()
		app.ServeHTTP(w, r)
		return w
	}
	if got := request("GET", "/healthz", ""); got.Code != http.StatusOK || got.Body.String() != "ok\n" {
		t.Fatalf("healthz: %d %q", got.Code, got.Body.String())
	}
	if got := request("POST", "/messages", `{"text":"Halo Firman"}`); got.Code != http.StatusCreated || !strings.Contains(got.Body.String(), `"id":1`) {
		t.Fatalf("create: %d %q", got.Code, got.Body.String())
	}
	if got := request("GET", "/messages", ""); got.Code != http.StatusOK || !strings.Contains(got.Body.String(), "Halo Firman") {
		t.Fatalf("list: %d %q", got.Code, got.Body.String())
	}
	if got := request("DELETE", "/messages/1", ""); got.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", got.Code)
	}
	if got := request("GET", "/messages", ""); got.Body.String() != "[]\n" {
		t.Fatalf("list after delete: %q", got.Body.String())
	}
	if got := request("GET", "/metrics", ""); got.Code != http.StatusOK || !strings.Contains(got.Body.String(), "message_board_api_requests_total 4\n") || !strings.Contains(got.Body.String(), "message_board_messages 0\n") {
		t.Fatalf("metrics: %d %q", got.Code, got.Body.String())
	}
}

func TestInvalidMessage(t *testing.T) {
	app := newServer().routes()
	for _, body := range []string{`{}`, `{"text":"   "}`, `{"text":"ok","unknown":1}`, `{"text":"ok"}{}`} {
		w := httptest.NewRecorder()
		app.ServeHTTP(w, httptest.NewRequest("POST", "/messages", strings.NewReader(body)))
		if w.Code != http.StatusBadRequest {
			t.Errorf("body %q: got %d", body, w.Code)
		}
	}
}
