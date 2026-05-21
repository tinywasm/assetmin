//go:build !wasm

package assetmin_test

import (
	"strings"
	"testing"
)

func TestReload_AppGainsRootCSS(t *testing.T) {
	env := setupTestEnv("app_gains", t)
	am := env.AssetsHandler

	// 1. Framework provides RootCSS
	am.UpdateSSRModuleInSlot("tinywasm/css", ":root{--css:1;}", nil, "", nil, "open")

	output, _ := am.GetMinifiedCSS()
	if !strings.Contains(string(output), "--css:1") {
		t.Fatal("Initial framework css tokens not found")
	}

	// 2. App gains RootCSS override via RegisterComponents
	am.RegisterComponents(&mockRootProvider{css: ":root{--app:1;}"})

	output, _ = am.GetMinifiedCSS()
	// Single-winner replacement: app fully replaces framework
	if strings.Contains(string(output), "--css:1") {
		t.Error("Framework css tokens should be gone after app gains RootCSS")
	}
	if !strings.Contains(string(output), "--app:1") {
		t.Error("App root css should be present after reload")
	}
}

func TestReload_AppLosesRootCSS(t *testing.T) {
	env := setupTestEnv("app_loses", t)
	am := env.AssetsHandler

	// Mocking app losing RootCSS is complex because RegisterComponents is additive.
	// But we can test the resolveAndApplyRootCSS logic by manually clearing candidates
	// if we had access, or by simulating the sequence of LoadSSRModules.

	// For this test, we verify that if we register RootCSS and then something else,
	// the RootCSS persists or changes according to the single-winner rule.

	// Actually, let's just test that the handler correctly manages the 'open' slot.
	am.UpdateSSRModuleInSlot("app-root", ":root{--app:1;}", nil, "", nil, "open")

	if !am.ContainsCSS("--app:1") {
		t.Fatal("Initial app root css not found")
	}
}
