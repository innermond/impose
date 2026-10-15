package main

import (
	"bytes"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/innermond/impose"
	"github.com/unidoc/unipdf/v3/creator"
	"github.com/unidoc/unipdf/v3/model"
)

// numberedPDF writes a pdf of n pages, each with its number
func numberedPDF(t *testing.T, n int) string {
	t.Helper()
	c := creator.New()
	c.SetPageSize(creator.PageSize{200, 300})
	for i := 1; i <= n; i++ {
		c.NewPage()
		if err := c.Draw(c.NewParagraph(fmt.Sprintf("P%d", i))); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(t.TempDir(), "in.pdf")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := c.Write(f); err != nil {
		t.Fatal(err)
	}
	return path
}

func newTestUI(t *testing.T) *ui {
	t.Helper()
	a := test.NewApp()
	t.Cleanup(a.Quit)
	w := test.NewWindow(nil)
	u := newUI(a, w, filepath.Join(t.TempDir(), "gui.json"))
	w.Resize(fyne.NewSize(1200, 800))
	return u
}

// waitFor waits for something that is done in background
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for i := 0; i < 250; i++ {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s", what)
}

func typeIn(t *testing.T, u *ui, key, text string) {
	t.Helper()
	e, ok := u.cache[key].(*widget.Entry)
	if !ok {
		t.Fatalf("no entry %q", key)
	}
	e.SetText(text)
}

// shot saves what the window looks like, when SHOT_DIR is set
func shot(t *testing.T, u *ui, name string) {
	dir := os.Getenv("SHOT_DIR")
	if dir == "" {
		return
	}
	img := u.win.Canvas().Capture()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".png"), buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func openAndWait(t *testing.T, u *ui, pages int) {
	t.Helper()
	u.openFile(numberedPDF(t, pages))
	waitFor(t, "the pdf", func() bool { return u.src != nil })
}

func TestStartsWithoutFile(t *testing.T) {
	u := newTestUI(t)
	if u.genBtn.Disabled() != true {
		t.Error("generate needs a pdf")
	}
	if u.prev.layout != nil || u.prev.msg == "" {
		t.Errorf("preview should ask for a pdf: %+v", u.prev)
	}
	shot(t, u, "start")
}

func TestGridPreviewFollowsTheForm(t *testing.T) {
	u := newTestUI(t)
	openAndWait(t, u, 4)

	// no grid yet: a suggestion
	waitFor(t, "grid suggestion", func() bool { return u.guess != nil })
	if !u.useGuess.Visible() {
		t.Error("the suggestion should be offered")
	}
	test.Tap(u.useGuess)
	waitFor(t, "layout", func() bool { return u.layout != nil })
	if u.s.Cols == "" || u.s.Rows == "" || u.guess != nil {
		t.Errorf("suggestion not used: %q x %q", u.s.Cols, u.s.Rows)
	}

	typeIn(t, u, "cols", "2")
	typeIn(t, u, "rows", "1")
	waitFor(t, "2x1", func() bool { return u.layout != nil && u.layout.Col == 2 && u.layout.Row == 1 })
	if n := len(u.layout.Sheets); n != 2 {
		t.Fatalf("4 pages on 2 columns are 2 sheets, got %d", n)
	}
	if u.prev.sheets() != 2 || u.sheetLabel.Text != "Sheet 1 / 2" {
		t.Errorf("label %q", u.sheetLabel.Text)
	}
	test.Tap(u.nextBtn)
	if u.sheetLabel.Text != "Sheet 2 / 2" || !u.nextBtn.Disabled() {
		t.Errorf("after next: %q", u.sheetLabel.Text)
	}
	if got := u.cmd.Text; got == "" || got[:7] != "impose " {
		t.Errorf("command %q", got)
	}
	// the command can not be edited
	u.cmd.SetText("rm -rf")
	if u.cmd.Text != u.cmdText || u.cmdText == "rm -rf" {
		t.Errorf("command was edited: %q", u.cmd.Text)
	}
	shot(t, u, "grid")

	// flow repeats pages: 1,1,2,2 then 3,3,4,4 are 8 places, 4 sheets of 2 columns
	typeIn(t, u, "flow", "1,1,2,2")
	waitFor(t, "flow", func() bool { return u.layout != nil && len(u.layout.Sheets) == 4 })
	if p := u.layout.Sheets[0].Placements; p[0].Page != 1 || p[1].Page != 1 {
		t.Errorf("flow sheet 1: %+v", p)
	}
	if p := u.layout.Sheets[3].Placements; p[0].Page != 4 || p[1].Page != 4 {
		t.Errorf("flow sheet 4: %+v", p)
	}

	// a wrong value is told, and the preview does not stay stale
	typeIn(t, u, "flow", "1,x")
	waitFor(t, "error", func() bool { return u.layout == nil && u.status.Text != "" })
	if u.status.Importance != widget.DangerImportance {
		t.Errorf("error should be a danger: %v", u.status.Importance)
	}
	shot(t, u, "error")
}

func TestModes(t *testing.T) {
	u := newTestUI(t)
	openAndWait(t, u, 8)

	u.modes.SetSelected(modeBooklet)
	waitFor(t, "booklet", func() bool { return u.layout != nil && len(u.layout.Sheets) == 4 })
	if p := u.layout.Sheets[0].Placements; p[0].Page != 8 || p[1].Page != 1 {
		t.Errorf("booklet sheet 1: %+v", p)
	}
	if u.sheetLabel.Text != "Sheet 1 / 4 face" {
		t.Errorf("label %q", u.sheetLabel.Text)
	}
	if u.acc.Items[3].Title != "Booklet" {
		t.Errorf("sections %q", u.acc.Items[3].Title)
	}
	typeIn(t, u, "creep", "0.4")
	typeIn(t, u, "turn", "180")
	waitFor(t, "turned faces", func() bool {
		return u.layout != nil && u.layout.Sheets[0].Placements[0].Angle == 180 && u.layout.Sheets[1].Placements[0].Angle == 0
	})
	shot(t, u, "booklet")

	u.modes.SetSelected(modeRepeat)
	typeIn(t, u, "clonex", "3")
	typeIn(t, u, "pages", "1-2")
	waitFor(t, "repeat", func() bool {
		return u.layout != nil && len(u.layout.Sheets) == 2 && len(u.layout.Sheets[0].Placements) == 3
	})
	if u.acc.Items[1].Title != "Repeat" || len(u.acc.Items) != 4 {
		t.Errorf("sections %q, %d", u.acc.Items[1].Title, len(u.acc.Items))
	}
	// the turn of the booklet does not stay hidden in repeat
	if p := u.layout.Sheets[0].Placements[0]; p.Angle != 0 {
		t.Errorf("repeat has the turn of the booklet: %v", p.Angle)
	}
	shot(t, u, "repeat")

	// back to grid: the fields of the grid come back with what was typed
	u.modes.SetSelected(modeGrid)
	typeIn(t, u, "pages", "")
	// clones are shared by the modes
	typeIn(t, u, "clonex", "1")
	typeIn(t, u, "cols", "4")
	typeIn(t, u, "rows", "1")
	typeIn(t, u, "weld", "2")
	if u.cache["weld"].(*widget.Entry).Disabled() != true {
		t.Error("weld needs duplex")
	}
	u.cache["duplex"].(*widget.Check).SetChecked(true)
	if u.cache["weld"].(*widget.Entry).Disabled() {
		t.Error("weld should be enabled in duplex")
	}
	waitFor(t, "duplex", func() bool { return u.layout != nil && u.layout.Col == 4 && len(u.layout.Sheets) == 2 })
	if u.sheetLabel.Text != "Sheet 1 / 2 face" {
		t.Errorf("label %q", u.sheetLabel.Text)
	}
	shot(t, u, "duplex")
}

func TestUnitFollowsLengths(t *testing.T) {
	u := newTestUI(t)
	u.unit.SetSelected("in")
	if u.s.Unit != "in" || u.s.Width != "12.5984" || u.s.Height != "17.7165" {
		t.Errorf("settings %q %q %q", u.s.Unit, u.s.Width, u.s.Height)
	}
	// the entries show it, and the size in points is the same
	if got := u.forms["sheet"].Items[0].Widget.(*fyne.Container).Objects[0].(*widget.Entry).Text; got != "12.5984" {
		t.Errorf("entry %q", got)
	}
	o, err := u.s.options()
	if err != nil || o.Width < 907 || o.Width > 908 {
		t.Errorf("width %v %v", o.Width, err)
	}
}

func TestPresetsAndSettingsAreKept(t *testing.T) {
	u := newTestUI(t)
	typeIn(t, u, "cols", "3")
	u.s.Width = "500"
	u.pullAll()
	u.store.Presets["wide"] = *u.s
	u.refreshPresets("wide")
	if len(u.presets.Options) != 1 {
		t.Errorf("presets %v", u.presets.Options)
	}

	*u.s = defaultSettings()
	u.pullAll()
	u.presets.SetSelected("wide")
	if u.s.Width != "500" || u.s.Cols != "3" {
		t.Errorf("preset not loaded: %+v", u.s)
	}
	u.close()
	again := loadStore(u.storeFile)
	if again.Current.Width != "500" || again.Presets["wide"].Cols != "3" {
		t.Errorf("not saved: %+v", again.Current)
	}
}

// fileWriter is where the save dialog would write
type fileWriter struct {
	*os.File
}

func (f fileWriter) URI() fyne.URI { return storage.NewFileURI(f.Name()) }

func TestGenerateWritesThePDF(t *testing.T) {
	u := newTestUI(t)
	openAndWait(t, u, 4)
	typeIn(t, u, "cols", "2")
	typeIn(t, u, "rows", "1")
	waitFor(t, "layout", func() bool { return u.layout != nil && u.layout.Col == 2 })

	out := filepath.Join(t.TempDir(), "out.pdf")
	f, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	opts, err := u.s.options()
	if err != nil {
		t.Fatal(err)
	}
	u.run(opts, fileWriter{f})
	if !u.cancelBtn.Visible() || !u.genBtn.Disabled() {
		t.Error("should show it is working")
	}
	waitFor(t, "saved", func() bool { return u.status.Text == "Saved "+out })
	if u.genBtn.Disabled() || u.cancelBtn.Visible() {
		t.Error("should be ready again")
	}
	rd, err := openPDF(out)
	if err != nil {
		t.Fatal(err)
	}
	if n, _ := rd.GetNumPages(); n != 2 {
		t.Errorf("sheets %d", n)
	}
}

func TestGenerateCancelLeavesNoFile(t *testing.T) {
	u := newTestUI(t)
	openAndWait(t, u, 4)
	typeIn(t, u, "cols", "2")
	typeIn(t, u, "rows", "1")
	out := filepath.Join(t.TempDir(), "out.pdf")
	f, _ := os.Create(out)
	opts, _ := u.s.options()
	// a stopped imposition gives no pdf
	u.run(opts, fileWriter{f})
	u.cancelBtn.OnTapped()
	waitFor(t, "end", func() bool { return !u.cancelBtn.Visible() })
	if _, err := os.Stat(out); err == nil && u.status.Text == "Cancelled" {
		t.Error("the empty file should be removed")
	}
}

func TestOpenBrokenFile(t *testing.T) {
	u := newTestUI(t)
	bad := filepath.Join(t.TempDir(), "bad.pdf")
	os.WriteFile(bad, []byte("not a pdf"), 0o644)
	u.openFile(bad)
	waitFor(t, "label", func() bool { return u.fileLabel.Text == "No file" })
	if u.src != nil || !u.genBtn.Disabled() {
		t.Error("a broken file must not be used")
	}
}

func openPDF(path string) (*model.PdfReader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return model.NewPdfReader(f)
}

var _ = impose.ModeGrid

// a busy layout, for a look at the preview: clones with gaps, margins, marks, rotation
func TestPreviewOfClonesAndMarks(t *testing.T) {
	u := newTestUI(t)
	openAndWait(t, u, 4)
	u.modes.SetSelected(modeRepeat)
	typeIn(t, u, "clonex", "2")
	typeIn(t, u, "cloney", "2")
	typeIn(t, u, "padx", "6")
	typeIn(t, u, "pady", "6")
	u.s.Width, u.s.Height, u.s.Left, u.s.Top, u.s.Bleed = "320", "450", "14", "14", "3"
	u.pullAll()
	u.changed()
	waitFor(t, "clones", func() bool { return u.layout != nil && len(u.layout.Sheets[0].Placements) == 4 })
	if n := len(u.layout.Sheets[0].Marks); n == 0 {
		t.Error("marks expected")
	}
	// the clones are apart by the gap
	p := u.layout.Sheets[0].Placements
	if gap := p[1].X - (p[0].X + p[0].W); gap < 6*impose.PointsPerMM-0.01 || gap > 6*impose.PointsPerMM+0.01 {
		t.Errorf("gap %v", gap)
	}
	shot(t, u, "clones")
}

// ---- quick open

func TestOpenTypedPath(t *testing.T) {
	u := newTestUI(t)
	path := numberedPDF(t, 4)

	// quotes around a pasted path are fine
	u.pathEntry.SetText(`"` + path + `"`)
	u.pathEntry.OnSubmitted(u.pathEntry.Text)
	waitFor(t, "the pdf", func() bool { return u.src != nil })
	if u.srcPath != path || u.pathEntry.Text != "" {
		t.Errorf("opened %q, entry %q", u.srcPath, u.pathEntry.Text)
	}
	if len(u.store.Recent) != 1 || u.recent.Disabled() || len(u.recent.Options) != 1 {
		t.Errorf("recent %v %v", u.store.Recent, u.recent.Options)
	}
	label := u.fileLabel.Text

	// a path that is not there says so and leaves the open file as it is
	u.openTyped(filepath.Join(t.TempDir(), "missing.pdf"))
	waitFor(t, "error", func() bool { return u.status.Importance == widget.DangerImportance })
	if u.srcPath != path || u.fileLabel.Text != label || !u.genBtn.Visible() || u.genBtn.Disabled() {
		t.Errorf("the open file was lost: %q %q", u.srcPath, u.fileLabel.Text)
	}
	if len(u.store.Recent) != 1 {
		t.Errorf("a missing file is not remembered: %v", u.store.Recent)
	}

	// empty does nothing
	u.openTyped("  ")
	shot(t, u, "quickopen")
}

func TestDropFiles(t *testing.T) {
	u := newTestUI(t)
	pdf := numberedPDF(t, 4)
	uri := func(p string) fyne.URI { return storage.NewFileURI(p) }

	// no pdf in what was dropped
	u.dropped(fyne.NewPos(0, 0), []fyne.URI{uri("/tmp/notes.txt")})
	if u.src != nil || u.status.Text != "Drop a PDF file" {
		t.Errorf("status %q", u.status.Text)
	}

	// a pdf with other files: the pdf is opened, the rest said to be ignored
	u.dropped(fyne.NewPos(0, 0), []fyne.URI{uri("/tmp/notes.txt"), uri(pdf), uri("/tmp/other.pdf")})
	waitFor(t, "the pdf", func() bool { return u.src != nil })
	if u.srcPath != pdf {
		t.Errorf("opened %q", u.srcPath)
	}
	if want := "(2 more dropped, ignored)"; !strings.HasSuffix(u.fileLabel.Text, want) {
		t.Errorf("label %q", u.fileLabel.Text)
	}

	// the extension may be in capitals
	upper := filepath.Join(t.TempDir(), "SCAN.PDF")
	data, _ := os.ReadFile(pdf)
	os.WriteFile(upper, data, 0o644)
	u.dropped(fyne.NewPos(0, 0), []fyne.URI{uri(upper)})
	waitFor(t, "capitals", func() bool { return u.srcPath == upper })
	if strings.Contains(u.fileLabel.Text, "ignored") {
		t.Errorf("label %q", u.fileLabel.Text)
	}
}

func TestRecentDropdown(t *testing.T) {
	u := newTestUI(t)
	if !u.recent.Disabled() {
		t.Error("nothing to choose yet")
	}
	a, b := numberedPDF(t, 4), numberedPDF(t, 8)
	u.openFile(a)
	waitFor(t, "a", func() bool { return u.srcPath == a })
	u.openFile(b)
	waitFor(t, "b", func() bool { return u.srcPath == b })
	if !reflect.DeepEqual(u.store.Recent, []string{b, a}) {
		t.Fatalf("recent %v", u.store.Recent)
	}
	// both are called in.pdf: the folder tells them apart
	if len(u.recent.Options) != 2 || u.recent.Options[0] == u.recent.Options[1] {
		t.Errorf("labels %v", u.recent.Options)
	}

	// choosing the older one opens it
	u.recent.SetSelected(u.recent.Options[1])
	waitFor(t, "reopen a", func() bool { return u.srcPath == a })
	if u.store.Recent[0] != a || u.recent.Selected != "" {
		t.Errorf("recent %v selected %q", u.store.Recent, u.recent.Selected)
	}

	// a file that is gone leaves the list when it is chosen
	if err := os.Remove(b); err != nil {
		t.Fatal(err)
	}
	u.recent.SetSelected(u.recent.Options[1])
	waitFor(t, "forgotten", func() bool { return len(u.store.Recent) == 1 })
	if u.srcPath != a || len(u.recent.Options) != 1 {
		t.Errorf("open %q, options %v", u.srcPath, u.recent.Options)
	}
}

// ---- the search dialog

// pdfFiles makes real pdfs with the given names (and a time in the past, in hours) in dir
func pdfFiles(t *testing.T, dir string, hoursAgo map[string]int) {
	t.Helper()
	data, err := os.ReadFile(numberedPDF(t, 2))
	if err != nil {
		t.Fatal(err)
	}
	for name, h := range hoursAgo {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		mod := time.Now().Add(-time.Duration(h) * time.Hour)
		os.Chtimes(path, mod, mod)
	}
}

// searchUI opens the dialog on a folder, nothing outside of it is looked at
func searchUI(t *testing.T, files map[string]int) (*ui, *searchDialog, string) {
	t.Helper()
	u := newTestUI(t)
	dir := t.TempDir()
	pdfFiles(t, dir, files)
	u.store.SearchDir = dir
	u.showSearch()
	if u.search == nil || u.search.folder.Text != dir {
		t.Fatalf("the dialog should start in %s", dir)
	}
	t.Cleanup(u.search.stop)
	return u, u.search, dir
}

func TestSearchDialogFindsAndOpens(t *testing.T) {
	u, d, dir := searchUI(t, map[string]int{
		"Invoice 2024.pdf":       48,
		"invoice-2023.pdf":       2,
		"report.pdf":             1,
		"work/Invoice march.pdf": 24,
	})
	// every pdf to begin with, newest first
	waitFor(t, "all", func() bool {
		return len(d.results) == 4 && d.status.Text != "" && d.status.Importance == widget.MediumImportance && strings.Contains(d.status.Text, "found in")
	})
	if filepath.Base(d.results[0].Path) != "report.pdf" || filepath.Base(d.results[3].Path) != "Invoice 2024.pdf" {
		t.Errorf("order %v", d.results)
	}

	d.query.SetText("invoice")
	waitFor(t, "invoice", func() bool { return len(d.results) == 3 && strings.HasPrefix(d.status.Text, "3 found") })
	want := []string{"invoice-2023.pdf", "Invoice march.pdf", "Invoice 2024.pdf"}
	for i, r := range d.results {
		if filepath.Base(r.Path) != want[i] {
			t.Errorf("result %d is %s, want %s", i, filepath.Base(r.Path), want[i])
		}
	}
	if !d.openBtn.Disabled() {
		t.Error("nothing is chosen yet")
	}
	shot(t, u, "search")

	// choose the second, and open it
	d.list.Select(1)
	if d.openBtn.Disabled() {
		t.Fatal("a result is chosen")
	}
	test.Tap(d.openBtn)
	waitFor(t, "opened", func() bool { return u.src != nil })
	if u.srcPath != filepath.Join(dir, "work", "Invoice march.pdf") {
		t.Errorf("opened %q", u.srcPath)
	}
	if u.search != nil || u.store.SearchDir != dir {
		t.Errorf("dialog should be closed and keep the folder: %v %q", u.search, u.store.SearchDir)
	}
	if u.store.Recent[0] != u.srcPath {
		t.Errorf("recent %v", u.store.Recent)
	}
}

func TestSearchDialogEnterOpensTheFirst(t *testing.T) {
	u, d, dir := searchUI(t, map[string]int{"a scan.pdf": 5, "b scan.pdf": 1})
	d.query.SetText("scan")
	waitFor(t, "results", func() bool { return len(d.results) == 2 && strings.HasPrefix(d.status.Text, "2 found") })
	d.query.OnSubmitted(d.query.Text)
	waitFor(t, "opened", func() bool { return u.src != nil })
	if u.srcPath != filepath.Join(dir, "b scan.pdf") {
		t.Errorf("the newest is first: %q", u.srcPath)
	}
}

func TestSearchDialogNothingAndErrors(t *testing.T) {
	_, d, dir := searchUI(t, map[string]int{"a.pdf": 1})
	d.query.SetText("zzz")
	waitFor(t, "nothing", func() bool { return strings.HasPrefix(d.status.Text, "Nothing found") })
	if len(d.results) != 0 || !d.openBtn.Disabled() {
		t.Errorf("results %v", d.results)
	}
	// enter with nothing to open does nothing
	d.query.OnSubmitted("zzz")

	d.folder.SetText(filepath.Join(dir, "no such folder"))
	waitFor(t, "error", func() bool { return d.status.Importance == widget.DangerImportance })

	d.folder.SetText(filepath.Join(dir, "a.pdf"))
	waitFor(t, "not a folder", func() bool { return strings.Contains(d.status.Text, "not a folder") })

	d.folder.SetText("")
	waitFor(t, "no folder", func() bool { return strings.HasPrefix(d.status.Text, "Choose a folder") })

	// and it recovers
	d.folder.SetText(`"` + dir + `"`)
	d.query.SetText("")
	waitFor(t, "back", func() bool { return len(d.results) == 1 })
}

func TestSearchDialogFolderNames(t *testing.T) {
	_, d, _ := searchUI(t, map[string]int{"Contracts/2024/final.pdf": 1, "other.pdf": 1})
	d.query.SetText("contracts")
	waitFor(t, "none", func() bool { return strings.HasPrefix(d.status.Text, "Nothing found") })
	d.inFolders.SetChecked(true)
	waitFor(t, "by folder", func() bool { return len(d.results) == 1 && filepath.Base(d.results[0].Path) == "final.pdf" })
}

func TestSearchDialogLimit(t *testing.T) {
	_, d, dir := searchUI(t, nil)
	data := []byte("%PDF-1.4")
	for i := 0; i < searchLimit+20; i++ {
		os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%04d.pdf", i)), data, 0o644)
	}
	d.query.SetText("f0")
	waitFor(t, "limit", func() bool {
		return len(d.results) == searchLimit && strings.HasPrefix(d.status.Text, fmt.Sprintf("The first %d", searchLimit))
	})
	if d.status.Importance != widget.WarningImportance {
		t.Errorf("importance %v", d.status.Importance)
	}
}

func TestSearchDialogReopen(t *testing.T) {
	u, first, dir := searchUI(t, map[string]int{"a.pdf": 1})
	waitFor(t, "first", func() bool { return len(first.results) == 1 })
	// opened again (Ctrl+F does this; the test canvas can not send keys), the old search is stopped
	u.showSearch()
	if u.search == nil || u.search == first {
		t.Fatal("a new dialog is expected")
	}
	second := u.search
	t.Cleanup(second.stop)
	waitFor(t, "second", func() bool { return len(second.results) == 1 })
	if second.folder.Text != dir || len(first.results) != 1 {
		t.Errorf("folder %q", second.folder.Text)
	}
	// the dialog starts where the last pdf came from when no search was done
	u2 := newTestUI(t)
	u2.store.Dir = dir
	if got := (&searchDialog{u: u2}).startFolder(); got != dir {
		t.Errorf("start folder %q", got)
	}
	u2.store.Dir = filepath.Join(dir, "gone")
	if got := (&searchDialog{u: u2}).startFolder(); got == "" || got == u2.store.Dir {
		t.Errorf("a folder that is gone is not used: %q", got)
	}
}

func TestQuickFolders(t *testing.T) {
	dir := t.TempDir()
	got := quickFolders(dir)
	found := false
	for _, f := range got {
		if f.path == dir && strings.HasPrefix(f.label, "Last opened: ") {
			found = true
		}
		if fi, err := os.Stat(f.path); err != nil || !fi.IsDir() {
			t.Errorf("%v is not a folder", f)
		}
	}
	if !found {
		t.Errorf("the last folder should be offered: %v", got)
	}
	for _, f := range quickFolders(filepath.Join(dir, "gone")) {
		if strings.HasPrefix(f.label, "Last opened") {
			t.Errorf("gone folder offered: %v", f)
		}
	}
}
