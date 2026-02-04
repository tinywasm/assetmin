package assetmin

import "errors"

func NewSvgHandler(ac *Config, outputName string) *asset {
	svgh := newAssetFile(outputName, "image/svg+xml", ac, nil)

	// Add the open tags to contentOpen
	svgh.contentOpen = append(svgh.contentOpen, &contentFile{
		path: "sprite-open.svg",
		content: []byte(`<svg class="sprite-icons" xmlns="http://www.w3.org/2000/svg" role="img" aria-hidden="true" focusable="false">
		<defs>`),
	})

	// Add the closing tags to contentClose
	svgh.contentClose = append(svgh.contentClose, &contentFile{
		path: "sprite-close.svg",
		content: []byte(`		</defs>
	</svg>`),
	})

	return svgh
}

// NewFaviconSvgHandler creates a handler for favicon.svg that simply minifies and copies the file
// without sprite wrapping. This handler processes standalone SVG files like favicon.svg
func NewFaviconSvgHandler(ac *Config, outputName string) *asset {
	return newAssetFile(outputName, "image/svg+xml", ac, nil)
}

// addIcon adds an icon to the sprite handler with collision detection.
// Returns an error if an icon with the same ID is already registered.
func (c *AssetMin) addIcon(id string, svgContent string) error {
	// Initialize map if nil (lazy initialization)
	if c.registeredIconIDs == nil {
		c.registeredIconIDs = make(map[string]bool)
	}

	// Collision check
	if c.registeredIconIDs[id] {
		return errors.New("icon already registered: " + id)
	}

	// Register the icon
	c.registeredIconIDs[id] = true

	// Wrap SVG content as a <symbol> for the sprite
	symbolContent := `<symbol id="` + id + `" viewBox="0 0 16 16">` + svgContent + `</symbol>`

	c.spriteSvgHandler.contentMiddle = append(
		c.spriteSvgHandler.contentMiddle,
		&contentFile{path: id + ".svg", content: []byte(symbolContent)},
	)
	c.spriteSvgHandler.cacheValid = false

	return nil
}
