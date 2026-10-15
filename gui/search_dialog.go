package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const (
	// searchDelay is the pause after the last key before a search starts
	searchDelay = 300 * time.Millisecond
	// searchLimit is how many results are kept
	searchLimit = 500
	// pushEvery is how often a running search shows what it has found
	pushEvery = 150 * time.Millisecond
)

// searchDialog finds a pdf by its name, in a folder and the folders under it.
type searchDialog struct {
	u   *ui
	dlg dialog.Dialog

	query     *widget.Entry
	folder    *widget.Entry
	inFolders *widget.Check
	status    *widget.Label
	list      *widget.List
	openBtn   *widget.Button

	// results and selected are used by the window thread only
	results  []result
	selected int

	mu     sync.Mutex
	timer  *time.Timer
	cancel context.CancelFunc
}

// showSearch opens the dialog that finds a pdf by its name.
func (u *ui) showSearch() {
	if u.search != nil {
		u.search.dlg.Hide()
		u.search.stop()
	}
	d := &searchDialog{u: u, selected: -1}
	u.search = d
	d.build()
	d.dlg.Show()
	u.win.Canvas().Focus(d.query)
	d.run()
}

func (d *searchDialog) build() {
	u := d.u
	d.query = widget.NewEntry()
	d.query.SetPlaceHolder("part of the file name, ex: invoice 2024")
	d.query.OnChanged = func(string) { d.schedule() }
	d.query.OnSubmitted = func(string) { d.openChosen() }

	d.folder = widget.NewEntry()
	d.folder.SetText(d.startFolder())
	d.folder.OnChanged = func(string) { d.schedule() }
	choose := widget.NewButtonWithIcon("", theme.FolderOpenIcon(), d.chooseFolder)

	quick := widget.NewSelect(nil, nil)
	quick.PlaceHolder = "Go to..."
	folders := quickFolders(u.store.Dir)
	labels := make([]string, len(folders))
	for i, f := range folders {
		labels[i] = f.label
	}
	quick.Options = labels
	quick.OnChanged = func(label string) {
		for _, f := range folders {
			if f.label == label {
				d.folder.SetText(f.path)
			}
		}
	}

	d.inFolders = widget.NewCheck("Also match folder names", func(bool) { d.schedule() })
	d.status = widget.NewLabel("")
	d.status.Wrapping = fyne.TextWrapWord

	d.list = widget.NewList(
		func() int { return len(d.results) },
		func() fyne.CanvasObject {
			name := widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
			name.Truncation = fyne.TextTruncateEllipsis
			dir := widget.NewLabel("")
			dir.Truncation = fyne.TextTruncateEllipsis
			// the text changes after the row is laid out, so its place has a size of its own
			info := container.NewGridWrap(fyne.NewSize(210, 36), widget.NewLabel(""))
			return container.NewBorder(nil, nil, nil, info, container.NewVBox(name, dir))
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			if id >= len(d.results) {
				return
			}
			r := d.results[id]
			row := item.(*fyne.Container)
			info := row.Objects[1].(*fyne.Container).Objects[0].(*widget.Label)
			text := row.Objects[0].(*fyne.Container)
			text.Objects[0].(*widget.Label).SetText(filepath.Base(r.Path))
			text.Objects[1].(*widget.Label).SetText(filepath.Dir(r.Path))
			info.SetText(humanSize(r.Size) + "  " + r.ModTime.Format("2006-01-02 15:04"))
		},
	)
	d.list.OnSelected = func(id widget.ListItemID) {
		d.selected = id
		d.openBtn.Enable()
	}
	d.list.OnUnselected = func(widget.ListItemID) {
		d.selected = -1
		d.openBtn.Disable()
	}

	d.openBtn = widget.NewButtonWithIcon("Open", theme.ConfirmIcon(), d.openChosen)
	d.openBtn.Importance = widget.HighImportance
	d.openBtn.Disable()

	top := container.NewVBox(
		d.query,
		container.NewBorder(nil, nil, widget.NewLabel("In"), choose, d.folder),
		container.NewHBox(quick, d.inFolders),
	)
	bottom := container.NewBorder(nil, nil, nil, d.openBtn, d.status)
	d.dlg = dialog.NewCustom("Find a PDF", "Close", container.NewBorder(top, bottom, nil, nil, d.list), u.win)
	d.dlg.SetOnClosed(d.stop)
	size := u.win.Canvas().Size()
	d.dlg.Resize(fyne.NewSize(size.Width*0.85, size.Height*0.85))
}

