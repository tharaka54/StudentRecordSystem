package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

type testClient struct {
	server http.Handler
	cookie *http.Cookie
	csrf   string
}

func (c *testClient) call(t *testing.T, method, path string, body any, want int) map[string]any {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, "http://localhost/api/"+path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.cookie != nil {
		req.AddCookie(c.cookie)
	}
	if c.csrf != "" {
		req.Header.Set("X-CSRF-Token", c.csrf)
	}
	rec := httptest.NewRecorder()
	c.server.ServeHTTP(rec, req)
	if rec.Code != want {
		t.Fatalf("%s %s: got %d, want %d: %s", method, path, rec.Code, want, rec.Body.String())
	}
	if path == "login" && want == 200 {
		for _, cookie := range rec.Result().Cookies() {
			if cookie.Name == "student_session" && cookie.Value != "" {
				c.cookie = cookie
			}
		}
	}
	var result map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("invalid response: %v: %s", err, rec.Body.String())
	}
	return result
}
func (c *testClient) signIn(t *testing.T, user, password string) {
	t.Helper()
	c.call(t, "POST", "login", map[string]any{"username": user, "password": password}, 200)
	c.csrf = c.call(t, "GET", "me", nil, 200)["csrf_token"].(string)
}
func numberID(row map[string]any) int64         { return int64(row["id"].(float64)) }
func resourcePath(name string, id int64) string { return fmt.Sprintf("%s/%d", name, id) }

