# Stage 2 — Selective SSR Load (Import-Based)

**Goal:** `LoadSSRModules` must load `ssr.go` only from modules and sub-packages that are explicitly imported by the project's production Go files. Unused components in a shared library (e.g., `tinywasm/components`) must not contribute any CSS, JS, or SVG to the bundle.

**Blocks:** Stage 3 cannot start until all tests here pass.  
**Requires:** Stage 1 completed.

---

## Problem

Current `ssr_loader.go` calls `go list -m -json all` and loads `ssr.go` from **every** module directory found. If a dependency like `tinywasm/components` contains 20 web components, all 20 contribute their CSS/JS even if the project imports only 1.

This causes:
- Bundle bloat with unused styles and scripts.
- Slow initial load for projects with large component libraries.

---

## Design

### Import Scanner (`assetmin/import_scanner.go`)

New internal package-level type `importScanner` responsible for:

1. Walking all `.go` files in `rootDir` (non-recursive beyond 1 level — see scope below).
2. Skipping `_test.go` files.
3. Parsing each file with `go/parser` (AST, imports only — use `parser.ImportsOnly` mode for speed).
4. Collecting all import paths including blank imports (`_ "pkg/path"`).
5. Caching results per file using `mtime` — only re-parses files that changed.

**Cache structure:**
```go
type fileImportCache struct {
    mtime   time.Time
    imports map[string]bool // import paths found in this file
}

type importScanner struct {
    mu    sync.RWMutex
    cache map[string]fileImportCache // key: absolute file path
}
```

**Public method:**
```go
// ScanProjectImports returns the set of all import paths used by
// non-test .go files in rootDir. Results are cached by file mtime.
func (s *importScanner) ScanProjectImports(rootDir string) (map[string]bool, error)
```

**File walk scope:**
- Root `.go` files: `rootDir/*.go`
- One level of subdirectories: `rootDir/*/*.go`
- Third level and deeper: **excluded**
- `_test.go` suffix: **excluded**

### Module Matcher

Internal helper that cross-references the import set against the module list from `go list`:

```go
// moduleUsed returns true if the given module (identified by its path prefix)
// has at least one sub-package present in the importedPaths set.
// It also returns the list of sub-package paths within the module that are imported.
func moduleSubpackagesUsed(modulePath string, moduleDir string, importedPaths map[string]bool) []string
```

Logic:
- For each `importedPath` in the set, check if it starts with `modulePath`.
- If yes, extract `subPath = importedPath[len(modulePath)+1:]` (e.g., `"button"` from `"github.com/user/components/button"`).
- If `subPath == ""`, the module root is imported directly → load `moduleDir/ssr.go`.
- If `subPath` has no `/` (one level), check if `moduleDir/subPath/ssr.go` exists → load it.
- If `subPath` has a `/` (two or more levels deep), **skip** — only one level of subdirectories is supported.

### Modified `ssr_loader.go` — `LoadSSRModules`

New flow:

```
1. Call go list -m -json all → []Module{Path, Dir}
2. Build modulePath→dir map
3. Call importScanner.ScanProjectImports(rootDir) → importedPaths set
4. For each module in the list:
   a. Find subpackages of this module that appear in importedPaths
      (use moduleSubpackagesUsed)
   b. For each matched subDir (or root if module root imported):
      - Call ExtractSSRAssets(subDir)
      - Determine slot (open/middle/close) based on module path
      - Call UpdateSSRModuleInSlot
5. Root project (isRootDir) always loads — it is the "close" slot, always included.
6. tinywasm/dom always loads — it is the "open" slot, always included.
```

**Always-load exceptions** (bypass import check):
- Module path contains `tinywasm/dom` → always load (open slot, theme).
- Module dir == `rootDir` → always load (close slot, project-owned assets).

### Cache Invalidation for Replace Modules

The `importScanner` cache uses mtime per file. This means:
- If a `.go` file in the project adds a new import, its mtime changes → scanner re-parses that file only.
- Other files use cached results.
- The full `ScanProjectImports` result is the union of all per-file results.
- No global invalidation needed — it is automatic at file granularity.

For non-replace modules (from `$GOPATH/pkg/mod`): their source does not change at runtime, so no cache invalidation is needed. Their dirs are resolved once from `go list` output.

---

## Files to Create / Modify

| File | Action |
|------|--------|
| `assetmin/import_scanner.go` | **Create** — `importScanner` struct, `ScanProjectImports`, `moduleSubpackagesUsed` |
| `assetmin/ssr_loader.go` | **Modify** — add scanner step between `go list` and `ExtractSSRAssets` loop |

