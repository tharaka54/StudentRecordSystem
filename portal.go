package main

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func today() string { return time.Now().Format("2006-01-02") }

// Personal endpoints never accept a student ID. Ownership comes from the session.
func (a *App) handleStudentOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apiError(w, 405, "Method not allowed.")
		return
	}
	acc, _, _ := a.currentUser(r)
	if acc.Role != "Student" || acc.StudentID < 1 {
		apiError(w, 403, "Student access required.")
		return
	}
	if len(r.URL.Query()) > 0 {
		apiError(w, 400, "This endpoint does not accept record identifiers or filters.")
		return
	}
	data, err := a.academicSummary(acc.StudentID)
	if err != nil {
		apiError(w, 500, "Unable to load your academic record.")
		return
	}
	writeJSON(w, 200, data)
}

// A reset invalidates every session, including the caller's. Sign in again afterward.
func (a *App) handlePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apiError(w, 405, "Method not allowed.")
		return
	}
	var in struct {
		Current string `json:"current_password"`
		New     string `json:"new_password"`
	}
	if err := readJSON(r, &in); err != nil {
		apiError(w, 400, err.Error())
		return
	}
	if len(in.Current) > 200 {
		apiError(w, 400, "Current password is incorrect.")
		return
	}
	acc, _, _ := a.currentUser(r)
	var old string
	if err := a.store.DB.QueryRow(`SELECT password_hash FROM users WHERE user_id=?`, acc.ID).Scan(&old); err != nil {
		apiError(w, 500, "Unable to update password.")
		return
	}
	if !verifyPassword(in.Current, old) {
		apiError(w, 400, "Current password is incorrect.")
		return
	}
	if in.Current == in.New {
		apiError(w, 400, "Choose a different password.")
		return
	}
	hashed, err := hashPassword(in.New)
	if err != nil {
		apiError(w, 400, err.Error())
		return
	}
	if _, err = a.store.DB.Exec(`UPDATE users SET password_hash=?,updated_at=CURRENT_TIMESTAMP WHERE user_id=?`, hashed, acc.ID); err != nil {
		apiError(w, 500, "Unable to update password.")
		return
	}
	a.revokeUserSessions(acc.ID)
	a.clearSession(w, r)
	writeJSON(w, 200, map[string]string{"status": "Password updated. Please sign in again."})
}

func (a *App) academicSummary(studentID int64) (map[string]any, error) {
	info, err := getOne(a.store.DB, `SELECT s.*,c.course_code,c.course_name,c.duration_years,c.description AS course_description FROM students s JOIN courses c ON c.course_id=s.course_id WHERE s.student_id=?`, studentID)
	if err != nil {
		return nil, err
	}
	rows, err := getMany(a.store.DB, `SELECT e.enrolment_id,e.unit_id,e.semester,e.academic_year,e.enrolment_date,e.status,u.unit_code,u.unit_name,u.credit_points,r.result_id,r.mark,r.grade FROM enrolments e JOIN units u ON u.unit_id=e.unit_id LEFT JOIN results r ON r.enrolment_id=e.enrolment_id WHERE e.student_id=? ORDER BY e.academic_year DESC,e.semester DESC,u.unit_code`, studentID)
	if err != nil {
		return nil, err
	}
	wam, n, err := computeWAM(a.store.DB, studentID)
	if err != nil {
		return nil, err
	}
	var wamValue any
	if n > 0 {
		wamValue = wam
	}
	var earned, enrolled int
	err = a.store.DB.QueryRow(`SELECT COALESCE(SUM(CASE WHEN e.status='Completed' AND r.mark>=50 THEN u.credit_points ELSE 0 END),0),COALESCE(SUM(CASE WHEN e.status='Enrolled' THEN 1 ELSE 0 END),0) FROM enrolments e JOIN units u ON u.unit_id=e.unit_id LEFT JOIN results r ON r.enrolment_id=e.enrolment_id WHERE e.student_id=?`, studentID).Scan(&earned, &enrolled)
	if err != nil {
		return nil, err
	}
	return map[string]any{"student": info, "units": rows, "wam": wamValue, "graded_units": n, "earned_credits": earned, "enrolled_units": enrolled}, nil
}

