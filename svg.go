package assetmin

import (
	"sort"
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

// renderSpriteNoLock combines every module's sprite into one, keeping the
// first occurrence of a given icon id (sorted by module name for a stable,
// deterministic order across scans). Reads typed Definitions via Icons() —
// never re-parses Sprite.String()'s markup — so this stays correct
// independent of how sprite.go formats its output.
func (c *AssetMin) renderSpriteNoLock() string {
	var keys []string
	for k := range c.moduleSprites {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	seen := make(map[string]bool)
	var deduped []sprite.Definition

	for _, k := range keys {
		s := c.moduleSprites[k]
		if s == nil {
			continue
		}
		for _, def := range s.Icons() {
			id := def.Icon.ID()
			if !seen[id] {
				seen[id] = true
				deduped = append(deduped, def)
			}
		}
	}

	out := sprite.NewSprite(deduped...).String()
	if out == "" {
		// Preserve the wrapper tag even with zero icons (matches the prior
		// behavior consumers may depend on, e.g. a stable injection point).
		out = `<svg aria-hidden="true" style="display:none"></svg>`
	}
	return out
}

func (c *AssetMin) renderSprite() string {
	c.spriteMu.RLock()
	defer c.spriteMu.RUnlock()
	return c.renderSpriteNoLock()
}

func (c *AssetMin) setModuleSprite(name string, icons *sprite.Sprite) {
	c.spriteMu.Lock()
	defer c.spriteMu.Unlock()
	if icons == nil {
		delete(c.moduleSprites, name)
	} else {
		if c.moduleSprites == nil {
			c.moduleSprites = make(map[string]*sprite.Sprite)
		}
		c.moduleSprites[name] = icons
	}
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

	s, ok := c.moduleSprites["_manual"]
	if !ok {
		s = sprite.NewSprite()
		if c.moduleSprites == nil {
			c.moduleSprites = make(map[string]*sprite.Sprite)
		}
		c.moduleSprites["_manual"] = s
	}

	s.AddRaw(id, content, viewBox)
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

	s, ok := c.moduleSprites["_manual"]
	if !ok {
		s = sprite.NewSprite()
		if c.moduleSprites == nil {
			c.moduleSprites = make(map[string]*sprite.Sprite)
		}
		c.moduleSprites["_manual"] = s
	}

	if err := s.AddFile(id, content); err != nil {
		return err
	}
	c.spriteSvgHandler.InvalidateCache()
	return nil
}

func (c *AssetMin) checkIconID(id string) error {
	current := c.renderSpriteNoLock()
	if strings.Contains(current, "id=\""+id+"\"") || strings.Contains(current, "id='"+id+"'") {
		return fmt.Err("icon ID already registered:", id)
	}
	return nil
}
