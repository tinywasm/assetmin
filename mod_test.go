package assetmin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsAssetMinUsedInThisPkg(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gomod_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	m := NewGoMod()
	m.SetRootPath(tmpDir)

	t.Run("dependency present", func(t *testing.T) {
		content := `module test
go 1.23
require ` + PackageName + ` v0.0.1
`
		err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(content), 0644)
		if err != nil {
			t.Fatal(err)
		}

		m.SetRootPath(tmpDir)

		if !m.IsUsed() {
			t.Errorf("expected true, got false")
		}
	})

	t.Run("dependency absent", func(t *testing.T) {
		content := `module test
go 1.23
require github.com/some/otherpkg v1.0.0
`
		err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(content), 0644)
		if err != nil {
			t.Fatal(err)
		}

		m.SetRootPath(tmpDir)

		if m.IsUsed() {
			t.Errorf("expected false, got true")
		}
	})

	t.Run("no go.mod file", func(t *testing.T) {
		os.Remove(filepath.Join(tmpDir, "go.mod"))
		m.SetRootPath(tmpDir) // Refresh state
		if m.IsUsed() {
			t.Errorf("expected false, got true")
		}
	})
}
