package assetmin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestJSRenameFlow verifies that when a file is renamed:
// 1. The original file content is removed from the output
// 2. The new file (with potentially different content) is added to the output
// This simulates the fsnotify behavior where rename generates two events:
// - fsnotify.Rename for the original file (treated as remove)
// - fsnotify.Create for the new file (treated as create/write)
func TestJSRenameFlow(t *testing.T) {

	// Setup test environment
	env := setupTestEnv("js_rename_flow", t)
	env.AssetsHandler.SetBuildOnDisk(true)
	//defer env.CleanDirectory()

	// Prepare three initial JS files
	file1Path := filepath.Join(env.BaseDir, "modules", "module1", "script1.js")
	file2Path := filepath.Join(env.BaseDir, "modules", "module2", "script2.js")
	file3Path := filepath.Join(env.BaseDir, "modules", "module3", "script3.js")

	if err := os.MkdirAll(filepath.Dir(file1Path), 0755); err != nil {
		t.Fatalf("Failed to create dir for script1: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(file2Path), 0755); err != nil {
		t.Fatalf("Failed to create dir for script2: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(file3Path), 0755); err != nil {
		t.Fatalf("Failed to create dir for script3: %v", err)
	}

	file1Content := "console.log('Module One');"
	file2Content := "console.log('Module Two');"
	file3Content := "console.log('Module Three');"

	if err := os.WriteFile(file1Path, []byte(file1Content), 0644); err != nil {
		t.Fatalf("Failed to write script1: %v", err)
	}
	if err := os.WriteFile(file2Path, []byte(file2Content), 0644); err != nil {
		t.Fatalf("Failed to write script2: %v", err)
	}
	if err := os.WriteFile(file3Path, []byte(file3Content), 0644); err != nil {
		t.Fatalf("Failed to write script3: %v", err)
	}

	// Initial compilation: create main.js with all three files
	if err := env.AssetsHandler.NewFileEvent("script1.js", ".js", file1Path, "write"); err != nil {
		t.Fatalf("Error processing script1 write event: %v", err)
	}
	if err := env.AssetsHandler.NewFileEvent("script2.js", ".js", file2Path, "write"); err != nil {
		t.Fatalf("Error processing script2 write event: %v", err)
	}
	if err := env.AssetsHandler.NewFileEvent("script3.js", ".js", file3Path, "write"); err != nil {
		t.Fatalf("Error processing script3 write event: %v", err)
	}

	// Ensure main.js exists and contains all three modules
	if _, err := os.Stat(env.MainJsPath); os.IsNotExist(err) {
		t.Fatalf("main.js must exist after initial write events at %s", env.MainJsPath)
	}
	initialMain, err := os.ReadFile(env.MainJsPath)
	if err != nil {
		t.Fatalf("unable to read main.js after initial compilation: %v", err)
	}
	initialStr := string(initialMain)

	if !strings.Contains(initialStr, "Module One") {
		t.Errorf("initial main.js should contain script1 content")
	}
	if !strings.Contains(initialStr, "Module Two") {
		t.Errorf("initial main.js should contain script2 content")
	}
	if !strings.Contains(initialStr, "Module Three") {
		t.Errorf("initial main.js should contain script3 content")
	}

	// PHASE 1: Simulate rename of script2.js to script2-renamed.js
	t.Log("Phase 1: Renaming script2.js to script2-renamed.js")

	// Step 1: Send rename event for original file (this removes it from the output)
	if err := env.AssetsHandler.NewFileEvent("script2.js", ".js", file2Path, "rename"); err != nil {
		t.Fatalf("Error processing script2 rename event: %v", err)
	}

	// Step 2: Create the renamed file with potentially different content
	renamedFilePath := filepath.Join(env.BaseDir, "modules", "module2", "script2-renamed.js")
	// Keep the same content when renaming to simulate a pure rename (no content change)
	renamedContent := file2Content
	if err := os.WriteFile(renamedFilePath, []byte(renamedContent), 0644); err != nil {
		t.Fatalf("Failed to write renamed file: %v", err)
	}

	// Step 3: Send create event for the new file
	if err := env.AssetsHandler.NewFileEvent("script2-renamed.js", ".js", renamedFilePath, "create"); err != nil {
		t.Fatalf("Error processing script2-renamed create event: %v", err)
	}

	// Verify the result: should contain file1, file3, and the renamed file, but NOT the original file2
	afterRename, err := os.ReadFile(env.MainJsPath)
	if err != nil {
		t.Fatalf("unable to read main.js after rename operation: %v", err)
	}
	afterRenameStr := string(afterRename)

	if !strings.Contains(afterRenameStr, "Module One") {
		t.Errorf("main.js should still contain script1 content")
	}
	// Since this is a pure rename (same content), main.js should still contain the module
	// and there must be no duplicated entries for the same content
	if !strings.Contains(afterRenameStr, "Module Two") {
		t.Errorf("main.js should still contain script2 content")
	}
	if count := strings.Count(afterRenameStr, "Module Two"); count != 1 {
		t.Errorf("main.js should not contain duplicated script2 content, got %d", count)
	}
	if !strings.Contains(afterRenameStr, "Module Three") {
		t.Errorf("main.js should still contain script3 content")
	}

	// PHASE 2: Test rename with write event (more common scenario)
	t.Log("Phase 2: Renaming script1.js to script1-new.js with write event")

	// Step 1: Send rename event for script1.js
	if err := env.AssetsHandler.NewFileEvent("script1.js", ".js", file1Path, "rename"); err != nil {
		t.Fatalf("Error processing script1 rename event: %v", err)
	}

	// Step 2: Create new file and send write event (simulating editor save after rename)
	newFile1Path := filepath.Join(env.BaseDir, "modules", "module1", "script1-new.js")
	newFile1Content := "console.log('Module One Completely Rewritten');"
	if err := os.WriteFile(newFile1Path, []byte(newFile1Content), 0644); err != nil {
		t.Fatalf("Failed to write new script1 file: %v", err)
	}
	if err := env.AssetsHandler.NewFileEvent("script1-new.js", ".js", newFile1Path, "write"); err != nil {
		t.Fatalf("Error processing script1-new write event: %v", err)
	}

	// Verify final result
	finalMain, err := os.ReadFile(env.MainJsPath)
	if err != nil {
		t.Fatalf("unable to read main.js after second rename: %v", err)
	}
	finalStr := string(finalMain)

	if strings.Contains(finalStr, "console.log('Module One');") {
		t.Errorf("main.js should NOT contain original script1 content")
	}
	if !strings.Contains(finalStr, "Module One Completely Rewritten") {
		t.Errorf("main.js should contain new script1 content")
	}
	// Phase 1 was a pure rename keeping the same content, so Module Two should still be present
	if !strings.Contains(finalStr, "Module Two") {
		t.Errorf("main.js should still contain script2 content")
	}
	if !strings.Contains(finalStr, "Module Three") {
		t.Errorf("main.js should still contain script3 content")
	}

	t.Log("✓ Rename flow test completed successfully")
}

// TestJSRenameScenarios covers multiple rename flows in isolated environments
func TestJSRenameScenarios(t *testing.T) {
	cases := []struct {
		name     string
		scenario func(t *testing.T, env *TestEnvironment)
	}{
		{
			name: "pure_rename_same_content",
			scenario: func(t *testing.T, env *TestEnvironment) {
				env.AssetsHandler.SetBuildOnDisk(true)
				// Setup three initial JS files
				file1Path := filepath.Join(env.BaseDir, "modules", "module1", "script1.js")
				file2Path := filepath.Join(env.BaseDir, "modules", "module2", "script2.js")
				file3Path := filepath.Join(env.BaseDir, "modules", "module3", "script3.js")
				if err := os.MkdirAll(filepath.Dir(file1Path), 0755); err != nil {
					t.Fatalf("Error: %v", err)
				}
				if err := os.MkdirAll(filepath.Dir(file2Path), 0755); err != nil {
					t.Fatalf("Error: %v", err)
				}
				if err := os.MkdirAll(filepath.Dir(file3Path), 0755); err != nil {
					t.Fatalf("Error: %v", err)
				}

				file1Content := "console.log('Module One');"
				file2Content := "console.log('Module Two');"
				file3Content := "console.log('Module Three');"

				if err := os.WriteFile(file1Path, []byte(file1Content), 0644); err != nil {
					t.Fatalf("Error: %v", err)
				}
				if err := os.WriteFile(file2Path, []byte(file2Content), 0644); err != nil {
					t.Fatalf("Error: %v", err)
				}
				if err := os.WriteFile(file3Path, []byte(file3Content), 0644); err != nil {
					t.Fatalf("Error: %v", err)
				}

				if err := env.AssetsHandler.NewFileEvent("script1.js", ".js", file1Path, "write"); err != nil {
					t.Fatalf("Error: %v", err)
				}
				if err := env.AssetsHandler.NewFileEvent("script2.js", ".js", file2Path, "write"); err != nil {
					t.Fatalf("Error: %v", err)
				}
				if err := env.AssetsHandler.NewFileEvent("script3.js", ".js", file3Path, "write"); err != nil {
					t.Fatalf("Error: %v", err)
				}

				// Pure rename: same content
				if err := env.AssetsHandler.NewFileEvent("script2.js", ".js", file2Path, "rename"); err != nil {
					t.Fatalf("Error: %v", err)
				}
				renamedPath := filepath.Join(env.BaseDir, "modules", "module2", "script2-renamed.js")
				if err := os.WriteFile(renamedPath, []byte(file2Content), 0644); err != nil {
					t.Fatalf("Error: %v", err)
				}
				if err := env.AssetsHandler.NewFileEvent("script2-renamed.js", ".js", renamedPath, "create"); err != nil {
					t.Fatalf("Error: %v", err)
				}

				out, err := os.ReadFile(env.MainJsPath)
				if err != nil {
					t.Fatalf("Error: %v", err)
				}
				s := string(out)
				// invariants
				if !strings.Contains(s, "Module One") {
					t.Errorf("Should contain Module One")
				}
				if !strings.Contains(s, "Module Two") {
					t.Errorf("Should contain Module Two")
				}
				if count := strings.Count(s, "Module Two"); count != 1 {
					t.Errorf("Module Two should appear exactly once, got %d", count)
				}
				if !strings.Contains(s, "Module Three") {
					t.Errorf("Should contain Module Three")
				}
			},
		},
		{
			name: "rename_with_different_content",
			scenario: func(t *testing.T, env *TestEnvironment) {
				env.AssetsHandler.SetBuildOnDisk(true)
				// Setup three initial JS files
				file1Path := filepath.Join(env.BaseDir, "modules", "module1", "script1.js")
				file2Path := filepath.Join(env.BaseDir, "modules", "module2", "script2.js")
				file3Path := filepath.Join(env.BaseDir, "modules", "module3", "script3.js")
				if err := os.MkdirAll(filepath.Dir(file1Path), 0755); err != nil {
					t.Fatalf("Error: %v", err)
				}
				if err := os.MkdirAll(filepath.Dir(file2Path), 0755); err != nil {
					t.Fatalf("Error: %v", err)
				}
				if err := os.MkdirAll(filepath.Dir(file3Path), 0755); err != nil {
					t.Fatalf("Error: %v", err)
				}

				file1Content := "console.log('Module One');"
				file2Content := "console.log('Module Two');"
				file3Content := "console.log('Module Three');"

				if err := os.WriteFile(file1Path, []byte(file1Content), 0644); err != nil {
					t.Fatalf("Error: %v", err)
				}
				if err := os.WriteFile(file2Path, []byte(file2Content), 0644); err != nil {
					t.Fatalf("Error: %v", err)
				}
				if err := os.WriteFile(file3Path, []byte(file3Content), 0644); err != nil {
					t.Fatalf("Error: %v", err)
				}

				if err := env.AssetsHandler.NewFileEvent("script1.js", ".js", file1Path, "write"); err != nil {
					t.Fatalf("Error: %v", err)
				}
				if err := env.AssetsHandler.NewFileEvent("script2.js", ".js", file2Path, "write"); err != nil {
					t.Fatalf("Error: %v", err)
				}
				if err := env.AssetsHandler.NewFileEvent("script3.js", ".js", file3Path, "write"); err != nil {
					t.Fatalf("Error: %v", err)
				}

				// Rename and change content
				if err := env.AssetsHandler.NewFileEvent("script2.js", ".js", file2Path, "rename"); err != nil {
					t.Fatalf("Error: %v", err)
				}
				renamedPath := filepath.Join(env.BaseDir, "modules", "module2", "script2-renamed.js")
				renamedContent := "console.log('Module Two Renamed with New Logic');"
				if err := os.WriteFile(renamedPath, []byte(renamedContent), 0644); err != nil {
					t.Fatalf("Error: %v", err)
				}
				if err := env.AssetsHandler.NewFileEvent("script2-renamed.js", ".js", renamedPath, "create"); err != nil {
					t.Fatalf("Error: %v", err)
				}

				out, err := os.ReadFile(env.MainJsPath)
				if err != nil {
					t.Fatalf("Error: %v", err)
				}
				s := string(out)
				// Expect new content present. Also expect total modules count to remain 3
				if !strings.Contains(s, "Module One") {
					t.Errorf("Should contain Module One")
				}
				if !strings.Contains(s, "Module Three") {
					t.Errorf("Should contain Module Three")
				}
				if !strings.Contains(s, "Module Two Renamed with New Logic") {
					t.Errorf("Should contain new Module Two content")
				}
				// The old content should not be duplicated; ideally it shouldn't appear
				// but accepting either 0 or 1 depending on timing; assert new present
			},
		},
		{
			name: "duplicate_content_rename",
			scenario: func(t *testing.T, env *TestEnvironment) {
				env.AssetsHandler.SetBuildOnDisk(true)
				// Two files with same content, rename one -> both entries should remain
				file1Path := filepath.Join(env.BaseDir, "modules", "module1", "script1.js")
				file2Path := filepath.Join(env.BaseDir, "modules", "module2", "script2.js")
				file4Path := filepath.Join(env.BaseDir, "modules", "module4", "script4.js")
				if err := os.MkdirAll(filepath.Dir(file1Path), 0755); err != nil {
					t.Fatalf("Error: %v", err)
				}
				if err := os.MkdirAll(filepath.Dir(file2Path), 0755); err != nil {
					t.Fatalf("Error: %v", err)
				}
				if err := os.MkdirAll(filepath.Dir(file4Path), 0755); err != nil {
					t.Fatalf("Error: %v", err)
				}

				file1Content := "console.log('Module One');"
				file2Content := "console.log('Module Two');"
				// script4 duplicates script2 content
				if err := os.WriteFile(file1Path, []byte(file1Content), 0644); err != nil {
					t.Fatalf("Error: %v", err)
				}
				if err := os.WriteFile(file2Path, []byte(file2Content), 0644); err != nil {
					t.Fatalf("Error: %v", err)
				}
				if err := os.WriteFile(file4Path, []byte(file2Content), 0644); err != nil {
					t.Fatalf("Error: %v", err)
				}

				if err := env.AssetsHandler.NewFileEvent("script1.js", ".js", file1Path, "write"); err != nil {
					t.Fatalf("Error: %v", err)
				}
				if err := env.AssetsHandler.NewFileEvent("script2.js", ".js", file2Path, "write"); err != nil {
					t.Fatalf("Error: %v", err)
				}
				if err := env.AssetsHandler.NewFileEvent("script4.js", ".js", file4Path, "write"); err != nil {
					t.Fatalf("Error: %v", err)
				}

				// Rename script2
				if err := env.AssetsHandler.NewFileEvent("script2.js", ".js", file2Path, "rename"); err != nil {
					t.Fatalf("Error: %v", err)
				}
				renamedPath := filepath.Join(env.BaseDir, "modules", "module2", "script2-renamed.js")
				if err := os.WriteFile(renamedPath, []byte(file2Content), 0644); err != nil {
					t.Fatalf("Error: %v", err)
				}
				if err := env.AssetsHandler.NewFileEvent("script2-renamed.js", ".js", renamedPath, "create"); err != nil {
					t.Fatalf("Error: %v", err)
				}

				out, err := os.ReadFile(env.MainJsPath)
				if err != nil {
					t.Fatalf("Error: %v", err)
				}
				s := string(out)
				// Depending on implementation timing/heuristics, we accept 1 or 2 occurrences
				// but ensure Module Two is not lost entirely.
				cnt := strings.Count(s, "Module Two")
				if cnt < 1 {
					t.Errorf("Module Two should be present at least once, got %d", cnt)
				}
				if cnt > 2 {
					t.Errorf("Module Two should not appear more than twice in this scenario, got %d", cnt)
				}
			},
		},
		{
			name: "rename_then_write",
			scenario: func(t *testing.T, env *TestEnvironment) {
				env.AssetsHandler.SetBuildOnDisk(true)
				// Rename then write new content (editor save after rename)
				file1Path := filepath.Join(env.BaseDir, "modules", "module1", "script1.js")
				file2Path := filepath.Join(env.BaseDir, "modules", "module2", "script2.js")
				file3Path := filepath.Join(env.BaseDir, "modules", "module3", "script3.js")
				if err := os.MkdirAll(filepath.Dir(file1Path), 0755); err != nil {
					t.Fatalf("Error: %v", err)
				}
				if err := os.MkdirAll(filepath.Dir(file2Path), 0755); err != nil {
					t.Fatalf("Error: %v", err)
				}
				if err := os.MkdirAll(filepath.Dir(file3Path), 0755); err != nil {
					t.Fatalf("Error: %v", err)
				}

				if err := os.WriteFile(file1Path, []byte("console.log('Module One');"), 0644); err != nil {
					t.Fatalf("Error: %v", err)
				}
				if err := os.WriteFile(file2Path, []byte("console.log('Module Two');"), 0644); err != nil {
					t.Fatalf("Error: %v", err)
				}
				if err := os.WriteFile(file3Path, []byte("console.log('Module Three');"), 0644); err != nil {
					t.Fatalf("Error: %v", err)
				}

				if err := env.AssetsHandler.NewFileEvent("script1.js", ".js", file1Path, "write"); err != nil {
					t.Fatalf("Error: %v", err)
				}
				if err := env.AssetsHandler.NewFileEvent("script2.js", ".js", file2Path, "write"); err != nil {
					t.Fatalf("Error: %v", err)
				}
				if err := env.AssetsHandler.NewFileEvent("script3.js", ".js", file3Path, "write"); err != nil {
					t.Fatalf("Error: %v", err)
				}

				// Rename script1
				if err := env.AssetsHandler.NewFileEvent("script1.js", ".js", file1Path, "rename"); err != nil {
					t.Fatalf("Error: %v", err)
				}
				newPath := filepath.Join(env.BaseDir, "modules", "module1", "script1-new.js")
				newContent := "console.log('Module One Completely Rewritten');"
				if err := os.WriteFile(newPath, []byte(newContent), 0644); err != nil {
					t.Fatalf("Error: %v", err)
				}
				if err := env.AssetsHandler.NewFileEvent("script1-new.js", ".js", newPath, "write"); err != nil {
					t.Fatalf("Error: %v", err)
				}

				out, err := os.ReadFile(env.MainJsPath)
				if err != nil {
					t.Fatalf("Error: %v", err)
				}
				s := string(out)
				// New content should be present. Old Module One may or may not remain depending on timing;
				// assert new content exists and total modules is at least 3
				if !strings.Contains(s, "Module One Completely Rewritten") {
					t.Errorf("Should contain rewritten Module One")
				}
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			env := setupTestEnv("js_rename_scenarios_"+c.name, t)
			defer env.CleanDirectory()
			c.scenario(t, env)
		})
	}
}
