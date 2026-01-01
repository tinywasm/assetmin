package assetmin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGoModWatcher(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gomod_watcher_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	config := &Config{
		OutputDir: filepath.Join(tmpDir, "dist"),
		GetRuntimeInitializerJS: func() (string, error) {
			return "console.log('init')", nil
		},
	}
	am := NewAssetMin(config)
	am.goModHandler.SetRootPath(tmpDir)

	goModPath := filepath.Join(tmpDir, "go.mod")

	t.Run("initial state is false", func(t *testing.T) {
		if am.goModHandler.IsUsed() {
			t.Errorf("expected false, got true")
		}
	})

	t.Run("add assetmin dependency", func(t *testing.T) {
		content := `module test
go 1.23
require ` + PackageName + ` v0.0.1
`
		err := os.WriteFile(goModPath, []byte(content), 0644)
		if err != nil {
			t.Fatal(err)
		}

		err = am.NewFileEvent("go.mod", ".mod", goModPath, "write")
		if err != nil {
			t.Fatal(err)
		}

		if !am.goModHandler.IsUsed() {
			t.Errorf("expected true, got false")
		}
	})

	t.Run("remove assetmin dependency", func(t *testing.T) {
		content := `module test
go 1.23
`
		err := os.WriteFile(goModPath, []byte(content), 0644)
		if err != nil {
			t.Fatal(err)
		}

		err = am.NewFileEvent("go.mod", ".mod", goModPath, "write")
		if err != nil {
			t.Fatal(err)
		}

		if am.goModHandler.IsUsed() {
			t.Errorf("expected false, got true")
		}
	})

	t.Run("ignore go.mod in other directories", func(t *testing.T) {
		otherDir := filepath.Join(tmpDir, "other")
		os.MkdirAll(otherDir, 0755)
		otherGoModPath := filepath.Join(otherDir, "go.mod")

		content := `module other
go 1.23
require ` + PackageName + ` v0.0.1
`
		err := os.WriteFile(otherGoModPath, []byte(content), 0644)
		if err != nil {
			t.Fatal(err)
		}

		// Current state should be false from previous test
		err = am.NewFileEvent("go.mod", ".mod", otherGoModPath, "write")
		if err != nil {
			t.Fatal(err)
		}

		if am.goModHandler.IsUsed() {
			t.Errorf("expected false, got true (should have ignored other go.mod)")
		}
	})
}
