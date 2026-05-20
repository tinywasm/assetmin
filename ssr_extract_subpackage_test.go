package assetmin

import (
	"os"
	"path/filepath"
	"testing"
)

// TestExtractSSRAssetsForModule_Subpackage reproduces the bug where SSR
// extraction returns empty assets when the requested module is a Go
// subpackage of an existing module (i.e. it has no go.mod of its own).
//
// Symptom in production: tinywasm/layout/platformd is a subpackage of the
// tinywasm/layout module. platformd.SSRInstance().RenderCSS().String()
// produces ~6.6 KB of valid CSS, but assetmin writes 0 bytes to /style.css.
//
// Root cause: extractSSRAssetsForModule(m, rootDir, allModules, "") calls
// invokeSSRExtractorOnce(rootDir, allModules) which generates a main.go
// importing only the modules listed in allModules. Subpackages never appear
// in `go list -m -json all`, so the extractor never imports them and the
// per-module results map has no entry for m.Path — the function returns
// SSRAssets{} silently.
//
// SRP: this test lives next to the responsible function
// (extractSSRAssetsForModule) rather than in the integration tests folder,
// because the bug is a local contract violation: the function promises to
// return the assets for `m`, regardless of whether `m` was discovered via
// `go list` or synthesized for a subpackage by the caller.
func TestExtractSSRAssetsForModule_Subpackage(t *testing.T) {
	parentDir := t.TempDir()

	// Parent module — single Go module, the subpackage lives INSIDE it
	// with no separate go.mod.
	parentGomod := "module example.com/parent\n\ngo 1.24\n"
	if err := os.WriteFile(filepath.Join(parentDir, "go.mod"), []byte(parentGomod), 0644); err != nil {
		t.Fatalf("write parent go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(parentDir, "parent.go"), []byte("package parent\n"), 0644); err != nil {
		t.Fatalf("write parent.go: %v", err)
	}

	// Subpackage with ssr.go exposing SSRInstance + RenderCSS,
	// mirroring the platformd shape.
	subDir := filepath.Join(parentDir, "sub")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	subSSR := `//go:build !wasm

package sub

type stylesheet string

func (s stylesheet) String() string { return string(s) }

type Sub struct{}

func (s *Sub) RenderCSS() stylesheet { return stylesheet(".sub{color:red}") }
`
	if err := os.WriteFile(filepath.Join(subDir, "ssr.go"), []byte(subSSR), 0644); err != nil {
		t.Fatalf("write sub/ssr.go: %v", err)
	}

	// Mirror what loadSSRModulesLocked does for subpackages:
	// allModules contains only the parent module (as `go list -m -json all`
	// would return), and subM is synthesized by the caller.
	allModules := []Module{
		{Path: "example.com/parent", Dir: parentDir},
	}
	subM := Module{Path: "example.com/parent/sub", Dir: subDir}

	// Reset the global SSR cache so previous tests do not poison this one.
	ssrGlobalCache = newSSRCache()

	assets, err := extractSSRAssetsForModule(subM, parentDir, allModules, "")
	if err != nil {
		t.Fatalf("extractSSRAssetsForModule returned error: %v", err)
	}
	if assets == nil {
		t.Fatal("extractSSRAssetsForModule returned nil assets")
	}

	const want = ".sub{color:red}"
	if assets.CSS != want {
		t.Fatalf("subpackage CSS not extracted\n  want: %q\n  got:  %q\n  module: %q",
			want, assets.CSS, assets.ModuleName)
	}
}
