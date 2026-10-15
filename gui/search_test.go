package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

// tree makes files, with the given modification time, under dir: names ending in / are folders
func tree(t *testing.T, dir string, files map[string]time.Time) {
	t.Helper()
	for name, mod := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if strings.HasSuffix(name, "/") {
			if err := os.MkdirAll(path, 0o755); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("%PDF-1.4 "+name), 0o644); err != nil {
			t.Fatal(err)
		}
		if !mod.IsZero() {
			os.Chtimes(path, mod, mod)
		}
	}
}

func find(t *testing.T, root, query string, inFolders bool, limit int) ([]string, bool) {
	t.Helper()
	var names []string
	truncated, err := searchPDFs(context.Background(), root, query, inFolders, limit, nil, func(r result) {
		rel, _ := filepath.Rel(root, r.Path)
		names = append(names, filepath.ToSlash(rel))
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(names)
	return names, truncated
}

func TestSearchMatchesNames(t *testing.T) {
	dir := t.TempDir()
	tree(t, dir, map[string]time.Time{
		"Invoice 2024.pdf":           {},
		"invoice-2023.PDF":           {},
		"notes.txt":                  {},
		"work/Invoice march.pdf":     {},
		"work/report 2024.pdf":       {},
		"Invoices/scan0001.pdf":      {},
		"Invoices/2024/scan2.pdf":    {},
		".hidden/invoice secret.pdf": {},
		"node_modules/invoice.pdf":   {},
		"fake.pdf.txt":               {},
	})
	tt := []struct {
		query     string
		inFolders bool
		want      []string
	}{
		{"invoice", false, []string{"Invoice 2024.pdf", "invoice-2023.PDF", "work/Invoice march.pdf"}},
		{"INVOICE 2024", false, []string{"Invoice 2024.pdf"}},
		{"2024 invoice", false, []string{"Invoice 2024.pdf"}},
		{"2024", false, []string{"Invoice 2024.pdf", "work/report 2024.pdf"}},
		{"nothing like it", false, nil},
		// the names of folders count when asked
		{"invoices 2024", true, []string{"Invoices/2024/scan2.pdf"}},
		{"invoices", true, []string{"Invoices/2024/scan2.pdf", "Invoices/scan0001.pdf"}},
		{"invoices", false, nil},
		// an empty query is every pdf, but not the hidden or the noisy folders, nor other extensions
		{"", false, []string{"Invoice 2024.pdf", "Invoices/2024/scan2.pdf", "Invoices/scan0001.pdf", "invoice-2023.PDF", "work/Invoice march.pdf", "work/report 2024.pdf"}},
		{"   ", false, []string{"Invoice 2024.pdf", "Invoices/2024/scan2.pdf", "Invoices/scan0001.pdf", "invoice-2023.PDF", "work/Invoice march.pdf", "work/report 2024.pdf"}},
	}
	for _, tc := range tt {
		got, truncated := find(t, dir, tc.query, tc.inFolders, 100)
		if !reflect.DeepEqual(got, tc.want) || truncated {
			t.Errorf("%q (folders %v): got %v want %v", tc.query, tc.inFolders, got, tc.want)
		}
	}
}

func TestSearchLimit(t *testing.T) {
	dir := t.TempDir()
	files := map[string]time.Time{}
	for _, n := range []string{"a", "b", "c", "d", "e"} {
		files[n+".pdf"] = time.Time{}
	}
	tree(t, dir, files)
	got, truncated := find(t, dir, "", false, 3)
	if len(got) != 3 || !truncated {
		t.Errorf("limit 3: %v truncated %v", got, truncated)
	}
	// exactly as many as the limit is not cut
	got, truncated = find(t, dir, "", false, 5)
	if len(got) != 5 || truncated {
		t.Errorf("limit 5: %v truncated %v", got, truncated)
	}
}

func TestSearchCancel(t *testing.T) {
	dir := t.TempDir()
	tree(t, dir, map[string]time.Time{"a.pdf": {}, "b/c.pdf": {}})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	n := 0
	_, err := searchPDFs(ctx, dir, "", false, 10, nil, func(result) { n++ })
	if err != context.Canceled || n != 0 {
		t.Errorf("err %v found %d", err, n)
	}
	// cancelled while looking
	ctx, cancel = context.WithCancel(context.Background())
	files := map[string]time.Time{}
	for i := 0; i < 50; i++ {
		files[filepath.Join("d", string(rune('a'+i%26)), string(rune('a'+i/26))+".pdf")] = time.Time{}
	}
	tree(t, dir, files)
	n = 0
	_, err = searchPDFs(ctx, dir, "", false, 1000, nil, func(result) {
		n++
		if n == 5 {
			cancel()
		}
	})
	if err != context.Canceled || n < 5 || n > 6 {
		t.Errorf("stopped by cancel: err %v found %d", err, n)
	}
}

func TestSearchRootErrors(t *testing.T) {
	dir := t.TempDir()
	if _, err := searchPDFs(context.Background(), filepath.Join(dir, "none"), "", false, 1, nil, nil); err == nil {
		t.Error("a folder that is not there is an error")
	}
	file := filepath.Join(dir, "a.pdf")
	os.WriteFile(file, []byte("x"), 0o644)
	if _, err := searchPDFs(context.Background(), file, "", false, 1, nil, nil); err == nil || !strings.Contains(err.Error(), "not a folder") {
		t.Errorf("a file is not a folder: %v", err)
	}
}

func TestSearchSkipsWhatItCannotRead(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("permissions are not enforced")
	}
	dir := t.TempDir()
	tree(t, dir, map[string]time.Time{"open/a.pdf": {}, "locked/b.pdf": {}, "z.pdf": {}})
	locked := filepath.Join(dir, "locked")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(locked, 0o755)
	got, _ := find(t, dir, "", false, 10)
	if !reflect.DeepEqual(got, []string{"open/a.pdf", "z.pdf"}) {
		t.Errorf("got %v", got)
	}
}

func TestSearchLinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("links need rights")
	}
	dir, other := t.TempDir(), t.TempDir()
	tree(t, other, map[string]time.Time{"far.pdf": {}})
	tree(t, dir, map[string]time.Time{"near.pdf": {}})
	// a link to a file is found, a link to a folder is not entered, a loop does no harm
	os.Symlink(filepath.Join(other, "far.pdf"), filepath.Join(dir, "linked.pdf"))
	os.Symlink(other, filepath.Join(dir, "otherdir"))
	os.Symlink(dir, filepath.Join(dir, "loop"))
	os.Symlink(filepath.Join(dir, "nowhere.pdf"), filepath.Join(dir, "broken.pdf"))
	got, _ := find(t, dir, "", false, 10)
	if !reflect.DeepEqual(got, []string{"linked.pdf", "near.pdf"}) {
		t.Errorf("got %v", got)
	}
}

func TestSortResultsAndSize(t *testing.T) {
	now := time.Now()
	rs := []result{
		{Path: "b", ModTime: now.Add(-time.Hour)},
		{Path: "z", ModTime: now},
		{Path: "a", ModTime: now},
	}
	sortResults(rs)
	if rs[0].Path != "a" || rs[1].Path != "z" || rs[2].Path != "b" {
		t.Errorf("order %v", rs)
	}
	for n, want := range map[int64]string{0: "0 B", 1023: "1023 B", 1024: "1.0 KB", 1536: "1.5 KB", 5 << 20: "5.0 MB", 3 << 30: "3.0 GB"} {
		if got := humanSize(n); got != want {
			t.Errorf("humanSize(%d) = %q want %q", n, got, want)
		}
	}
}
