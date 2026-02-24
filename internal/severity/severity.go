package severity

// Severity represents the severity level of a diagnostic.
type Severity int

const (
	// _ reserves the iota value -1 for a potential debug severity level.
	_ Severity = iota - 1
	// SeverityInfo indicates an informational diagnostic.
	// Info diagnostics do not affect the exit code and are not reported.
	SeverityInfo
	// _ reserves the iota value 1 for a potential notice severity level.
	_
	// SeverityWarn indicates a warning diagnostic.
	// Warn diagnostics are reported to stderr.
	// If no Error or Critical diagnostics are present and fix mode is disabled,
	// the process exits with code 3.
	SeverityWarn
	// SeverityError indicates an error diagnostic.
	// Error diagnostics are reported to stderr.
	// The remaining phases in the pipeline continue to execute, but
	// the process exits with code 1 after all phases complete.
	SeverityError
	// SeverityCritical indicates a critical diagnostic.
	// Critical diagnostics abort the pipeline during the current phase,
	// skipping all subsequent phases and exiting with code 1.
	SeverityCritical
	// _ reserves the iota value 4 for a potential fatal severity level.
	_
	// _ reserves the iota value 5 for a potential emergency severity level.
	_
)

// CategoryRule maps a diagnostic category string to a severity level.
// The severity determines both the output destination and exit code contribution.
type CategoryRule struct {
	// Category is the diagnostic category string to match.
	Category string
	// Severity is the severity level for this category.
	Severity Severity
}

// DiagnosticPolicy defines the complete mapping from diagnostic categories to severity levels.
type DiagnosticPolicy struct {
	// Rules is an ordered list of category-to-severity mappings.
	// The first matching rule wins.
	Rules []CategoryRule
	// DefaultSeverity is applied when no rule matches a diagnostic's category.
	DefaultSeverity Severity
}
