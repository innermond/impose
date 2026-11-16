package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
	"github.com/innermond/impose"
)

// pullAll puts the settings in all the widgets.
func (u *ui) pullAll() {
	u.loading = true
	defer func() { u.loading = false }()
	u.unit.SetSelected(u.s.Unit)
	u.modes.SetSelected(u.s.Mode)
	u.applyMode()
	for _, f := range u.pull {
		f()
	}
}

// suggestCreep refreshes Creep from Thickness, the source's page count and the current
// bleed, unless Creep already holds a value the user typed in directly rather than one
// suggestCreep put there itself - that value takes precedence and is left alone.
func (u *ui) suggestCreep() {
	if u.src == nil {
		return
	}
	cur := strings.TrimSpace(u.s.Creep)
	if cur != "" && cur != u.lastSuggestedCreep {
		return
	}
	up := u.unitPoints()
	p := &parser{unit: up}
	thickness := p.length("thickness", u.s.Thickness)
	bleedx := p.length("bleed x", u.s.BleedX)
	bleedy := p.length("bleed y", u.s.BleedY)
	suggested := impose.SuggestedCreep(thickness, u.src.NumPages, bleedx, bleedy)
	u.s.Creep = formatNumber(suggested / up)
	u.lastSuggestedCreep = u.s.Creep
	u.pullAll()
	u.changed()
}

// suggestBleed refreshes BleedX/BleedY from the open file's own suggestion (its BleedBox,
// or half the margin its original content already has past TrimBox), unless a field
// already holds a value the user typed in directly rather than one suggestBleed put there
// itself - that value takes precedence and is left alone, per axis independently.
func (u *ui) suggestBleed() {
	if u.src == nil {
		return
	}
	up := u.unitPoints()
	p := &parser{unit: up}
	trimw := p.length("trim width", u.s.TrimW)
	trimh := p.length("trim height", u.s.TrimH)
	sx, sy, err := u.src.SuggestedBleed(trimw, trimh)
	if err != nil {
		return
	}
	changed := false
	if cur := strings.TrimSpace(u.s.BleedX); cur == "" || cur == u.lastSuggestedBleedX {
		u.s.BleedX = formatNumber(sx / up)
		u.lastSuggestedBleedX = u.s.BleedX
		changed = true
	}
	if cur := strings.TrimSpace(u.s.BleedY); cur == "" || cur == u.lastSuggestedBleedY {
		u.s.BleedY = formatNumber(sy / up)
		u.lastSuggestedBleedY = u.s.BleedY
		changed = true
	}
	if changed {
		u.pullAll()
		u.changed()
	}
}

// updateCreepInfo refreshes creepInfo from the current Creep and the open file's page
// count: Booklet's step formula applies this per-page shift to one of the outermost
// sheet's two mate pages (the other gets its negative) - twice that apart from each other.
func (u *ui) updateCreepInfo() {
	if u.src == nil {
		u.creepInfo.SetText("")
		return
	}
	sheets := (u.src.NumPages + 3) / 4
	if sheets <= 1 {
		u.creepInfo.SetText("")
		return
	}
	up := u.unitPoints()
	p := &parser{unit: up}
	creep := p.length("creep", u.s.Creep)
	perPage := creep * 4 * float64(sheets-1) / float64(sheets)
	perSheet := 2 * perPage
	u.creepInfo.SetText(fmt.Sprintf(
		"Outermost sheet: %s%s per page, %s%s per sheet (twice that, the two faces moving apart)",
		formatNumber(perPage/up), u.s.Unit, formatNumber(perSheet/up), u.s.Unit))
}

// changed is called after every change of the settings: the preview and the saved settings follow.
func (u *ui) changed() {
	if u.loading {
		return
	}
	u.refreshCommand()
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.timer != nil {
		u.timer.Stop()
	}
	u.timer = time.AfterFunc(previewDelay, func() { fyne.Do(u.replan) })
	if u.saveTimer != nil {
		u.saveTimer.Stop()
	}
	u.saveTimer = time.AfterFunc(time.Second, u.saveStore)
}

func (u *ui) saveStore() {
	if u.storeFile == "" {
		return
	}
	fyne.Do(func() {
		_ = u.store.save(u.storeFile)
	})
}

// refreshCommand writes the command line that does the same
func (u *ui) refreshCommand() {
	file := u.srcPath
	if file == "" {
		file = "input.pdf"
	}
	cmd, err := u.s.command(file)
	if err != nil {
		cmd = ""
	}
	u.cmdText = cmd
	u.cmd.SetText(cmd)
}

