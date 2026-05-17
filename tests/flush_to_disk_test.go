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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// B1 — Stale on-disk bytes must be overwritten by current in-memory minified bytes.
func TestFlushToDisk_OverwritesStaleFile(t *testing.T) {
	t.Skip("see docs/PLAN.md — requires FlushToDisk API")

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

	// TODO(agent): env.AssetsHandler.FlushToDisk()

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
	t.Skip("see docs/PLAN.md — requires enumeration of all assets via c.allAssets")

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

	// TODO(agent): env.AssetsHandler.FlushToDisk()

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
	t.Skip("see docs/PLAN.md — requires error propagation + diskMirrored false-on-error")

	// TODO(agent):
	// 1. Construct an AssetMin whose OutputDir is unwritable (e.g. a file path,
	//    not a directory; or chmod 0500 on a parent).
	// 2. err := am.FlushToDisk(); assert err != nil.
	// 3. Trigger a subsequent in-memory mutation; assert it does NOT reach disk
	//    (diskMirrored must remain false after a failed flush).
}

// §3 — Same outputPath registered multiple times must produce ONE disk write.
func TestFlushToDisk_DedupesByOutputPath(t *testing.T) {
	t.Skip("see docs/PLAN.md — requires c.allAssets as a map keyed by outputPath")

	env := setupTestEnv("flush_dedupe", t)
	defer env.CleanDirectory()

	// Trigger the same JS file twice (create then write event).
	jsFilePath := filepath.Join(env.BaseDir, "x.js")
	os.WriteFile(jsFilePath, []byte("a"), 0644)
	env.AssetsHandler.NewFileEvent("x.js", ".js", jsFilePath, "create")
	os.WriteFile(jsFilePath, []byte("a;b"), 0644)
	env.AssetsHandler.NewFileEvent("x.js", ".js", jsFilePath, "write")

	// TODO(agent): instrument FileWrite or count via a hookable writer
	// and assert exactly 1 call per outputPath during FlushToDisk().
}

// §2 — After successful FlushToDisk, subsequent in-memory mutations must reach disk.
func TestDiskMirrored_AfterFlushPropagates(t *testing.T) {
	t.Skip("see docs/PLAN.md — requires post-flush disk-mirrored mode")

	env := setupTestEnv("disk_mirrored", t)
	defer env.CleanDirectory()

	// TODO(agent): env.AssetsHandler.FlushToDisk() (no assets yet — should be no-op success)

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
	t.Skip("see docs/PLAN.md — requires EnableSSRMode API")

	// TODO(agent):
	// env.AssetsHandler.EnableSSRMode()
	// Assert (via public observable, e.g. an inspect.go method or behavior of
	// NewFileEvent on a .css event) that the SSR branch is taken even though
	// SetSSRCompiler has NEVER been called.
}

// B3 / §1 — SetSSRCompiler is a pure setter; must NOT invoke fn at registration.
func TestSetSSRCompiler_DoesNotAutoInvoke(t *testing.T) {
	t.Skip("see docs/PLAN.md — requires SetSSRCompiler as pure setter")

	// TODO(agent):
	// var calls int
	// env.AssetsHandler.SetSSRCompiler(func() error { calls++; return nil })
	// if calls != 0 { t.Fatalf("SetSSRCompiler must not auto-invoke; got %d calls", calls) }
}

// B3 / §1 — SetSSRCompiler(nil) clears any previously registered compiler.
func TestSetSSRCompiler_NilUnregisters(t *testing.T) {
	t.Skip("see docs/PLAN.md — requires nil-unregister semantics")

	// TODO(agent):
	// var calls int
	// env.AssetsHandler.EnableSSRMode()
	// env.AssetsHandler.SetSSRCompiler(func() error { calls++; return nil })
	// env.AssetsHandler.SetSSRCompiler(nil)
	// Trigger a .go event (or whatever path invokes the compiler).
	// Assert calls == 0 (compiler cleared) AND no panic (nil handled).
}
