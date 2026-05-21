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
