package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

// SetupTestModule creates a temporary Go module directory with the given files.
func SetupTestModule(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	gomod := "module example.com/test\n\ngo 1.25\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0644); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}
