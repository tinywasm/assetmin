package assetmin_test

import (
	"strings"
	"testing"
)

func TestRefreshAsset(t *testing.T) {
	currentMode := "go"
	mockTinyWasmHandler := func() (string, error) {
		switch currentMode {
		case "go":
			return "// Go WASM Runtime", nil
		case "tinygo":
			return "// TinyGo WASM Runtime", nil
		default:
			return "", nil
		}
	}

	env := setupTestEnv("refresh_asset", t, mockTinyWasmHandler)
	am := env.AssetsHandler

	// 1. Verify initial
	initCode, _ := am.GetInitCodeJS()
	if !strings.Contains(initCode, "Go WASM Runtime") {
		t.Errorf("Expected Go runtime, got %s", initCode)
	}

	// 2. Change mode and refresh
	currentMode = "tinygo"
	am.RefreshWasmAssets()

	// Verify update
	initCode, _ = am.GetInitCodeJS()
	if !strings.Contains(initCode, "TinyGo WASM Runtime") {
		t.Errorf("Expected TinyGo runtime, got %s", initCode)
	}
}

func TestRefreshAssetMultipleFiles(t *testing.T) {
	currentMode := "standard"
	mockInit := func() (string, error) {
		if currentMode == "standard" {
			return "// standardInit", nil
		}
		return "// enhancedInit", nil
	}

	env := setupTestEnv("refresh_multiple", t, mockInit)
	am := env.AssetsHandler

	initCode, _ := am.GetInitCodeJS()
	if !strings.Contains(initCode, "standardInit") {
		t.Errorf("Expected standardInit, got %s", initCode)
	}

	currentMode = "enhanced"
	am.RefreshWasmAssets()

	initCode, _ = am.GetInitCodeJS()
	if !strings.Contains(initCode, "enhancedInit") {
		t.Errorf("Expected enhancedInit, got %s", initCode)
	}
}
