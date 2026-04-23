package assetmin_test

import (
	"testing"
)

type mockComponent struct {
	css string
}

func (m *mockComponent) RenderCSS() string { return m.css }

func TestSSRRegistration(t *testing.T) {
	t.Run("UpdateSSRModuleNoDuplicate", func(t *testing.T) {
		env := setupTestEnv("ssr_reg", t)
		am := env.AssetsHandler

		am.UpdateSSRModule("test", ".a{color:red;}", "", "", nil)
		if !am.ContainsCSS(".a{color:red;}") {
			t.Error("CSS not found")
		}

		// Second call replaces
		am.UpdateSSRModule("test", ".a{color:blue;}", "", "", nil)
		if am.ContainsCSS("color:red;") {
			t.Error("Old CSS still present")
		}
		if !am.ContainsCSS("color:blue;") {
			t.Error("New CSS not found")
		}
	})

	t.Run("RegisterComponents", func(t *testing.T) {
		env := setupTestEnv("reg_comp", t)
		am := env.AssetsHandler

		comp := &mockComponent{css: ".comp{margin:0;}"}
		am.RegisterComponents(comp)

		if !am.ContainsCSS(".comp{margin:0;}") {
			t.Error("Component CSS not found")
		}
	})
}