// Search identifiers are allowlisted; all search values are bound parameters.
func (a *App) listResource(w http.ResponseWriter, r *http.Request, name string, def resourceDef, id int64) {
	base := " FROM (" + def.Select + ") AS records"
	if id > 0 {
		row, err := getOne(a.store.DB, "SELECT *"+base+" WHERE "+def.PK+"=?", id)
		if err == sql.ErrNoRows {
			apiError(w, 404, "Record not found.")
			return
		}
		if err != nil {
			apiError(w, 500, "Unable to load record.")
			return
		}
		writeJSON(w, 200, row)
		return
	}
	query := r.URL.Query()
	page, size := 1, 10
	for key, dst := range map[string]*int{"page": &page, "page_size": &size} {
		if query.Get(key) != "" {
			n, err := strconv.Atoi(query.Get(key))
			if err != nil || n < 1 || n > 1000000 {
				apiError(w, 400, "Invalid pagination.")
				return
			}
			*dst = n
		}
	}
	if size > 100 {
		apiError(w, 400, "Page size must be between 1 and 100.")
		return
	}
	conditions := []string{}
	args := []any{}
	term := strings.TrimSpace(query.Get("q"))
	if len(term) > 100 {
		apiError(w, 400, "Search must be 100 characters or fewer.")
		return
	}
	if term != "" {
		columns := map[string][]string{
			"users": {"username", "full_name", "email"}, "students": {"student_number", "first_name", "last_name", "email", "course_name"},
			"courses": {"course_code", "course_name"}, "units": {"unit_code", "unit_name", "course_name"},
			"enrolments": {"student_number", "student_name", "unit_code", "unit_name"}, "results": {"student_number", "student_name", "unit_code", "unit_name", "grade"},
		}[name]
		search := []string{}
		term = strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(term)
		for _, col := range columns {
			search = append(search, col+` LIKE ? ESCAPE '\'`)
			args = append(args, "%"+term+"%")
		}
		conditions = append(conditions, "("+strings.Join(search, " OR ")+")")
	}
	allowed := map[string][]string{"students": {"status", "course_id"}, "courses": {}, "units": {"course_id"}, "enrolments": {"status", "academic_year", "semester", "student_id"}, "results": {"academic_year", "semester", "student_id", "grade"}, "users": {"role", "is_active"}}[name]
	for _, col := range allowed {
		if v := query.Get(col); v != "" {
			conditions = append(conditions, col+"=?")
			args = append(args, v)
		}
	}
	if len(conditions) > 0 {
		base += " WHERE " + strings.Join(conditions, " AND ")
	}
	var total int
	if err := a.store.DB.QueryRow("SELECT COUNT(*)"+base, args...).Scan(&total); err != nil {
		apiError(w, 500, "Unable to count records.")
		return
	}
	params := append(append([]any{}, args...), size, (page-1)*size)
	rows, err := getMany(a.store.DB, "SELECT *"+base+" ORDER BY "+def.PK+" DESC LIMIT ? OFFSET ?", params...)
	if err != nil {
		apiError(w, 500, "Unable to load records.")
		return
	}
	writeJSON(w, 200, map[string]any{"items": rows, "total": total, "page": page, "page_size": size})
}

// Only Admin receives relationship options used by management forms.
func (a *App) handleLookups(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		apiError(w, 405, "Method not allowed.")
		return
	}
	queries := map[string]string{
		"courses":    `SELECT course_id,course_code,course_name FROM courses ORDER BY course_code`,
		"students":   `SELECT s.student_id,s.student_number,s.first_name,s.last_name,s.email,s.status,s.course_id,u.user_id FROM students s LEFT JOIN users u ON u.student_id=s.student_id ORDER BY s.student_number`,
		"units":      `SELECT unit_id,unit_code,unit_name,course_id,credit_points FROM units ORDER BY unit_code`,
		"enrolments": `SELECT e.enrolment_id,e.student_id,e.unit_id,e.semester,e.academic_year,e.status,s.student_number,s.first_name,s.last_name,u.unit_code,r.result_id FROM enrolments e JOIN students s ON s.student_id=e.student_id JOIN units u ON u.unit_id=e.unit_id LEFT JOIN results r ON r.enrolment_id=e.enrolment_id ORDER BY e.academic_year DESC,e.semester DESC,s.student_number`,
	}
	result := map[string]any{}
	for key, query := range queries {
		rows, err := getMany(a.store.DB, query)
		if err != nil {
			apiError(w, 500, "Unable to load form options.")
			return
		}
		result[key] = rows
	}
	writeJSON(w, 200, result)
}

