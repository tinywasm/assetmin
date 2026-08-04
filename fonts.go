package assetmin

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/tinywasm/font"
)

// copyDeclaredFonts copies the four faces of d from RootDir/<Dir()> into OutputDir.
// Skips a face when the destination exists and is not older than the source.
// A missing source face is an error that names the file.
func (c *AssetMin) copyDeclaredFonts(d font.Declaration) error {
	if d.Family() == "" {
		return nil
	}
	if err := os.MkdirAll(c.OutputDir, 0755); err != nil {
		return err
	}
	for s := font.Regular; s <= font.BoldItalic; s++ {
		name := d.Family().Face(s) + ".ttf"
		src := filepath.Join(c.RootDir, d.Dir(), name)
		dst := filepath.Join(c.OutputDir, name)
		if err := copyFileIfStale(src, dst); err != nil {
			return fmt.Errorf("font face %s: %w", name, err)
		}
	}
	return nil
}

func copyFileIfStale(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if dstInfo, err := os.Stat(dst); err == nil {
		if !srcInfo.ModTime().After(dstInfo.ModTime()) {
			return nil
		}
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// fontOutputPaths returns the four destination paths for the current root fonts, or nil.
func (c *AssetMin) fontOutputPaths() []string {
	c.fontsMu.RLock()
	d := c.fonts
	c.fontsMu.RUnlock()
	if d.Family() == "" {
		return nil
	}
	out := make([]string, 0, 4)
	for s := font.Regular; s <= font.BoldItalic; s++ {
		out = append(out, filepath.Join(c.OutputDir, d.Family().Face(s)+".ttf"))
	}
	return out
}

func (c *AssetMin) setFonts(d font.Declaration) {
	c.fontsMu.Lock()
	c.fonts = d
	c.fontsMu.Unlock()
}
