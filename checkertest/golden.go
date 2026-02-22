package checkertest

import (
	"go/types"
	"os"
	"sort"
	"strings"

	"github.com/miyamo2/phasedchecker/checkertest/internal"
	"golang.org/x/tools/go/analysis/checker"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/txtar"

	"github.com/miyamo2/phasedchecker/internal/x/tools/diff"
	"github.com/miyamo2/phasedchecker/internal/x/tools/driverutil"
)

// goldenFiles tracks original file content for golden comparison.
type goldenFiles struct {
	// originals maps filename to its original content (before fixes).
	originals map[string][]byte
}

func newGoldenFiles() *goldenFiles {
	return &goldenFiles{originals: make(map[string][]byte)}
}

// capture records the original content of all source files in the packages.
func (gf *goldenFiles) capture(pkgs []*packages.Package) {
	packages.Visit(
		pkgs, func(pkg *packages.Package) bool {
			for _, f := range pkg.Syntax {
				tokFile := pkg.Fset.File(f.FileStart)
				filename := tokFile.Name()
				if _, ok := gf.originals[filename]; !ok {
					content, err := os.ReadFile(filename)
					if err == nil {
						gf.originals[filename] = content
					}
				}
			}
			return true
		}, nil,
	)
}

// fileEditsInfo groups edits for a single file by fix message.
type fileEditsInfo struct {
	pkg   *types.Package
	byMsg map[string][]diff.Edit
}

// compareGolden applies edits to the original source, formats the result,
// and compares it against a .golden file.
//
// Golden files can be in two forms:
//  1. Plain file (file.go.golden): the golden file is the expected output
//     after applying ALL fixes.
//  2. txtar archive: each section is named by the fix message. Each section
//     contains the expected output after applying only fixes with that message.
func compareGolden(t internal.T, graphs []*checker.Graph, gf *goldenFiles) {
	t.Helper()

	// Collect all edits grouped by (filename, fix message).
	type fixEdit struct {
		message string
		file    string
		pkg     *types.Package
		edit    diff.Edit
	}
	var allFixEdits []fixEdit

	for _, graph := range graphs {
		for act := range graph.All() {
			if !act.IsRoot || len(act.Diagnostics) == 0 {
				continue
			}
			for _, d := range act.Diagnostics {
				for i := range d.SuggestedFixes {
					fix := &d.SuggestedFixes[i]
					if i > 0 {
						continue // only apply the first fix per diagnostic
					}
					for _, edit := range fix.TextEdits {
						file := act.Package.Fset.File(edit.Pos)
						allFixEdits = append(
							allFixEdits, fixEdit{
								message: fix.Message,
								file:    file.Name(),
								pkg:     act.Package.Types,
								edit: diff.Edit{
									Start: file.Offset(edit.Pos),
									End:   file.Offset(edit.End),
									New:   string(edit.NewText),
								},
							},
						)
					}
				}
			}
		}
	}

	// Group edits by file, then by fix message.
	fileEdits := make(map[string]*fileEditsInfo)
	for _, fe := range allFixEdits {
		ffe, ok := fileEdits[fe.file]
		if !ok {
			ffe = &fileEditsInfo{
				pkg:   fe.pkg,
				byMsg: make(map[string][]diff.Edit),
			}
			fileEdits[fe.file] = ffe
		}
		ffe.byMsg[fe.message] = append(ffe.byMsg[fe.message], fe.edit)
	}

	// For each file that has edits, compare against golden.
	var files []string
	for f := range fileEdits {
		files = append(files, f)
	}
	sort.Strings(files)

	for _, filename := range files {
		ffe := fileEdits[filename]
		original, ok := gf.originals[filename]
		if !ok {
			t.Errorf("no original content captured for %s", filename)
			continue
		}

		goldenPath := filename + ".golden"
		goldenBytes, err := os.ReadFile(goldenPath)
		if err != nil {
			t.Errorf("reading golden file %s: %v", goldenPath, err)
			continue
		}

		ar := txtar.Parse(goldenBytes)

		if len(ar.Files) > 0 {
			// txtar archive form: each section is a fix message.
			compareArchiveGolden(t, filename, original, ffe, ar)
		} else {
			// Plain form: ar.Comment is the full expected output.
			compareAllFixesGolden(t, filename, original, ffe, ar.Comment)
		}
	}
}

// compareAllFixesGolden applies all fixes to the original and compares against expected.
func compareAllFixesGolden(t internal.T, filename string, original []byte, ffe *fileEditsInfo, expected []byte) {
	t.Helper()

	// Merge all edits.
	var accum []diff.Edit
	msgs := sortedKeys(ffe.byMsg)
	for _, msg := range msgs {
		edits := ffe.byMsg[msg]
		if len(accum) == 0 {
			accum = edits
		} else {
			merged, ok := diff.Merge(accum, edits)
			if !ok {
				t.Errorf("%s: conflicting edits when merging fix %q", filename, msg)
				return
			}
			accum = merged
		}
	}

	got, err := diff.ApplyBytes(original, accum)
	if err != nil {
		t.Errorf("%s: applying edits: %v", filename, err)
		return
	}

	formatted, err := driverutil.FormatSourceRemoveImports(ffe.pkg, got)
	if err == nil {
		got = formatted
	}

	formattedExpected, err := driverutil.FormatSourceRemoveImports(ffe.pkg, expected)
	if err == nil {
		expected = formattedExpected
	}

	if string(got) != string(expected) {
		unified := diff.Unified(filename+".golden", filename+" (actual)", string(expected), string(got))
		t.Errorf("%s: golden file mismatch (-want +got):\n%s", filename, unified)
	}
}

// compareArchiveGolden handles the txtar archive golden format.
func compareArchiveGolden(t internal.T, filename string, original []byte, ffe *fileEditsInfo, ar *txtar.Archive) {
	t.Helper()

	for _, section := range ar.Files {
		msg := strings.TrimSpace(section.Name)
		edits, ok := ffe.byMsg[msg]
		if !ok {
			t.Errorf("%s: golden section %q has no matching fix", filename, msg)
			continue
		}

		got, err := diff.ApplyBytes(original, edits)
		if err != nil {
			t.Errorf("%s: applying edits for fix %q: %v", filename, msg, err)
			continue
		}

		formatted, err := driverutil.FormatSourceRemoveImports(ffe.pkg, got)
		if err == nil {
			got = formatted
		}

		expected := section.Data
		formattedExpected, err := driverutil.FormatSourceRemoveImports(ffe.pkg, expected)
		if err == nil {
			expected = formattedExpected
		}

		if string(got) != string(expected) {
			unified := diff.Unified(filename+".golden ("+msg+")", filename+" (actual)", string(expected), string(got))
			t.Errorf("%s: golden section %q mismatch (-want +got):\n%s", filename, msg, unified)
		}
	}
}

// sortedKeys returns the sorted keys of a map.
func sortedKeys(m map[string][]diff.Edit) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
