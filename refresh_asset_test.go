package assetmin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRefreshAsset verifies that the RefreshAsset method correctly rebuilds
// the asset handler when external dependencies (like TinyWasm's initializer JS) change.
// It tests that:
// 1. Initial compilation includes the mock WASM runtime JS
// 2. After changing the mock mode and calling RefreshAsset, the output is updated with new WASM JS
// 3. User JS files remain intact during the refresh
// 4. The method works for both .js and .css extensions
func TestRefreshAsset(t *testing.T) {
	// Mock TinyWasm handler that simulates different JavaScript for different modes
	currentMode := "go" // Initial mode
	mockTinyWasmHandler := func() (string, error) {
		switch currentMode {
		case "go":
			return `
// Go WASM Runtime
const goRuntime = new Go();
WebAssembly.instantiateStreaming(fetch("main.wasm"), goRuntime.importObject).then((result) => {
	goRuntime.run(result.instance);
});`, nil
		case "tinygo":
			return `
// TinyGo WASM Runtime 
const tinyGoRuntime = new Go();
WebAssembly.instantiateStreaming(fetch("main.wasm"), tinyGoRuntime.importObject).then((result) => {
	tinyGoRuntime.run(result.instance);
});`, nil
		case "minimal":
			return `
// Minimal WASM Runtime
const minimalRuntime = new Go();
WebAssembly.instantiateStreaming(fetch("main.wasm"), minimalRuntime.importObject).then((result) => {
	minimalRuntime.run(result.instance);
});`, nil
		default:
			return "", nil
		}
	}

	// Setup test environment with mock TinyWasm handler
	env := setupTestEnv("refresh_asset", t, mockTinyWasmHandler)
	env.AssetsHandler.SetBuildOnDisk(true)
	defer env.CleanDirectory()

	// Prepare JS files in different directories to simulate a real project
	file1Path := filepath.Join(env.BaseDir, "modules", "module1", "app.js")
	file2Path := filepath.Join(env.BaseDir, "modules", "module2", "utils.js")
	file3Path := filepath.Join(env.BaseDir, "web", "theme", "theme.js")

	if err := os.MkdirAll(filepath.Dir(file1Path), 0755); err != nil {
		t.Fatalf("Failed to create dir module1: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(file2Path), 0755); err != nil {
		t.Fatalf("Failed to create dir module2: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(file3Path), 0755); err != nil {
		t.Fatalf("Failed to create dir theme: %v", err)
	}

	file1Content := "console.log('App Module');"
	file2Content := "console.log('Utils Module');"
	file3Content := "console.log('Theme Styles');"

	if err := os.WriteFile(file1Path, []byte(file1Content), 0644); err != nil {
		t.Fatalf("Failed to write app.js: %v", err)
	}
	if err := os.WriteFile(file2Path, []byte(file2Content), 0644); err != nil {
		t.Fatalf("Failed to write utils.js: %v", err)
	}
	if err := os.WriteFile(file3Path, []byte(file3Content), 0644); err != nil {
		t.Fatalf("Failed to write theme.js: %v", err)
	}

	// Phase 1: Initial compilation with Go mode
	t.Log("Phase 1: Initial compilation with Go WASM mode")
	if err := env.AssetsHandler.NewFileEvent("app.js", ".js", file1Path, "write"); err != nil {
		t.Fatalf("Error processing app.js write: %v", err)
	}
	if err := env.AssetsHandler.NewFileEvent("utils.js", ".js", file2Path, "write"); err != nil {
		t.Fatalf("Error processing utils.js write: %v", err)
	}
	if err := env.AssetsHandler.NewFileEvent("theme.js", ".js", file3Path, "write"); err != nil {
		t.Fatalf("Error processing theme.js write: %v", err)
	}

	// Verify initial compilation
	if _, err := os.Stat(env.MainJsPath); os.IsNotExist(err) {
		t.Fatalf("script.js must exist after initial compilation at %s", env.MainJsPath)
	}
	initialMain, err := os.ReadFile(env.MainJsPath)
	if err != nil {
		t.Fatalf("unable to read script.js after initial compilation: %v", err)
	}

	initialMainStr := string(initialMain)
	if !strings.Contains(initialMainStr, "goRuntime") {
		t.Errorf("script.js should contain Go WASM runtime")
	}
	if !strings.Contains(initialMainStr, "App Module") {
		t.Errorf("script.js should contain app.js content")
	}
	if !strings.Contains(initialMainStr, "Utils Module") {
		t.Errorf("script.js should contain utils.js content")
	}
	if !strings.Contains(initialMainStr, "Theme Styles") {
		t.Errorf("script.js should contain theme.js content")
	}
	t.Log("✓ Initial compilation successful with Go WASM runtime")

	// Phase 2: Change mock mode to TinyGo and refresh asset
	t.Log("Phase 2: Changing WASM mode to TinyGo and refreshing JS asset")
	currentMode = "tinygo"

	// Call RefreshAsset to trigger rebuild
	env.AssetsHandler.RefreshAsset(".js")

	// Verify that script.js was updated with new WASM runtime
	afterRefresh, err := os.ReadFile(env.MainJsPath)
	if err != nil {
		t.Fatalf("unable to read script.js after RefreshAsset: %v", err)
	}
	afterRefreshStr := string(afterRefresh)

	if !strings.Contains(afterRefreshStr, "tinyGoRuntime") {
		t.Errorf("script.js should contain TinyGo WASM runtime after refresh")
	}
	if strings.Contains(afterRefreshStr, "goRuntime") {
		t.Errorf("script.js should NOT contain Go WASM runtime after refresh")
	}

	// Verify user JS files are still present
	if !strings.Contains(afterRefreshStr, "App Module") {
		t.Errorf("script.js should still contain app.js content")
	}
	if !strings.Contains(afterRefreshStr, "Utils Module") {
		t.Errorf("script.js should still contain utils.js content")
	}
	if !strings.Contains(afterRefreshStr, "Theme Styles") {
		t.Errorf("script.js should still contain theme.js content")
	}
	t.Log("✓ RefreshAsset successfully updated WASM runtime while preserving user JS")

	// Phase 3: Change to minimal mode and refresh again
	t.Log("Phase 3: Changing to minimal WASM mode and refreshing")
	currentMode = "minimal"
	env.AssetsHandler.RefreshAsset(".js")

	afterSecondRefresh, err := os.ReadFile(env.MainJsPath)
	if err != nil {
		t.Fatalf("unable to read script.js after second RefreshAsset: %v", err)
	}
	afterSecondRefreshStr := string(afterSecondRefresh)

	if !strings.Contains(afterSecondRefreshStr, "minimalRuntime") {
		t.Errorf("script.js should contain minimal WASM runtime")
	}
	if strings.Contains(afterSecondRefreshStr, "tinyGoRuntime") {
		t.Errorf("script.js should NOT contain TinyGo runtime")
	}
	if strings.Contains(afterSecondRefreshStr, "goRuntime") {
		t.Errorf("script.js should NOT contain Go runtime")
	}

	// Verify user JS files are still present
	if !strings.Contains(afterSecondRefreshStr, "App Module") {
		t.Errorf("script.js should still contain app.js content")
	}
	if !strings.Contains(afterSecondRefreshStr, "Utils Module") {
		t.Errorf("script.js should still contain utils.js content")
	}
	if !strings.Contains(afterSecondRefreshStr, "Theme Styles") {
		t.Errorf("script.js should still contain theme.js content")
	}
	t.Log("✓ Second RefreshAsset successful")

	// Phase 4: Verify that refreshing without mode change doesn't corrupt the file
	t.Log("Phase 4: Refreshing without mode change")
	beforeIdempotent, err := os.ReadFile(env.MainJsPath)
	if err != nil {
		t.Fatalf("Failed to read before indentpotent: %v", err)
	}

	env.AssetsHandler.RefreshAsset(".js")

	afterIdempotent, err := os.ReadFile(env.MainJsPath)
	if err != nil {
		t.Fatalf("Failed to read after indentpotent: %v", err)
	}

	// Content should be identical (idempotent operation)
	if string(beforeIdempotent) != string(afterIdempotent) {
		t.Errorf("RefreshAsset should be idempotent when no changes occur")
	}
	t.Log("✓ RefreshAsset is idempotent")
}

// TestRefreshAssetCSS verifies that RefreshAsset works for CSS files
func TestRefreshAssetCSS(t *testing.T) {
	env := setupTestEnv("refresh_asset_css", t)
	env.AssetsHandler.SetBuildOnDisk(true)
	defer env.CleanDirectory()

	// Create CSS files
	file1Path := filepath.Join(env.BaseDir, "modules", "styles.css")
	file2Path := filepath.Join(env.BaseDir, "web", "theme", "theme.css")

	if err := os.MkdirAll(filepath.Dir(file1Path), 0755); err != nil {
		t.Fatalf("Error: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(file2Path), 0755); err != nil {
		t.Fatalf("Error: %v", err)
	}

	file1Content := ".styles { color: blue; }"
	file2Content := ".theme { color: red; }"

	if err := os.WriteFile(file1Path, []byte(file1Content), 0644); err != nil {
		t.Fatalf("Error: %v", err)
	}
	if err := os.WriteFile(file2Path, []byte(file2Content), 0644); err != nil {
		t.Fatalf("Error: %v", err)
	}

	// Initial compilation
	if err := env.AssetsHandler.NewFileEvent("styles.css", ".css", file1Path, "write"); err != nil {
		t.Fatalf("Error: %v", err)
	}
	if err := env.AssetsHandler.NewFileEvent("theme.css", ".css", file2Path, "write"); err != nil {
		t.Fatalf("Error: %v", err)
	}

	// Verify initial CSS
	if _, err := os.Stat(env.MainCssPath); os.IsNotExist(err) {
		t.Fatalf("style.css must exist")
	}
	initialCSS, err := os.ReadFile(env.MainCssPath)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}

	initialCSSStr := string(initialCSS)
	if !strings.Contains(initialCSSStr, "styles") {
		t.Errorf("should contain styles.css content")
	}
	if !strings.Contains(initialCSSStr, "theme") {
		t.Errorf("should contain theme.css content")
	}

	// Call RefreshAsset for CSS
	env.AssetsHandler.RefreshAsset(".css")

	// Verify CSS is still intact after refresh
	afterRefresh, err := os.ReadFile(env.MainCssPath)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}

	// Should be identical since nothing changed
	if string(initialCSS) != string(afterRefresh) {
		t.Errorf("CSS should remain unchanged when RefreshAsset is called without changes")
	}

	t.Log("✓ RefreshAsset works correctly for CSS files")
}

// TestRefreshAssettrue verifies that RefreshAsset works correctly in true.
func TestRefreshAssettrue(t *testing.T) {
	env := setupTestEnv("refresh_asset_disk_mode", t)
	defer env.CleanDirectory()

	// Set true
	env.AssetsHandler.SetBuildOnDisk(true)

	// Create a JS file
	filePath := filepath.Join(env.BaseDir, "modules", "test.js")
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		t.Fatalf("Error: %v", err)
	}
	if err := os.WriteFile(filePath, []byte("console.log('test');"), 0644); err != nil {
		t.Fatalf("Error: %v", err)
	}

	// Process the file (should write to disk)
	if err := env.AssetsHandler.NewFileEvent("test.js", ".js", filePath, "write"); err != nil {
		t.Fatalf("Error: %v", err)
	}

	// Call RefreshAsset
	env.AssetsHandler.RefreshAsset(".js")

	// Verify that the file was written to disk
	if _, err := os.Stat(env.MainJsPath); os.IsNotExist(err) {
		t.Fatalf("script.js should exist after RefreshAsset")
	}

	content, err := os.ReadFile(env.MainJsPath)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	if !strings.Contains(string(content), "test") {
		t.Errorf("script.js should contain the test file content")
	}

	t.Log("✓ RefreshAsset works correctly in true")
}

