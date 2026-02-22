package severity

import (
	"testing"
)

func TestSeverityConstants(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		got  Severity
		want int
	}{
		{"SeverityInfo", SeverityInfo, 0},
		{"SeverityWarn", SeverityWarn, 1},
		{"SeverityError", SeverityError, 2},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				if int(tt.got) != tt.want {
					t.Errorf("%s = %d, want %d", tt.name, tt.got, tt.want)
				}
			},
		)
	}
}
