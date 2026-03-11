package main

import (
	"testing"

	"github.com/globulario/services/golang/backup_manager/backup_managerpb"
)

// --- validateServiceDataPath tests ---

func TestValidateServiceDataPath_Valid(t *testing.T) {
	valid := []string{
		"/var/lib/globular/title/associations",
		"/var/lib/globular/search/index",
		"/var/lib/globular/data/something",
		"/var/data/globular/mydata",
	}
	for _, p := range valid {
		if err := validateServiceDataPath(p); err != nil {
			t.Errorf("expected valid path %q, got error: %v", p, err)
		}
	}
}

func TestValidateServiceDataPath_Empty(t *testing.T) {
	if err := validateServiceDataPath(""); err == nil {
		t.Error("expected error for empty path")
	}
}

func TestValidateServiceDataPath_Relative(t *testing.T) {
	relative := []string{
		"var/lib/globular/data",
		"./data",
		"../etc/shadow",
	}
	for _, p := range relative {
		if err := validateServiceDataPath(p); err == nil {
			t.Errorf("expected error for relative path %q", p)
		}
	}
}

func TestValidateServiceDataPath_OutsideAllowed(t *testing.T) {
	bad := []string{
		"/etc/shadow",
		"/tmp/data",
		"/var/lib/other/data",
		"/home/user/files",
		"/var/lib/globular/../../etc/shadow", // traversal attack
	}
	for _, p := range bad {
		if err := validateServiceDataPath(p); err == nil {
			t.Errorf("expected error for disallowed path %q", p)
		}
	}
}

// --- filterServiceDataByPolicy tests ---

func newTestServer(enable, includeRebuildable, restoreRebuildable bool) *server {
	return &server{
		EnableServiceData:             enable,
		IncludeRebuildableServiceData: includeRebuildable,
		RestoreRebuildableServiceData: restoreRebuildable,
	}
}

func makeEntry(service, name, path, class string, exists bool) *backup_managerpb.ServiceDataEntry {
	return &backup_managerpb.ServiceDataEntry{
		ServiceName: service,
		DatasetName: name,
		Path:        path,
		DataClass:   class,
		PathExists:  exists,
	}
}

func TestFilterServiceData_Disabled(t *testing.T) {
	srv := newTestServer(false, true, false)
	entries := []*backup_managerpb.ServiceDataEntry{
		makeEntry("title", "assoc", "/var/lib/globular/title/assoc", "AUTHORITATIVE", true),
	}
	result := srv.filterServiceDataByPolicy(entries)
	if len(result) != 0 {
		t.Errorf("expected 0 entries when disabled, got %d", len(result))
	}
}

func TestFilterServiceData_AuthoritativeOnly(t *testing.T) {
	srv := newTestServer(true, false, false) // no rebuildable
	entries := []*backup_managerpb.ServiceDataEntry{
		makeEntry("title", "assoc", "/var/lib/globular/title/assoc", "AUTHORITATIVE", true),
		makeEntry("search", "bleve", "/var/lib/globular/search/index", "REBUILDABLE", true),
	}
	result := srv.filterServiceDataByPolicy(entries)
	if len(result) != 1 {
		t.Fatalf("expected 1 entry (authoritative only), got %d", len(result))
	}
	if result[0].DatasetName != "assoc" {
		t.Errorf("expected assoc entry, got %s", result[0].DatasetName)
	}
}

func TestFilterServiceData_IncludeRebuildable(t *testing.T) {
	srv := newTestServer(true, true, false)
	entries := []*backup_managerpb.ServiceDataEntry{
		makeEntry("title", "assoc", "/var/lib/globular/title/assoc", "AUTHORITATIVE", true),
		makeEntry("search", "bleve", "/var/lib/globular/search/index", "REBUILDABLE", true),
	}
	result := srv.filterServiceDataByPolicy(entries)
	if len(result) != 2 {
		t.Errorf("expected 2 entries with rebuildable included, got %d", len(result))
	}
}

func TestFilterServiceData_CacheNeverIncluded(t *testing.T) {
	srv := newTestServer(true, true, true) // everything enabled
	entries := []*backup_managerpb.ServiceDataEntry{
		makeEntry("title", "assoc", "/var/lib/globular/title/assoc", "AUTHORITATIVE", true),
		makeEntry("search", "tmp_cache", "/var/lib/globular/search/cache", "CACHE", true),
	}
	result := srv.filterServiceDataByPolicy(entries)
	if len(result) != 1 {
		t.Fatalf("expected 1 entry (CACHE excluded), got %d", len(result))
	}
	if result[0].DatasetName != "assoc" {
		t.Errorf("expected assoc entry, got %s", result[0].DatasetName)
	}
}

func TestFilterServiceData_RejectsInvalidPath(t *testing.T) {
	srv := newTestServer(true, true, false)
	entries := []*backup_managerpb.ServiceDataEntry{
		makeEntry("evil", "steal", "/etc/shadow", "AUTHORITATIVE", true),
		makeEntry("title", "assoc", "/var/lib/globular/title/assoc", "AUTHORITATIVE", true),
	}
	result := srv.filterServiceDataByPolicy(entries)
	if len(result) != 1 {
		t.Fatalf("expected 1 entry (invalid path rejected), got %d", len(result))
	}
	if result[0].DatasetName != "assoc" {
		t.Errorf("expected assoc entry, got %s", result[0].DatasetName)
	}
}

