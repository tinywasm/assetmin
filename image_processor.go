package assetmin

// ImageProcessor procesa imágenes declaradas en los image.go de los módulos.
// Implementado por github.com/tinywasm/image/min; inyectado por el composition root (app).
type ImageProcessor interface {
	LoadImages() error                   // escaneo completo inicial (startup)
	ReloadModule(moduleDir string) error // reproceso de un módulo (image.go cambió)
	UnobservedFiles() []string           // outputs .webp a excluir del watcher
}

// SetImageProcessor inyecta el pipeline de imágenes. Pasar nil lo desactiva.
func (c *AssetMin) SetImageProcessor(p ImageProcessor) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.imageProcessor = p
}
