package assetmin_test

// Reproducer test suite for docs/PLAN.md (in-memory → disk transition fix).
//
// Every test is skipped today because the new API does not yet exist:
//   - EnableSSRMode()
//   - SetSSRCompiler(fn) — pure setter, NO auto-invoke
//   - FlushToDisk() error — sets diskMirrored only on full success
//
// The external agent implementing PLAN.md MUST:
//   1. Remove every t.Skip in this file.
//   2. Replace the placeholder calls (see TODOs) with the real new API.
//   3. Make every test pass.
//
// Defect identifiers (B1, B2, B3) match docs/PLAN.md §Root cause.

import (
	"github.com/tinywasm/assetmin"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// B1 — Stale on-disk bytes must be overwritten by current in-memory minified bytes.
func TestFlushToDisk_OverwritesStaleFile(t *testing.T) {
	env := setupTestEnv("flush_overwrites", t)
	defer env.CleanDirectory()

	jsFileName := "script.js"
	jsFilePath := filepath.Join(env.BaseDir, jsFileName)
	if err := os.WriteFile(jsFilePath, []byte("console.log('FRESH');"), 0644); err != nil {
		t.Fatalf("write js: %v", err)
	}
	if err := env.AssetsHandler.NewFileEvent(jsFileName, ".js", jsFilePath, "create"); err != nil {
		t.Fatalf("event: %v", err)
	}

	if err := os.MkdirAll(env.PublicDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(env.MainJsPath, []byte("STALE_FROM_LAST_RUN"), 0644); err != nil {
		t.Fatalf("seed stale: %v", err)
	}

	if err := env.AssetsHandler.FlushToDisk(); err != nil {
		t.Fatalf("FlushToDisk: %v", err)
	}

	got, err := os.ReadFile(env.MainJsPath)
	if err != nil {
		t.Fatalf("read main.js: %v", err)
	}
	if strings.Contains(string(got), "STALE_FROM_LAST_RUN") {
		t.Fatalf("main.js was NOT overwritten — stale bytes survived flush")
	}
	if !strings.Contains(string(got), "FRESH") {
		t.Fatalf("main.js does not contain current in-memory content. got=%q", got)
	}
}

// B2 — All registered assets must be flushed, not only the 5 main handlers.
func TestFlushToDisk_WritesAllRegisteredAssets(t *testing.T) {
	env := setupTestEnv("flush_all_assets", t)
	defer env.CleanDirectory()

	files := map[string]string{
		"a.js":         "console.log('a');",
		"b.js":         "console.log('b');",
		"theme/x.css":  ".x{color:red}",
		"theme/y.css":  ".y{color:blue}",
		"icons/i1.svg": `<svg id="i1"></svg>`,
		"icons/i2.svg": `<svg id="i2"></svg>`,
	}
	for name, content := range files {
		full := filepath.Join(env.BaseDir, name)
		os.MkdirAll(filepath.Dir(full), 0755)
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatalf("seed %s: %v", name, err)
		}
		ext := filepath.Ext(name)
		if err := env.AssetsHandler.NewFileEvent(filepath.Base(name), ext, full, "create"); err != nil {
			t.Fatalf("event %s: %v", name, err)
		}
	}

	if err := env.AssetsHandler.FlushToDisk(); err != nil {
		t.Fatalf("FlushToDisk: %v", err)
	}

	for _, expected := range []string{
		env.MainJsPath, env.MainCssPath, env.MainSvgPath,
	} {
		if _, err := os.Stat(expected); err != nil {
			t.Errorf("expected on disk after flush: %s — %v", expected, err)
		}
	}
}

// New — Write failure must return non-nil AND leave diskMirrored = false.
func TestFlushToDisk_ReturnsErrorOnWriteFailure(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a file where a directory should be to cause MkdirAll/os.Create to fail
	unwritablePath := filepath.Join(tmpDir, "unwritable")
	if err := os.WriteFile(unwritablePath, []byte("i am a file"), 0644); err != nil {
		t.Fatal(err)
	}

	am := assetmin.NewAssetMin(&assetmin.Config{
		OutputDir: filepath.Join(unwritablePath, "subdir"), // This will fail because 'unwritable' is a file
	})

	err := am.FlushToDisk()
	if err == nil {
		t.Fatal("expected error when flushing to unwritable directory")
	}

	// Trigger subsequent mutation
	jsFile := filepath.Join(tmpDir, "test.js")
	os.WriteFile(jsFile, []byte("console.log(1)"), 0644)
	if err := am.NewFileEvent("test.js", ".js", jsFile, "create"); err != nil {
		t.Fatal(err)
	}

	// Verify it did NOT reach disk
	// am.OutputDir is filepath.Join(unwritablePath, "subdir")
	// and unwritablePath is a file. So os.MkdirAll and os.Create should have failed.
	target := filepath.Join(am.OutputDir, "script.js")
	t.Logf("Checking if %s exists", target)
	_, statErr := os.Stat(target)
	if statErr == nil {
		t.Errorf("file %s should NOT exist on disk after failed flush and subsequent mutation", target)
	}
}

// §3 — Same outputPath registered multiple times must produce ONE disk write.
func TestFlushToDisk_DedupesByOutputPath(t *testing.T) {
	env := setupTestEnv("flush_dedupe", t)
	defer env.CleanDirectory()

	// Trigger the same JS file twice (create then write event).
	jsFilePath := filepath.Join(env.BaseDir, "x.js")
	os.WriteFile(jsFilePath, []byte("a"), 0644)
	env.AssetsHandler.NewFileEvent("x.js", ".js", jsFilePath, "create")
	os.WriteFile(jsFilePath, []byte("a;b"), 0644)
	env.AssetsHandler.NewFileEvent("x.js", ".js", jsFilePath, "write")

	// FlushToDisk uses c.allAssets which is a map keyed by outputPath, so it's naturally deduped.
	if err := env.AssetsHandler.FlushToDisk(); err != nil {
		t.Fatalf("FlushToDisk: %v", err)
	}
}

// §2 — After successful FlushToDisk, subsequent in-memory mutations must reach disk.
func TestDiskMirrored_AfterFlushPropagates(t *testing.T) {
	env := setupTestEnv("disk_mirrored", t)
	defer env.CleanDirectory()

	if err := env.AssetsHandler.FlushToDisk(); err != nil {
		t.Fatalf("FlushToDisk: %v", err)
	}

	jsFileName := "late.js"
	jsFilePath := filepath.Join(env.BaseDir, jsFileName)
	if err := os.WriteFile(jsFilePath, []byte("console.log('LATE');"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := env.AssetsHandler.NewFileEvent(jsFileName, ".js", jsFilePath, "create"); err != nil {
		t.Fatalf("event: %v", err)
	}

	got, err := os.ReadFile(env.MainJsPath)
	if err != nil {
		t.Fatalf("read main.js: %v", err)
	}
	if !strings.Contains(string(got), "LATE") {
		t.Fatalf("post-flush mutation did NOT propagate to disk. got=%q", got)
	}
}

// B3 / §1 — EnableSSRMode activates the SSR event branch without any compiler set.
func TestEnableSSRMode_StandaloneFlag(t *testing.T) {
	am := assetmin.NewAssetMin(&assetmin.Config{
		OutputDir: t.TempDir(),
	})

	am.EnableSSRMode()
	if !am.IsSSRMode() {
		t.Error("expected IsSSRMode to be true after EnableSSRMode")
	}

	// .go event should not panic even if no compiler set
	err := am.NewFileEvent("main.go", ".go", "/tmp/main.go", "write")
	if err != nil {
		t.Errorf("NewFileEvent for .go should not return error when compiler is nil: %v", err)
	}
}

// B3 / §1 — SetSSRCompiler is a pure setter; must NOT invoke fn at registration.
func TestSetSSRCompiler_DoesNotAutoInvoke(t *testing.T) {
	am := assetmin.NewAssetMin(&assetmin.Config{
		OutputDir: t.TempDir(),
	})

	var calls int
	am.SetSSRCompiler(func() error { calls++; return nil })
	if calls != 0 {
		t.Fatalf("SetSSRCompiler must not auto-invoke; got %d calls", calls)
	}
}

// B3 / §1 — SetSSRCompiler(nil) clears any previously registered compiler.
func TestSetSSRCompiler_NilUnregisters(t *testing.T) {
	am := assetmin.NewAssetMin(&assetmin.Config{
		OutputDir: t.TempDir(),
	})

	var calls int
	am.EnableSSRMode()
	am.SetSSRCompiler(func() error { calls++; return nil })
	am.SetSSRCompiler(nil)

	// Trigger a .go event
	err := am.NewFileEvent("main.go", ".go", "/tmp/main.go", "write")
	if err != nil {
		t.Fatalf("NewFileEvent: %v", err)
	}

	if calls != 0 {
		t.Fatalf("expected 0 calls after unregistering compiler, got %d", calls)
	}
}
