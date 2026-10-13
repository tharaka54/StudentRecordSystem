package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func markPointer(value float64) *float64 { return &value }

// Boundary cases distinguish absence from zero and exercise unequal credits.
func TestAcademicSummaryTotals(t *testing.T) {
	tests := []struct {
		name                     string
		units                    []AcademicUnit
		wantWAM                  *float64
		graded, earned, enrolled int
	}{
		{"empty", nil, nil, 0, 0, 0},
		{"zero counts and weights differ", []AcademicUnit{
			{Status: "Completed", Mark: markPointer(80), CreditPoints: 10},
			{Status: "Completed", Mark: markPointer(0), CreditPoints: 30},
			{Status: "Completed", Mark: nil, CreditPoints: 20},
			{Status: "Withdrawn", Mark: markPointer(100), CreditPoints: 20},
			{Status: "Enrolled", Mark: nil, CreditPoints: 20},
		}, markPointer(20), 2, 10, 1},
		{"pass boundary and rounding", []AcademicUnit{
			{Status: "Completed", Mark: markPointer(50), CreditPoints: 10},
			{Status: "Completed", Mark: markPointer(79.99), CreditPoints: 20},
		}, markPointer(69.99), 2, 30, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			summary := AcademicSummary{Units: tc.units}
			summary.calculateTotals()
			if (summary.WAM == nil) != (tc.wantWAM == nil) ||
				(summary.WAM != nil && *summary.WAM != *tc.wantWAM) ||
				summary.GradedUnits != tc.graded || summary.EarnedCredits != tc.earned || summary.EnrolledUnits != tc.enrolled {
				t.Fatalf("unexpected totals: %+v", summary)
			}
		})
	}
}

type failingAcademicReader struct{ err error }

func (f failingAcademicReader) AcademicSummary(context.Context, int64) (AcademicSummary, error) {
	return AcademicSummary{}, f.err
}

// Tests the real SQLite/JSON contract, wrapped errors and the interface seam.
func TestAcademicSummaryContractAndCancellation(t *testing.T) {
	store, err := openStore(filepath.Join(t.TempDir(), "summary.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err = store.initialize(true); err != nil {
		t.Fatal(err)
	}
	summary, err := store.AcademicSummary(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Student.Number != "2026001" || summary.WAM == nil || *summary.WAM != 78 {
		t.Fatalf("wrong typed summary: %+v", summary)
	}
	if _, err = store.AcademicSummary(context.Background(), -1); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing student error lost: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = store.AcademicSummary(ctx, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled request not propagated: %v", err)
	}
	// A newly registered student has JSON null WAM and an empty array, not null units.
	res, err := store.DB.Exec(`INSERT INTO students(student_number,first_name,last_name,email,course_id) VALUES('EMPTY','Empty','Student','empty@example.local',1)`)
	if err != nil {
		t.Fatal(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	empty, err := store.AcademicSummary(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(empty)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err = json.Unmarshal(encoded, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["wam"] != nil || len(payload["units"].([]any)) != 0 {
		t.Fatalf("empty API contract: %s", encoded)
	}
	app := newApp(store)
	admin := &testClient{server: app.routes()}
	admin.signIn(t, "admin", "Admin@12345")
	app.academics = failingAcademicReader{fmt.Errorf("wrapped: %w", sql.ErrNoRows)}
	admin.call(t, "GET", "students/1/summary", nil, 404)
	app.academics = failingAcademicReader{errors.New("simulated storage failure")}
	admin.call(t, "GET", "students/1/summary", nil, 500)
}

// Independent real HTTP requests must retain each student's identity.
// This is a correctness check under concurrency, not a throughput benchmark.
func TestConcurrentStudentSummaries(t *testing.T) {
	store, err := openStore(filepath.Join(t.TempDir(), "concurrent.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err = store.initialize(true); err != nil {
		t.Fatal(err)
	}
	app := newApp(store)
	handler := app.routes()
	first, second := &testClient{server: handler}, &testClient{server: handler}
	first.signIn(t, "s2026001", "Student@12345")
	second.signIn(t, "s2026002", "Student@12345")
	server := httptest.NewServer(handler)
	defer server.Close()
	results := make(chan error, 24)
	for i := 0; i < 24; i++ {
		client, want := first, "2026001"
		if i%2 == 1 {
			client, want = second, "2026002"
		}
		go func(cookie *http.Cookie, expected string) {
			req, err := http.NewRequest(http.MethodGet, server.URL+"/api/student/overview", nil)
			if err != nil {
				results <- err
				return
			}
			req.AddCookie(cookie)
			response, err := server.Client().Do(req)
			if err != nil {
				results <- err
				return
			}
			defer response.Body.Close()
			var summary AcademicSummary
			if err = json.NewDecoder(response.Body).Decode(&summary); err != nil {
				results <- err
				return
			}
			if response.StatusCode != 200 || summary.Student.Number != expected {
				results <- fmt.Errorf("status=%d student=%s want=%s", response.StatusCode, summary.Student.Number, expected)
				return
			}
			results <- nil
		}(client.cookie, want)
	}
	for i := 0; i < 24; i++ {
		if err := <-results; err != nil {
			t.Error(err)
		}
	}
}

// A write can occur between requests, but each summary must agree with its own
// returned units. The writer uses atomic SQL updates while the reader runs.
func TestAcademicSummarySnapshotDuringWrites(t *testing.T) {
	store, err := openStore(filepath.Join(t.TempDir(), "snapshot.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err = store.initialize(true); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		for i := 0; i < 30; i++ {
			_, err := store.DB.Exec(`UPDATE results SET mark=? WHERE enrolment_id IN (SELECT enrolment_id FROM enrolments WHERE student_id=1)`, float64(50+i))
			if err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}()
	for i := 0; i < 30; i++ {
		summary, err := store.AcademicSummary(context.Background(), 1)
		if err != nil {
			t.Error(err)
			continue
		}
		var weighted float64
		var credits int
		for _, unit := range summary.Units {
			if unit.Status == "Completed" && unit.Mark != nil {
				weighted += *unit.Mark * float64(unit.CreditPoints)
				credits += unit.CreditPoints
			}
		}
		if summary.WAM == nil || credits == 0 || *summary.WAM != weighted/float64(credits) {
			t.Errorf("summary and its returned units disagree: %+v", summary)
		}
	}
	if err := <-done; err != nil {
		t.Error(err)
	}
}

// Measures just the typed SQLite summary over the seeded classroom dataset.
// Authentication, network, browser rendering and deployment load are excluded.
func BenchmarkAcademicSummary(b *testing.B) {
	store, err := openStore(filepath.Join(b.TempDir(), "bench.db"))
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()
	if _, err = store.initialize(true); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := store.AcademicSummary(context.Background(), 1); err != nil {
			b.Fatal(err)
		}
	}
}
