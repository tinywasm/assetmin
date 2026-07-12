//go:build !wasm

package assetmin_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tinywasm/assetmin"
)

// fakeExtractor stands in for tinywasm/ssr: the real one shells out to `go run`
// on a generated main.go, which is slow and toolchain-dependent. Here the module's
// css.go IS the asset — its file content is returned verbatim as the module CSS,
// so a test can rewrite css.go on disk and assert the served CSS followed.
type fakeExtractor struct {
	calls int
}

func (f *fakeExtractor) ExtractModule(moduleDir string) (*assetmin.SSRAssets, error) {
	f.calls++
	css, err := os.ReadFile(filepath.Join(moduleDir, "css.go"))
	if err != nil {
		return nil, err
	}
	return &assetmin.SSRAssets{
		ModuleName: filepath.Base(moduleDir),
		CSS:        string(css),
	}, nil
}

func (f *fakeExtractor) ExtractAll() ([]*assetmin.SSRAssets, error) {
	return nil, nil
}

type fakeImageProcessor struct {
	reloaded []string
}

func (f *fakeImageProcessor) LoadImages() error { return nil }
func (f *fakeImageProcessor) ReloadModule(moduleDir string) error {
	f.reloaded = append(f.reloaded, moduleDir)
	return nil
}
func (f *fakeImageProcessor) UnobservedFiles() []string { return nil }

