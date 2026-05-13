package assetmin

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type SSRAssets struct {
	ModuleName string
	RootCSS    string
	CSS        string
	JS         string
	HTML       string
	Icons      map[string]string
}

// ExtractSSRAssets uses compile-and-invoke to extract assets from a module.
// The module must be a proper Go module with go.mod and ssr.go files,
// and should declare a SSRInstance() function that returns an instance implementing SSR interfaces.
func ExtractSSRAssets(moduleDir string) (*SSRAssets, error) {
	// Verify module has go.mod and ssr.go
	if _, err := os.Stat(filepath.Join(moduleDir, "go.mod")); err != nil {
		return nil, fmt.Errorf("no go.mod found: %w", err)
	}
	if _, err := os.Stat(filepath.Join(moduleDir, "ssr.go")); err != nil {
		return nil, fmt.Errorf("ssr.go not found in %s", moduleDir)
	}

	// Determine the project root by looking for the nearest go.mod above moduleDir
	rootDir, err := findProjectRoot(moduleDir)
	if err != nil {
		return nil, fmt.Errorf("failed to find project root: %w", err)
	}

	// Discover all modules in the project
	modules, err := discoverModules(rootDir)
	if err != nil {
		// If discovery fails (e.g., in test environments), use just the current module
		modules = []Module{{Path: filepath.Base(moduleDir), Dir: moduleDir}}
	}

	// Get the module path for the requested directory
	var targetModulePath string
	for _, m := range modules {
		if m.Dir == moduleDir {
			targetModulePath = m.Path
			break
		}
	}

	if targetModulePath == "" {
		// If module not found, use base name as path
		targetModulePath = filepath.Base(moduleDir)
	}

	// Check cache
	ssrCacheMu.RLock()
	cachedResults, hasCached := ssrExtractCache[rootDir]
	ssrCacheMu.RUnlock()

	if !hasCached {
		// Do compile-and-invoke
		results, err := invokeSSRExtractorOnce(rootDir, modules)
		if err != nil {
			return nil, err
		}

		// Cache the results
		ssrCacheMu.Lock()
		ssrExtractCache[rootDir] = results
		ssrCacheMu.Unlock()

		cachedResults = results
	}

	// Extract the SSRAssets for the requested module
	output, ok := cachedResults[targetModulePath]
	if !ok {
		return &SSRAssets{
			ModuleName: filepath.Base(moduleDir),
			Icons:      make(map[string]string),
		}, nil
	}

	return &SSRAssets{
		ModuleName: targetModulePath,
		RootCSS:    output.Root,
		CSS:        output.Render,
		JS:         output.JS,
		HTML:       output.HTML,
		Icons:      output.Icons,
	}, nil
}

// findProjectRoot finds the project root by locating the nearest go.mod file above or at startDir.
func findProjectRoot(startDir string) (string, error) {
	dir := startDir
	for {
		gomodPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(gomodPath); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod found in %s or parent directories", startDir)
		}
		dir = parent
	}
}

// discoverModules discovers all modules in the project using go list.
func discoverModules(rootDir string) ([]Module, error) {
	cmd := exec.Command("go", "list", "-m", "-json", "all")
	cmd.Dir = rootDir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list failed: %w", err)
	}

	var modules []Module
	dec := json.NewDecoder(strings.NewReader(string(out)))
	for dec.More() {
		var m Module
		if err := dec.Decode(&m); err == nil && m.Dir != "" {
			modules = append(modules, m)
		}
	}

	return modules, nil
}