func (u *ui) showMessage(msg string) {
	u.layout, u.guess = nil, nil
	u.prev.setLayout(nil, msg)
	u.useGuess.Hide()
	u.status.SetText("")
	u.clearBoxes()
	u.sheetChanged()
}

// clearBoxes empties the page boxes section, there is no file to show it for.
func (u *ui) clearBoxes() {
	u.mediaBoxLabel.SetText("")
	u.trimBoxLabel.SetText("")
	u.cropBoxLabel.SetText("")
}

// showBoxes displays the page boxes, in mm.
func (u *ui) showBoxes(pb impose.PageBoxes, err error) {
	if err != nil {
		u.clearBoxes()
		return
	}
	mm := impose.PointsPerMM
	box := func(w, h float64) string {
		return fmt.Sprintf("%s x %s mm", formatNumber(w/mm), formatNumber(h/mm))
	}
	u.mediaBoxLabel.SetText(box(pb.MediaW, pb.MediaH))
	u.trimBoxLabel.SetText(box(pb.TrimW, pb.TrimH))
	if pb.HasCrop {
		u.cropBoxLabel.SetText(box(pb.CropW, pb.CropH))
	} else {
		u.cropBoxLabel.SetText("not declared")
	}
}

func (u *ui) showError(err error) {
	u.layout, u.guess = nil, nil
	u.prev.setLayout(nil, "")
	u.useGuess.Hide()
	u.status.Importance = widget.DangerImportance
	u.status.SetText(err.Error())
	u.sheetChanged()
}

// paired tells the sheets come in pairs: a face then its back
func (u *ui) paired() bool {
	return u.s.Duplex && u.s.Mode == modeGrid || u.s.Mode == modeBooklet
}

// sheetChanged updates what tells which sheet is shown
func (u *ui) sheetChanged() {
	n := u.prev.sheets()
	if n == 0 {
		u.sheetLabel.SetText("")
		u.prevBtn.Disable()
		u.nextBtn.Disable()
		return
	}
	text := fmt.Sprintf("Sheet %d / %d", u.prev.sheet+1, n)
	if u.paired() {
		// sheets come in pairs: face then back
		if u.prev.sheet%2 == 0 {
			text += " face"
		} else {
			text += " back"
		}
	}
	u.sheetLabel.SetText(text)
	if u.prev.sheet > 0 {
		u.prevBtn.Enable()
	} else {
		u.prevBtn.Disable()
	}
	if u.prev.sheet < n-1 {
		u.nextBtn.Enable()
	} else {
		u.nextBtn.Disable()
	}
}

// replan works out the layout of the settings, in background, and shows it.
func (u *ui) replan() {
	if u.planCancel != nil {
		u.planCancel()
		u.planCancel = nil
	}
	if u.src == nil {
		u.showMessage("Open a PDF to see the imposition")
		return
	}
	opts, err := u.s.options()
	if err != nil {
		u.showError(err)
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	u.planCancel = cancel
	src := u.src
	go func() {
		l, err := impose.Plan(ctx, opts, src)
		if ctx.Err() != nil {
			// a newer plan took the place
			return
		}
		pb, pbErr := src.Boxes(opts.Bleedx, opts.Bleedy, opts.TrimWidth, opts.TrimHeight)
		fyne.Do(func() {
			u.showPlan(l, err)
			u.showBoxes(pb, pbErr)
		})
	}()
}

func (u *ui) showPlan(l *impose.Layout, err error) {
	u.prev.pairs = u.paired()
	if u.prev.pairs {
		u.seeThrough.Enable()
	} else {
		u.seeThrough.Disable()
	}
	var gg *impose.GridGuessError
	switch {
	case errors.As(err, &gg):
		u.layout, u.guess = nil, gg
		u.prev.setLayout(nil, "The grid is not set")
		u.status.Importance = widget.WarningImportance
		if gg.Col < 1 || gg.Row < 1 {
			u.status.SetText(gg.Error())
			u.useGuess.Hide()
		} else {
			u.status.SetText(fmt.Sprintf("The grid is not set: %d x %d pages fit on the sheet", gg.Col, gg.Row))
			u.useGuess.SetText(fmt.Sprintf("Use %d x %d", gg.Col, gg.Row))
			u.useGuess.Show()
		}
		u.sheetChanged()
		return
	case err != nil:
		u.showError(err)
		return
	}
	u.layout, u.guess = l, nil
	u.prev.setLayout(l, "")
	u.useGuess.Hide()
	u.status.Importance = widget.MediumImportance
	u.status.SetText(fmt.Sprintf("%d sheets of %s x %s %s, grid %d x %d",
		len(l.Sheets), formatNumber(l.Width/u.unitPoints()), formatNumber(l.Height/u.unitPoints()), u.s.Unit, l.Col, l.Row))
	u.sheetChanged()
}

func (u *ui) unitPoints() float64 {
	p, err := impose.UnitPoints(u.s.Unit)
	if err != nil {
		return 1
	}
	return p
}

// ---- the pdf

func (u *ui) openDialog() {
	d := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
		if err != nil {
			dialog.ShowError(err, u.win)
			return
		}
		if r == nil {
			return
		}
		path := r.URI().Path()
		r.Close()
		u.openFile(path)
	}, u.win)
	d.SetFilter(storage.NewExtensionFileFilter([]string{".pdf", ".PDF"}))
	if u.store.Dir != "" {
		if dir, err := storage.ListerForURI(storage.NewFileURI(u.store.Dir)); err == nil {
			d.SetLocation(dir)
		}
	}
	size := u.win.Canvas().Size()
	d.Resize(fyne.NewSize(size.Width*0.9, size.Height*0.9))
	d.Show()
}

