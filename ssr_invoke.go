package assetmin

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
)

// ssrCollectorOutput is the structure produced by the generated main.go
type ssrCollectorOutput struct {
	Root   string            `json:"root"`
	Render string            `json:"render"`
	HTML   string            `json:"html"`
	JS     string            `json:"js"`
	Icons  map[string]string `json:"icons"`
}

type moduleAlias struct {
	Path  string
	Alias string
}

// Global cache for SSR extraction results (keyed by hash set of all modules)
var (
	ssrExtractCache = make(map[string]map[string]ssrCollectorOutput)
	ssrCacheMu      sync.RWMutex
)

// invokeSSRExtractorOnce generates a combined main.go, runs it once, and returns the aggregated output.
// Results are cached by the hash of all module Go files.
func invokeSSRExtractorOnce(rootDir string, modules []Module) (map[string]ssrCollectorOutput, error) {
	tmpDir, err := os.MkdirTemp("", "assetmin-extract-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Generate main.go that imports all modules
	mainFile := filepath.Join(tmpDir, "main.go")
	if err := generateExtractorMain(mainFile, modules); err != nil {
		return nil, fmt.Errorf("failed to generate main.go: %w", err)
	}

	// Run go run main.go and capture JSON output
	cmd := exec.Command("go", "run", mainFile)
	cmd.Dir = rootDir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go run failed: %w", err)
	}

	// Parse the JSON output
	var results map[string]ssrCollectorOutput
	if err := json.Unmarshal(out, &results); err != nil {
		return nil, fmt.Errorf("failed to parse extractor output: %w", err)
	}

	return results, nil
}

// generateExtractorMain writes a main.go file that imports all modules and collects their assets.
func generateExtractorMain(outputFile string, modules []Module) error {
	tmpl := template.Must(template.New("extractor").Parse(`package main

import (
	"encoding/json"
	"os"
	{{range .Modules}}
	{{.Alias}} "{{.Path}}"
	{{end}}
)

type ssr struct {
	Root   string            ` + "`json:\"root\"`" + `
	Render string            ` + "`json:\"render\"`" + `
	HTML   string            ` + "`json:\"html\"`" + `
	JS     string            ` + "`json:\"js\"`" + `
	Icons  map[string]string ` + "`json:\"icons\"`" + `
}

func collect(inst interface {
	RenderCSS() interface{ String() string }
	RenderHTML() string
	RenderJS() string
	IconSvg() map[string]string
}) ssr {
	out := ssr{
		Render: inst.RenderCSS().String(),
		HTML:   inst.RenderHTML(),
		JS:     inst.RenderJS(),
		Icons:  inst.IconSvg(),
	}

	// Check if instance also provides RootCSS (optional interface)
	if rootProvider, ok := inst.(interface{ RootCSS() interface{ String() string } }); ok {
		out.Root = rootProvider.RootCSS().String()
	}

	return out
}

func main() {
	all := map[string]ssr{
		{{range .Modules}}"{{.Path}}": collect({{.Alias}}.SSRInstance()),
		{{end}}
	}
	json.NewEncoder(os.Stdout).Encode(all)
}
`))

	data := struct {
		Modules []moduleAlias
	}{
		Modules: modulesToAliases(modules),
	}

	f, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer f.Close()

	return tmpl.Execute(f, data)
}

// modulesToAliases converts module information to alias mappings.
func modulesToAliases(modules []Module) []moduleAlias {
	var aliases []moduleAlias
	for _, m := range modules {
		// Use the last component of the path as the alias, with underscores
		parts := strings.Split(m.Path, "/")
		alias := strings.ReplaceAll(parts[len(parts)-1], "-", "_")
		aliases = append(aliases, moduleAlias{
			Path:  m.Path,
			Alias: alias,
		})
	}
	return aliases
}
