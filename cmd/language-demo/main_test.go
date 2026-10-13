package main

import "testing"

func TestLanguageDemonstrations(t *testing.T) {
	results := concurrentMarks()
	if len(results) != 3 || results[0].student != "A" || results[2].mark != 0 {
		t.Fatal(results)
	}
	if !nilPointerPanics() {
		t.Fatal("expected controlled nil panic")
	}
	if _, err := checkedMark(nil); err == nil {
		t.Fatal("missing mark accepted")
	}
	zero := 0.0
	if value, err := checkedMark(&zero); err != nil || value != 0 {
		t.Fatal(value, err)
	}
}