// openFile reads a pdf and shows its imposition.
func (u *ui) openFile(path string) { u.openFileNote(path, "") }

// openFileNote is openFile with a note, shown after the name of the file.
func (u *ui) openFileNote(path, note string) {
	before := u.fileLabel.Text
	u.fileLabel.SetText("Reading " + filepath.Base(path) + "...")
	go func() {
		f, err := os.Open(path)
		var src *impose.Source
		var w, h float64
		if err == nil {
			if src, err = impose.OpenSource(f); err == nil {
				w, h, err = src.PageSize(0, 0, 0, 0)
			}
			if err != nil {
				f.Close()
			}
		}
		fyne.Do(func() {
			if err != nil {
				// what was open stays open
				u.fileLabel.SetText(before)
				if errors.Is(err, os.ErrNotExist) {
					u.store.forget(path)
					u.refreshRecent()
				}
				u.status.Importance = widget.DangerImportance
				u.status.SetText(fmt.Sprintf("%s: %v", filepath.Base(path), err))
				return
			}
			if u.file != nil {
				u.file.Close()
			}
			u.file, u.src, u.srcPath = f, src, path
			u.suggestBleed()
			u.prev.thumbs.setSource(src)
			u.store.Dir = filepath.Dir(path)
			u.store.remember(path)
			u.refreshRecent()
			u.pathEntry.SetText("")
			mm := impose.PointsPerMM
			label := fmt.Sprintf("%s  -  %d pages of %s x %s mm", filepath.Base(path), src.NumPages, formatNumber(w/mm), formatNumber(h/mm))
			if note != "" {
				label += "  (" + note + ")"
			}
			u.fileLabel.SetText(label)
			u.win.SetTitle("impose - " + filepath.Base(path))
			u.genBtn.Enable()
			u.updateCreepInfo()
			u.refreshCommand()
			u.replan()
		})
	}()
}

// openTyped opens the path that was typed or pasted.
func (u *ui) openTyped(text string) {
	path := cleanPath(text)
	if path == "" {
		return
	}
	u.openFile(path)
}

// dropped opens the first pdf of what was dropped on the window.
func (u *ui) dropped(_ fyne.Position, uris []fyne.URI) {
	var pdfs []string
	for _, uri := range uris {
		if uri.Scheme() == "file" && strings.EqualFold(filepath.Ext(uri.Path()), ".pdf") {
			pdfs = append(pdfs, uri.Path())
		}
	}
	if len(pdfs) == 0 {
		u.status.Importance = widget.WarningImportance
		u.status.SetText("Drop a PDF file")
		return
	}
	note := ""
	if ignored := len(uris) - 1; ignored > 0 {
		note = fmt.Sprintf("%d more dropped, ignored", ignored)
	}
	u.openFileNote(pdfs[0], note)
}

// recentLabels name the recent files; two files with the same name show their folder too.
func recentLabels(paths []string) []string {
	seen := map[string]int{}
	for _, p := range paths {
		seen[filepath.Base(p)]++
	}
	labels := make([]string, len(paths))
	for i, p := range paths {
		labels[i] = filepath.Base(p)
		if seen[labels[i]] > 1 {
			labels[i] += "  -  " + filepath.Dir(p)
		}
	}
	return labels
}

