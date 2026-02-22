package severity

// Severity represents the severity level of a diagnostic.
type Severity int

const (
	SeverityDebug Severity = iota - 1
	SeverityInfo
	SeverityWarn
	SeverityError
	SeverityCritical
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