// TestRefreshAssetRebuildsInitCode verifies that RefreshAsset re-fetches
// the GetRuntimeInitializerJS content and includes it in the output
func TestRefreshAssetRebuildsInitCode(t *testing.T) {
	// Mock that returns different content on each call (simulating external state change)
	callCount := 0
	mockInitializer := func() (string, error) {
		callCount++
		return "\nconst initVersion" + string(rune('0'+callCount)) + "=true;", nil
	}

	env := setupTestEnv("refresh_asset_init_code", t, mockInitializer)
	env.AssetsHandler.SetBuildOnDisk(true)
	defer env.CleanDirectory()

	// Create a simple JS file
	filePath := filepath.Join(env.BaseDir, "modules", "app.js")
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		t.Fatalf("Error: %v", err)
	}
	if err := os.WriteFile(filePath, []byte("console.log('app');"), 0644); err != nil {
		t.Fatalf("Error: %v", err)
	}

	// Initial compilation
	if err := env.AssetsHandler.NewFileEvent("app.js", ".js", filePath, "write"); err != nil {
		t.Fatalf("Error: %v", err)
	}

	initialContent, err := os.ReadFile(env.MainJsPath)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	initialStr := string(initialContent)

	// Should contain version 2 of init code (version 1 was consumed by SetBuildOnDisk)
	if !strings.Contains(initialStr, "initVersion2") {
		t.Errorf("Initial compilation should include second version of init code, got:\n%s", initialStr)
	}
	if callCount != 2 {
		t.Errorf("Should have called mockInitializer twice, got %d", callCount)
	}

	// Call RefreshAsset - this should trigger another call to GetRuntimeInitializerJS
	env.AssetsHandler.RefreshAsset(".js")

	// Verify that GetRuntimeInitializerJS was called again
	if callCount != 3 {
		t.Errorf("RefreshAsset should have called mockInitializer again, got %d", callCount)
	}

	// Verify new content is included
	refreshedContent, err := os.ReadFile(env.MainJsPath)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	refreshedStr := string(refreshedContent)

	if !strings.Contains(refreshedStr, "initVersion3") {
		t.Errorf("After RefreshAsset, should include third version of init code")
	}
	if strings.Contains(refreshedStr, "initVersion2") {
		t.Errorf("After RefreshAsset, should NOT include previous version")
	}

	// Verify user content is still there
	if !strings.Contains(refreshedStr, "app") {
		t.Errorf("User JS content should be preserved")
	}

	t.Log("✓ RefreshAsset correctly re-fetches and includes GetRuntimeInitializerJS content")
}