func TestFilterServiceData_RejectsNonExistentPath(t *testing.T) {
	srv := newTestServer(true, true, false)
	entries := []*backup_managerpb.ServiceDataEntry{
		makeEntry("title", "assoc", "/var/lib/globular/title/assoc", "AUTHORITATIVE", false), // PathExists=false
	}
	result := srv.filterServiceDataByPolicy(entries)
	if len(result) != 0 {
		t.Errorf("expected 0 entries (path does not exist), got %d", len(result))
	}
}

func TestFilterServiceData_DeduplicatesPaths(t *testing.T) {
	srv := newTestServer(true, true, false)
	entries := []*backup_managerpb.ServiceDataEntry{
		makeEntry("title", "assoc1", "/var/lib/globular/title/assoc", "AUTHORITATIVE", true),
		makeEntry("title", "assoc2", "/var/lib/globular/title/assoc", "AUTHORITATIVE", true), // same path
	}
	result := srv.filterServiceDataByPolicy(entries)
	if len(result) != 1 {
		t.Errorf("expected 1 entry (deduplicated), got %d", len(result))
	}
}

func TestFilterServiceData_TraversalAttack(t *testing.T) {
	srv := newTestServer(true, true, false)
	entries := []*backup_managerpb.ServiceDataEntry{
		makeEntry("evil", "traversal", "/var/lib/globular/../../etc/passwd", "AUTHORITATIVE", true),
	}
	result := srv.filterServiceDataByPolicy(entries)
	if len(result) != 0 {
		t.Errorf("expected 0 entries (traversal attack rejected), got %d", len(result))
	}
}

// --- serviceDataForRestore tests ---

func TestServiceDataForRestore_FilterByService(t *testing.T) {
	srv := newTestServer(true, true, true)
	entries := []*backup_managerpb.ServiceDataEntry{
		makeEntry("title", "assoc", "/var/lib/globular/title/assoc", "AUTHORITATIVE", true),
		makeEntry("search", "bleve", "/var/lib/globular/search/index", "REBUILDABLE", true),
	}
	result := srv.serviceDataForRestore(entries, []string{"title"})
	if len(result) != 1 {
		t.Fatalf("expected 1 entry filtered by service, got %d", len(result))
	}
	if result[0].ServiceName != "title" {
		t.Errorf("expected title entry, got %s", result[0].ServiceName)
	}
}

func TestServiceDataForRestore_SkipRebuildable(t *testing.T) {
	srv := newTestServer(true, true, false) // RestoreRebuildableServiceData=false
	entries := []*backup_managerpb.ServiceDataEntry{
		makeEntry("title", "assoc", "/var/lib/globular/title/assoc", "AUTHORITATIVE", true),
		makeEntry("search", "bleve", "/var/lib/globular/search/index", "REBUILDABLE", true),
	}
	result := srv.serviceDataForRestore(entries, nil) // all services
	if len(result) != 1 {
		t.Fatalf("expected 1 entry (rebuildable skipped), got %d", len(result))
	}
	if result[0].ServiceName != "title" {
		t.Errorf("expected title entry, got %s", result[0].ServiceName)
	}
}

func TestServiceDataForRestore_IncludeRebuildable(t *testing.T) {
	srv := newTestServer(true, true, true) // RestoreRebuildableServiceData=true
	entries := []*backup_managerpb.ServiceDataEntry{
		makeEntry("title", "assoc", "/var/lib/globular/title/assoc", "AUTHORITATIVE", true),
		makeEntry("search", "bleve", "/var/lib/globular/search/index", "REBUILDABLE", true),
	}
	result := srv.serviceDataForRestore(entries, nil)
	if len(result) != 2 {
		t.Errorf("expected 2 entries with rebuildable included in restore, got %d", len(result))
	}
}

func TestServiceDataForRestore_CacheNeverRestored(t *testing.T) {
	srv := newTestServer(true, true, true) // everything enabled
	entries := []*backup_managerpb.ServiceDataEntry{
		makeEntry("title", "assoc", "/var/lib/globular/title/assoc", "AUTHORITATIVE", true),
		makeEntry("search", "tmp_cache", "/var/lib/globular/search/cache", "CACHE", true),
	}
	result := srv.serviceDataForRestore(entries, nil)
	if len(result) != 1 {
		t.Fatalf("expected 1 entry (CACHE never restored), got %d", len(result))
	}
	if result[0].DatasetName != "assoc" {
		t.Errorf("expected assoc entry, got %s", result[0].DatasetName)
	}
}

// --- collectServiceDataEntries tests ---

func TestCollectServiceDataEntries(t *testing.T) {
	results := []*backup_managerpb.HookResult{
		{
			ServiceName: "title",
			Ok:          true,
			ServiceData: []*backup_managerpb.ServiceDataEntry{
				makeEntry("title", "assoc", "/var/lib/globular/title/assoc", "AUTHORITATIVE", true),
			},
		},
		{
			ServiceName: "search",
			Ok:          true,
			ServiceData: []*backup_managerpb.ServiceDataEntry{
				makeEntry("search", "bleve", "/var/lib/globular/search/index", "REBUILDABLE", true),
			},
		},
	}
	all := collectServiceDataEntries(results)
	if len(all) != 2 {
		t.Errorf("expected 2 entries collected, got %d", len(all))
	}
}

func TestCollectServiceDataEntries_Nil(t *testing.T) {
	all := collectServiceDataEntries(nil)
	if len(all) != 0 {
		t.Errorf("expected 0 entries for nil results, got %d", len(all))
	}
}
