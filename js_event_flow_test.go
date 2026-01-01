package assetmin

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestJSEventFlow verifies that when a project is already compiled and the watcher
// sends "create" events for existing files the output file remains unchanged.
// Then, when a new empty file is created the output should still remain unchanged.
// Finally, after writing content to the new file and sending a "write" event,
// the output should contain the previous files plus the new one.
// UPDATED: Now includes mock TinyWasm handler to verify WASM JS initialization is included.
func TestJSEventFlow(t *testing.T) {

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
		default:
			return "", nil
		}
	}

	// Setup test environment with mock TinyWasm handler
	env := setupTestEnv("js_event_flow", t, mockTinyWasmHandler)
	env.AssetsHandler.SetBuildOnDisk(true)
	//defer env.CleanDirectory()

	// Prepare three distinct JS files in different directories
	file1Path := filepath.Join(env.BaseDir, "modules", "module1", "script1.js")
	file2Path := filepath.Join(env.BaseDir, "extras", "module2", "script2.js")
	file3Path := filepath.Join(env.BaseDir, "web", "theme", "theme.js")

	if err := os.MkdirAll(filepath.Dir(file1Path), 0755); err != nil {
		t.Fatalf("Failed to create dir for script1: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(file2Path), 0755); err != nil {
		t.Fatalf("Failed to create dir for script2: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(file3Path), 0755); err != nil {
		t.Fatalf("Failed to create dir for theme: %v", err)
	}

	file1Content := "console.log('Module One');"
	file2Content := "console.log('Module Two');"
	file3Content := "console.log('Theme Code');"

	if err := os.WriteFile(file1Path, []byte(file1Content), 0644); err != nil {
		t.Fatalf("Failed to write script1: %v", err)
	}
	if err := os.WriteFile(file2Path, []byte(file2Content), 0644); err != nil {
		t.Fatalf("Failed to write script2: %v", err)
	}
	if err := os.WriteFile(file3Path, []byte(file3Content), 0644); err != nil {
		t.Fatalf("Failed to write theme: %v", err)
	}

	// Simulate initial compilation: send write events so main.js is produced
	if err := env.AssetsHandler.NewFileEvent("script1.js", ".js", file1Path, "write"); err != nil {
		t.Fatalf("Error processing script1 write event: %v", err)
	}
	if err := env.AssetsHandler.NewFileEvent("script2.js", ".js", file2Path, "write"); err != nil {
		t.Fatalf("Error processing script2 write event: %v", err)
	}
	if err := env.AssetsHandler.NewFileEvent("theme.js", ".js", file3Path, "write"); err != nil {
		t.Fatalf("Error processing theme write event: %v", err)
	}

	// Ensure main.js exists and capture its content
	if _, err := os.Stat(env.MainJsPath); os.IsNotExist(err) {
		t.Fatalf("main.js must exist after initial write events at %s", env.MainJsPath)
	}
	initialMain, err := os.ReadFile(env.MainJsPath)
	if err != nil {
		t.Fatalf("unable to read main.js after initial compilation: %v", err)
	}

	// Verify that mock WASM JS is included in main.js
	initialMainStr := string(initialMain)
	if !strings.Contains(initialMainStr, "goRuntime") {
		t.Errorf("main.js should contain mock Go WASM runtime initialization")
	}
	if !strings.Contains(initialMainStr, "WebAssembly.instantiateStreaming") {
		t.Errorf("main.js should contain WASM instantiation code")
	}
	t.Log("✓ Verified: main.js contains Go WASM initialization JavaScript")

	// 1) Send "create" events for the same three files (simulating watcher initial registration)
	t.Log("Phase 1: sending 'create' events for existing files — expect no change")
	if err := env.AssetsHandler.NewFileEvent("script1.js", ".js", file1Path, "create"); err != nil {
		t.Fatalf("Error processing script1 create event: %v", err)
	}
	if err := env.AssetsHandler.NewFileEvent("script2.js", ".js", file2Path, "create"); err != nil {
		t.Fatalf("Error processing script2 create event: %v", err)
	}
	if err := env.AssetsHandler.NewFileEvent("theme.js", ".js", file3Path, "create"); err != nil {
		t.Fatalf("Error processing theme create event: %v", err)
	}

	afterCreates, err := os.ReadFile(env.MainJsPath)
	if err != nil {
		t.Fatalf("unable to read main.js after create events: %v", err)
	}
	// main.js should remain exactly the same
	if !bytes.Equal(initialMain, afterCreates) {
		t.Errorf("main.js changed after duplicate 'create' events")
	}

	// 2) Create a new empty JS file and send a 'create' event — output should remain unchanged
	newFilePath := filepath.Join(env.BaseDir, "modules", "module3", "newfile.js")
	if err := os.MkdirAll(filepath.Dir(newFilePath), 0755); err != nil {
		t.Fatalf("Failed to create dir for newfile: %v", err)
	}
	if err := os.WriteFile(newFilePath, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to write empty newfile: %v", err)
	}
	if err := env.AssetsHandler.NewFileEvent("newfile.js", ".js", newFilePath, "create"); err != nil {
		t.Fatalf("Error processing newfile create event: %v", err)
	}

	afterEmptyCreate, err := os.ReadFile(env.MainJsPath)
	if err != nil {
		t.Fatalf("unable to read main.js after creating empty file: %v", err)
	}
	if !bytes.Equal(initialMain, afterEmptyCreate) {
		t.Errorf("main.js changed after creating an empty file with 'create' event")
	}

	// 3) Write content to the new file and send a 'write' event — expect previous content + new file
	addedContent := "console.log('New Module added');"
	if err := os.WriteFile(newFilePath, []byte(addedContent), 0644); err != nil {
		t.Fatalf("Failed to write to newfile: %v", err)
	}
	if err := env.AssetsHandler.NewFileEvent("newfile.js", ".js", newFilePath, "write"); err != nil {
		t.Fatalf("Error processing newfile write event: %v", err)
	}

	finalMain, err := os.ReadFile(env.MainJsPath)
	if err != nil {
		t.Fatalf("unable to read main.js after writing new file: %v", err)
	}
	finalStr := string(finalMain)

	if !strings.Contains(finalStr, "Module One") {
		t.Errorf("final main.js should contain content from module1/script1.js")
	}
	if !strings.Contains(finalStr, "Module Two") {
		t.Errorf("final main.js should contain content from module2/script2.js")
	}
	if !strings.Contains(finalStr, "Theme Code") {
		t.Errorf("final main.js should contain content from web/theme/theme.js")
	}
	if !strings.Contains(finalStr, "New Module added") {
		t.Errorf("final main.js should contain the newly written file content")
	}
	if !strings.Contains(finalStr, "goRuntime") {
		t.Errorf("final main.js should still contain Go WASM runtime")
	}

	// 3.5) Test WASM mode change: Change mock to TinyGo mode and trigger regeneration
	t.Log("Phase 3.5: testing WASM mode change from Go to TinyGo")
	currentMode = "tinygo" // Change mock mode

	// Trigger a JS file event to force regeneration
	dummyJsPath := filepath.Join(env.BaseDir, "modules", "dummy.js")
	if err := os.WriteFile(dummyJsPath, []byte("console.log('trigger');"), 0644); err != nil {
		t.Fatalf("Failed to write dummy JS: %v", err)
	}
	if err := env.AssetsHandler.NewFileEvent("dummy.js", ".js", dummyJsPath, "write"); err != nil {
		t.Fatalf("Error processing dummy write event: %v", err)
	}

	afterModeChange, err := os.ReadFile(env.MainJsPath)
	if err != nil {
		t.Fatalf("unable to read main.js after mode change: %v", err)
	}
	afterModeChangeStr := string(afterModeChange)

	if !strings.Contains(afterModeChangeStr, "tinyGoRuntime") {
		t.Errorf("main.js should contain TinyGo WASM runtime after mode change")
	}
	if strings.Contains(afterModeChangeStr, "goRuntime") {
		t.Errorf("main.js should NOT contain Go WASM runtime after mode change")
	}
	t.Log("✓ Verified: WASM JavaScript changes when mock mode changes")

	// 4) Test rename operation: rename one of the existing files
	t.Log("Phase 4: testing rename operation")
	renamedFilePath := filepath.Join(env.BaseDir, "modules", "module1", "script1-renamed.js")
	renamedContent := "console.log('Module One Renamed');"

	// First send rename event for original file (removes it)
	if err := env.AssetsHandler.NewFileEvent("script1.js", ".js", file1Path, "rename"); err != nil {
		t.Fatalf("Error processing script1 rename: %v", err)
	}

	// Create new file and send create event (adds it)
	if err := os.WriteFile(renamedFilePath, []byte(renamedContent), 0644); err != nil {
		t.Fatalf("Failed to write renamed JS: %v", err)
	}
	if err := env.AssetsHandler.NewFileEvent("script1-renamed.js", ".js", renamedFilePath, "create"); err != nil {
		t.Fatalf("Error processing renamed JS create event: %v", err)
	}

	afterRename, err := os.ReadFile(env.MainJsPath)
	if err != nil {
		t.Fatalf("unable to read main.js after rename operation: %v", err)
	}
	afterRenameStr := string(afterRename)

	if strings.Contains(afterRenameStr, "console.log('Module One');") {
		t.Errorf("main.js should NOT contain original script1 content after rename")
	}
	if !strings.Contains(afterRenameStr, "Module One Renamed") {
		t.Errorf("main.js should contain renamed file content")
	}
	if !strings.Contains(afterRenameStr, "Module Two") {
		t.Errorf("main.js should still contain script2 content")
	}
	if !strings.Contains(afterRenameStr, "Theme Code") {
		t.Errorf("main.js should still contain theme content")
	}
	if !strings.Contains(afterRenameStr, "New Module added") {
		t.Errorf("main.js should still contain the new module content")
	}
}
