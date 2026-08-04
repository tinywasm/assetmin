//go:build !wasm

package assetmin_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tinywasm/assetmin"
	"github.com/tinywasm/font"
)

type fontsExtractor struct {
	assets *assetmin.SSRAssets
}

func (f *fontsExtractor) ExtractModule(moduleDir string) (*assetmin.SSRAssets, error) {
	return f.assets, nil
}

func (f *fontsExtractor) ExtractAll() ([]*assetmin.SSRAssets, error) {
	if f.assets == nil {
		return nil, nil
	}
	return []*assetmin.SSRAssets{f.assets}, nil
}

func writeFaceFiles(t *testing.T, dir string, family font.Family, skip styleSkip) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for s := font.Regular; s <= font.BoldItalic; s++ {
		if skip[s] {
			continue
		}
		name := family.Face(s) + ".ttf"
		// Minimal non-empty payload; content is irrelevant to the copy path.
		if err := os.WriteFile(filepath.Join(dir, name), []byte("ttf-"+name), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

type styleSkip map[font.Style]bool

func TestFonts_CopyAndFontFaceCSS(t *testing.T) {
	root := t.TempDir()
	out := t.TempDir()
	fontDir := filepath.Join(root, "config", "fonts")
	writeFaceFiles(t, fontDir, "Roboto", nil)

	am := assetmin.NewAssetMin(&assetmin.Config{
		RootDir:         root,
		OutputDir:       out,
		AssetsURLPrefix: "assets",
	})
	d := font.Declare("Roboto", "config/fonts")
	am.SetSSRExtractor(&fontsExtractor{assets: &assetmin.SSRAssets{
		ModuleName: "app",
		IsRoot:     true,
		Fonts:      d,
	}})
	am.LoadSSRModules()
	am.WaitForSSRLoad(5 * time.Second)

	for s := font.Regular; s <= font.BoldItalic; s++ {
		name := d.Family().Face(s) + ".ttf"
		dst := filepath.Join(out, name)
		b, err := os.ReadFile(dst)
		if err != nil {
			t.Fatalf("expected copied face %s: %v", name, err)
		}
		if string(b) != "ttf-"+name {
			t.Errorf("face %s content = %q", name, b)
		}
	}

	css, err := am.GetMinifiedCSS()
	if err != nil {
		t.Fatal(err)
	}
	got := string(css)
	if !strings.Contains(got, "@font-face") {
		t.Fatalf("CSS missing @font-face: %s", got)
	}
	if !strings.Contains(got, `format("truetype")`) && !strings.Contains(got, `format(truetype)`) {
		// minify may strip quotes around the format token
		t.Fatalf("CSS missing truetype format: %s", got)
	}
	for s := font.Regular; s <= font.BoldItalic; s++ {
		wantURL := "/assets/" + d.Family().Face(s) + ".ttf"
		if !strings.Contains(got, wantURL) {
			t.Errorf("CSS missing url %s in: %s", wantURL, got)
		}
	}

	// UnobservedFiles must list the four destinations.
	unobs := am.UnobservedFiles()
	for s := font.Regular; s <= font.BoldItalic; s++ {
		want := filepath.Join(out, d.Family().Face(s)+".ttf")
		found := false
		for _, p := range unobs {
			if p == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("UnobservedFiles missing %s; got %v", want, unobs)
		}
	}
}

func TestFonts_MissingFaceErrors(t *testing.T) {
	root := t.TempDir()
	out := t.TempDir()
	fontDir := filepath.Join(root, "config", "fonts")
	// Skip Bold intentionally.
	writeFaceFiles(t, fontDir, "Roboto", styleSkip{font.Bold: true})

	var logs []string
	am := assetmin.NewAssetMin(&assetmin.Config{
		RootDir:   root,
		OutputDir: out,
	})
	am.SetLog(func(msg ...any) {
		var b strings.Builder
		for _, m := range msg {
			b.WriteString(strings.TrimSpace(toString(m)))
			b.WriteByte(' ')
		}
		logs = append(logs, b.String())
	})
	am.SetSSRExtractor(&fontsExtractor{assets: &assetmin.SSRAssets{
		ModuleName: "app",
		IsRoot:     true,
		Fonts:      font.Declare("Roboto", "config/fonts"),
	}})

	// Via ReloadSSRModule the error must surface with the missing file name.
	err := am.ReloadSSRModule(root)
	if err == nil {
		t.Fatal("expected error for missing Bold face")
	}
	if !strings.Contains(err.Error(), "Roboto-Bold.ttf") {
		t.Errorf("error should name Roboto-Bold.ttf, got: %v", err)
	}
}

func TestFonts_SkipCopyWhenUpToDate(t *testing.T) {
	root := t.TempDir()
	out := t.TempDir()
	fontDir := filepath.Join(root, "config", "fonts")
	writeFaceFiles(t, fontDir, "Roboto", nil)

	am := assetmin.NewAssetMin(&assetmin.Config{
		RootDir:   root,
		OutputDir: out,
	})
	d := font.Declare("Roboto", "config/fonts")
	ex := &fontsExtractor{assets: &assetmin.SSRAssets{
		ModuleName: "app",
		IsRoot:     true,
		Fonts:      d,
	}}
	am.SetSSRExtractor(ex)

	if err := am.ReloadSSRModule(root); err != nil {
		t.Fatal(err)
	}
	// Stamp destinations as newer than sources.
	future := time.Now().Add(time.Hour)
	for s := font.Regular; s <= font.BoldItalic; s++ {
		name := d.Family().Face(s) + ".ttf"
		dst := filepath.Join(out, name)
		if err := os.Chtimes(dst, future, future); err != nil {
			t.Fatal(err)
		}
		// Overwrite content so we can detect a re-copy.
		if err := os.WriteFile(dst, []byte("kept"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(dst, future, future); err != nil {
			t.Fatal(err)
		}
	}

	if err := am.ReloadSSRModule(root); err != nil {
		t.Fatal(err)
	}
	for s := font.Regular; s <= font.BoldItalic; s++ {
		name := d.Family().Face(s) + ".ttf"
		b, err := os.ReadFile(filepath.Join(out, name))
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != "kept" {
			t.Errorf("%s was re-copied; want stale skip, got %q", name, b)
		}
	}
}

func TestFonts_NonRootIgnored(t *testing.T) {
	root := t.TempDir()
	out := t.TempDir()
	fontDir := filepath.Join(root, "vendor", "fonts")
	writeFaceFiles(t, fontDir, "Roboto", nil)

	var logs []string
	am := assetmin.NewAssetMin(&assetmin.Config{
		RootDir:   root,
		OutputDir: out,
	})
	am.SetLog(func(msg ...any) {
		var b strings.Builder
		for _, m := range msg {
			b.WriteString(strings.TrimSpace(toString(m)))
			b.WriteByte(' ')
		}
		logs = append(logs, b.String())
	})
	am.SetSSRExtractor(&fontsExtractor{assets: &assetmin.SSRAssets{
		ModuleName: "some/dep",
		IsRoot:     false,
		Fonts:      font.Declare("Roboto", "vendor/fonts"),
	}})
	if err := am.ReloadSSRModule(filepath.Join(root, "vendor")); err != nil {
		t.Fatal(err)
	}
	// No faces copied.
	entries, _ := os.ReadDir(out)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".ttf") {
			t.Errorf("non-root fonts must not be copied, found %s", e.Name())
		}
	}
	css, _ := am.GetMinifiedCSS()
	if strings.Contains(string(css), "@font-face") {
		t.Error("non-root Fonts must not inject @font-face")
	}
	joined := strings.Join(logs, "\n")
	if !strings.Contains(joined, "Fonts()") {
		t.Errorf("expected warning about Fonts(), logs: %s", joined)
	}
}

func TestFonts_NoDeclarationNoOp(t *testing.T) {
	root := t.TempDir()
	am := assetmin.NewAssetMin(&assetmin.Config{
		RootDir:   root,
		OutputDir: t.TempDir(),
	})
	am.SetSSRExtractor(&fontsExtractor{assets: &assetmin.SSRAssets{
		ModuleName: "app",
		IsRoot:     true,
	}})
	if err := am.ReloadSSRModule(root); err != nil {
		t.Fatal(err)
	}
	css, err := am.GetMinifiedCSS()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(css), "@font-face") {
		t.Error("zero-value Fonts must not emit @font-face")
	}
	if paths := filterTTF(am.UnobservedFiles()); len(paths) != 0 {
		t.Errorf("no fonts → no ttf in UnobservedFiles, got %v", paths)
	}
}

func TestFonts_HotReloadUpdatesCSS(t *testing.T) {
	root := t.TempDir()
	out := t.TempDir()
	writeFaceFiles(t, filepath.Join(root, "config", "fonts"), "Roboto", nil)
	writeFaceFiles(t, filepath.Join(root, "config", "fonts"), "Inter", nil)

	ex := &fontsExtractor{assets: &assetmin.SSRAssets{
		ModuleName: "app",
		IsRoot:     true,
		Fonts:      font.Declare("Roboto", "config/fonts"),
	}}
	am := assetmin.NewAssetMin(&assetmin.Config{
		RootDir:         root,
		OutputDir:       out,
		AssetsURLPrefix: "assets",
	})
	am.SetSSRExtractor(ex)

	if err := am.ReloadSSRModule(root); err != nil {
		t.Fatal(err)
	}
	css, _ := am.GetMinifiedCSS()
	if !strings.Contains(string(css), "Roboto-Regular.ttf") {
		t.Fatalf("expected Roboto in CSS: %s", css)
	}

	// Simulate editing fonts.go → new Declaration.
	ex.assets = &assetmin.SSRAssets{
		ModuleName: "app",
		IsRoot:     true,
		Fonts:      font.Declare("Inter", "config/fonts"),
	}
	w := am.NewSSRFileWatcher(nil)
	if err := w.NewFileEvent("fonts.go", ".go", filepath.Join(root, "config", "fonts.go"), "write"); err != nil {
		t.Fatal(err)
	}
	css, _ = am.GetMinifiedCSS()
	got := string(css)
	if strings.Contains(got, "Roboto-Regular.ttf") {
		t.Error("stale Roboto still in CSS after fonts.go reload")
	}
	if !strings.Contains(got, "Inter-Regular.ttf") {
		t.Errorf("expected Inter after reload: %s", got)
	}
}

func filterTTF(paths []string) []string {
	var out []string
	for _, p := range paths {
		if strings.HasSuffix(p, ".ttf") {
			out = append(out, p)
		}
	}
	return out
}

func toString(v any) string {
	return fmt.Sprint(v)
}