// refreshRecent shows the recent files in the dropdown
func (u *ui) refreshRecent() {
	u.recentPaths = append([]string(nil), u.store.Recent...)
	u.loading = true
	defer func() { u.loading = false }()
	u.recent.Options = recentLabels(u.recentPaths)
	u.recent.ClearSelected()
	if len(u.recentPaths) == 0 {
		u.recent.Disable()
	} else {
		u.recent.Enable()
	}
	u.recent.Refresh()
}

// openRecent opens a file of the dropdown
func (u *ui) openRecent(label string) {
	if u.loading {
		return
	}
	for i, l := range recentLabels(u.recentPaths) {
		if l == label {
			path := u.recentPaths[i]
			u.openFile(path)
			// the dropdown does not keep it selected, choosing it again must work
			u.loading = true
			u.recent.ClearSelected()
			u.loading = false
			return
		}
	}
}

// ---- generating the pdf

func (u *ui) generate() {
	if u.src == nil {
		return
	}
	opts, err := u.s.options()
	if err != nil {
		dialog.ShowError(err, u.win)
		return
	}
	d := dialog.NewFileSave(func(w fyne.URIWriteCloser, err error) {
		if err != nil {
			dialog.ShowError(err, u.win)
			return
		}
		if w == nil {
			return
		}
		u.run(opts, w)
	}, u.win)
	d.SetFilter(storage.NewExtensionFileFilter([]string{".pdf", ".PDF"}))
	name := strings.TrimSuffix(filepath.Base(u.srcPath), filepath.Ext(u.srcPath)) + "-imposed.pdf"
	d.SetFileName(name)
	size := u.win.Canvas().Size()
	d.Resize(fyne.NewSize(size.Width*0.9, size.Height*0.9))
	d.Show()
}

// run writes the imposed pdf to w, in background
func (u *ui) run(opts impose.Options, w fyne.URIWriteCloser) {
	ctx, cancel := context.WithCancel(context.Background())
	u.genCancel = cancel
	u.setBusy(true)
	src := u.src
	go func() {
		err := impose.Run(ctx, opts, src, w, nil)
		if cerr := w.Close(); err == nil {
			err = cerr
		}
		fyne.Do(func() {
			cancel()
			u.genCancel = nil
			u.setBusy(false)
			switch {
			case errors.Is(err, context.Canceled):
				// do not leave an empty file
				_ = os.Remove(w.URI().Path())
				u.status.Importance = widget.MediumImportance
				u.status.SetText("Cancelled")
			case err != nil:
				_ = os.Remove(w.URI().Path())
				dialog.ShowError(err, u.win)
			default:
				u.status.Importance = widget.SuccessImportance
				u.status.SetText("Saved " + w.URI().Path())
			}
		})
	}()
}

func (u *ui) setBusy(busy bool) {
	if busy {
		u.busy.Show()
		u.busy.Start()
		u.cancelBtn.Show()
		u.genBtn.Disable()
		return
	}
	u.busy.Stop()
	u.busy.Hide()
	u.cancelBtn.Hide()
	u.genBtn.Enable()
}

// ---- presets

func (u *ui) presetNames() []string {
	names := make([]string, 0, len(u.store.Presets))
	for n := range u.store.Presets {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func (u *ui) loadPreset(name string) {
	if u.loading {
		return
	}
	p, ok := u.store.Presets[name]
	if !ok {
		return
	}
	*u.s = p
	u.pullAll()
	u.changed()
}

func (u *ui) savePresetDialog() {
	e := widget.NewEntry()
	dialog.ShowForm("Save preset", "Save", "Cancel", []*widget.FormItem{widget.NewFormItem("Name", e)}, func(ok bool) {
		name := strings.TrimSpace(e.Text)
		if !ok || name == "" {
			return
		}
		u.store.Presets[name] = *u.s
		u.refreshPresets(name)
		u.saveStore()
	}, u.win)
}

func (u *ui) deletePreset() {
	name := u.presets.Selected
	if name == "" {
		return
	}
	dialog.ShowConfirm("Delete preset", fmt.Sprintf("Delete the preset %q?", name), func(ok bool) {
		if !ok {
			return
		}
		delete(u.store.Presets, name)
		u.refreshPresets("")
		u.saveStore()
	}, u.win)
}

func (u *ui) refreshPresets(selected string) {
	u.loading = true
	defer func() { u.loading = false }()
	u.presets.Options = u.presetNames()
	if selected == "" {
		u.presets.ClearSelected()
	} else {
		u.presets.SetSelected(selected)
	}
	u.presets.Refresh()
}

// close keeps the settings for the next run
func (u *ui) close() {
	if u.src != nil && u.file != nil {
		u.file.Close()
	}
	if u.storeFile != "" {
		_ = u.store.save(u.storeFile)
	}
}
