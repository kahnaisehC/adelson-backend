package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORSAllowsConfiguredOrigin(t *testing.T) {
	handler := newCORSHandler([]string{"https://inmobiliariafront.joaquinr-val.workers.dev"}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	request := httptest.NewRequest(http.MethodOptions, "/api/v1/inmuebles", nil)
	request.Header.Set("Origin", "https://inmobiliariafront.joaquinr-val.workers.dev")
	request.Header.Set("Access-Control-Request-Method", http.MethodGet)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", response.Code)
	}
	if origin := response.Header().Get("Access-Control-Allow-Origin"); origin != "https://inmobiliariafront.joaquinr-val.workers.dev" {
		t.Fatalf("unexpected allowed origin: %q", origin)
	}
}

func TestCORSRejectsUnknownOrigin(t *testing.T) {
	handler := newCORSHandler([]string{"https://inmobiliariafront.joaquinr-val.workers.dev"}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	request := httptest.NewRequest(http.MethodOptions, "/api/v1/inmuebles", nil)
	request.Header.Set("Origin", "https://example.com")
	request.Header.Set("Access-Control-Request-Method", http.MethodGet)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if origin := response.Header().Get("Access-Control-Allow-Origin"); origin != "" {
		t.Fatalf("expected no allowed origin, got %q", origin)
	}
}
