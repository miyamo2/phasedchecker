package runner

import (
	"github.com/miyamo2/phasedchecker/internal/severity"
)

// ResolveSeverity finds the severity for a given category.
func ResolveSeverity(category string, policy severity.DiagnosticPolicy) severity.Severity {
	for _, rule := range policy.Rules {
		if rule.Category == category {
			return rule.Severity
		}
	}
	return policy.DefaultSeverity
}
