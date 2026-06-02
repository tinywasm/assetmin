package assetmin

import (
	"path/filepath"
	"slices"
)

// ssrTextAssetFiles: archivos Go cuyo contenido se EXTRAE (string) y se fusiona/inyecta.
var ssrTextAssetFiles = []string{
	"css.go",
	"js.go",
	"svg.go",
	"html.go",
}

// imageAssetFile: archivo Go que DECLARA imágenes a procesar (no se extrae string).
const imageAssetFile = "image.go"

// SSRFileWatcher implements devwatch.FilesEventHandlers.
// Watches .go events; routes only recognized asset-source files.
type SSRFileWatcher struct {
	am              *AssetMin
	onBrowserReload func() error
}

// NewSSRFileWatcher creates an SSRFileWatcher bound to this AssetMin instance.
func (am *AssetMin) NewSSRFileWatcher(onBrowserReload func() error) *SSRFileWatcher {
	return &SSRFileWatcher{am: am, onBrowserReload: onBrowserReload}
}

func (w *SSRFileWatcher) MainInputFileRelativePath() string { return "go.mod" }
func (w *SSRFileWatcher) SupportedExtensions() []string     { return []string{".go"} }
func (w *SSRFileWatcher) UnobservedFiles() []string         { return nil }

// NewFileEvent routes a .go event to the correct action.
func (w *SSRFileWatcher) NewFileEvent(fileName, extension, filePath, event string) error {
	moduleDir := filepath.Dir(filePath)

	switch {
	case slices.Contains(ssrTextAssetFiles, fileName):
		if err := w.am.ReloadSSRModule(moduleDir); err != nil {
			w.am.Logger("SSR hot reload error:", moduleDir, err)
			return err
		}
	case fileName == imageAssetFile:
		if w.am.imageProcessor == nil {
			return nil
		}
		if err := w.am.imageProcessor.ReloadModule(moduleDir); err != nil {
			w.am.Logger("image hot reload error:", moduleDir, err)
			return err
		}
	default:
		return nil
	}

	if w.onBrowserReload != nil {
		if err := w.onBrowserReload(); err != nil {
			w.am.Logger("browser reload error:", err)
		}
	}
	return nil
}
