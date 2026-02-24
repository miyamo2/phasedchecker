package runner

import (
	"testing"

	"github.com/miyamo2/phasedchecker/internal/severity"
)

func Test_ResolveSeverity(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		policy   severity.DiagnosticPolicy
		category string
		want     severity.Severity
	}{
		{
			name:     "no rules returns DefaultSeverity",
			policy:   severity.DiagnosticPolicy{DefaultSeverity: severity.SeverityInfo},
			category: "anything",
			want:     severity.SeverityInfo,
		},
		{
			name: "unmatched category returns DefaultSeverity",
			policy: severity.DiagnosticPolicy{
				Rules: []severity.CategoryRule{
					{
						Category: "other", Severity: severity.SeverityError,
					},
				},
				DefaultSeverity: severity.SeverityWarn,
			},
			category: "nomatch",
			want:     severity.SeverityWarn,
		},
		{
			name: "exact match returns Error",
			policy: severity.DiagnosticPolicy{
				Rules: []severity.CategoryRule{{Category: "err", Severity: severity.SeverityError}},
			},
			category: "err",
			want:     severity.SeverityError,
		},
		{
			name: "exact match returns Warn",
			policy: severity.DiagnosticPolicy{
				Rules: []severity.CategoryRule{{Category: "warn", Severity: severity.SeverityWarn}},
			},
			category: "warn",
			want:     severity.SeverityWarn,
		},
		{
			name: "exact match returns Info",
			policy: severity.DiagnosticPolicy{
				Rules:           []severity.CategoryRule{{Category: "info", Severity: severity.SeverityInfo}},
				DefaultSeverity: severity.SeverityError,
			},
			category: "info",
			want:     severity.SeverityInfo,
		},
		{
			name: "first matching rule wins",
			policy: severity.DiagnosticPolicy{
				Rules: []severity.CategoryRule{
					{Category: "cat", Severity: severity.SeverityWarn},
					{Category: "cat", Severity: severity.SeverityError},
				},
			},
			category: "cat",
			want:     severity.SeverityWarn,
		},
		{
			name: "empty category matches",
			policy: severity.DiagnosticPolicy{
				Rules:           []severity.CategoryRule{{Category: "", Severity: severity.SeverityError}},
				DefaultSeverity: severity.SeverityInfo,
			},
			category: "",
			want:     severity.SeverityError,
		},
		{
			name:     "zero value Policy returns SeverityInfo",
			policy:   severity.DiagnosticPolicy{},
			category: "anything",
			want:     severity.SeverityInfo,
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				t.Parallel()
				got := ResolveSeverity(tt.category, tt.policy)
				if got != tt.want {
					t.Errorf("resolveSeverity(%q) = %d, want %d", tt.category, got, tt.want)
				}
			},
		)
	}
}
