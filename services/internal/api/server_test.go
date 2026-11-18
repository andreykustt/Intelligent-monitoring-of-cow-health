package api

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRoleAndFarm(t *testing.T) {
	for _, role := range []string{"operator", "admin"} {
		r := httptest.NewRequest("POST", "/api/v1/users", nil)
		r = r.WithContext(context.WithValue(r.Context(), identityKey{}, identity{Role: role, Farms: []string{"a"}}))
		w := httptest.NewRecorder()
		if admin(w, r) != (role == "admin") {
			t.Fatal("role bypass")
		}
		if farmAllowed(r, "b") || !farmAllowed(r, "a") {
			t.Fatal("farm scope bypass")
		}
	}
}
func TestManualValidation(t *testing.T) {
	b := manualInput{Cow: "COW-001", Observed: time.Now().UTC(), Payload: map[string]any{"bcs": 3.25, "hoof": "без замечаний"}}
	if validateManual("body-condition-records", b) != "" {
		t.Fatal("valid BCS rejected")
	}
	b.Payload["bcs"] = 3.1
	if validateManual("body-condition-records", b) == "" {
		t.Fatal("BCS scale ignored")
	}
	b.Payload = map[string]any{"test": "bhb", "unit": "cells/mL", "value": 1.2}
	if validateManual("laboratory-results", b) == "" {
		t.Fatal("lab unit mismatch accepted")
	}
}
func TestPageBounds(t *testing.T) {
	for _, query := range []string{"page=0", "page_size=101", "page=abc", "page_size=-1"} {
		r := httptest.NewRequest("GET", "/?"+query, nil)
		if _, _, ok := page(httptest.NewRecorder(), r); ok {
			t.Fatal(query)
		}
	}
}
