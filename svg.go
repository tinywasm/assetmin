package assetmin

import (
	"strings"

	"github.com/tinywasm/fmt"
	"github.com/tinywasm/svg/sprite"
)

func NewSvgHandler(ac *Config, filename string) *asset {
	return newAssetFile(filename, "image/svg+xml", ac, nil)
}

func NewFaviconSvgHandler(ac *Config, filename string) *asset {
	return newAssetFile(filename, "image/svg+xml", ac, nil)
}

func (c *AssetMin) mergeSprite(s *sprite.Sprite) {
	c.spriteMu.Lock()
	defer c.spriteMu.Unlock()
	c.masterSprite.Merge(s)
	c.spriteSvgHandler.InvalidateCache()
}

// addIcon adds an icon body with its explicit viewBox (the InjectSpriteIcon path).
// viewBox is required: a symbol rendered in a box it was not drawn for is clipped
// or misaligned, and no default can recover the source coordinate system.
func (c *AssetMin) addIcon(id, content, viewBox string) error {
	c.spriteMu.Lock()
	defer c.spriteMu.Unlock()

	if err := c.checkIconID(id); err != nil {
		return err
	}
	if viewBox == "" {
		return fmt.Err("icon requires a viewBox:", id)
	}

	c.masterSprite.AddRaw(id, content, viewBox)
	c.spriteSvgHandler.InvalidateCache()
	return nil
}

// addIconFile adds a whole .svg file as an icon. Reading the file's viewBox and
// stripping its root element is sprite's job — assetmin does not parse SVG.
func (c *AssetMin) addIconFile(id, content string) error {
	c.spriteMu.Lock()
	defer c.spriteMu.Unlock()

	if err := c.checkIconID(id); err != nil {
		return err
	}
	if err := c.masterSprite.AddFile(id, content); err != nil {
		return err
	}
	c.spriteSvgHandler.InvalidateCache()
	return nil
}

func (c *AssetMin) checkIconID(id string) error {
	current := c.masterSprite.String()
	if strings.Contains(current, "id=\""+id+"\"") || strings.Contains(current, "id='"+id+"'") {
		return fmt.Err("icon ID already registered:", id)
	}
	return nil
}
