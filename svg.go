package assetmin

import (
	"strings"

	"github.com/tinywasm/fmt"
	"github.com/tinywasm/svg"
)

func NewSvgHandler(ac *Config, filename string) *asset {
	return newAssetFile(filename, "image/svg+xml", ac, nil)
}

func NewFaviconSvgHandler(ac *Config, filename string) *asset {
	return newAssetFile(filename, "image/svg+xml", ac, nil)
}

func (c *AssetMin) mergeSprite(s *svg.Sprite) {
	c.spriteMu.Lock()
	defer c.spriteMu.Unlock()
	c.masterSprite.Merge(s)
	c.spriteSvgHandler.InvalidateCache()
}

// addIcon stays for favicon and raw SVG file events (from events.go)
func (c *AssetMin) addIcon(id string, content string) error {
	c.spriteMu.Lock()
	defer c.spriteMu.Unlock()

	if strings.Contains(c.masterSprite.String(), "id=\""+id+"\"") || strings.Contains(c.masterSprite.String(), "id='"+id+"'") {
		return fmt.Err("icon ID already registered:", id)
	}
	// Create a temporary sprite to parse the raw icon and then merge it
	s := svg.New()
	s.Add(id, content)
	c.masterSprite.Merge(s)
	c.spriteSvgHandler.InvalidateCache()
	return nil
}
