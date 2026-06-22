package cache

import (
	"os"
	"path/filepath"
	"testing"
)

type rec struct {
	Name string `json:"name"`
	N    int    `json:"n"`
}

func TestLoadMissing(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	if _, ok := Load[[]rec]("nope"); ok {
		t.Error("Load of a missing file returned ok=true")
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	want := []rec{{Name: "a", N: 1}, {Name: "b", N: 2}}
	if err := Save("things", want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, ok := Load[[]rec]("things")
	if !ok {
		t.Fatal("Load after Save returned ok=false")
	}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("round-trip = %+v, want %+v", got, want)
	}
}

func TestSaveIsAtomic(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	if err := Save("x", []rec{{Name: "a"}}); err != nil {
		t.Fatal(err)
	}
	// No leftover temp files beside the final json.
	entries, _ := os.ReadDir(filepath.Join(dir, "agenda"))
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" || filepath.Ext(e.Name()) != ".json" {
			t.Errorf("unexpected leftover file: %s", e.Name())
		}
	}
}
