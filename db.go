package main

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// Store serializes validated writes so relationship checks and saves cannot race.
type Store struct {
	DB      *sql.DB
	writeMu sync.Mutex
}

func (s *Store) Close() error { return s.DB.Close() }

func openStore(file string) (*Store, error) {
	db, err := sql.Open("sqlite", file)
	if err != nil {
		return nil, err
	}
	// One connection avoids SQLite foreign-key pragma inconsistency and write-lock contention.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{DB: db}, nil
}
func (s *Store) initialize(demo bool) (string, error) {
	sqlText, err := bundled.ReadFile("schema.sql")
	if err != nil {
		return "", err
	}
	for _, statement := range strings.Split(string(sqlText), ";") {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}
		if _, err = s.DB.Exec(statement); err != nil {
			return "", fmt.Errorf("schema initialization failed: %w", err)
		}
	}
	var count int
	if err = s.DB.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return "", err
	}
	if count != 0 {
		return "", nil
	}
	adminPass := ""
	if demo {
		adminPass = "Admin@12345"
	} else {
		b := make([]byte, 18)
		if _, err = rand.Read(b); err != nil {
			return "", err
		}
		adminPass = fmt.Sprintf("Admin-%x", b)
	}
	h, err := hashPassword(adminPass)
	if err != nil {
		return "", err
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`INSERT INTO users(username,password_hash,full_name,email,role) VALUES(?,?,?,?,?)`, "admin", h, "System Administrator", "admin@example.local", "Admin"); err != nil {
		return "", err
	}
	if demo {
		if err = seedDemo(tx); err != nil {
			return "", err
		}
	}

	if err = tx.Commit(); err != nil {
		return "", err
	}
	if demo {
		return "", nil
	}
	return adminPass, nil
}

func rowsToMaps(rows *sql.Rows) ([]map[string]any, error) {
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0)
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err = rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		entry := make(map[string]any, len(cols))
		for i, c := range cols {
			v := vals[i]
			if b, ok := v.([]byte); ok {
				v = string(b)
			}
			entry[c] = v
		}
		out = append(out, entry)
	}
	return out, rows.Err()
}
func getMany(db *sql.DB, query string, args ...any) ([]map[string]any, error) {
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	return rowsToMaps(rows)
}
func getOne(db *sql.DB, query string, args ...any) (map[string]any, error) {
	rs, err := getMany(db, query, args...)
	if err != nil {
		return nil, err
	}
	if len(rs) == 0 {
		return nil, sql.ErrNoRows
	}
	return rs[0], nil
}
func gradeForMark(m float64) string {
	switch {
	case m >= 80:
		return "HD"
	case m >= 70:
		return "D"
	case m >= 60:
		return "C"
	case m >= 50:
		return "P"
	default:
		return "F"
	}
}
func requireDatabaseConstraint(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "unique constraint"):
		return "A record already exists with one of these unique values (code, username, email, or enrolment)."
	case strings.Contains(msg, "foreign key constraint"):
		return "This record is referenced by another record, or a selected item no longer exists. Remove related records first."
	case strings.Contains(msg, "check constraint"):
		return "A value is outside its allowed range."
	default:
		return "Database operation failed. Please check the form and try again."
	}
}
func computeWAM(db *sql.DB, studentID int64) (float64, int, error) {
	var weighted sql.NullFloat64
	var credits sql.NullFloat64
	var n int
	err := db.QueryRow(`SELECT SUM(r.mark*u.credit_points), SUM(u.credit_points), COUNT(*) FROM results r JOIN enrolments e ON e.enrolment_id=r.enrolment_id JOIN units u ON u.unit_id=e.unit_id WHERE e.student_id=? AND e.status='Completed'`, studentID).Scan(&weighted, &credits, &n)
	if err != nil {
		return 0, 0, err
	}
	if !credits.Valid || credits.Float64 <= 0 {
		return 0, 0, nil
	}
	return math.Round(weighted.Float64/credits.Float64*100) / 100, n, nil
}
func parseDate(s string) bool {
	if s == "" {
		return true
	}
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}
func exists(db *sql.DB, table, idField string, id int64) bool {
	// Only call with constants, never user-controlled table/column identifiers.
	var n int
	return db.QueryRow("SELECT COUNT(*) FROM "+table+" WHERE "+idField+"=?", id).Scan(&n) == nil && n > 0
}

var errBadRelationship = errors.New("the chosen student and unit must belong to the same course")

func validateEnrolment(db *sql.DB, studentID, unitID int64) error {
	var a, b int64
	var active string
	err := db.QueryRow(`SELECT course_id,status FROM students WHERE student_id=?`, studentID).Scan(&a, &active)
	if err != nil {
		return errors.New("student does not exist")
	}
	if active != "Active" {
		return errors.New("cannot enrol an inactive student")
	}
	if err = db.QueryRow(`SELECT course_id FROM units WHERE unit_id=?`, unitID).Scan(&b); err != nil {
		return errors.New("unit does not exist")
	}
	if a != b {
		return errBadRelationship
	}
	return nil
}
func deleteWithDependencies(db *sql.DB, resource string, id int64) error {
	// Enrolment results are automatically removed by FK CASCADE. Other FKs use RESTRICT.
	_, err := db.Exec("DELETE FROM "+resource+" WHERE "+primaryKey(resource)+"=?", id)
	return err
}
func primaryKey(r string) string {
	switch r {
	case "users":
		return "user_id"
	case "students":
		return "student_id"
	case "courses":
		return "course_id"
	case "units":
		return "unit_id"
	case "enrolments":
		return "enrolment_id"
	case "results":
		return "result_id"
	default:
		return ""
	}
}
func checkDBWritable(file string) error {
	f, err := os.Stat(file)
	if err != nil {
		return err
	}
	if f.IsDir() {
		return errors.New("database path is a directory")
	}
	return nil
}
