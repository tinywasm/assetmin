package assetmin

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/tinywasm/fmt"
	"github.com/tinywasm/js"
)

var ssrSourceFiles = []string{"css.go", "js.go", "svg.go", "html.go", "ssr.go"}

type SSRAssets struct {
	ModuleName string
	RootCSS    string
	CSS        string
	JS         []*js.Script
	HTML       string
	Icons      map[string]string
}

// ExtractSSRAssets uses compile-and-invoke to extract assets from a module.
// moduleDir may be a sub-package; the project root (which contains go.mod) is found by traversing up.
func ExtractSSRAssets(moduleDir string) (*SSRAssets, error) {
	// Determine the project root by looking for the nearest go.mod above or at moduleDir.
	// Sub-packages don't have their own go.mod; they share the root's.
	rootDir, err := findProjectRoot(moduleDir)
	if err != nil {
		return nil, fmt.Err("failed to find project root from", moduleDir, err)
	}

	// One of ssrSourceFiles must exist in moduleDir (the sub-package), not at the root.
	foundSSR := false
	for _, f := range ssrSourceFiles {
		if _, err := os.Stat(filepath.Join(moduleDir, f)); err == nil {
			foundSSR = true
			break
		}
	}
	if !foundSSR {
		return nil, fmt.Err("no SSR source files found in", moduleDir)
	}

	// Discover all modules in the project
	modules, err := discoverModules(rootDir)
	if err != nil {
		// If discovery fails (e.g., in test environments), use just the current module
		modules = []Module{{Path: filepath.Base(moduleDir), Dir: moduleDir}}
	}

	// Find the module object for the requested directory
	var targetModule Module
	found := false
	for _, m := range modules {
		if m.Dir == moduleDir {
			targetModule = m
			found = true
			break
		}
	}

	if !found {
		targetModule = Module{Path: filepath.Base(moduleDir), Dir: moduleDir}
	}

	return extractSSRAssetsForModule(targetModule, rootDir, modules, "")
}

// extractSSRAssetsForModule is the internal implementation that takes a resolved Module.
// binCachePath is reserved for future optimization (Problem 7).
func extractSSRAssetsForModule(m Module, rootDir string, allModules []Module, binCachePath string) (*SSRAssets, error) {
	// Ensure m is in the extractor's module set, so the generated main.go
	// imports it and the results map carries an entry for m.Path.
	modulesForExtract := allModules
	if !containsModule(allModules, m) {
		modulesForExtract = append(append([]Module(nil), allModules...), m)
	}

	// Compute hash of all modules to check global cache
	hashKey, err := computeModuleHashSet(modulesForExtract)
	if err != nil {
		return nil, fmt.Err("failed to compute module hash", err)
	}

	// Check cache
	ssrExtractMu.Lock()
	cachedResults, hasCached := ssrGlobalCache.get(hashKey)
	if !hasCached {
		// Do compile-and-invoke
		results, err := invokeSSRExtractorOnce(rootDir, modulesForExtract)
		if err != nil {
			ssrExtractMu.Unlock()
			return nil, err
		}

		// Cache the results
		ssrGlobalCache.set(hashKey, results)
		cachedResults = results
	}
	ssrExtractMu.Unlock()

	// Extract the SSRAssets for the requested module
	output, ok := cachedResults[m.Path]
	if !ok {
		return &SSRAssets{
			ModuleName: filepath.Base(m.Dir),
			Icons:      make(map[string]string),
		}, nil
	}

	scripts := make([]*js.Script, 0, len(output.Scripts))
	for _, s := range output.Scripts {
		scripts = append(scripts, &js.Script{
			Name:    s.Name,
			Content: s.Content,
		})
	}

	return &SSRAssets{
		ModuleName: m.Path,
		RootCSS:    output.Root,
		CSS:        output.Render,
		JS:         scripts,
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
			return "", fmt.Err("no go.mod found in", startDir, "or parent directories")
		}
		dir = parent
	}
}

func containsModule(mods []Module, m Module) bool {
	for _, x := range mods {
		if x.Path == m.Path && x.Dir == m.Dir {
			return true
		}
	}
	return false
}

// discoverModules discovers all modules in the project using go list.
func discoverModules(rootDir string) ([]Module, error) {
	cmd := exec.Command("go", "list", "-m", "-json", "all")
	cmd.Dir = rootDir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Err("go list failed", err)
	}

	var modules []Module
	dec := json.NewDecoder(bytes.NewReader(out))
	for dec.More() {
		var m Module
		if err := dec.Decode(&m); err == nil && m.Dir != "" {
			modules = append(modules, m)
		}
	}

	return modules, nil
}
