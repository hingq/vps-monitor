package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestQueryBool(t *testing.T) {
	cases := map[string]bool{
		"true":  true,
		"1":     true,
		"false": false,
		"0":     false,
		"yes":   false,
		"":      false,
	}
	for raw, want := range cases {
		req := httptest.NewRequest(http.MethodGet, "/?reset="+raw, nil)
		if got := queryBool(req, "reset"); got != want {
			t.Errorf("queryBool(reset=%q) = %v, want %v", raw, got, want)
		}
	}
}

func TestRequireParamPresent(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?name=foo", nil)
	rec := httptest.NewRecorder()
	got, ok := requireParam(rec, req, "name")
	if !ok || got != "foo" {
		t.Errorf("got (%q,%v), want (foo,true)", got, ok)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("should not have written status, got %d", rec.Code)
	}
}

func TestRequireParamMissing(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	_, ok := requireParam(rec, req, "name")
	if ok {
		t.Fatal("expected ok=false for missing param")
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON body: %v", err)
	}
	if body["error"] == "" {
		t.Error("expected error message in body")
	}
}

func TestWriteError(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError(rec, http.ErrHandlerTimeout)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("content-type = %q", ct)
	}
}