// startFolder is where the search starts: where the last one was, or where the last pdf came from.
func (d *searchDialog) startFolder() string {
	for _, dir := range []string{d.u.store.SearchDir, d.u.store.Dir} {
		if dir != "" {
			if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
				return dir
			}
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return ""
}

type quickFolder struct{ label, path string }

// quickFolders are the places most pdf files are in, those that exist
func quickFolders(last string) []quickFolder {
	var out []quickFolder
	add := func(label, path string) {
		if path == "" {
			return
		}
		if fi, err := os.Stat(path); err == nil && fi.IsDir() {
			out = append(out, quickFolder{label, path})
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		add("Home", home)
		for _, name := range []string{"Documents", "Downloads", "Desktop"} {
			add(name, filepath.Join(home, name))
		}
	}
	if last != "" {
		add("Last opened: "+filepath.Base(last), last)
	}
	return out
}

func (d *searchDialog) chooseFolder() {
	fd := dialog.NewFolderOpen(func(dir fyne.ListableURI, err error) {
		if err != nil {
			dialog.ShowError(err, d.u.win)
			return
		}
		if dir != nil {
			d.folder.SetText(dir.Path())
		}
	}, d.u.win)
	if lister, err := storage.ListerForURI(storage.NewFileURI(d.folder.Text)); err == nil {
		fd.SetLocation(lister)
	}
	size := d.u.win.Canvas().Size()
	fd.Resize(fyne.NewSize(size.Width*0.8, size.Height*0.8))
	fd.Show()
}

// schedule starts a search after the user stopped typing.
func (d *searchDialog) schedule() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.timer != nil {
		d.timer.Stop()
	}
	d.timer = time.AfterFunc(searchDelay, func() { fyne.Do(d.run) })
}

// stop ends a search that is running and the one that is waiting to start.
func (d *searchDialog) stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.timer != nil {
		d.timer.Stop()
	}
	if d.cancel != nil {
		d.cancel()
		d.cancel = nil
	}
}

func (d *searchDialog) setStatus(text string, importance widget.Importance) {
	d.status.Importance = importance
	d.status.SetText(text)
}

func (d *searchDialog) setResults(rs []result) {
	d.results = rs
	d.selected = -1
	d.openBtn.Disable()
	d.list.UnselectAll()
	d.list.Refresh()
}

// run looks for what the dialog asks, in background; what is found is shown as it comes.
func (d *searchDialog) run() {
	d.stop()
	root := cleanPath(d.folder.Text)
	query := d.query.Text
	inFolders := d.inFolders.Checked
	d.setResults(nil)
	if root == "" {
		d.setStatus("Choose a folder to look in", widget.WarningImportance)
		return
	}
	d.setStatus("Searching...", widget.MediumImportance)

	ctx, cancel := context.WithCancel(context.Background())
	d.mu.Lock()
	d.cancel = cancel
	d.mu.Unlock()

	go func() {
		var (
			mu     sync.Mutex
			found  []result
			dirs   int
			lastUI time.Time
		)
		// push shows what is known, from the window thread; an old search shows nothing
		push := func(final bool, truncated bool, err error) {
			mu.Lock()
			snapshot := append([]result(nil), found...)
			n := dirs
			mu.Unlock()
			fyne.Do(func() {
				if ctx.Err() != nil {
					return
				}
				d.show(snapshot, n, final, truncated, err)
			})
		}
		truncated, err := searchPDFs(ctx, root, query, inFolders, searchLimit,
			func(n int) {
				mu.Lock()
				dirs = n
				due := time.Since(lastUI) > pushEvery
				if due {
					lastUI = time.Now()
				}
				mu.Unlock()
				if due {
					push(false, false, nil)
				}
			},
			func(r result) {
				mu.Lock()
				found = append(found, r)
				due := time.Since(lastUI) > pushEvery
				if due {
					lastUI = time.Now()
				}
				mu.Unlock()
				if due {
					push(false, false, nil)
				}
			})
		if ctx.Err() != nil {
			return
		}
		push(true, truncated, err)
	}()
}

// show puts what a search has found in the list, on the window thread.
func (d *searchDialog) show(rs []result, dirs int, final, truncated bool, err error) {
	switch {
	case err != nil:
		d.setResults(nil)
		d.setStatus(err.Error(), widget.DangerImportance)
		return
	case final:
		sortResults(rs)
	}
	// what is chosen stays chosen while more comes in
	var chosen string
	if d.selected >= 0 && d.selected < len(d.results) {
		chosen = d.results[d.selected].Path
	}
	d.results = rs
	d.list.Refresh()
	d.selected = -1
	for i, r := range rs {
		if chosen != "" && r.Path == chosen {
			d.list.Select(i)
			break
		}
	}
	if d.selected < 0 {
		d.openBtn.Disable()
	}

	switch {
	case !final:
		d.setStatus(fmt.Sprintf("Searching... %d found, %d folders looked in", len(rs), dirs), widget.MediumImportance)
	case truncated:
		d.setStatus(fmt.Sprintf("The first %d are shown, type more of the name to narrow the search", len(rs)), widget.WarningImportance)
	case len(rs) == 0:
		d.setStatus(fmt.Sprintf("Nothing found in %d folders", dirs), widget.WarningImportance)
	default:
		d.setStatus(fmt.Sprintf("%d found in %d folders, newest first", len(rs), dirs), widget.MediumImportance)
	}
}

// openChosen opens the chosen file, or the first one when none is chosen.
func (d *searchDialog) openChosen() {
	i := d.selected
	if i < 0 && len(d.results) > 0 {
		i = 0
	}
	if i < 0 || i >= len(d.results) {
		return
	}
	path := d.results[i].Path
	d.u.store.SearchDir = cleanPath(d.folder.Text)
	d.stop()
	d.dlg.Hide()
	d.u.search = nil
	d.u.openFile(path)
}

// bindSearchShortcut makes Ctrl+F open the search
func (u *ui) bindSearchShortcut() {
	u.win.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyF, Modifier: fyne.KeyModifierControl},
		func(fyne.Shortcut) { u.showSearch() })
}
