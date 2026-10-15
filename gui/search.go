package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// result is a pdf found by a search.
type result struct {
	Path    string
	Size    int64
	ModTime time.Time
}

// skipDirs are folders that never hold what is looked for and are big or slow.
var skipDirs = map[string]bool{
	"node_modules":              true,
	"__pycache__":               true,
	"$RECYCLE.BIN":              true,
	"System Volume Information": true,
}

// errEnough stops the walk when there are as many results as wanted.
var errEnough = errors.New("enough results")

// searchPDFs looks in root, and the folders in it, for pdf files whose name has all the words of query,
// not minding capitals. With inFolders the names of the folders under root count too. An empty query
// finds every pdf. Hidden folders (starting with a dot) and links to folders are not entered, folders
// that can not be read are skipped. found is called for every pdf, progress now and then with the count of
// folders looked in. It stops after limit results (truncated is then true) or when ctx is done.
func searchPDFs(ctx context.Context, root, query string, inFolders bool, limit int,
	progress func(dirs int), found func(result)) (truncated bool, err error) {

	info, err := os.Stat(root)
	if err != nil {
		return false, err
	}
	if !info.IsDir() {
		return false, fmt.Errorf("%s is not a folder", root)
	}
	words := strings.Fields(strings.ToLower(query))
	dirs, count := 0, 0

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			// not readable: look elsewhere
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if path != root && (strings.HasPrefix(d.Name(), ".") || skipDirs[d.Name()]) {
				return fs.SkipDir
			}
			dirs++
			if progress != nil && dirs%50 == 0 {
				progress(dirs)
			}
			return nil
		}
		if !strings.EqualFold(filepath.Ext(d.Name()), ".pdf") {
			return nil
		}
		target := strings.ToLower(d.Name())
		if inFolders {
			if rel, err := filepath.Rel(root, path); err == nil {
				target = strings.ToLower(rel)
			}
		}
		for _, w := range words {
			if !strings.Contains(target, w) {
				return nil
			}
		}
		// a link to a pdf counts, Stat follows it
		fi, err := os.Stat(path)
		if err != nil || !fi.Mode().IsRegular() {
			return nil
		}
		if count >= limit {
			truncated = true
			return errEnough
		}
		count++
		found(result{Path: path, Size: fi.Size(), ModTime: fi.ModTime()})
		return nil
	})
	if errors.Is(err, errEnough) {
		err = nil
	}
	if progress != nil {
		progress(dirs)
	}
	return truncated, err
}

// sortResults puts the newest files first, files modified at the same time by path.
func sortResults(rs []result) {
	sort.SliceStable(rs, func(i, j int) bool {
		if !rs[i].ModTime.Equal(rs[j].ModTime) {
			return rs[i].ModTime.After(rs[j].ModTime)
		}
		return rs[i].Path < rs[j].Path
	})
}

// humanSize writes a size of file the way people read it.
func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	size, suffix := float64(n), "KMGT"
	i := -1
	for size >= unit && i < len(suffix)-1 {
		size /= unit
		i++
	}
	return fmt.Sprintf("%.1f %cB", size, suffix[i])
}
