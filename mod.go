package assetmin

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const PackageName = "github.com/tinywasm/assetmin"

type GoMod struct {
	rootPath       string
	isAssetMinUsed bool
	mu             sync.RWMutex
}

func NewGoMod() *GoMod {
	m := &GoMod{
		rootPath: ".",
	}
	m.isAssetMinUsed = m.checkDiskState()
	return m
}

func (m *GoMod) SetRootPath(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rootPath = path
	m.isAssetMinUsed = m.checkDiskState()
}

func (m *GoMod) IsUsed() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isAssetMinUsed
}

func (m *GoMod) CheckAndUpdate(filePath string) bool {
	if filepath.Base(filePath) != "go.mod" {
		return false
	}

	absRoot, _ := filepath.Abs(m.rootPath)
	absFile, _ := filepath.Abs(filePath)

	if absFile != filepath.Join(absRoot, "go.mod") {
		return false
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	oldState := m.isAssetMinUsed
	m.isAssetMinUsed = m.checkDiskState()

	return oldState != m.isAssetMinUsed
}

func (m *GoMod) checkDiskState() bool {
	content := fileExists(filepath.Join(m.rootPath, "go.mod"))
	if content == "" {
		return false
	}
	return strings.Contains(content, PackageName)
}

func (m *GoMod) NewFileEvent(filePath string, logger func(...any)) bool {
	if !m.CheckAndUpdate(filePath) {
		return false
	}

	if logger != nil {
		if m.IsUsed() {
			logger("AssetMin dependency detected in go.mod")
		} else {
			logger("AssetMin dependency removed from go.mod")
		}
	}
	return true
}

func fileExists(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}
