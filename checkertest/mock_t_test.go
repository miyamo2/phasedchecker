package checkertest

import (
	"fmt"
	"runtime"

	"github.com/miyamo2/phasedchecker/checkertest/internal"
)

// mockT implements internal.T for unit testing helpers without a real *testing.T.
type mockT struct {
	internal.T
	errors []string
	fatals []string
}

func (m *mockT) Helper() {}

func (m *mockT) Errorf(format string, args ...any) {
	m.errors = append(m.errors, fmt.Sprintf(format, args...))
}

func (m *mockT) Fatal(args ...any) {
	m.fatals = append(m.fatals, fmt.Sprint(args...))
	runtime.Goexit()
}

func (m *mockT) Fatalf(format string, args ...any) {
	m.fatals = append(m.fatals, fmt.Sprintf(format, args...))
	runtime.Goexit()
}
