package checkertest

import (
	"fmt"
	"go/token"
	"regexp"
	"strconv"
	"strings"
	"text/scanner"

	"github.com/miyamo2/phasedchecker/checkertest/internal"
	"golang.org/x/tools/go/packages"
)

// expectation represents a single expected diagnostic message pattern.
type expectation struct {
	rx *regexp.Regexp
}

// expectKey identifies a specific location in source code.
type expectKey struct {
	file string
	line int
}

// parseExpectations parses the text following "// want" into a line delta
// and a list of expectations. It uses text/scanner to tokenize the input.
//
// Supported forms:
//   - "regex"           — expect diagnostic matching regex
//   - "regex1" "regex2" — multiple expectations on same line
//   - +N "regex"        — expect diagnostic N lines ahead
//
// Fact expectations (name:"pattern") are rejected as errors since
// the checker does not use analysis facts.
func parseExpectations(text string) (int, []*expectation, error) {
	var s scanner.Scanner
	s.Init(strings.NewReader(text))
	s.Mode = scanner.ScanIdents | scanner.ScanStrings | scanner.ScanRawStrings | scanner.ScanInts
	s.Filename = "want"
	var scanErr string
	s.Error = func(_ *scanner.Scanner, msg string) {
		scanErr = msg
	}

	lineDelta := 0
	var expects []*expectation
	for {
		tok := s.Scan()
		switch tok {
		case scanner.EOF:
			if scanErr != "" {
				return 0, nil, fmt.Errorf("%s", scanErr)
			}
			return lineDelta, expects, nil

		case '+':
			tok = s.Scan()
			if tok != scanner.Int {
				return 0, nil, fmt.Errorf("expected line number after '+', got %s", scanner.TokenString(tok))
			}
			n, err := strconv.Atoi(s.TokenText())
			if err != nil {
				return 0, nil, fmt.Errorf("invalid line offset: %v", err)
			}
			lineDelta = n

		case scanner.String, scanner.RawString:
			pattern, err := strconv.Unquote(s.TokenText())
			if err != nil {
				return 0, nil, fmt.Errorf("invalid string literal: %v", err)
			}
			rx, err := regexp.Compile(pattern)
			if err != nil {
				return 0, nil, fmt.Errorf("invalid regex %q: %v", pattern, err)
			}
			expects = append(expects, &expectation{rx: rx})

		case scanner.Ident:
			// Fact expectation form (name:"pattern") — not supported
			return 0, nil, fmt.Errorf("fact expectations (name:\"pattern\") are not supported in checkertest")

		default:
			return 0, nil, fmt.Errorf("unexpected token %s", scanner.TokenString(tok))
		}
	}
}

// collectExpectations extracts all // want directives from packages' source files.
// It returns a map from (file, line) to the list of expected patterns at that location.
func collectExpectations(t internal.T, pkgs []*packages.Package) map[expectKey][]*expectation {
	t.Helper()
	wants := make(map[expectKey][]*expectation)

	packages.Visit(
		pkgs, func(pkg *packages.Package) bool {
			for _, f := range pkg.Syntax {
				tokFile := pkg.Fset.File(f.FileStart)
				filename := tokFile.Name()
				for _, cg := range f.Comments {
					for _, c := range cg.List {
						text := c.Text
						posn := pkg.Fset.Position(c.Pos())

						// Handle // comments
						if rest, ok := strings.CutPrefix(text, "//"); ok {
							text = rest
						} else if rest, ok := strings.CutPrefix(text, "/*"); ok {
							text = strings.TrimSuffix(rest, "*/")
						}

						// Support "//...// want" pattern (comment on comment).
						if idx := strings.LastIndex(text, "// want"); idx >= 0 {
							text = text[idx+len("// want"):]
						} else if rest, ok := strings.CutPrefix(strings.TrimSpace(text), "want"); ok {
							text = rest
						} else {
							continue
						}

						lineDelta, expects, err := parseExpectations(text)
						if err != nil {
							t.Errorf("%s:%d: in 'want' comment: %s", filename, posn.Line, err)
							continue
						}
						if len(expects) > 0 {
							k := expectKey{file: filename, line: posn.Line + lineDelta}
							wants[k] = append(wants[k], expects...)
						}
					}
				}
			}
			return true
		}, nil,
	)

	return wants
}

// checkDiagnostics matches diagnostics against expectations.
// It removes matched expectations from the map and reports unexpected diagnostics.
// Returns false if any unexpected diagnostic was found.
func checkDiagnostics(t internal.T, wants map[expectKey][]*expectation, posn token.Position, message string) {
	t.Helper()
	k := expectKey{file: posn.Filename, line: posn.Line}
	expects := wants[k]

	for i, exp := range expects {
		if exp.rx.MatchString(message) {
			// Remove matched expectation.
			expects[i] = expects[len(expects)-1]
			expects = expects[:len(expects)-1]
			if len(expects) == 0 {
				delete(wants, k)
			} else {
				wants[k] = expects
			}
			return
		}
	}

	// No match found — unexpected diagnostic.
	if len(expects) > 0 {
		var patterns []string
		for _, exp := range expects {
			patterns = append(patterns, fmt.Sprintf("%q", exp.rx.String()))
		}
		t.Errorf(
			"%s:%d: diagnostic %q does not match any of %s",
			posn.Filename, posn.Line, message, strings.Join(patterns, ", "),
		)
	} else {
		t.Errorf("%s:%d: unexpected diagnostic %q", posn.Filename, posn.Line, message)
	}
}

// reportUnmatched reports all remaining unmatched expectations as test errors.
func reportUnmatched(t internal.T, wants map[expectKey][]*expectation) {
	t.Helper()
	for k, expects := range wants {
		for _, exp := range expects {
			t.Errorf(
				"%s:%d: no diagnostic was reported matching %q",
				k.file, k.line, exp.rx.String(),
			)
		}
	}
}
