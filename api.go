package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
)

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
func apiError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
func readJSON(r *http.Request, out any) error {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		return errors.New("send JSON with Content-Type: application/json")
	}
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return errors.New("invalid JSON request or request too large")
	}
	var extra any
	if e := decoder.Decode(&extra); e != io.EOF {
		return errors.New("only one JSON object is allowed")
	}
	return nil
}

type fieldDef struct {
	Name, Kind string
	Required   bool
	Max        int
}
type resourceDef struct {
	Table, PK, Select, Alias string
	Fields                   []fieldDef
}

func defs() map[string]resourceDef {
	text := func(n string, required bool, max int) fieldDef { return fieldDef{n, "text", required, max} }
	intf := func(n string, required bool) fieldDef { return fieldDef{n, "int", required, 0} }
	return map[string]resourceDef{
		"users":      {"users", "user_id", `SELECT user_id,username,full_name,email,role,student_id,is_active,created_at,updated_at FROM users`, "users", []fieldDef{text("username", true, 50), text("full_name", true, 100), text("email", true, 200), text("role", true, 10), {"student_id", "nullableInt", false, 0}, {"is_active", "bool", true, 0}, text("password", false, 200)}},
		"courses":    {"courses", "course_id", `SELECT courses.* FROM courses`, "courses", []fieldDef{text("course_code", true, 30), text("course_name", true, 120), intf("duration_years", true), text("description", false, 500)}},
		"students":   {"students", "student_id", `SELECT students.*,courses.course_code,courses.course_name FROM students JOIN courses ON students.course_id=courses.course_id`, "students", []fieldDef{text("student_number", true, 30), text("first_name", true, 100), text("last_name", true, 100), text("email", true, 200), text("phone", false, 30), text("date_of_birth", false, 10), text("address", false, 300), text("status", true, 15), intf("course_id", true)}},
		"units":      {"units", "unit_id", `SELECT units.*,courses.course_code,courses.course_name FROM units JOIN courses ON units.course_id=courses.course_id`, "units", []fieldDef{text("unit_code", true, 30), text("unit_name", true, 120), intf("credit_points", true), intf("course_id", true)}},
		"enrolments": {"enrolments", "enrolment_id", `SELECT enrolments.*,students.student_number,students.first_name || ' ' || students.last_name AS student_name,units.unit_code,units.unit_name,units.credit_points,students.course_id FROM enrolments JOIN students ON enrolments.student_id=students.student_id JOIN units ON enrolments.unit_id=units.unit_id`, "enrolments", []fieldDef{intf("student_id", true), intf("unit_id", true), intf("semester", true), intf("academic_year", true), text("enrolment_date", true, 10), text("status", true, 15)}},
		"results":    {"results", "result_id", `SELECT results.*,enrolments.student_id,enrolments.unit_id,enrolments.semester,enrolments.academic_year,students.course_id,students.student_number,students.first_name || ' ' || students.last_name AS student_name,units.unit_code,units.unit_name,units.credit_points FROM results JOIN enrolments ON results.enrolment_id=enrolments.enrolment_id JOIN students ON students.student_id=enrolments.student_id JOIN units ON units.unit_id=enrolments.unit_id`, "results", []fieldDef{intf("enrolment_id", true), {"mark", "float", true, 0}}},
	}
}
func (a *App) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/login", a.login)
	mux.HandleFunc("/api/me", a.protect(a.handleMe, false))
	mux.HandleFunc("/api/logout", a.protect(a.handleLogout, false))
	mux.HandleFunc("/api/dashboard", a.protect(a.handleDashboard, true))
	mux.HandleFunc("/api/student/overview", a.protect(a.handleStudentOverview, false))
	mux.HandleFunc("/api/password", a.protect(a.handlePassword, false))
	mux.HandleFunc("/api/lookups", a.protect(a.handleLookups, true))
	mux.HandleFunc("/api/", a.protect(a.handleResource, true))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "HEAD" {
			apiError(w, 405, "Method not allowed.")
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if path != "index.html" && path != "style.css" && path != "app.js" && path != "resources.js" {
			http.NotFound(w, r)
			return
		}
		b, e := bundled.ReadFile("web/" + path)
		if e != nil {
			http.Error(w, "internal error", 500)
			return
		}
		switch path {
		case "index.html":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
		case "style.css":
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
		default:
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Write(b)
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; object-src 'none'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		if !sameOrigin(r) {
			apiError(w, 403, "cross-origin request blocked")
			return
		}
		mux.ServeHTTP(w, r)
	})
}
func (a *App) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		apiError(w, 405, "method not allowed")
		return
	}
	counts := map[string]int{}
	for _, tab := range []string{"students", "courses", "units", "enrolments", "results", "users"} {
		if tab == "users" {
			continue
		}
		var n int
		if err := a.store.DB.QueryRow("SELECT COUNT(*) FROM " + tab).Scan(&n); err != nil {
			apiError(w, 500, "dashboard unavailable")
			return
		}
		counts[tab] = n
	}
	var mean sql.NullFloat64
	e := a.store.DB.QueryRow(`SELECT SUM(r.mark*u.credit_points)/NULLIF(SUM(u.credit_points),0) FROM results r JOIN enrolments e ON e.enrolment_id=r.enrolment_id JOIN units u ON u.unit_id=e.unit_id WHERE e.status='Completed'`).Scan(&mean)
	if e != nil {
		apiError(w, 500, "dashboard unavailable")
		return
	}
	latest, err := getMany(a.store.DB, `SELECT s.student_id,s.student_number,s.first_name || ' ' || s.last_name AS student_name,s.status,c.course_name FROM students s JOIN courses c ON c.course_id=s.course_id ORDER BY s.student_id DESC LIMIT 5`)
	if err != nil {
		apiError(w, 500, "dashboard unavailable")
		return
	}
	var wam any = nil
	if mean.Valid {
		wam = mean.Float64
	}
	writeJSON(w, 200, map[string]any{"counts": counts, "overall_wam": wam, "recent_students": latest})
}
func (a *App) handleResource(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/"), "/")
	pieces := strings.Split(path, "/")
	if len(pieces) > 3 || len(pieces) == 0 {
		apiError(w, 404, "not found")
		return
	}
	name := pieces[0]
	def, ok := defs()[name]
	if !ok {
		apiError(w, 404, "unknown resource")
		return
	}
	acc, _, ok := a.currentUser(r)
	if !ok {
		apiError(w, 401, "please log in")
		return
	}
	if name == "users" && acc.Role != "Admin" {
		apiError(w, 403, "admin only")
		return
	}
	if r.Method != "GET" && acc.Role != "Admin" {
		apiError(w, 403, "Administrator access required.")
		return
	}
	var id int64
	if len(pieces) > 1 {
		n, err := strconv.ParseInt(pieces[1], 10, 64)
		if err != nil || n < 1 {
			apiError(w, 400, "invalid record ID")
			return
		}
		id = n
	}
	if len(pieces) == 3 && name == "students" && pieces[2] == "summary" && r.Method == "GET" {
		a.studentSummary(w, id)
		return
	}
	if len(pieces) == 3 {
		apiError(w, 404, "not found")
		return
	}
	switch r.Method {
	case "GET":
		a.listResource(w, r, name, def, id)
	case "POST":
		if id != 0 {
			apiError(w, 405, "use POST on the collection")
			return
		}
		a.saveResource(w, r, name, def, 0)
	case "PUT":
		if id == 0 {
			apiError(w, 405, "record ID required")
			return
		}
		a.saveResource(w, r, name, def, id)
	case "DELETE":
		if id == 0 {
			apiError(w, 405, "record ID required")
			return
		}
		a.removeResource(w, name, def, id, acc.ID)
	default:
		apiError(w, 405, "method not allowed")
	}
}
func validEmail(v string) bool {
	address, err := mail.ParseAddress(v)
	return err == nil && address.Address == v && len(v) <= 200
}
func validOption(v string, opts ...string) bool {
	for _, s := range opts {
		if v == s {
			return true
		}
	}
	return false
}
func (a *App) validateResource(name string, def resourceDef, raw map[string]json.RawMessage, updating bool) (map[string]any, error) {
	out := map[string]any{}
	allow := map[string]bool{}
	for _, f := range def.Fields {
		allow[f.Name] = true
		value, found := raw[f.Name]
		if !found {
			if f.Required {
				return nil, fmt.Errorf("%s is required", f.Name)
			}
			continue
		}
		if string(value) == "null" && f.Kind == "nullableInt" {
			out[f.Name] = nil
			continue
		}
		if string(value) == "null" {
			return nil, fmt.Errorf("%s cannot be null", f.Name)
		}
		switch f.Kind {
		case "text":
			var s string
			if err := json.Unmarshal(value, &s); err != nil {
				return nil, fmt.Errorf("%s must be text", f.Name)
			}
			if f.Name != "password" {
				s = strings.TrimSpace(s)
			}
			if f.Required && s == "" {
				return nil, fmt.Errorf("%s is required", f.Name)
			}
			if len(s) > f.Max {
				return nil, fmt.Errorf("%s is too long (max %d bytes)", f.Name, f.Max)
			}
			out[f.Name] = s
		case "int", "nullableInt":
			var i int64
			if err := json.Unmarshal(value, &i); err != nil {
				return nil, fmt.Errorf("%s must be a whole number", f.Name)
			}
			out[f.Name] = i
		case "float":
			var num float64
			if err := json.Unmarshal(value, &num); err != nil {
				return nil, fmt.Errorf("%s must be a number", f.Name)
			}
			if num != num || num < 0 || num > 100 {
				return nil, errors.New("mark must be between 0 and 100")
			}
			out[f.Name] = num
		case "bool":
			var v bool
			if err := json.Unmarshal(value, &v); err != nil {
				return nil, fmt.Errorf("%s must be true or false", f.Name)
			}
			if v {
				out[f.Name] = 1
			} else {
				out[f.Name] = 0
			}
		}
	}
	for k := range raw {
		if !allow[k] {
			return nil, fmt.Errorf("unrecognised field: %s", k)
		}
	}
	str := func(k string) string { v, _ := out[k].(string); return v }
	num := func(k string) int64 { v, _ := out[k].(int64); return v }
	switch name {
	case "users":
		if !validEmail(str("email")) {
			return nil, errors.New("a valid email is required")
		}
		if !validOption(str("role"), "Admin", "Student") {
			return nil, errors.New("role must be Admin or Student")
		}
		if str("role") == "Student" {
			if !exists(a.store.DB, "students", "student_id", num("student_id")) {
				return nil, errors.New("Select a student for this account.")
			}
		} else {
			if num("student_id") != 0 {
				return nil, errors.New("An Admin account cannot be linked to a student.")
			}
			out["student_id"] = nil
		}
		if len(str("username")) < 3 {
			return nil, errors.New("username must be at least 3 characters")
		}
		if !updating && str("password") == "" {
			return nil, errors.New("password is required for new users")
		}
		if str("password") != "" {
			hashed, err := hashPassword(str("password"))
			if err != nil {
				return nil, err
			}
			out["password_hash"] = hashed
		}
		delete(out, "password")
	case "students":
		if !validEmail(str("email")) {
			return nil, errors.New("a valid email is required")
		}
		if !validOption(str("status"), "Active", "Inactive") {
			return nil, errors.New("invalid student status")
		}
		if !parseDate(str("date_of_birth")) || (str("date_of_birth") != "" && str("date_of_birth") > today()) {
			return nil, errors.New("date of birth must be a valid past or present date")
		}
		if !exists(a.store.DB, "courses", "course_id", num("course_id")) {
			return nil, errors.New("select an existing course")
		}
	case "courses":
		if num("duration_years") < 1 || num("duration_years") > 10 {
			return nil, errors.New("course duration must be between 1 and 10 years")
		}
	case "units":
		if num("credit_points") < 1 || num("credit_points") > 100 {
			return nil, errors.New("unit credits must be between 1 and 100")
		}
		if !exists(a.store.DB, "courses", "course_id", num("course_id")) {
			return nil, errors.New("select an existing course")
		}
	case "enrolments":
		if num("semester") < 1 || num("semester") > 3 {
			return nil, errors.New("semester must be 1, 2 or 3")
		}
		if num("academic_year") < 2000 || num("academic_year") > 2100 {
			return nil, errors.New("enter a valid academic year")
		}
		if !parseDate(str("enrolment_date")) || str("enrolment_date") == "" {
			return nil, errors.New("enrolment date must be YYYY-MM-DD")
		}
		if !validOption(str("status"), "Enrolled", "Completed", "Withdrawn") {
			return nil, errors.New("invalid enrolment status")
		}
		if err := validateEnrolment(a.store.DB, num("student_id"), num("unit_id")); err != nil {
			return nil, err
		}
	case "results":
		if !exists(a.store.DB, "enrolments", "enrolment_id", num("enrolment_id")) {
			return nil, errors.New("select an existing enrolment")
		}
		var status string
		if err := a.store.DB.QueryRow(`SELECT status FROM enrolments WHERE enrolment_id=?`, num("enrolment_id")).Scan(&status); err != nil {
			return nil, errors.New("invalid enrolment")
		}
		if status == "Withdrawn" {
			return nil, errors.New("cannot add a result to a withdrawn enrolment")
		}
		out["grade"] = gradeForMark(out["mark"].(float64))
	}
	return out, nil
}
func (a *App) saveResource(w http.ResponseWriter, r *http.Request, name string, def resourceDef, id int64) {
	var raw map[string]json.RawMessage
	if err := readJSON(r, &raw); err != nil {
		apiError(w, 400, err.Error())
		return
	}
	if raw == nil {
		apiError(w, 400, "JSON object required")
		return
	}
	values, err := a.validateResource(name, def, raw, id > 0)
	if err != nil {
		apiError(w, 400, err.Error())
		return
	}
	if id > 0 && !exists(a.store.DB, name, def.PK, id) {
		apiError(w, 404, "record not found")
		return
	}
	// Account ownership is fixed after creation; resetting a password never changes ownership.
	if name == "users" && id > 0 {
		var oldRole string
		var oldStudent sql.NullInt64
		if err := a.store.DB.QueryRow(`SELECT role,student_id FROM users WHERE user_id=?`, id).Scan(&oldRole, &oldStudent); err != nil {
			apiError(w, 500, "Unable to read account.")
			return
		}
		newStudent, _ := values["student_id"].(int64)
		if oldRole != values["role"] || oldStudent.Int64 != newStudent {
			apiError(w, 400, "Account role and student link cannot be changed. Create a new account instead.")
			return
		}
	}
	// Prevent removing the last active admin.
	if name == "users" && id > 0 {
		if err = a.checkAdminChanges(id, values); err != nil {
			apiError(w, 400, err.Error())
			return
		}
	}
	// Changing a student's course would invalidate existing unit enrolments.
	if name == "students" && id > 0 {
		var oldCourse int64
		_ = a.store.DB.QueryRow(`SELECT course_id FROM students WHERE student_id=?`, id).Scan(&oldCourse)
		if oldCourse != values["course_id"] {
			var n int
			_ = a.store.DB.QueryRow(`SELECT COUNT(*) FROM enrolments WHERE student_id=?`, id).Scan(&n)
			if n > 0 {
				apiError(w, 400, "remove the student's enrolments before changing their course")
				return
			}
		}
	}
	if name == "units" && id > 0 {
		var oldCourse int64
		_ = a.store.DB.QueryRow(`SELECT course_id FROM units WHERE unit_id=?`, id).Scan(&oldCourse)
		if oldCourse != values["course_id"] {
			var n int
			_ = a.store.DB.QueryRow(`SELECT COUNT(*) FROM enrolments WHERE unit_id=?`, id).Scan(&n)
			if n > 0 {
				apiError(w, 400, "remove existing enrolments before changing the unit's course")
				return
			}
		}
	}
	if name == "enrolments" && id > 0 {
		var n int
		_ = a.store.DB.QueryRow(`SELECT COUNT(*) FROM results WHERE enrolment_id=?`, id).Scan(&n)
		if n > 0 && values["status"] != "Completed" {
			apiError(w, 400, "an enrolment with a recorded result must remain Completed; delete its result first")
			return
		}
		if n > 0 {
			var sid, uid int64
			_ = a.store.DB.QueryRow(`SELECT student_id,unit_id FROM enrolments WHERE enrolment_id=?`, id).Scan(&sid, &uid)
			if sid != values["student_id"] || uid != values["unit_id"] {
				apiError(w, 400, "cannot change student or unit after a result is recorded")
				return
			}
		}
	}
	if name == "results" && id > 0 {
		var enrolmentID int64
		if e := a.store.DB.QueryRow(`SELECT enrolment_id FROM results WHERE result_id=?`, id).Scan(&enrolmentID); e != nil {
			apiError(w, 500, "Unable to read result.")
			return
		}
		if enrolmentID != values["enrolment_id"] {
			apiError(w, 400, "A result cannot be moved to another enrolment.")
			return
		}
	}
	if name == "enrolments" && values["status"] == "Completed" {
		var n int
		if err := a.store.DB.QueryRow(`SELECT COUNT(*) FROM results WHERE enrolment_id=?`, id).Scan(&n); err != nil {
			apiError(w, 500, "Unable to check results.")
			return
		}
		if n == 0 {
			apiError(w, 400, "Enter a result to complete an enrolment.")
			return
		}
	}
	columns := make([]string, 0, len(values))
	args := make([]any, 0, len(values)+1)
	// Fixed ordering ensures consistent SQL and safe parameter binding.
	for _, field := range def.Fields {
		col := field.Name
		if col == "password" {
			col = "password_hash"
		}
		if v, ok := values[col]; ok {
			columns = append(columns, col)
			args = append(args, v)
		}
	}
	if name == "results" {
		columns = append(columns, "grade")
		args = append(args, values["grade"])
	}
	if len(columns) == 0 {
		apiError(w, 400, "no values supplied")
		return
	}
	var sqlText string
	if id == 0 {
		placeholders := make([]string, len(columns))
		for i := range placeholders {
			placeholders[i] = "?"
		}
		sqlText = "INSERT INTO " + name + " (" + strings.Join(columns, ",") + ") VALUES (" + strings.Join(placeholders, ",") + ")"
	} else {
		updates := make([]string, len(columns))
		for i, c := range columns {
			updates[i] = c + "=?"
		}
		sqlText = "UPDATE " + name + " SET " + strings.Join(updates, ",") + ",updated_at=CURRENT_TIMESTAMP WHERE " + def.PK + "=?"
		args = append(args, id)
	}
	// Result saves also complete their associated enrolment in one database transaction.
	if name == "results" {
		tx, e := a.store.DB.Begin()
		if e != nil {
			apiError(w, 500, "database unavailable")
			return
		}
		defer tx.Rollback()
		result, e := tx.Exec(sqlText, args...)
		if e != nil {
			apiError(w, 400, requireDatabaseConstraint(e))
			return
		}
		if id == 0 {
			id, e = result.LastInsertId()
			if e != nil {
				apiError(w, 500, "could not read new ID")
				return
			}
		}
		_, e = tx.Exec(`UPDATE enrolments SET status='Completed',updated_at=CURRENT_TIMESTAMP WHERE enrolment_id=?`, values["enrolment_id"])
		if e != nil {
			apiError(w, 400, requireDatabaseConstraint(e))
			return
		}
		if e = tx.Commit(); e != nil {
			apiError(w, 500, "unable to save result")
			return
		}
	} else {
		res, e := a.store.DB.Exec(sqlText, args...)
		if e != nil {
			apiError(w, 400, requireDatabaseConstraint(e))
			return
		}
		if id == 0 {
			id, e = res.LastInsertId()
			if e != nil {
				apiError(w, 500, "could not read new ID")
				return
			}
		}
	}
	if name == "users" && id > 0 {
		a.revokeUserSessions(id)
	}
	writeJSON(w, 200, map[string]any{"id": id, "status": "saved"})
}
func (a *App) checkAdminChanges(id int64, values map[string]any) error {
	var role string
	var active int
	err := a.store.DB.QueryRow(`SELECT role,is_active FROM users WHERE user_id=?`, id).Scan(&role, &active)
	if err != nil {
		return err
	}
	if role != "Admin" {
		return nil
	}
	if values["role"] == "Admin" && values["is_active"] == 1 {
		return nil
	}
	var count int
	err = a.store.DB.QueryRow(`SELECT COUNT(*) FROM users WHERE role='Admin' AND is_active=1`).Scan(&count)
	if err != nil {
		return err
	}
	if count <= 1 {
		return errors.New("cannot disable or demote the last active admin")
	}
	return nil
}
func (a *App) removeResource(w http.ResponseWriter, name string, def resourceDef, id, callerID int64) {
	if !exists(a.store.DB, name, def.PK, id) {
		apiError(w, 404, "record not found")
		return
	}
	if name == "users" {
		if id == callerID {
			apiError(w, 400, "you cannot delete your own account")
			return
		}
		var role string
		var active int
		_ = a.store.DB.QueryRow(`SELECT role,is_active FROM users WHERE user_id=?`, id).Scan(&role, &active)
		if role == "Admin" && active == 1 {
			var count int
			_ = a.store.DB.QueryRow(`SELECT COUNT(*) FROM users WHERE role='Admin' AND is_active=1`).Scan(&count)
			if count <= 1 {
				apiError(w, 400, "cannot delete last active admin")
				return
			}
		}
	}
	err := a.deleteRecord(name, id)
	if err != nil {
		apiError(w, 400, requireDatabaseConstraint(err))
		return
	}
	if name == "users" {
		a.revokeUserSessions(id)
	}
	writeJSON(w, 200, map[string]string{"status": "deleted"})
}

// studentSummary is reached only through the Admin-protected resource router.
func (a *App) studentSummary(w http.ResponseWriter, id int64) {
	data, err := a.academicSummary(id)
	if err == sql.ErrNoRows {
		apiError(w, 404, "Student not found.")
		return
	}
	if err != nil {
		log.Printf("summary: %v", err)
		apiError(w, 500, "Unable to load academic summary.")
		return
	}
	writeJSON(w, 200, data)
}
