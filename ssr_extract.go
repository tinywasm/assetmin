package assetmin

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type SSRAssets struct {
	ModuleName string
	CSS        string
	JS         string
	HTML       string
	Icons      map[string]string
}

// ExtractSSRAssets parsea el archivo ssr.go en moduleDir y retorna los assets.
func ExtractSSRAssets(moduleDir string) (*SSRAssets, error) {
	ssrFile := filepath.Join(moduleDir, "ssr.go")
	if _, err := os.Stat(ssrFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("ssr.go not found in %s", moduleDir)
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, ssrFile, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("error parsing ssr.go: %w", err)
	}

	assets := &SSRAssets{
		ModuleName: filepath.Base(moduleDir),
		Icons:      make(map[string]string),
	}

	// Detect //go:embed directives
	embeds := make(map[string]string) // varName -> fileName
	findEmbedVars(f, embeds)

	// Traverse AST to find functions
	ast.Inspect(f, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}

		switch fn.Name.Name {
		case "RenderCSS":
			assets.CSS = extractReturnString(fn, embeds, moduleDir)
		case "RenderJS":
			assets.JS = extractReturnString(fn, embeds, moduleDir)
		case "RenderHTML":
			assets.HTML = extractReturnString(fn, embeds, moduleDir)
		case "IconSvg":
			assets.Icons = extractReturnMap(fn)
		}
		return true
	})

	return assets, nil
}

func findEmbedVars(f *ast.File, embeds map[string]string) {
	for _, decl := range f.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		if gen.Doc == nil {
			continue
		}
		for _, comment := range gen.Doc.List {
			if strings.HasPrefix(comment.Text, "//go:embed ") {
				fileName := strings.TrimPrefix(comment.Text, "//go:embed ")
				fileName = strings.TrimSpace(fileName)
				for _, spec := range gen.Specs {
					vspec := spec.(*ast.ValueSpec)
					for _, name := range vspec.Names {
						embeds[name.Name] = fileName
					}
				}
			}
		}
	}
}

func extractReturnString(fn *ast.FuncDecl, embeds map[string]string, moduleDir string) string {
	if fn.Body == nil {
		return ""
	}
	for _, stmt := range fn.Body.List {
		ret, ok := stmt.(*ast.ReturnStmt)
		if !ok || len(ret.Results) == 0 {
			continue
		}

		return evalStringExpr(ret.Results[0], embeds, moduleDir)
	}
	return ""
}

func evalStringExpr(expr ast.Expr, embeds map[string]string, moduleDir string) string {
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			s, _ := strconv.Unquote(e.Value)
			return s
		}
	case *ast.Ident:
		if fileName, ok := embeds[e.Name]; ok {
			content, err := os.ReadFile(filepath.Join(moduleDir, fileName))
			if err != nil {
				return ""
			}
			return string(content)
		}
	case *ast.BinaryExpr:
		if e.Op == token.ADD {
			return evalStringExpr(e.X, embeds, moduleDir) + evalStringExpr(e.Y, embeds, moduleDir)
		}
	}
	return ""
}

func extractReturnMap(fn *ast.FuncDecl) map[string]string {
	res := make(map[string]string)
	if fn.Body == nil {
		return res
	}
	for _, stmt := range fn.Body.List {
		ret, ok := stmt.(*ast.ReturnStmt)
		if !ok || len(ret.Results) == 0 {
			continue
		}

		cl, ok := ret.Results[0].(*ast.CompositeLit)
		if !ok {
			continue
		}

		for _, elt := range cl.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			key := evalStringExpr(kv.Key, nil, "")
			val := evalStringExpr(kv.Value, nil, "")
			if key != "" {
				res[key] = val
			}
		}
	}
	return res
}
