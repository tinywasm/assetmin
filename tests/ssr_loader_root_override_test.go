package assetmin_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoader_DomDefaultWins_NoAppRoot(t *testing.T) {
	env := setupTestEnv("dom_wins", t)
	am := env.AssetsHandler

	rootModule := env.BaseDir
	domModule := filepath.Join(env.BaseDir, "vendor", "tinywasm", "dom")
	os.MkdirAll(domModule, 0755)

	os.WriteFile(filepath.Join(domModule, "ssr.go"), []byte(`package dom
func RootCSS() string { return ":root{--dom:1;}" }
`), 0644)

	am.RootDir = rootModule
	am.SetListModulesFn(func(root string) ([]string, error) {
		return []string{domModule, rootModule}, nil
	})

	am.LoadSSRModules()
	am.WaitForSSRLoad(1 * time.Second)

	css, _ := am.GetMinifiedCSS()
	if !strings.Contains(string(css), "--dom:1") {
		t.Errorf("Expected dom root css, got: %s", string(css))
	}
}

func TestLoader_AppOverridesDom(t *testing.T) {
	env := setupTestEnv("app_overrides", t)
	am := env.AssetsHandler

	rootModule := env.BaseDir
	domModule := filepath.Join(env.BaseDir, "vendor", "tinywasm", "dom")
	os.MkdirAll(domModule, 0755)

	os.WriteFile(filepath.Join(rootModule, "ssr.go"), []byte(`package root
func RootCSS() string { return ":root{--app:1;}" }
`), 0644)
	os.WriteFile(filepath.Join(domModule, "ssr.go"), []byte(`package dom
func RootCSS() string { return ":root{--dom:1;}" }
`), 0644)

	am.RootDir = rootModule
	am.SetListModulesFn(func(root string) ([]string, error) {
		return []string{domModule, rootModule}, nil
	})

	am.LoadSSRModules()
	am.WaitForSSRLoad(1 * time.Second)

	css, _ := am.GetMinifiedCSS()
	if !strings.Contains(string(css), "--app:1") {
		t.Errorf("Expected app root css to override dom, got: %s", string(css))
	}
	if strings.Contains(string(css), "--dom:1") {
		t.Errorf("Dom root css should have been overridden")
	}
}

func TestLoader_ThirdPartyIgnored(t *testing.T) {
	env := setupTestEnv("third_party_ignored", t)
	am := env.AssetsHandler

	rootModule := env.BaseDir
	domModule := filepath.Join(env.BaseDir, "vendor", "tinywasm", "dom")
	thirdModule := filepath.Join(env.BaseDir, "vendor", "other", "module")
	os.MkdirAll(domModule, 0755)
	os.MkdirAll(thirdModule, 0755)

	var logs []string
	am.SetLog(func(m ...any) {
		logs = append(logs, strings.Join(strings.Split(strings.Trim(fmt.Sprint(m...), "[]"), " "), " "))
	})

	os.WriteFile(filepath.Join(domModule, "ssr.go"), []byte(`package dom
func RootCSS() string { return ":root{--dom:1;}" }
`), 0644)
	os.WriteFile(filepath.Join(thirdModule, "ssr.go"), []byte(`package third
func RootCSS() string { return ":root{--third:1;}" }
`), 0644)

	am.RootDir = rootModule
	am.SetListModulesFn(func(root string) ([]string, error) {
		return []string{domModule, thirdModule, rootModule}, nil
	})

	// Import third party to ensure it's loaded
	os.WriteFile(filepath.Join(rootModule, "main.go"), []byte(`package main
import _ "other/module"
`), 0644)

	am.LoadSSRModules()
	am.WaitForSSRLoad(1 * time.Second)

	css, _ := am.GetMinifiedCSS()
	if !strings.Contains(string(css), "--dom:1") {
		t.Error("Dom root css missing")
	}
	if strings.Contains(string(css), "--third:1") {
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
