// language-demo is a controlled supplement to the real application.
// It demonstrates goroutines/channels and two language trade-offs safely.
package main

import (
	"errors"
	"fmt"
	"sort"
)

type result struct {
	student string
	mark    float64
}

// Each worker owns its local value; a channel transports completed results.
func concurrentMarks() []result {
	done := make(chan result, 3)
	for _, item := range []result{{"A", 80}, {"B", 50}, {"C", 0}} {
		go func(value result) { done <- value }(item)
	}
	results := make([]result, 0, 3)
	for i := 0; i < 3; i++ {
		results = append(results, <-done)
	}
	// Completion order is unspecified. Sorting makes classroom output reproducible.
	sort.Slice(results, func(i, j int) bool { return results[i].student < results[j].student })
	return results
}

func checkedMark(mark *float64) (float64, error) {
	if mark == nil {
		return 0, errors.New("mark is absent")
	}
	if *mark < 0 || *mark > 100 {
		return 0, errors.New("mark outside 0-100")
	}
	return *mark, nil
}

// Deliberately unsafe example is isolated from the application and recovered.
// recover handles this demonstration panic, not ordinary validation errors.
func nilPointerPanics() (panicked bool) {
	defer func() {
		if recover() != nil {
			panicked = true
		}
	}()
	var mark *float64
	_ = *mark
	return false
}

func main() {
	for _, value := range concurrentMarks() {
		fmt.Printf("goroutine result: %s = %.0f\n", value.student, value.mark)
	}
	fmt.Printf("unsafe nil dereference panicked: %t\n", nilPointerPanics())
	if _, err := checkedMark(nil); err != nil {
		fmt.Println("explicit error branch:", err)
	}
	zero := 0.0
	value, err := checkedMark(&zero)
	if err != nil {
		fmt.Println("explicit error branch:", err)
		return
	}
	fmt.Printf("present zero mark: %.0f\n", value)
}