---

## Tests — `assetmin/tests/selective_load_test.go`

All tests use temporary directories to simulate project structures. Use `t.TempDir()`.

### `TestImportScanner_SingleFile`
One `.go` file importing two packages.

```
given: rootDir/main.go imports ["github.com/a/pkg", "github.com/b/other"]
when:  ScanProjectImports(rootDir)
then:  result contains both paths, len == 2
```

### `TestImportScanner_ExcludesTestFiles`
A `_test.go` file imports a package not in production files.

```
given: rootDir/main.go imports ["github.com/a/prod"]
       rootDir/main_test.go imports ["github.com/a/testonly"]
when:  ScanProjectImports(rootDir)
then:  result contains "github.com/a/prod"
       result does NOT contain "github.com/a/testonly"
```

### `TestImportScanner_BlankImportCounted`
Blank import (`_ "pkg"`) must be included.

```
given: rootDir/main.go has: import _ "github.com/user/components/button"
when:  ScanProjectImports(rootDir)
then:  result contains "github.com/user/components/button"
```

### `TestImportScanner_OneLevelSubdirs`
Files in one-level subdirectory are scanned.

```
given: rootDir/handlers/routes.go imports ["github.com/user/components/modal"]
       rootDir/main.go imports nothing relevant
when:  ScanProjectImports(rootDir)
then:  result contains "github.com/user/components/modal"
```

### `TestImportScanner_ThirdLevelExcluded`
Files deeper than one subdirectory level are not scanned.

```
given: rootDir/a/b/deep.go imports ["github.com/user/deeponly"]
       no other file imports "github.com/user/deeponly"
when:  ScanProjectImports(rootDir)
then:  result does NOT contain "github.com/user/deeponly"
```

### `TestImportScanner_MtimeCache`
Second call does not re-parse unchanged files (verify via call count or mtime check).

```
given: rootDir/main.go (scanned once, cached)
when:  ScanProjectImports called a second time without file change
then:  file is NOT re-parsed (instrument with a parse counter or check mtime unchanged)
```

### `TestModuleSubpackagesUsed_RootImport`
Module root is imported directly.

```
given: modulePath="github.com/user/components", moduleDir="/tmp/components"
       importedPaths={"github.com/user/components"}
when:  moduleSubpackagesUsed(...)
then:  returns [""] (root)
```

### `TestModuleSubpackagesUsed_SubpackageMatch`
Only imported subpackages are returned.

```
given: modulePath="github.com/user/components"
       moduleDir has: button/ssr.go, modal/ssr.go, badge/ssr.go
       importedPaths={"github.com/user/components/button", "github.com/user/components/modal"}
when:  moduleSubpackagesUsed(...)
then:  returns ["button", "modal"]  — "badge" not included
```

### `TestModuleSubpackagesUsed_DeepSubpackageSkipped`
Sub-packages more than one level deep are skipped.

```
given: importedPaths={"github.com/user/components/a/b"}
when:  moduleSubpackagesUsed(...)
then:  returns [] (empty — "a/b" has a slash, more than one level)
```

### `TestLoadSSRModules_SelectiveLoad`
Integration test: simulate a project that imports only one of three components.

```
given: fake module "github.com/user/components" with dirs: button/, modal/, badge/
       each subdir has a ssr.go with unique CSS
       project's main.go imports only "github.com/user/components/button"
       listModulesFn returns [moduleDir] (fake go list)
when:  LoadSSRModules()
then:  bundle CSS contains button's CSS
       bundle CSS does NOT contain modal's CSS
       bundle CSS does NOT contain badge's CSS
```

### `TestLoadSSRModules_AlwaysLoadsDomAndRoot`
`tinywasm/dom` and root project always load regardless of import scan.

```
given: import scan returns empty set (no imports found)
       module list includes tinywasm/dom and rootDir
when:  LoadSSRModules()
then:  dom assets loaded (open slot)
       root project assets loaded (close slot)
```

---

## Acceptance Criteria

- [ ] All tests in `selective_load_test.go` pass
- [ ] `LoadSSRModules` does not load assets from un-imported sub-packages
- [ ] `tinywasm/dom` and root project always load (unconditional)
- [ ] Third-level subdirs never scanned for imports or ssr.go
- [ ] `_test.go` files excluded from import scan
- [ ] Blank imports (`_`) included in import set
- [ ] Import scan result cached per-file by mtime
