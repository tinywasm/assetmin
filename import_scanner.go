package assetmin

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type fileImportCache struct {
	mtime   time.Time
	imports map[string]bool
}

type importScanner struct {
	mu    sync.RWMutex
	cache map[string]fileImportCache
}

func newImportScanner() *importScanner {
	return &importScanner{
		cache: make(map[string]fileImportCache),
	}
}

func (s *importScanner) ScanProjectImports(rootDir string) (map[string]bool, error) {
	allImports := make(map[string]bool)

	// Files in rootDir
	err := s.scanDir(rootDir, allImports)
	if err != nil {
		return nil, err
	}

	// Files in one level of subdirectories
	entries, err := os.ReadDir(rootDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			// Skip hidden dirs (like .git)
			if strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			err := s.scanDir(filepath.Join(rootDir, entry.Name()), allImports)
			if err != nil {
				return nil, err
			}
		}
	}

	return allImports, nil
}

func (s *importScanner) scanDir(dir string, allImports map[string]bool) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") && !strings.HasSuffix(entry.Name(), "_test.go") {
			path := filepath.Join(dir, entry.Name())
			imports, err := s.scanFile(path)
			if err != nil {
				continue // Skip problematic files
			}
			for imp := range imports {
				allImports[imp] = true
			}
		}
	}
	return nil
}

func (s *importScanner) scanFile(path string) (map[string]bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	s.mu.RLock()
	cached, ok := s.cache[path]
	s.mu.RUnlock()

	if ok && cached.mtime.Equal(info.ModTime()) {
		return cached.imports, nil
	}

	// Parse file
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}

	imports := make(map[string]bool)
	for _, imp := range f.Imports {
		if imp.Path != nil {
			path := strings.Trim(imp.Path.Value, "\"")
			imports[path] = true
		}
	}

	s.mu.Lock()
	s.cache[path] = fileImportCache{
		mtime:   info.ModTime(),
		imports: imports,
	}
	s.mu.Unlock()

	return imports, nil
}

func moduleSubpackagesUsed(modulePath string, moduleDir string, importedPaths map[string]bool) []string {
	var usedSubpackages []string
	seen := make(map[string]bool)

	for imp := range importedPaths {
		if imp == modulePath {
			if !seen[""] {
				usedSubpackages = append(usedSubpackages, "")
				seen[""] = true
			}
			continue
		}

		if strings.HasPrefix(imp, modulePath+"/") {
			subPath := strings.TrimPrefix(imp, modulePath+"/")

			// Only support one level of subdirectories
			if !strings.Contains(subPath, "/") {
				if !seen[subPath] {
					usedSubpackages = append(usedSubpackages, subPath)
					seen[subPath] = true
				}
			}
		}
	}

	return usedSubpackages
}
