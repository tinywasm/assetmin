package assetmin

import (
	"os"
	"path/filepath"
	"strings"
)

const PackageName = "github.com/tinywasm/assetmin"

type GoMod struct {
	rootPath string
}

func NewGoMod() *GoMod {
	return &GoMod{
		rootPath: ".",
	}
}

func (m *GoMod) SetRootPath(path string) {
	m.rootPath = path
}

func (m *GoMod) IsAssetMinUsedInThisPkg() bool {
	content := fileExists(filepath.Join(m.rootPath, "go.mod"))
	if content == "" {
		return false
	}
	return strings.Contains(content, PackageName)
}

func fileExists(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}
