package runner

import (
	"github.com/miyamo2/phasedchecker/internal/severity"
)

// ResolveSeverity finds the severity for a given category.
// If the resolved severity is [severity.SeverityDefault], it is treated as [severity.SeverityWarn].
func ResolveSeverity(category string, policy severity.DiagnosticPolicy) severity.Severity {
	sv := policy.DefaultSeverity
	for _, rule := range policy.Rules {
		if rule.Category == category {
			sv = rule.Severity
			break
		}
	}
	if sv == severity.SeverityDefault {
		return severity.SeverityWarn
	}
	return sv
}
