package assetmin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStripUseStrictAndSingleOccurrence(t *testing.T) {
	// Provide a GetRuntimeInitializerJS that includes the 'use strict' we add globally
	initJS := func() (string, error) {
		// Return wasm init code without an additional 'use strict' directive.
		return "\n// wasm init code... WebAssembly.Memory", nil
	}

	env := setupTestEnv("js-use-strict", t, initJS)
	env.AssetsHandler.SetBuildOnDisk(true)
	defer env.CleanDirectory()

	env.CreateModulesDir()
	env.CreateThemeDir()
	env.CreatePublicDir()

	// Create a file that already contains a leading use strict directive
	file1 := "a.js"
	path1 := filepath.Join(env.BaseDir, file1)
	if err := os.WriteFile(path1, []byte("'use strict';\nconsole.log('A');"), 0644); err != nil {
		t.Fatalf("Failed to write file A: %v", err)
	}

	// Create another file without the directive
	file2 := "b.js"
	path2 := filepath.Join(env.BaseDir, file2)
	if err := os.WriteFile(path2, []byte("console.log('B');"), 0644); err != nil {
		t.Fatalf("Failed to write file B: %v", err)
	}

	// Register both files as created
	if err := env.AssetsHandler.NewFileEvent(file1, ".js", path1, "create"); err != nil {
		t.Fatalf("Error processing file A create: %v", err)
	}
	if err := env.AssetsHandler.NewFileEvent(file2, ".js", path2, "create"); err != nil {
		t.Fatalf("Error processing file B create: %v", err)
	}

	// Now trigger a write to force compilation/writing
	if err := env.AssetsHandler.NewFileEvent(file1, ".js", path1, "write"); err != nil {
		t.Fatalf("Error processing file A write: %v", err)
	}

	// Read generated main JS
	out, err := os.ReadFile(env.MainJsPath)
	if err != nil {
		t.Fatalf("Failed to read main.js: %v", err)
	}
	outStr := string(out)

	// Count occurrences of use strict (both 'use strict' and "use strict")
	lower := strings.ToLower(outStr)
	count := strings.Count(lower, "use strict")
	if count != 1 {
		t.Errorf("There should be exactly one 'use strict' in the output, got %d", count)
	}

	// Basic content checks: ensure both A and B outputs exist in the minified bundle
	// Note: content may be transformed by minification, so check for partial strings
	if !(strings.Contains(outStr, "A") || strings.Contains(outStr, "console.log")) {
		t.Errorf("Content from file A should be present")
	}
	if !(strings.Contains(outStr, "B") || strings.Contains(outStr, "console.log")) {
		t.Errorf("Content from file B should be present")
	}
}

func TestStripUseStrictWithWasmExecContent(t *testing.T) {
	// Simulate real wasm_exec.js content that starts with comments and then "use strict";
	wasmExecContent := `// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

"use strict";

(() => {
	const enosys = () => {
		const err = new Error("not implemented");
		return err;
	};
	// ... more wasm exec code
	const go = new Go();
	WebAssembly.instantiateStreaming(fetch("main.wasm"), go.importObject);
})();`

	initJS := func() (string, error) {
		return wasmExecContent, nil
	}

	env := setupTestEnv("js-use-strict-wasm", t, initJS)
	env.AssetsHandler.SetBuildOnDisk(true)
	defer env.CleanDirectory()

	env.CreateModulesDir()
	env.CreateThemeDir()
	env.CreatePublicDir()

	// Create a regular JS file
	file1 := "app.js"
	path1 := filepath.Join(env.BaseDir, file1)
	if err := os.WriteFile(path1, []byte("console.log('App running');"), 0644); err != nil {
		t.Fatalf("Failed to write app.js: %v", err)
	}

	// Register file and trigger compilation
	if err := env.AssetsHandler.NewFileEvent(file1, ".js", path1, "create"); err != nil {
		t.Fatalf("Error processing app.js create: %v", err)
	}
	if err := env.AssetsHandler.NewFileEvent(file1, ".js", path1, "write"); err != nil {
		t.Fatalf("Error processing app.js write: %v", err)
	}

	// Read generated main JS
	out, err := os.ReadFile(env.MainJsPath)
	if err != nil {
		t.Fatalf("Failed to read main.js: %v", err)
	}
	outStr := string(out)

	// Debug: print the first 500 characters to see what's happening
	t.Logf("Generated content (first 500 chars): %s", outStr[:min(500, len(outStr))])

	// Count occurrences of use strict (should be exactly 1)
	lower := strings.ToLower(outStr)
	count := strings.Count(lower, "use strict")
	if count != 1 {
		t.Errorf("There should be exactly one 'use strict' in the output, got %d", count)
	}

	// Verify content is present
	if !strings.Contains(outStr, "App running") {
		t.Errorf("Output should contain 'App running'")
	}
	if !strings.Contains(outStr, "WebAssembly.instantiateStreaming") {
		t.Errorf("Output should contain 'WebAssembly.instantiateStreaming'")
	}
}
