package database

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestLoadMigrationsOrdersByParsedVersion(t *testing.T) {
	files := fstest.MapFS{
		"migrations/0010_tenth.sql":  &fstest.MapFile{Data: []byte("SELECT 10;")},
		"migrations/0002_second.sql": &fstest.MapFile{Data: []byte("SELECT 2;")},
		"migrations/0001_first.sql":  &fstest.MapFile{Data: []byte("SELECT 1;")},
	}
	got, err := loadMigrationsFrom(files)
	if err != nil {
		t.Fatal(err)
	}
	wantVersions := []int{1, 2, 10}
	wantNames := []string{"first", "second", "tenth"}
	if len(got) != len(wantVersions) {
		t.Fatalf("migrations = %d, want %d", len(got), len(wantVersions))
	}
	for i, m := range got {
		if m.version != wantVersions[i] || m.name != wantNames[i] {
			t.Errorf("migration %d = (%d, %q), want (%d, %q)",
				i, m.version, m.name, wantVersions[i], wantNames[i])
		}
	}
}

func TestLoadMigrationsRejectsDuplicateVersion(t *testing.T) {
	files := fstest.MapFS{
		"migrations/0001_one.sql": &fstest.MapFile{Data: []byte("SELECT 1;")},
		"migrations/0001_two.sql": &fstest.MapFile{Data: []byte("SELECT 1;")},
	}
	_, err := loadMigrationsFrom(files)
	if err == nil || !strings.Contains(err.Error(), "duplicate migration version") {
		t.Fatalf("loadMigrationsFrom error = %v, want duplicate version error", err)
	}
}
