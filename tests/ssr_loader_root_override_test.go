package assetmin_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoader_CssDefaultWins_NoAppRoot(t *testing.T) {
	env := setupTestEnv("css_wins", t)
	am := env.AssetsHandler

	rootModule := env.BaseDir
	cssModule := filepath.Join(env.BaseDir, "vendor", "tinywasm", "css")
	os.MkdirAll(cssModule, 0755)

	os.WriteFile(filepath.Join(cssModule, "ssr.go"), []byte(`package css
func RootCSS() string { return ":root{--css:1;}" }
`), 0644)

	am.RootDir = rootModule
	am.SetListModulesFn(func(root string) ([]string, error) {
		return []string{cssModule, rootModule}, nil
	})

	am.LoadSSRModules()
	am.WaitForSSRLoad(1 * time.Second)

	output, _ := am.GetMinifiedCSS()
	if !strings.Contains(string(output), "--css:1") {
		t.Errorf("Expected framework css tokens, got: %s", string(output))
	}
}

func TestLoader_AppAndCssBothInjected(t *testing.T) {
	env := setupTestEnv("app_and_css", t)
	am := env.AssetsHandler

	rootModule := env.BaseDir
	cssModule := filepath.Join(env.BaseDir, "vendor", "tinywasm", "css")
	os.MkdirAll(cssModule, 0755)

	os.WriteFile(filepath.Join(rootModule, "ssr.go"), []byte(`package root
func RootCSS() string { return ":root{--app:1;}" }
`), 0644)
	os.WriteFile(filepath.Join(cssModule, "ssr.go"), []byte(`package css
func RootCSS() string { return ":root{--css:1;}" }
`), 0644)

	am.RootDir = rootModule
	am.SetListModulesFn(func(root string) ([]string, error) {
		return []string{cssModule, rootModule}, nil
	})

	am.LoadSSRModules()
	am.WaitForSSRLoad(1 * time.Second)

	output, _ := am.GetMinifiedCSS()
	// Both must be present: framework tokens first, app override second.
	// CSS cascade resolves variable conflicts — app wins for tokens it redeclares.
	if !strings.Contains(string(output), "--css:1") {
		t.Errorf("Expected framework css tokens, got: %s", string(output))
	}
	if !strings.Contains(string(output), "--app:1") {
		t.Errorf("Expected app root css override, got: %s", string(output))
	}
}

func TestLoader_ThirdPartyIgnored(t *testing.T) {
	env := setupTestEnv("third_party_ignored", t)
	am := env.AssetsHandler

	rootModule := env.BaseDir
	cssModule := filepath.Join(env.BaseDir, "vendor", "tinywasm", "css")
	thirdModule := filepath.Join(env.BaseDir, "vendor", "other", "module")
	os.MkdirAll(cssModule, 0755)
	os.MkdirAll(thirdModule, 0755)

	var logs []string
	am.SetLog(func(m ...any) {
		logs = append(logs, strings.Join(strings.Split(strings.Trim(fmt.Sprint(m...), "[]"), " "), " "))
	})

	os.WriteFile(filepath.Join(cssModule, "ssr.go"), []byte(`package css
func RootCSS() string { return ":root{--css:1;}" }
`), 0644)
	os.WriteFile(filepath.Join(thirdModule, "ssr.go"), []byte(`package third
func RootCSS() string { return ":root{--third:1;}" }
`), 0644)

	am.RootDir = rootModule
	am.SetListModulesFn(func(root string) ([]string, error) {
		return []string{cssModule, thirdModule, rootModule}, nil
	})

	// Import third party to ensure it's loaded
	os.WriteFile(filepath.Join(rootModule, "main.go"), []byte(`package main
import _ "other/module"
`), 0644)

	am.LoadSSRModules()
	am.WaitForSSRLoad(1 * time.Second)

	output, _ := am.GetMinifiedCSS()
	if !strings.Contains(string(output), "--css:1") {
		t.Error("Framework css tokens missing")
	}
	if strings.Contains(string(output), "--third:1") {
		t.Error("Third party root css should be ignored")
	}

	foundWarning := false
	for _, l := range logs {
		if strings.Contains(l, "declares RootCSS() but only the root project or") {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Error("Expected warning for third party RootCSS(), got none")
	}
}

func TestLoader_NoHardcodedDomInSlot(t *testing.T) {
	env := setupTestEnv("no_hardcoded_dom", t)
	am := env.AssetsHandler

	rootModule := env.BaseDir
	fooModule := filepath.Join(env.BaseDir, "vendor", "tinywasm", "foo")
	os.MkdirAll(fooModule, 0755)

	os.WriteFile(filepath.Join(fooModule, "ssr.go"), []byte(`package foo
func RootCSS() string { return ":root{--foo:1;}" }
`), 0644)

	am.RootDir = rootModule
	am.SetListModulesFn(func(root string) ([]string, error) {
		return []string{fooModule, rootModule}, nil
	})

	// Import foo
	os.WriteFile(filepath.Join(rootModule, "main.go"), []byte(`package main
import _ "tinywasm/foo"
`), 0644)

	am.LoadSSRModules()
	am.WaitForSSRLoad(1 * time.Second)

	css, _ := am.GetMinifiedCSS()
	if strings.Contains(string(css), "--foo:1") {
		t.Error("tinywasm/foo should be treated as third-party, but its RootCSS was loaded")
	}
}