// ssrWatcherEnv builds an AssetMin in SSR mode with a module dir on disk whose
// css.go carries the given marker.
func ssrWatcherEnv(t *testing.T, marker string) (*assetmin.AssetMin, string, *fakeExtractor, *int) {
	t.Helper()

	root := t.TempDir()
	moduleDir := filepath.Join(root, "platformd")
	if err := os.MkdirAll(moduleDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeCSSGo(t, moduleDir, marker)

	am := assetmin.NewAssetMin(&assetmin.Config{
		OutputDir: t.TempDir(),
		RootDir:   root,
		DevMode:   true,
	})
	am.EnableSSRMode()

	ex := &fakeExtractor{}
	am.SetSSRExtractor(ex)

	reloads := 0
	return am, moduleDir, ex, &reloads
}

func writeCSSGo(t *testing.T, moduleDir, marker string) {
	t.Helper()
	css := ".probe{color:" + marker + "}"
	if err := os.WriteFile(filepath.Join(moduleDir, "css.go"), []byte(css), 0644); err != nil {
		t.Fatal(err)
	}
}

// TestSSRWatcher_Contract pins the two declarations devwatch routes on.
//
// devwatch applies its depfind ownership gate ONLY to handlers whose main input
// is a .go file. SSRFileWatcher deliberately declares "go.mod" so it bypasses
// that gate and receives every .go event, self-filtering by basename. Changing
// either value silently stops SSR hot reload — the symptom is "editing css.go
// does nothing until the daemon restarts".
func TestSSRWatcher_Contract(t *testing.T) {
	am := assetmin.NewAssetMin(&assetmin.Config{OutputDir: t.TempDir()})
	w := am.NewSSRFileWatcher(nil)

	if got := w.MainInputFileRelativePath(); got != "go.mod" {
		t.Errorf("MainInputFileRelativePath() = %q, want \"go.mod\" (a non-.go main input is what bypasses devwatch's depfind gate)", got)
	}
	exts := w.SupportedExtensions()
	if len(exts) != 1 || exts[0] != ".go" {
		t.Errorf("SupportedExtensions() = %v, want [\".go\"]", exts)
	}
}

// TestSSRWatcher_ReloadsAssetSource is the routing guard: a .go asset-source file
// must re-extract its module and refresh what is served.
func TestSSRWatcher_ReloadsAssetSource(t *testing.T) {
	am, moduleDir, ex, reloads := ssrWatcherEnv(t, "#111111")

	if err := am.ReloadSSRModule(moduleDir); err != nil {
		t.Fatalf("initial load: %v", err)
	}
	if !am.ContainsCSS("#111111") {
		t.Fatal("precondition: initial CSS not served")
	}

	// The developer edits css.go; devwatch hands the event to the watcher.
	writeCSSGo(t, moduleDir, "#222222")
	w := am.NewSSRFileWatcher(func() error { *reloads++; return nil })
	if err := w.NewFileEvent("css.go", ".go", filepath.Join(moduleDir, "css.go"), "write"); err != nil {
		t.Fatalf("NewFileEvent: %v", err)
	}

	if am.ContainsCSS("#111111") {
		t.Error("stale CSS still served after css.go changed")
	}
	if !am.ContainsCSS("#222222") {
		t.Error("updated CSS not served after css.go changed")
	}
	if *reloads != 1 {
		t.Errorf("browser reload fired %d times, want 1", *reloads)
	}
	if ex.calls != 2 {
		t.Errorf("extractor called %d times, want 2 (initial load + reload)", ex.calls)
	}
}

// TestSSRWatcher_RoutesByBasename covers the self-filtering the whole design leans
// on: devwatch sends EVERY .go event, so the watcher itself must decide.
func TestSSRWatcher_RoutesByBasename(t *testing.T) {
	assetSources := []string{"css.go", "js.go", "svg.go", "html.go"}

	for _, name := range assetSources {
		t.Run(name+" re-extracts the module", func(t *testing.T) {
			am, moduleDir, ex, reloads := ssrWatcherEnv(t, "#111111")
			w := am.NewSSRFileWatcher(func() error { *reloads++; return nil })

			if err := w.NewFileEvent(name, ".go", filepath.Join(moduleDir, name), "write"); err != nil {
				t.Fatalf("NewFileEvent(%s): %v", name, err)
			}
			if ex.calls != 1 {
				t.Errorf("extractor called %d times, want 1", ex.calls)
			}
			if *reloads != 1 {
				t.Errorf("browser reload fired %d times, want 1", *reloads)
			}
		})
	}

	t.Run("image.go goes to the image processor, not the extractor", func(t *testing.T) {
		am, moduleDir, ex, reloads := ssrWatcherEnv(t, "#111111")
		img := &fakeImageProcessor{}
		am.SetImageProcessor(img)
		w := am.NewSSRFileWatcher(func() error { *reloads++; return nil })

		if err := w.NewFileEvent("image.go", ".go", filepath.Join(moduleDir, "image.go"), "write"); err != nil {
			t.Fatalf("NewFileEvent: %v", err)
		}
		if len(img.reloaded) != 1 || img.reloaded[0] != moduleDir {
			t.Errorf("image processor reloaded %v, want [%s]", img.reloaded, moduleDir)
		}
		if ex.calls != 0 {
			t.Errorf("SSR extractor ran for image.go (%d calls); it must not", ex.calls)
		}
		if *reloads != 1 {
			t.Errorf("browser reload fired %d times, want 1", *reloads)
		}
	})

	t.Run("any other .go file is ignored", func(t *testing.T) {
		am, moduleDir, ex, reloads := ssrWatcherEnv(t, "#111111")
		w := am.NewSSRFileWatcher(func() error { *reloads++; return nil })

		for _, name := range []string{"main.go", "handler.go", "models.go"} {
			if err := w.NewFileEvent(name, ".go", filepath.Join(moduleDir, name), "write"); err != nil {
				t.Fatalf("NewFileEvent(%s): %v", name, err)
			}
		}
		if ex.calls != 0 {
			t.Errorf("extractor ran %d times for non-asset .go files; want 0", ex.calls)
		}
		if *reloads != 0 {
			t.Errorf("browser reload fired %d times for non-asset .go files; want 0", *reloads)
		}
	})
}

// TestSSRWatcher_ExtractorErrorSurfaces: a broken module must report, not swallow.
func TestSSRWatcher_ExtractorErrorSurfaces(t *testing.T) {
	am, moduleDir, _, reloads := ssrWatcherEnv(t, "#111111")
	w := am.NewSSRFileWatcher(func() error { *reloads++; return nil })

	// css.go is gone → the fake extractor fails to read it, like the real one
	// fails to compile a broken module.
	if err := os.Remove(filepath.Join(moduleDir, "css.go")); err != nil {
		t.Fatal(err)
	}

	err := w.NewFileEvent("css.go", ".go", filepath.Join(moduleDir, "css.go"), "write")
	if err == nil {
		t.Fatal("expected the extraction error to surface")
	}
	if !strings.Contains(err.Error(), "css.go") {
		t.Errorf("error should name the offending file, got: %v", err)
	}
	if *reloads != 0 {
		t.Errorf("browser reloaded (%d) despite a failed extraction", *reloads)
	}
}
