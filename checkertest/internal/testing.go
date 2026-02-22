// Package internal provides shared interfaces for the checkertest package.
package internal

// T is a minimal subset of [testing.T] used by checkertest internals.
// It enables unit testing of the test framework itself without depending
// on a real [testing.T].
type T interface {
	Errorf(format string, args ...any)
	Fatal(args ...any)
	Fatalf(format string, args ...any)
	Helper()
}
