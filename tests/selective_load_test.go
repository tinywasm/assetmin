package assetmin_test

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/tinywasm/assetmin"
)

func TestImportScanner(t *testing.T) {
	tmpDir := t.TempDir()

	mainGo := filepath.Join(tmpDir, "main.go")
	mainContent := `package main
import (
	"fmt"
	"github.com/user/pkg1"
	_ "github.com/user/blank"
)
func main() { fmt.Println("hello") }
`
	os.WriteFile(mainGo, []byte(mainContent), 0644)

	handlersDir := filepath.Join(tmpDir, "handlers")
	os.Mkdir(handlersDir, 0755)
	routesGo := filepath.Join(handlersDir, "routes.go")
	routesContent := `package handlers
import "github.com/user/pkg2"
`
	os.WriteFile(routesGo, []byte(routesContent), 0644)

	deepDir := filepath.Join(tmpDir, "utils", "deep")
	os.MkdirAll(deepDir, 0755)
	deepGo := filepath.Join(deepDir, "unused.go")
	deepContent := `package deep
import "github.com/user/deep"
`
	os.WriteFile(deepGo, []byte(deepContent), 0644)

	testGo := filepath.Join(tmpDir, "main_test.go")
	testContent := `package main_test
import "github.com/user/testonly"
`
	os.WriteFile(testGo, []byte(testContent), 0644)

	am := assetmin.NewAssetMin(&assetmin.Config{RootDir: tmpDir})

	imports, err := am.TestOnly_ScanImports()
	if err != nil {
		t.Fatalf("ScanProjectImports failed: %v", err)
	}

	expected := []string{"fmt", "github.com/user/pkg1", "github.com/user/blank", "github.com/user/pkg2"}
	for _, exp := range expected {
		if !imports[exp] {
			t.Errorf("expected import %s not found", exp)
		}
	}

	if imports["github.com/user/deep"] {
		t.Error("import from deep subdirectory should have been excluded")
	}

	if imports["github.com/user/testonly"] {
		t.Error("import from _test.go should have been excluded")
	}
}

func TestImportScanner_Caching(t *testing.T) {
	tmpDir := t.TempDir()
	mainGo := filepath.Join(tmpDir, "main.go")
	os.WriteFile(mainGo, []byte(`package main; import "pkg1"`), 0644)

	am := assetmin.NewAssetMin(&assetmin.Config{RootDir: tmpDir})

	imports1, _ := am.TestOnly_ScanImports()
	if !imports1["pkg1"] { t.Fatal("pkg1 missing") }

	// Update file
	time.Sleep(10 * time.Millisecond) // Ensure mtime changes
	os.WriteFile(mainGo, []byte(`package main; import "pkg2"`), 0644)

	imports2, _ := am.TestOnly_ScanImports()
	if imports2["pkg1"] { t.Error("pkg1 should be gone") }
	if !imports2["pkg2"] { t.Error("pkg2 missing") }
}

func TestModuleSubpackagesUsed(t *testing.T) {
	am := assetmin.NewAssetMin(&assetmin.Config{})

	tests := []struct {
		name          string
		modulePath    string
		importedPaths map[string]bool
		want          []string
	}{
		{
			name:       "root import",
			modulePath: "github.com/user/comp",
			importedPaths: map[string]bool{
				"github.com/user/comp": true,
			},
			want: []string{""},
		},
		{
			name:       "subpackage match",
			modulePath: "github.com/user/comp",
			importedPaths: map[string]bool{
				"github.com/user/comp/button": true,
				"github.com/user/comp/modal":  true,
				"other/pkg":                   true,
			},
			want: []string{"button", "modal"},
		},
		{
			name:       "deep subpackage skipped",
			modulePath: "github.com/user/comp",
			importedPaths: map[string]bool{
				"github.com/user/comp/a/b": true,
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := am.TestOnly_ModuleSubpackagesUsed(tt.modulePath, "", tt.importedPaths)
			sort.Strings(got)
			sort.Strings(tt.want)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
