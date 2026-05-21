package assetmin_test

import (
	"github.com/tinywasm/js"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type standaloneComponent struct {
	bundled    string
	standalone string
	sw         string
}

func (c *standaloneComponent) RenderJS() []*js.Script {
	var scripts []*js.Script
	if c.bundled != "" {
		scripts = append(scripts, &js.Script{Name: "", Content: c.bundled})
	}
	if c.standalone != "" {
		scripts = append(scripts, &js.Script{Name: "extra.js", Content: c.standalone})
	}
	if c.sw != "" {
		scripts = append(scripts, &js.Script{Name: "sw.js", Content: c.sw})
	}
	return scripts
}

type standaloneComponentOther struct {
	standaloneComponent
}

// --- Tests nombrados del plan (Stage 8) ---

func TestRegister_BundledScript(t *testing.T) {
	env := setupTestEnv("reg_bundled", t)
	am := env.AssetsHandler

	comp := &standaloneComponent{bundled: "console.log('x')"}
	if err := am.RegisterComponents(comp); err != nil {
		t.Fatalf("RegisterComponents: %v", err)
	}
	if !am.ContainsJS("x") {
		t.Error("bundled script not found in main bundle")
	}
}

func TestRegister_StandaloneScript(t *testing.T) {
	env := setupTestEnv("reg_standalone", t)
	am := env.AssetsHandler

	comp := &standaloneComponent{sw: "self.addEventListener('install',()=>{})"}
	if err := am.RegisterComponents(comp); err != nil {
		t.Fatalf("RegisterComponents: %v", err)
	}
	if err := am.FlushToDisk(); err != nil {
		t.Fatalf("FlushToDisk: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(env.OutDir, "sw.js"))
	if err != nil {
		t.Fatalf("sw.js not written: %v", err)
	}
	if !strings.Contains(string(content), "install") {
		t.Errorf("sw.js content mismatch: %s", content)
	}
	if am.ContainsJS("install") {
		t.Error("standalone script should NOT appear in main bundle")
	}
}

func TestRegister_MixedScripts(t *testing.T) {
	env := setupTestEnv("reg_mixed", t)
	am := env.AssetsHandler

	comp := &standaloneComponent{
		bundled: "console.log('bundle')",
		sw:      "self.addEventListener('fetch',()=>{})",
	}
	if err := am.RegisterComponents(comp); err != nil {
		t.Fatalf("RegisterComponents: %v", err)
	}
	if err := am.FlushToDisk(); err != nil {
		t.Fatalf("FlushToDisk: %v", err)
	}

	if !am.ContainsJS("bundle") {
		t.Error("bundled part not in script.js")
	}
	content, err := os.ReadFile(filepath.Join(env.OutDir, "sw.js"))
	if err != nil {
		t.Fatalf("sw.js not written: %v", err)
	}
	if !strings.Contains(string(content), "fetch") {
		t.Errorf("sw.js missing standalone content: %s", content)
	}
}

func TestRegister_NameCollision(t *testing.T) {
	// Two different component types both emit extra.js → content merges (no silent data loss).
	env := setupTestEnv("reg_collision", t)
	am := env.AssetsHandler

	comp1 := &standaloneComponent{}
	comp1.standalone = "moduleA"
	comp2 := &standaloneComponentOther{}
	comp2.standalone = "moduleB"

	if err := am.RegisterComponents(comp1, comp2); err != nil {
		t.Fatalf("RegisterComponents: %v", err)
	}
	if err := am.FlushToDisk(); err != nil {
		t.Fatalf("FlushToDisk: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(env.OutDir, "extra.js"))
	if err != nil {
		t.Fatalf("extra.js not written: %v", err)
	}
	if !strings.Contains(string(content), "moduleA") || !strings.Contains(string(content), "moduleB") {
		t.Errorf("extra.js should contain content from both modules, got %q", content)
	}
}

func TestRegister_InvalidName(t *testing.T) {
	cases := []string{"a/b.js", "../x.js", "sub/dir/sw.js"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			env := setupTestEnv("reg_invalid_"+strings.ReplaceAll(name, "/", "_"), t)
			am := env.AssetsHandler

			comp := &customNameComponent{name: name, content: "bad"}
			err := am.RegisterComponents(comp)
			if err == nil {
				t.Errorf("expected error for invalid standalone name %q, got nil", name)
			}
		})
	}
}

func TestFlushToDisk_WritesStandalone(t *testing.T) {
	env := setupTestEnv("flush_standalone", t)
	am := env.AssetsHandler

	comp := &standaloneComponent{sw: "// sw content"}
	am.RegisterComponents(comp)

	if err := am.FlushToDisk(); err != nil {
		t.Fatalf("FlushToDisk: %v", err)
	}
	if _, err := os.Stat(filepath.Join(env.OutDir, "sw.js")); err != nil {
		t.Errorf("sw.js not found on disk after FlushToDisk: %v", err)
	}
}

func TestHTTP_ServesStandalone(t *testing.T) {
	env := setupTestEnv("http_standalone", t)
	am := env.AssetsHandler

	comp := &standaloneComponent{sw: "self.addEventListener('activate',()=>{})"}
	am.RegisterComponents(comp)

	mux := newTestMux(am)
	srv := newTestServer(mux)
	defer srv.Close()

	resp, body := doGet(t, srv.URL+"/sw.js")
	if resp.StatusCode != 200 {
		t.Fatalf("GET /sw.js status %d", resp.StatusCode)
	}
	if !strings.Contains(body, "activate") {
		t.Errorf("GET /sw.js missing expected content, got: %s", body)
	}
}

func TestHotReload_RemovesOrphanStandalone(t *testing.T) {
	env := setupTestEnv("hotreload_orphan", t)
	am := env.AssetsHandler

	comp := &standaloneComponent{standalone: "v1", sw: "sw-init"}
	am.RegisterComponents(comp)
	am.FlushToDisk()

	// Simulate hot reload: component no longer emits extra.js
	comp.standalone = ""
	am.RegisterComponents(comp)
	am.FlushToDisk()

	content, _ := os.ReadFile(filepath.Join(env.OutDir, "extra.js"))
	if strings.TrimSpace(string(content)) != "" {
		t.Errorf("orphan extra.js should be empty after hot reload, got %q", content)
	}
	swContent, _ := os.ReadFile(filepath.Join(env.OutDir, "sw.js"))
	if !strings.Contains(string(swContent), "sw-init") {
		t.Errorf("sw.js should still exist, got %q", swContent)
	}
}

// customNameComponent permite probar nombres arbitrarios
type customNameComponent struct {
	name    string
	content string
}

func (c *customNameComponent) RenderJS() []*js.Script {
	return []*js.Script{{Name: c.name, Content: c.content}}
}

func TestStandaloneJS(t *testing.T) {
	t.Run("BundledAndStandaloneRegistration", func(t *testing.T) {
		env := setupTestEnv("standalone_reg", t)
		am := env.AssetsHandler

		comp := &standaloneComponent{
			bundled:    "console.log('bundled');",
			standalone: "console.log('standalone');",
		}

		if err := am.RegisterComponents(comp); err != nil {
			t.Fatalf("RegisterComponents failed: %v", err)
		}

		// Check bundled
		if !am.ContainsJS("bundled") {
			t.Error("Bundled JS not found in main bundle")
		}

		// Check standalone handler exists and has content
		// We use FlushToDisk to verify they are written
		if err := am.FlushToDisk(); err != nil {
			t.Fatalf("FlushToDisk failed: %v", err)
		}

		standalonePath := filepath.Join(env.OutDir, "extra.js")
		content, err := os.ReadFile(standalonePath)
		if err != nil {
			t.Fatalf("Standalone file not written: %v", err)
		}
		if !strings.Contains(string(content), "standalone") {
			t.Errorf("Standalone file content mismatch: %s", string(content))
		}
	})

	t.Run("StandaloneOrphanCleanup", func(t *testing.T) {
		env := setupTestEnv("standalone_cleanup", t)
		am := env.AssetsHandler

		comp := &standaloneComponent{
			standalone: "v1",
			sw:         "sw-v1",
		}

		am.RegisterComponents(comp)
		am.FlushToDisk()

		if _, err := os.Stat(filepath.Join(env.OutDir, "extra.js")); err != nil {
			t.Error("extra.js should exist")
		}
		if _, err := os.Stat(filepath.Join(env.OutDir, "sw.js")); err != nil {
			t.Error("sw.js should exist")
		}

		// Update component: remove extra.js, update sw.js
		comp.standalone = ""
		comp.sw = "sw-v2"
		am.RegisterComponents(comp)
		am.FlushToDisk()

		// extra.js handler content should be empty now
		content, _ := os.ReadFile(filepath.Join(env.OutDir, "extra.js"))
		if len(strings.TrimSpace(string(content))) > 0 {
			t.Errorf("extra.js should be empty, got %q", string(content))
		}

		content, _ = os.ReadFile(filepath.Join(env.OutDir, "sw.js"))
		if !strings.Contains(string(content), "sw-v2") {
			t.Errorf("sw.js should be updated, got %q", string(content))
		}
	})

	t.Run("StandaloneCollision", func(t *testing.T) {
		env := setupTestEnv("standalone_collision", t)
		am := env.AssetsHandler

		// Use different types to have different names in RegisterComponents
		comp1 := &standaloneComponent{}
		comp1.standalone = "comp1"
		comp2 := &standaloneComponentOther{}
		comp2.standalone = "comp2"

		am.RegisterComponents(comp1, comp2)
		am.FlushToDisk()

		content, _ := os.ReadFile(filepath.Join(env.OutDir, "extra.js"))
		if !strings.Contains(string(content), "comp1") || !strings.Contains(string(content), "comp2") {
			t.Errorf("extra.js should contain content from both components, got %q", string(content))
		}
	})
}
