package main

import (
	"context"
	"database/sql"
	"fmt"
	"math"
)

// academicReader is satisfied structurally: Store needs no implements keyword.
// Handlers can use the same contract with a test double instead of SQLite.
type academicReader interface {
	AcademicSummary(context.Context, int64) (AcademicSummary, error)
}

var _ academicReader = (*Store)(nil)

// StudentProfile models the personal record; its JSON names preserve the API.
type StudentProfile struct {
	ID                int64  `json:"student_id"`
	Number            string `json:"student_number"`
	FirstName         string `json:"first_name"`
	LastName          string `json:"last_name"`
	Email             string `json:"email"`
	Phone             string `json:"phone"`
	DateOfBirth       string `json:"date_of_birth"`
	Address           string `json:"address"`
	Status            string `json:"status"`
	CourseID          int64  `json:"course_id"`
	CreatedAt         string `json:"created_at"`
	UpdatedAt         string `json:"updated_at"`
	CourseCode        string `json:"course_code"`
	CourseName        string `json:"course_name"`
	DurationYears     int    `json:"duration_years"`
	CourseDescription string `json:"course_description"`
}

// Pointers distinguish an absent result from a recorded mark of zero.
type AcademicUnit struct {
	EnrolmentID   int64    `json:"enrolment_id"`
	UnitID        int64    `json:"unit_id"`
	Semester      int      `json:"semester"`
	AcademicYear  int      `json:"academic_year"`
	EnrolmentDate string   `json:"enrolment_date"`
	Status        string   `json:"status"`
	UnitCode      string   `json:"unit_code"`
	UnitName      string   `json:"unit_name"`
	CreditPoints  int      `json:"credit_points"`
	ResultID      *int64   `json:"result_id"`
	Mark          *float64 `json:"mark"`
	Grade         *string  `json:"grade"`
}

type AcademicSummary struct {
	Student       StudentProfile `json:"student"`
	Units         []AcademicUnit `json:"units"`
	WAM           *float64       `json:"wam"`
	GradedUnits   int            `json:"graded_units"`
	EarnedCredits int            `json:"earned_credits"`
	EnrolledUnits int            `json:"enrolled_units"`
}

// AcademicSummary reads profile and units in one transaction so all fields
// describe the same SQLite snapshot, even while another request saves a result.
// Context-aware queries stop work when the HTTP request is cancelled.
func (s *Store) AcademicSummary(ctx context.Context, studentID int64) (AcademicSummary, error) {
	out := AcademicSummary{Units: make([]AcademicUnit, 0)}
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return out, fmt.Errorf("begin academic summary: %w", err)
	}
	defer tx.Rollback()
	p := &out.Student
	err = tx.QueryRowContext(ctx, `SELECT s.student_id,s.student_number,s.first_name,
		s.last_name,s.email,s.phone,s.date_of_birth,s.address,s.status,s.course_id,
		s.created_at,s.updated_at,c.course_code,c.course_name,c.duration_years,c.description
		FROM students s JOIN courses c ON c.course_id=s.course_id WHERE s.student_id=?`, studentID).
		Scan(&p.ID, &p.Number, &p.FirstName, &p.LastName, &p.Email, &p.Phone,
			&p.DateOfBirth, &p.Address, &p.Status, &p.CourseID, &p.CreatedAt,
			&p.UpdatedAt, &p.CourseCode, &p.CourseName, &p.DurationYears, &p.CourseDescription)
	if err != nil {
		return out, fmt.Errorf("read student profile: %w", err)
	}
	rows, err := tx.QueryContext(ctx, `SELECT e.enrolment_id,e.unit_id,e.semester,
		e.academic_year,e.enrolment_date,e.status,u.unit_code,u.unit_name,
		u.credit_points,r.result_id,r.mark,r.grade FROM enrolments e
		JOIN units u ON u.unit_id=e.unit_id LEFT JOIN results r ON r.enrolment_id=e.enrolment_id
		WHERE e.student_id=? ORDER BY e.academic_year DESC,e.semester DESC,u.unit_code`, studentID)
	if err != nil {
		return out, fmt.Errorf("read academic units: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var u AcademicUnit
		if err = rows.Scan(&u.EnrolmentID, &u.UnitID, &u.Semester, &u.AcademicYear,
			&u.EnrolmentDate, &u.Status, &u.UnitCode, &u.UnitName, &u.CreditPoints,
			&u.ResultID, &u.Mark, &u.Grade); err != nil {
			return out, fmt.Errorf("scan academic unit: %w", err)
		}
		out.Units = append(out.Units, u)
	}
	if err = rows.Err(); err != nil {
		return out, fmt.Errorf("iterate academic units: %w", err)
	}
	if err = rows.Close(); err != nil {
		return out, fmt.Errorf("close academic units: %w", err)
	}
	out.calculateTotals()
	if err = tx.Commit(); err != nil {
		return out, fmt.Errorf("finish academic summary: %w", err)
	}
	return out, nil
}

// calculateTotals derives statistics from the exact units returned to the browser.
// Failed and zero marks count in WAM; only passed units earn credits.
func (s *AcademicSummary) calculateTotals() {
	s.WAM, s.GradedUnits, s.EarnedCredits, s.EnrolledUnits = nil, 0, 0, 0
	var weighted float64
	var credits int
	for _, u := range s.Units {
		if u.Status == "Enrolled" {
			s.EnrolledUnits++
		}
		if u.Status != "Completed" || u.Mark == nil {
			continue
		}
		s.GradedUnits++
		weighted += *u.Mark * float64(u.CreditPoints)
		credits += u.CreditPoints
		if *u.Mark >= 50 {
			s.EarnedCredits += u.CreditPoints
		}
	}
	if credits > 0 {
		value := math.Round(weighted/float64(credits)*100) / 100
		s.WAM = &value
	}
}
