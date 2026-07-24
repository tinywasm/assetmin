package assetmin

import (
	"strings"
	"testing"
	"time"
)

// Reproduce un SSRExtractor inconsistente: la misma clave de módulo llega
// primero en slot "close" (como isRoot=true) y luego en slot "middle" (como
// isRoot=false para el mismo módulo). Tras la segunda llamada debe quedar
// UNA sola entrada, con el contenido nuevo, y ninguna en el slot viejo.
func TestUpdateSSRModuleInSlot_ReplacesAcrossSlots(t *testing.T) {
	c := NewAssetMin(&Config{OutputDir: t.TempDir()})

	if err := c.updateSSRModuleInSlot("mod", ".mod{color:blue}", nil, "", nil, "close"); err != nil {
		t.Fatal(err)
	}
	if err := c.updateSSRModuleInSlot("mod", ".mod{color:red}", nil, "", nil, "middle"); err != nil {
		t.Fatal(err)
	}

	css, err := c.GetMinifiedCSS()
	if err != nil {
		t.Fatal(err)
	}
	got := string(css)

	if want := ".mod{color:red}"; !strings.Contains(got, want) {
		t.Errorf("missing new rule %q in %q", want, got)
	}
	if bad := ".mod{color:blue}"; strings.Contains(got, bad) {
		t.Errorf("stale rule %q still present in %q — old slot was not cleared", bad, got)
	}
}

func TestUpdateContentInSlot_NewEntriesInsertSortedByPath(t *testing.T) {
	c := NewAssetMin(&Config{OutputDir: t.TempDir()})

	// Arrival order deliberately NOT sorted: zeta, then alpha, then beta.
	if err := c.updateSSRModuleInSlot("zeta", ".z{}", nil, "", nil, "middle"); err != nil {
		t.Fatal(err)
	}
	if err := c.updateSSRModuleInSlot("alpha", ".a{}", nil, "", nil, "middle"); err != nil {
		t.Fatal(err)
	}
	if err := c.updateSSRModuleInSlot("beta", ".b{}", nil, "", nil, "middle"); err != nil {
		t.Fatal(err)
	}

	css, err := c.GetMinifiedCSS()
	if err != nil {
		t.Fatal(err)
	}
	got := string(css)

	// Since we minified, the spaces might be removed, so let's check exact substring order.
	// We expect the rules to be sorted alphabetically by ModuleName: alpha (.a{}), beta (.b{}), zeta (.z{}).
	idxA := strings.Index(got, ".a{}")
	idxB := strings.Index(got, ".b{}")
	idxZ := strings.Index(got, ".z{}")

	if idxA == -1 || idxB == -1 || idxZ == -1 {
		t.Fatalf("one or more expected CSS classes not found in: %q", got)
	}

	if idxA > idxB {
		t.Errorf("expected alpha (.a{}) before beta (.b{}), got idxA=%d, idxB=%d", idxA, idxB)
	}
	if idxB > idxZ {
		t.Errorf("expected beta (.b{}) before zeta (.z{}), got idxB=%d, idxZ=%d", idxB, idxZ)
	}
}

type fakeExtractor struct {
	extractAll    func() ([]*SSRAssets, error)
	extractModule func(dir string) (*SSRAssets, error)
}

func (f *fakeExtractor) ExtractAll() ([]*SSRAssets, error) {
	if f.extractAll != nil {
		return f.extractAll()
	}
	return nil, nil
}

func (f *fakeExtractor) ExtractModule(dir string) (*SSRAssets, error) {
	if f.extractModule != nil {
		return f.extractModule(dir)
	}
	return nil, nil
}

func TestReloadSSRModule_RetriesFullScanAfterPermanentExtractAllFailure(t *testing.T) {
	c := NewAssetMin(&Config{OutputDir: t.TempDir(), RootDir: t.TempDir()})

	extractAllCalls := 0
	c.SetSSRExtractor(&fakeExtractor{
		extractAll: func() ([]*SSRAssets, error) {
			extractAllCalls++
			return nil, nil // succeed instantly to avoid retry/backoff sleep
		},
		extractModule: func(dir string) (*SSRAssets, error) { return nil, nil },
	})

	c.initialLoadFailed = true // simula que ScheduleSSRLoad ya agotó sus 5 reintentos

	_ = c.ReloadSSRModule(t.TempDir())

	// Wait for the asynchronous ScheduleSSRLoad to finish
	c.WaitForSSRLoad(1 * time.Second)

	if extractAllCalls == 0 {
		t.Error("ReloadSSRModule after a permanent ExtractAll failure must retry the full scan, not just the single module")
	}
	if c.initialLoadFailed {
		t.Error("initialLoadFailed must be cleared once a retry has been scheduled")
	}
}
