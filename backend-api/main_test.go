package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestHealthHandler_DBReachable(t *testing.T) {
	mockDB, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer mockDB.Close()

	mock.ExpectPing()
	db = mockDB

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	healthHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestHealthHandler_DBUnreachable(t *testing.T) {
	mockDB, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer mockDB.Close()

	mock.ExpectPing().WillReturnError(sqlErr)
	db = mockDB

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	healthHandler(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", rec.Code)
	}
}

func TestMetricsHandler_ReturnsRows(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer mockDB.Close()
	db = mockDB

	rows := sqlmock.NewRows([]string{"pod_name", "namespace", "restart_count", "recorded_at"}).
		AddRow("nginx-test-1", "default", 0, "2026-01-01T00:00:00Z")

	mock.ExpectQuery("SELECT pod_name, namespace, restart_count, recorded_at FROM metrics").
		WillReturnRows(rows)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()

	metricsHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	if !containsSubstring(rec.Body.String(), "nginx-test-1") {
		t.Errorf("expected response to contain pod name, got: %s", rec.Body.String())
	}
}

// small helper, avoids pulling in strings just for one check
func containsSubstring(haystack, needle string) bool {
	return len(haystack) >= len(needle) &&
		(func() bool {
			for i := 0; i+len(needle) <= len(haystack); i++ {
				if haystack[i:i+len(needle)] == needle {
					return true
				}
			}
			return false
		})()
}

var sqlErr = &mockError{"connection refused"}

type mockError struct{ msg string }

func (e *mockError) Error() string { return e.msg }
