//go:build !wasm

package assetmin_test

import (
	"errors"
	"testing"
	"time"

	"github.com/tinywasm/assetmin"
	"github.com/tinywasm/svg/sprite"
)

type retryExtractor struct {
	calls      int
	failCount  int
	failResult error
	successRes []*assetmin.SSRAssets
}

func (r *retryExtractor) ExtractModule(moduleDir string) (*assetmin.SSRAssets, error) {
	return nil, nil
}

func (r *retryExtractor) ExtractAll() ([]*assetmin.SSRAssets, error) {
	r.calls++
	if r.calls <= r.failCount {
		return nil, r.failResult
	}
	return r.successRes, nil
}

func TestSSRMassScanRetry(t *testing.T) {
	root := t.TempDir()
	am := assetmin.NewAssetMin(&assetmin.Config{
		OutputDir: t.TempDir(),
		RootDir:   root,
		DevMode:   true,
	})
	am.EnableSSRMode()

	// Create test assets
	icons := sprite.NewSprite()
	icons.AddRaw("icon-retry", "<path d='M1 2'/>", "0 0 16 16")

	successAssets := []*assetmin.SSRAssets{
		{
			ModuleName: "mod-retry",
			CSS:        ".retry{color:blue}",
			HTML:       "<div>retry</div>",
			Icons:      icons,
		},
	}

	ex := &retryExtractor{
		failCount:  2,
		failResult: errors.New("transient compilation error"),
		successRes: successAssets,
	}
	am.SetSSRExtractor(ex)

	// In ScheduleSSRLoad, the extractor is run in a background goroutine.
	// We call LoadSSRModules, which triggers ScheduleSSRLoad.
	am.LoadSSRModules()

	// Wait with a reasonable timeout for SSR load.
	// Since there is a 200ms sleep on retry, waiting 2-3 seconds is perfect.
	am.WaitForSSRLoad(3 * time.Second)

	// Verify that we successfully extracted assets after the retry!
	if ex.calls != 3 {
		t.Errorf("Expected exactly 3 calls to ExtractAll (2 failures + 1 success), got %d", ex.calls)
	}

	if !am.ContainsCSS(".retry{color:blue}") {
		t.Error("Expected CSS from successful retry to be present")
	}

	if !am.ContainsHTML("<div>retry</div>") {
		t.Error("Expected HTML from successful retry to be present")
	}

	if !am.ContainsSVG("icon-retry") {
		t.Error("Expected SVG from successful retry to be present")
	}
}
