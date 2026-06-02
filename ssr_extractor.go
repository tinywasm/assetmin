package assetmin

import (
	"github.com/tinywasm/js"
	"github.com/tinywasm/svg"
)

// SSRAssets es el DTO de assets crudos por módulo (lo produce tinywasm/ssr).
type SSRAssets struct {
	ModuleName  string
	RootCSS     string
	CSS         string
	JS          []*js.Script
	HTML        string
	Icons       *svg.Sprite
	IsRoot      bool
	IsFramework bool
}

// SSRExtractor lo implementa github.com/tinywasm/ssr; lo inyecta app.
type SSRExtractor interface {
	ExtractModule(moduleDir string) (*SSRAssets, error)
	ExtractAll() ([]*SSRAssets, error)
}

func (c *AssetMin) SetSSRExtractor(e SSRExtractor) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ssrExtractor = e
}
