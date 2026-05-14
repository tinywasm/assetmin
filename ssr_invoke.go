package assetmin

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sync"
	"text/template"

	"github.com/tinywasm/fmt"
)

// ssrCollectorOutput is the structure produced by the generated main.go
type ssrCollectorOutput struct {
	Root   string            `json:"root"`
	Render string            `json:"render"`
	HTML   string            `json:"html"`
	JS     string            `json:"js"`
	Icons  map[string]string `json:"icons"`
}

type ModuleAlias struct {
	Path        string
	Alias       string
	HasInstance bool
	HasRoot     bool
	HasRender   bool
	HasHTML     bool
	HasJS       bool
	HasIcons    bool
}

func (m ModuleAlias) HasAnyFeature() bool {
	return m.HasInstance || m.HasRoot || m.HasRender || m.HasHTML || m.HasJS || m.HasIcons
}

// Global mutex for SSR extraction protection
var ssrExtractMu sync.Mutex

// invokeSSRExtractorOnce generates a combined main.go, runs it once, and returns the aggregated output.
// Results are cached by the hash of all module Go files.
func invokeSSRExtractorOnce(rootDir string, modules []Module) (map[string]ssrCollectorOutput, error) {
	tmpDir, err := os.MkdirTemp("", "assetmin-extract-*")
	if err != nil {
		return nil, fmt.Err("failed to create temp dir", err)
	}
	defer os.RemoveAll(tmpDir)

	// Generate main.go that imports all modules
	mainFile := filepath.Join(tmpDir, "main.go")
	if err := GenerateExtractorMain(mainFile, modules); err != nil {
		return nil, fmt.Err("failed to generate main.go", err)
	}

	// Run go run main.go and capture JSON output
	cmd := exec.Command("go", "run", mainFile)
	cmd.Dir = rootDir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Err("go run failed", err, stderr.String())
	}

	// Parse the JSON output
	var results map[string]ssrCollectorOutput
	if err := json.Unmarshal(out, &results); err != nil {
		return nil, fmt.Err("failed to parse extractor output", err)
	}

	return results, nil
}

// GenerateExtractorMain writes a main.go file that imports all modules and collects their assets.
func GenerateExtractorMain(outputFile string, modules []Module) error {
	tmpl := template.Must(template.New("extractor").Parse(`package main

import (
	"encoding/json"
	"os"
	{{range .Modules}}
	{{if .HasAnyFeature}}{{.Alias}} "{{.Path}}"{{end}}
	{{end}}
)

type ssr struct {
	Root   string            ` + "`json:\"root\"`" + `
	Render string            ` + "`json:\"render\"`" + `
	HTML   string            ` + "`json:\"html\"`" + `
	JS     string            ` + "`json:\"js\"`" + `
	Icons  map[string]string ` + "`json:\"icons\"`" + `
}

func main() {
	all := make(map[string]ssr)
	{{range .Modules}}
	{{if .HasAnyFeature}}
	{
		var s ssr
		{{if .HasInstance}}
		{
			inst := {{.Alias}}.SSRInstance()
			{{if .HasRoot}}s.Root = inst.RootCSS().String(){{end}}
			{{if .HasRender}}s.Render = inst.RenderCSS().String(){{end}}
			{{if .HasHTML}}s.HTML = inst.RenderHTML(){{end}}
			{{if .HasJS}}s.JS = inst.RenderJS(){{end}}
			{{if .HasIcons}}s.Icons = inst.IconSvg(){{end}}
		}
		{{else}}
		{{if .HasRoot}}s.Root = {{.Alias}}.RootCSS().String(){{end}}
		{{if .HasRender}}s.Render = {{.Alias}}.RenderCSS().String(){{end}}
		{{if .HasHTML}}s.HTML = {{.Alias}}.RenderHTML(){{end}}
		{{if .HasJS}}s.JS = {{.Alias}}.RenderJS(){{end}}
		{{if .HasIcons}}s.Icons = {{.Alias}}.IconSvg(){{end}}
		{{end}}
		all["{{.Path}}"] = s
	}
	{{end}}
	{{end}}
	json.NewEncoder(os.Stdout).Encode(all)
}
`))

	data := struct {
		Modules []ModuleAlias
	}{
		Modules: ModulesToAliases(modules),
	}

	f, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer f.Close()

	return tmpl.Execute(f, data)
}

var (
	reSSRInstance = regexp.MustCompile(`(?m)^func SSRInstance\(`)
	reRootCSS     = regexp.MustCompile(`(?m)^func.*RootCSS\(\)`)
	reRenderCSS   = regexp.MustCompile(`(?m)^func.*RenderCSS\(\)`)
	reRenderHTML  = regexp.MustCompile(`(?m)^func.*RenderHTML\(\)`)
	reRenderJS    = regexp.MustCompile(`(?m)^func.*RenderJS\(\)`)
	reIconSvg     = regexp.MustCompile(`(?m)^func.*IconSvg\(\)`)
)

// ModulesToAliases converts module information to alias mappings and detects features via regex.
func ModulesToAliases(modules []Module) []ModuleAlias {
	var aliases []ModuleAlias
	for _, m := range modules {
		parts := fmt.Convert(m.Path).Split("/")
		alias := fmt.Convert(parts[len(parts)-1]).Replace("-", "_").String()

		// If alias starts with a digit or is empty, prepend an underscore to make it a valid Go identifier
		if len(alias) == 0 || (alias[0] >= '0' && alias[0] <= '9') {
			alias = "_" + alias
		}

		ma := ModuleAlias{
			Path:  m.Path,
			Alias: alias,
		}

		// Read ssr.go to detect features
		if m.Dir != "" {
			if content, err := os.ReadFile(filepath.Join(m.Dir, "ssr.go")); err == nil {
				ma.HasInstance = reSSRInstance.Match(content)
				ma.HasRoot = reRootCSS.Match(content)
				ma.HasRender = reRenderCSS.Match(content)
				ma.HasHTML = reRenderHTML.Match(content)
				ma.HasJS = reRenderJS.Match(content)
				ma.HasIcons = reIconSvg.Match(content)
			}
		}

		aliases = append(aliases, ma)
	}
	return aliases
}
