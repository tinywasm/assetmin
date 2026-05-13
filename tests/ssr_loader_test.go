package assetmin_test

import (
	"strings"
	"testing"
)

func TestSSRLoader(t *testing.T) {
	t.Run("LoadSSRModulesOrder", func(t *testing.T) {
		env := setupTestEnv("loader_order", t)
		am := env.AssetsHandler

		// Mock module directories injection
		am.UpdateSSRModuleInSlot("tinywasm/css", ".dom{color:red;}", "", "", nil, "open")
		am.UpdateSSRModuleInSlot("other/module", ".ext{color:green;}", "", "", nil, "middle")
		am.UpdateSSRModuleInSlot("root", ".root{color:blue;}", "", "", nil, "close")

		// Verify presence
		if !am.ContainsCSS(".dom") || !am.ContainsCSS(".ext") || !am.ContainsCSS(".root") {
			t.Error("Some CSS missing")
		}

		// Verify order via minified output
		css, _ := am.GetMinifiedCSS()
		cssStr := string(css)

		idxDom := strings.Index(cssStr, ".dom")
		idxExt := strings.Index(cssStr, ".ext")
		idxRoot := strings.Index(cssStr, ".root")

		if idxDom == -1 || idxExt == -1 || idxRoot == -1 {
			t.Fatalf("Missing CSS parts in bundle: %s", cssStr)
		}

		if !(idxDom < idxExt && idxExt < idxRoot) {
			t.Errorf("Wrong CSS order: dom=%d, ext=%d, root=%d", idxDom, idxExt, idxRoot)
		}
	})

	t.Run("LoadIconsFromLocalRoot", func(t *testing.T) {
		env := setupTestEnv("local_icons", t)
		am := env.AssetsHandler

		am.UpdateSSRModule("root", "", "", "", map[string]string{
			"local-icon": "<path d='M0 0l1 1'/>",
		})

		if !am.HasIcon("local-icon") {
			t.Error("Icon from local root not loaded")
		}
	})
}
