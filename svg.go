package assetmin

import (
	"encoding/xml"
	"errors"
	"path/filepath"
	"strings"
)

func NewSvgHandler(ac *Config, outputName string) *asset {
	svgh := newAssetFile(outputName, "image/svg+xml", ac, nil)

	// Add the open tags to contentOpen
	svgh.contentOpen = append(svgh.contentOpen, &ContentFile{
		Path: "sprite-open.svg",
		Content: []byte(`<svg class="sprite-icons" xmlns="http://www.w3.org/2000/svg" style="display:none" role="img" aria-hidden="true" focusable="false">
		<defs>`),
	})

	// Add the closing tags to contentClose
	svgh.contentClose = append(svgh.contentClose, &ContentFile{
		Path: "sprite-close.svg",
		Content: []byte(`		</defs>
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
// It assumes the caller holds c.mu.
func (c *AssetMin) addIcon(id string, svgContent string) error {
	// Strip extension if present to use base name as ID
	if filepath.Ext(id) == ".svg" {
		id = strings.TrimSuffix(id, ".svg")
	}

	// Collision check
	if c.registeredIconIDs[id] {
		return errors.New("icon already registered: " + id)
	}

	// Register the icon
	c.registeredIconIDs[id] = true

	// Default viewBox
	viewBox := "0 0 16 16"

	// Use XML decoder for robust attribute extraction
	decoder := xml.NewDecoder(strings.NewReader(svgContent))
	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}
		if se, ok := token.(xml.StartElement); ok {
			if se.Name.Local == "svg" {
				for _, attr := range se.Attr {
					if attr.Name.Local == "viewBox" {
						viewBox = attr.Value
					}
				}
			}
			break
		}
	}

	// Strip outer <svg> wrapper if present to avoid nested SVGs in symbol
	contentToWrap := svgContent
	trimmed := strings.TrimSpace(svgContent)
	if strings.HasPrefix(trimmed, "<svg") && strings.HasSuffix(trimmed, "</svg>") {
		if endOpen := strings.Index(trimmed, ">"); endOpen != -1 {
			contentToWrap = trimmed[endOpen+1 : len(trimmed)-6]
		}
	}

	// Wrap SVG content as a <symbol> for the sprite
	symbolContent := `<symbol id="` + id + `" viewBox="` + viewBox + `">` + contentToWrap + `</symbol>`

	c.spriteSvgHandler.AddContentMiddle(id+".svg", []byte(symbolContent))

	return nil
}