// This integration suite uses an isolated database and actual HTTP handlers.
func TestApplicationWorkflowAndStudentIsolation(t *testing.T) {
	file := filepath.Join(t.TempDir(), "records.db")
	store, err := openStore(file)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err = store.initialize(true); err != nil {
		t.Fatal(err)
	}
	server := newApp(store).routes()
	admin := &testClient{server: server}
	first := &testClient{server: server}
	second := &testClient{server: server}
	guest := &testClient{server: server}
	admin.signIn(t, "admin", "Admin@12345")
	first.signIn(t, "s2026001", "Student@12345")
	second.signIn(t, "s2026002", "Student@12345")
	t.Run("authentication and ownership", func(t *testing.T) {
		guest.call(t, "GET", "students", nil, 401)
		mine := first.call(t, "GET", "student/overview", nil, 200)
		if mine["student"].(map[string]any)["student_number"] != "2026001" || mine["wam"].(float64) != 78 {
			t.Fatalf("wrong student summary: %v", mine)
		}
		other := second.call(t, "GET", "student/overview", nil, 200)
		if other["student"].(map[string]any)["student_number"] != "2026002" {
			t.Fatal("student data leaked")
		}
		first.call(t, "GET", "student/overview?student_id=2", nil, 400)
		for _, path := range []string{"students", "students/2", "students/2/summary", "results", "results/3", "enrolments", "courses", "units", "users", "lookups", "dashboard", "student/2"} {
			first.call(t, "GET", path, nil, 403)
		}
		for _, method := range []string{"POST", "PUT", "DELETE"} {
			first.call(t, method, "results/1", map[string]any{"mark": 100}, 403)
		}
		first.call(t, "POST", "student/overview", map[string]any{"student_id": 2}, 405)
		admin.call(t, "GET", "student/overview", nil, 403)
		first.call(t, "GET", "logout", nil, 405)
		first.call(t, "GET", "student/overview", nil, 200)
		invalid := *admin
		invalid.csrf = "wrong"
		invalid.call(t, "POST", "courses", map[string]any{}, 403)
	})
	t.Run("database account constraints", func(t *testing.T) {
		for _, q := range []string{
			`INSERT INTO users(username,password_hash,full_name,email,role) VALUES ('bad','hash','Bad','bad@example.edu','Student')`,
			`INSERT INTO users(username,password_hash,full_name,email,role) VALUES ('bad','hash','Bad','bad@example.edu','General')`,
			`INSERT INTO users(username,password_hash,full_name,email,role,student_id) VALUES ('bad','hash','Bad','bad@example.edu','Admin',1)`,
			`INSERT INTO users(username,password_hash,full_name,email,role,student_id) VALUES ('bad','hash','Bad','bad@example.edu','Student',999)`,
			`INSERT INTO users(username,password_hash,full_name,email,role,student_id) VALUES ('bad','hash','Bad','bad@example.edu','Student',1)`,
		} {
			if _, err := store.DB.Exec(q); err == nil {
				t.Fatal("invalid user relationship accepted")
			}
		}
	})
	t.Run("server search and pagination", func(t *testing.T) {
		page := admin.call(t, "GET", "students?q=amara&page_size=1&page=1", nil, 200)
		if page["total"].(float64) != 1 || page["items"].([]any)[0].(map[string]any)["student_number"] != "2026002" {
			t.Fatalf("bad search: %v", page)
		}
		page = admin.call(t, "GET", "students?page_size=3&page=2", nil, 200)
		if len(page["items"].([]any)) != 3 || page["total"].(float64) != 8 {
			t.Fatal("pagination incorrect")
		}
		page = admin.call(t, "GET", "students?status=Inactive", nil, 200)
		if page["total"].(float64) != 1 {
			t.Fatal("status filter incorrect")
		}
		page = admin.call(t, "GET", "students?q=%27%20OR%201%3D1--", nil, 200)
		if page["total"].(float64) != 0 {
			t.Fatal("unsafe search")
		}
		admin.call(t, "GET", "students?page_size=101", nil, 400)
		users := admin.call(t, "GET", "users?role=Student", nil, 200)
		if strings.Contains(fmt.Sprint(users), "password_hash") {
			t.Fatal("password hashes exposed")
		}
	})
	t.Run("academic workflow and weighted marks", func(t *testing.T) {
		courseData := map[string]any{"course_code": "TEST", "course_name": "Test Course", "duration_years": 3, "description": "Testing"}
		cid := numberID(admin.call(t, "POST", "courses", courseData, 200))
		cp := resourcePath("courses", cid)
		courseData["course_name"] = "Updated Course"
		admin.call(t, "PUT", cp, courseData, 200)
		unit := func(code string, credits int) int64 {
			return numberID(admin.call(t, "POST", "units", map[string]any{"unit_code": code, "unit_name": code, "credit_points": credits, "course_id": cid}, 200))
		}
		u1, u2, u3 := unit("TEST1", 10), unit("TEST2", 30), unit("TEST3", 20)
		sd := map[string]any{"student_number": "900001", "first_name": "Test", "last_name": "Student", "email": "test.student@example.edu", "phone": "", "date_of_birth": "2003-01-02", "address": "", "status": "Active", "course_id": cid}
		sid := numberID(admin.call(t, "POST", "students", sd, 200))
		sp := resourcePath("students", sid)
		admin.call(t, "POST", "students", sd, 400)
		sd["phone"] = "0771234567"
		admin.call(t, "PUT", sp, sd, 200)
		newSummary := admin.call(t, "GET", sp+"/summary", nil, 200)
		if newSummary["wam"] != nil {
			t.Fatal("ungraded student should have no WAM")
		}
		enrol := func(uid int64, status string, want int) map[string]any {
			return admin.call(t, "POST", "enrolments", map[string]any{"student_id": sid, "unit_id": uid, "semester": 2, "academic_year": 2026, "enrolment_date": "2026-08-01", "status": status}, want)
		}
		enrol(1, "Enrolled", 400)
		enrol(u1, "Completed", 400)
		e1 := numberID(enrol(u1, "Enrolled", 200))
		e2 := numberID(enrol(u2, "Enrolled", 200))
		e3 := numberID(enrol(u3, "Withdrawn", 200))
		enrol(u1, "Enrolled", 400)
		admin.call(t, "POST", "results", map[string]any{"enrolment_id": e1, "mark": 101}, 400)
		admin.call(t, "POST", "results", map[string]any{"enrolment_id": e3, "mark": 85}, 400)
		r1 := numberID(admin.call(t, "POST", "results", map[string]any{"enrolment_id": e1, "mark": 80}, 200))
		r2 := numberID(admin.call(t, "POST", "results", map[string]any{"enrolment_id": e2, "mark": 40}, 200))
		summary := admin.call(t, "GET", sp+"/summary", nil, 200)
		if summary["wam"].(float64) != 50 {
			t.Fatalf("unequal-credit WAM: %v", summary)
		}
		admin.call(t, "PUT", resourcePath("results", r2), map[string]any{"enrolment_id": e2, "mark": 0}, 200)
		summary = admin.call(t, "GET", sp+"/summary", nil, 200)
		if math.Abs(summary["wam"].(float64)-20) > 0.001 {
			t.Fatal("zero-mark WAM incorrect")
		}
		admin.call(t, "PUT", resourcePath("results", r1), map[string]any{"enrolment_id": e2, "mark": 90}, 400)
		admin.call(t, "DELETE", cp, nil, 400)
		admin.call(t, "DELETE", resourcePath("results", r2), nil, 200)
		row := admin.call(t, "GET", resourcePath("enrolments", e2), nil, 200)
		if row["status"] != "Enrolled" {
			t.Fatal("result deletion did not reopen enrolment")
		}
		summary = admin.call(t, "GET", sp+"/summary", nil, 200)
		if summary["wam"].(float64) != 80 {
			t.Fatal("WAM did not recalculate")
		}
		for _, eid := range []int64{e1, e2, e3} {
			admin.call(t, "DELETE", resourcePath("enrolments", eid), nil, 200)
		}
		admin.call(t, "GET", resourcePath("results", r1), nil, 404)
		admin.call(t, "DELETE", sp, nil, 200)
		for _, uid := range []int64{u1, u2, u3} {
			admin.call(t, "DELETE", resourcePath("units", uid), nil, 200)
		}
		admin.call(t, "DELETE", cp, nil, 200)
	})
	t.Run("password change reset disable and logout", func(t *testing.T) {
		second.call(t, "POST", "password", map[string]any{"current_password": "incorrect", "new_password": "ChangedPassword!1"}, 400)
		second.call(t, "POST", "password", map[string]any{"current_password": "Student@12345", "new_password": "ChangedPassword!1"}, 200)
		second.call(t, "GET", "student/overview", nil, 401)
		second.call(t, "POST", "login", map[string]any{"username": "s2026002", "password": "Student@12345"}, 401)
		second.signIn(t, "s2026002", "ChangedPassword!1")
		account := admin.call(t, "GET", "users/3", nil, 200)
		update := map[string]any{"username": account["username"], "full_name": account["full_name"], "email": account["email"], "role": "Student", "student_id": account["student_id"], "is_active": true, "password": "ResetPassword!1"}
		admin.call(t, "PUT", "users/3", update, 200)
		second.call(t, "GET", "student/overview", nil, 401)
		second.signIn(t, "s2026002", "ResetPassword!1")
		update["is_active"] = false
		delete(update, "password")
		admin.call(t, "PUT", "users/3", update, 200)
		second.call(t, "GET", "student/overview", nil, 401)
		second.call(t, "POST", "login", map[string]any{"username": "s2026002", "password": "ResetPassword!1"}, 401)
		update["is_active"] = true
		admin.call(t, "PUT", "users/3", update, 200)
		second.signIn(t, "s2026002", "ResetPassword!1")
		second.call(t, "POST", "logout", nil, 200)
		second.call(t, "GET", "student/overview", nil, 401)
		update["student_id"] = 1
		admin.call(t, "PUT", "users/3", update, 400)
		admin.call(t, "DELETE", "users/1", nil, 400)
	})
	t.Run("reopen persistence", func(t *testing.T) {
		if err := store.Close(); err != nil {
			t.Fatal(err)
		}
		reopened, err := openStore(file)
		if err != nil {
			t.Fatal(err)
		}
		defer reopened.Close()
		if _, err = reopened.initialize(false); err != nil {
			t.Fatal(err)
		}
		var count int
		if err = reopened.DB.QueryRow(`SELECT COUNT(*) FROM students`).Scan(&count); err != nil || count != 8 {
			t.Fatalf("persistence: %d %v", count, err)
		}
		var hash string
		if err = reopened.DB.QueryRow(`SELECT password_hash FROM users WHERE username='s2026002'`).Scan(&hash); err != nil || !verifyPassword("ResetPassword!1", hash) {
			t.Fatal("password reset did not persist")
		}
		var sid sql.NullInt64
		if err = reopened.DB.QueryRow(`SELECT student_id FROM users WHERE username='s2026002'`).Scan(&sid); err != nil || sid.Int64 != 2 {
			t.Fatal("account ownership did not persist")
		}
	})
}
