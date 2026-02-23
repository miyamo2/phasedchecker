package phasedchecker

import (
	"strings"
	"testing"
)

func Test_parseArgs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		args      []string
		wantFix   bool
		wantDiff  bool
		wantDebug string
		wantPats  []string
		wantErr   string
	}{
		{
			name:     "basic pattern",
			args:     []string{"./..."},
			wantPats: []string{"./..."},
		},
		{
			name:     "fix flag",
			args:     []string{"-fix", "./..."},
			wantFix:  true,
			wantPats: []string{"./..."},
		},
		{
			name:     "diff flag",
			args:     []string{"-diff", "./..."},
			wantDiff: true,
			wantPats: []string{"./..."},
		},
		{
			name:     "all flags combined",
			args:     []string{"-fix", "-diff", "./..."},
			wantFix:  true,
			wantDiff: true,
			wantPats: []string{"./..."},
		},
		{
			name:      "debug all flags",
			args:      []string{"-debug=vpstf", "./..."},
			wantDebug: "vpstf",
			wantPats:  []string{"./..."},
		},
		{
			name:      "debug subset",
			args:      []string{"-debug=vt", "./..."},
			wantDebug: "vt",
			wantPats:  []string{"./..."},
		},
		{
			name:     "no debug flag",
			args:     []string{"./..."},
			wantPats: []string{"./..."},
		},
		{
			name:     "multiple patterns",
			args:     []string{"pkg1", "pkg2"},
			wantPats: []string{"pkg1", "pkg2"},
		},
		{
			name:    "no packages",
			args:    []string{},
			wantErr: "no packages specified",
		},
		{
			name:    "unknown flag",
			args:    []string{"-unknown", "./..."},
			wantErr: "flag provided but not defined",
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				t.Parallel()
				args, err := parseArgs("test", tt.args)
				if tt.wantErr != "" {
					if err == nil {
						t.Fatalf("expected error containing %q, got nil", tt.wantErr)
					}
					if !strings.Contains(err.Error(), tt.wantErr) {
						t.Fatalf("error = %q, want containing %q", err, tt.wantErr)
					}
					return
				}
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				if args.Fix != tt.wantFix {
					t.Errorf("Fix = %v, want %v", args.Fix, tt.wantFix)
				}
				if args.PrintDiff != tt.wantDiff {
					t.Errorf("PrintDiff = %v, want %v", args.PrintDiff, tt.wantDiff)
				}
				if args.Debug != tt.wantDebug {
					t.Errorf("Debug = %q, want %q", args.Debug, tt.wantDebug)
				}
				if len(args.Patterns) != len(tt.wantPats) {
					t.Fatalf("Patterns = %v, want %v", args.Patterns, tt.wantPats)
				}
				for i, p := range args.Patterns {
					if p != tt.wantPats[i] {
						t.Errorf("Patterns[%d] = %q, want %q", i, p, tt.wantPats[i])
					}
				}
			},
		)
	}
}