// TestRefreshAssetMultipleFiles verifies that RefreshAsset works correctly
// when multiple JS files exist in the project
func TestRefreshAssetMultipleFiles(t *testing.T) {
	currentMode := "standard"
	mockHandler := func() (string, error) {
		if currentMode == "standard" {
			return "\nconst standardInit=true;\n", nil
		}
		return "\nconst enhancedInit=true;\n", nil
	}

	env := setupTestEnv("refresh_asset_multiple", t, mockHandler)
	env.AssetsHandler.SetBuildOnDisk(true)
	defer env.CleanDirectory()

	// Create multiple JS files in different locations
	files := []struct {
		path    string
		content string
	}{
		{filepath.Join(env.BaseDir, "modules", "auth", "login.js"), "console.log('login');"},
		{filepath.Join(env.BaseDir, "modules", "auth", "register.js"), "console.log('register');"},
		{filepath.Join(env.BaseDir, "modules", "ui", "menu.js"), "console.log('menu');"},
		{filepath.Join(env.BaseDir, "web", "theme", "base.js"), "console.log('base');"},
		{filepath.Join(env.BaseDir, "web", "theme", "layout.js"), "console.log('layout');"},
	}

	// Create and process all files
	for _, f := range files {
		if err := os.MkdirAll(filepath.Dir(f.path), 0755); err != nil {
			t.Fatalf("Error: %v", err)
		}
		if err := os.WriteFile(f.path, []byte(f.content), 0644); err != nil {
			t.Fatalf("Error: %v", err)
		}
		if err := env.AssetsHandler.NewFileEvent(filepath.Base(f.path), ".js", f.path, "write"); err != nil {
			t.Fatalf("Error: %v", err)
		}
	}

	// Verify initial state
	initialContent, err := os.ReadFile(env.MainJsPath)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	initialStr := string(initialContent)

	if !strings.Contains(initialStr, "standardInit") {
		t.Errorf("Should have standard init code")
	}
	for _, f := range files {
		keyword := strings.TrimSuffix(filepath.Base(f.path), ".js")
		if !strings.Contains(initialStr, keyword) {
			t.Errorf("Should contain content from %s", filepath.Base(f.path))
		}
	}

	// Change mode and refresh
	currentMode = "enhanced"
	env.AssetsHandler.RefreshAsset(".js")

	// Verify after refresh
	refreshedContent, err := os.ReadFile(env.MainJsPath)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	refreshedStr := string(refreshedContent)

	if !strings.Contains(refreshedStr, "enhancedInit") {
		t.Errorf("Should have new init code")
	}
	if strings.Contains(refreshedStr, "standardInit") {
		t.Errorf("Should not have old init code")
	}

	// All user files should still be present
	for _, f := range files {
		keyword := strings.TrimSuffix(filepath.Base(f.path), ".js")
		if !strings.Contains(refreshedStr, keyword) {
			t.Errorf("Should still contain content from %s", filepath.Base(f.path))
		}
	}

	t.Log("✓ RefreshAsset works correctly with multiple JS files")
}
