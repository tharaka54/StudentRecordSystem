package main

import (
	"encoding/hex"
	"testing"
)

func TestPBKDF2KnownVector(t *testing.T) {
	got := hex.EncodeToString(pbkdf2SHA256([]byte("password"), []byte("salt"), 2, 32))
	const want = "ae4d0c95af6b46d32d0adff928f06dd02a303f8ef3c251dfd6e2d85a95474c43"
	if got != want {
		t.Fatalf("PBKDF2-SHA256 vector: got %s, want %s", got, want)
	}
}

func TestPasswordHashing(t *testing.T) {
	h, e := hashPassword("StrongPassword!1")
	if e != nil {
		t.Fatal(e)
	}
	if h == "StrongPassword!1" {
		t.Fatal("password stored in plaintext")
	}
	if !verifyPassword("StrongPassword!1", h) {
		t.Fatal("valid password rejected")
	}
	if verifyPassword("wrong", h) {
		t.Fatal("wrong password accepted")
	}
}
func TestMarkGrades(t *testing.T) {
	tests := []struct {
		mark  float64
		grade string
	}{{85, "HD"}, {80, "HD"}, {79.99, "D"}, {70, "D"}, {69.99, "C"}, {60, "C"}, {50, "P"}, {49.99, "F"}, {0, "F"}}
	for _, tc := range tests {
		if got := gradeForMark(tc.mark); got != tc.grade {
			t.Errorf("%.2f: got %s expected %s", tc.mark, got, tc.grade)
		}
	}
}