func (a *App) deleteRecord(name string, id int64) error {
	if name != "results" {
		return deleteWithDependencies(a.store.DB, name, id)
	}
	tx, err := a.store.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var enrolmentID int64
	if err = tx.QueryRow(`SELECT enrolment_id FROM results WHERE result_id=?`, id).Scan(&enrolmentID); err != nil {
		return err
	}
	if _, err = tx.Exec(`DELETE FROM results WHERE result_id=?`, id); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE enrolments SET status='Enrolled',updated_at=CURRENT_TIMESTAMP WHERE enrolment_id=?`, enrolmentID); err != nil {
		return err
	}
	return tx.Commit()
}

// Seed records are fictional and created only with -demo on a new database.
func seedDemo(tx *sql.Tx) error {
	statements := []string{
		`INSERT INTO courses(course_code,course_name,duration_years,description) VALUES ('BIT','Bachelor of Information Technology',3,'School of Computing'),('BBA','Bachelor of Business Administration',3,'School of Business'),('BSE','Bachelor of Software Engineering',4,'School of Computing')`,
		`INSERT INTO units(unit_code,unit_name,credit_points,course_id) VALUES ('IT2101','Database Systems',15,1),('IT2102','Computer Networks',15,1),('IT3101','Information Security',20,1),('IT1102','Systems Analysis',15,1),('BUS1101','Business Foundations',15,2),('BUS2102','Financial Management',20,2),('BUS2103','Organisational Behaviour',15,2),('SE2101','Software Design',20,3),('SE2102','Data Structures',15,3),('SE3101','Software Quality Assurance',15,3)`,
		`INSERT INTO students(student_number,first_name,last_name,email,phone,date_of_birth,address,status,course_id) VALUES ('2026001','Nimal','Perera','nimal.perera@example.edu','0770001001','2003-04-12','Colombo','Active',1),('2026002','Amara','Silva','amara.silva@example.edu','0770001002','2003-08-22','Kandy','Active',1),('2026003','Sahan','Fernando','sahan.fernando@example.edu','0770001003','2002-11-05','Galle','Active',2),('2026004','Dinithi','Jayasekara','dinithi.j@example.edu','0770001004','2003-02-17','Colombo','Active',3),('2026005','Kavindu','Rathnayake','kavindu.r@example.edu','0770001005','2002-09-14','Kurunegala','Active',1),('2026006','Anjali','Wijesinghe','anjali.w@example.edu','0770001006','2003-06-09','Matara','Active',2),('2026007','Ravindu','Senanayake','ravindu.s@example.edu','0770001007','2002-12-21','Negombo','Active',3),('2026008','Hasini','Dissanayake','hasini.d@example.edu','0770001008','2003-01-30','Kandy','Inactive',1)`,
		`INSERT INTO enrolments(student_id,unit_id,semester,academic_year,enrolment_date,status) VALUES (1,1,1,2026,'2026-02-02','Completed'),(1,2,1,2026,'2026-02-02','Completed'),(1,3,2,2026,'2026-08-03','Enrolled'),(1,4,2,2026,'2026-08-03','Enrolled'),(2,1,1,2026,'2026-02-02','Completed'),(2,3,2,2026,'2026-08-03','Enrolled'),(3,5,1,2026,'2026-02-02','Completed'),(3,6,2,2026,'2026-08-03','Enrolled'),(4,8,1,2026,'2026-02-02','Completed'),(4,9,2,2026,'2026-08-03','Enrolled'),(5,1,1,2026,'2026-02-02','Completed'),(5,2,2,2026,'2026-08-03','Enrolled'),(6,5,1,2026,'2026-02-02','Completed'),(6,7,2,2026,'2026-08-03','Enrolled'),(7,8,1,2026,'2026-02-02','Completed'),(7,10,2,2026,'2026-08-03','Enrolled'),(8,1,1,2026,'2026-02-02','Completed')`,
		`INSERT INTO results(enrolment_id,mark,grade) VALUES (1,82,'HD'),(2,74,'D'),(5,91,'HD'),(7,68,'C'),(9,86,'HD'),(11,57,'P'),(13,76,'D'),(15,45,'F'),(17,64,'C')`,
	}
	for _, q := range statements {
		if _, err := tx.Exec(q); err != nil {
			return err
		}
	}
	rows, err := tx.Query(`SELECT student_id,student_number,first_name || ' ' || last_name,email FROM students ORDER BY student_id`)
	if err != nil {
		return err
	}
	type seedAccount struct {
		id                  int64
		number, name, email string
	}
	accounts := []seedAccount{}
	for rows.Next() {
		var u seedAccount
		if err = rows.Scan(&u.id, &u.number, &u.name, &u.email); err != nil {
			rows.Close()
			return err
		}
		accounts = append(accounts, u)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, u := range accounts {
		hash, err := hashPassword("Student@12345")
		if err != nil {
			return err
		}
		if _, err = tx.Exec(`INSERT INTO users(username,password_hash,full_name,email,role,student_id) VALUES (?,?,?,?,'Student',?)`, "s"+u.number, hash, u.name, u.email, u.id); err != nil {
			return err
		}
	}
	return nil
}
