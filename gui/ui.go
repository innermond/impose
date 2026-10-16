package main

import (
	"context"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/innermond/impose"
)

// delay between the last change in the form and the new preview
const previewDelay = 250 * time.Millisecond

// ui is the window: the form on the left, the preview on the right.
type ui struct {
	app fyne.App
	win fyne.Window

	store     store
	storeFile string
	s         *Settings

	// the pdf to impose
	file    *os.File
	src     *impose.Source
	srcPath string

	// what the last plan gave
	layout *impose.Layout
	guess  *impose.GridGuessError

	// widgets
	fileLabel *widget.Label
	pathEntry *widget.Entry
	recent    *widget.Select
	search    *searchDialog
	// recentPaths are the files the options of recent stand for
	recentPaths []string
	prev        *preview
	seeThrough  *widget.Select
	sheetLabel  *widget.Label
	prevBtn     *widget.Button
	nextBtn     *widget.Button
	status      *widget.Label
	useGuess    *widget.Button
	cmd         *widget.Entry
	genBtn      *widget.Button
	cancelBtn   *widget.Button
	busy        *widget.ProgressBarInfinite
	modes       *widget.RadioGroup
	unit        *widget.Select
	presets     *widget.Select
	acc         *widget.Accordion
	forms       map[string]*widget.Form

	// cmdText is the command line shown
	cmdText string
	// widgets used by some modes only, they are made once
	cache map[string]fyne.CanvasObject
	// pull copies settings into the widgets
	pull []func()
	// loading is true while the widgets are changed by the program, not by the user
	loading bool

	mu         sync.Mutex
	timer      *time.Timer
	saveTimer  *time.Timer
	planCancel context.CancelFunc
	genCancel  context.CancelFunc
}

func newUI(app fyne.App, win fyne.Window, storeFile string) *ui {
	u := &ui{app: app, win: win, storeFile: storeFile}
	u.store = loadStore(storeFile)
	u.s = &u.store.Current
	u.build()
	win.SetOnDropped(u.dropped)
	u.bindSearchShortcut()
	return u
}

// ---- widgets bound to settings

func (u *ui) entry(p *string, placeholder string) *widget.Entry {
	e := widget.NewEntry()
	e.SetPlaceHolder(placeholder)
	e.SetText(*p)
	e.OnChanged = func(t string) {
		*p = t
		u.changed()
	}
	u.pull = append(u.pull, func() { e.SetText(*p) })
	return e
}

func (u *ui) check(label string, p *bool, after func()) *widget.Check {
	c := widget.NewCheck(label, nil)
	c.SetChecked(*p)
	c.OnChanged = func(v bool) {
		*p = v
		if after != nil {
			after()
		}
		u.changed()
	}
	u.pull = append(u.pull, func() { c.SetChecked(*p) })
	return c
}

func item(label string, o fyne.CanvasObject) *widget.FormItem { return widget.NewFormItem(label, o) }

func pair(a, b fyne.CanvasObject) fyne.CanvasObject { return container.NewGridWithColumns(2, a, b) }

// build makes the whole window content.
func (u *ui) build() {
	// widgets are set to the settings, that is not a change made by the user
	u.loading = true
	defer func() { u.loading = false }()
	s := u.s
	u.forms = map[string]*widget.Form{}

	// sheet
	width, height := u.entry(&s.Width, ""), u.entry(&s.Height, "")
	padding := u.entry(&s.Autopadding, "")
	auto := u.check("Size the sheet to the pages", &s.Autopage, func() {
		if s.Autopage {
			width.Disable()
			height.Disable()
			padding.Enable()
		} else {
			width.Enable()
			height.Enable()
			padding.Disable()
		}
	})
	if s.Autopage {
		width.Disable()
		height.Disable()
	} else {
		padding.Disable()
	}
	u.pull = append(u.pull, func() { auto.OnChanged(s.Autopage) })
	u.forms["sheet"] = widget.NewForm(
		item("Sheet", pair(width, height)),
		item("", auto),
		item("Padding", padding),
		item("Margin top, right", pair(u.entry(&s.Top, "0"), u.entry(&s.Right, "0"))),
		item("Margin bottom, left", pair(u.entry(&s.Bottom, "0"), u.entry(&s.Left, "0"))),
		item("Center", container.NewHBox(u.check("across", &s.CenterX, nil), u.check("down", &s.CenterY, nil))),
		item("Rotate pages", u.entry(&s.Angle, "degrees, ex: 90")),
	)

	// grid
	u.forms["grid"] = widget.NewForm()

	// pages
	u.forms["pages"] = widget.NewForm()

	// duplex, booklet, rotation
	u.forms["flip"] = widget.NewForm()

	// marks
	u.forms["marks"] = widget.NewForm(
		item("", u.check("Cropmarks", &s.Marks, nil)),
		item("Bleed", u.entry(&s.Bleed, "0")),
		item("Distance from trim", u.entry(&s.Offset, "")),
		item("Mark size", u.entry(&s.MarkSize, "")),
		item("Only on sheets", u.entry(&s.MarkPages, "all, ex: 1,3")),
		item("", u.check("Draw a box around pages", &s.Outline, nil)),
	)

	u.acc = widget.NewAccordion()
	u.acc.MultiOpen = true

	// top bar
	open := widget.NewButtonWithIcon("Open PDF...", theme.FolderOpenIcon(), u.openDialog)
	u.fileLabel = widget.NewLabel("No file")
	u.fileLabel.Truncation = fyne.TextTruncateEllipsis
	u.modes = widget.NewRadioGroup([]string{modeGrid, modeRepeat, modeBooklet}, func(m string) {
		if m == "" {
			// choosing the selected mode again would clear it
			u.modes.SetSelected(s.Mode)
			return
		}
		if u.loading {
			return
		}
		s.Mode = m
		u.applyMode()
		u.changed()
	})
	u.modes.Horizontal = true
	u.modes.SetSelected(s.Mode)
	u.unit = widget.NewSelect(impose.Units, func(unit string) {
		if u.loading || unit == s.Unit {
			return
		}
		if err := s.convertUnit(unit); err != nil {
			dialog.ShowError(err, u.win)
			return
		}
		u.pullAll()
		u.changed()
	})
	u.unit.SetSelected(s.Unit)
	u.presets = widget.NewSelect(u.presetNames(), u.loadPreset)
	u.presets.PlaceHolder = "Presets"
	savePreset := widget.NewButtonWithIcon("", theme.DocumentSaveIcon(), u.savePresetDialog)
	delPreset := widget.NewButtonWithIcon("", theme.DeleteIcon(), u.deletePreset)
	reset := widget.NewButtonWithIcon("", theme.ViewRefreshIcon(), func() {
		src := s.Mode
		*s = defaultSettings()
		s.Mode = src
		u.pullAll()
		u.changed()
	})
	find := widget.NewButtonWithIcon("Find...", theme.SearchIcon(), u.showSearch)
	u.recent = widget.NewSelect(nil, u.openRecent)
	u.recent.PlaceHolder = "Recent files"
	u.pathEntry = widget.NewEntry()
	u.pathEntry.SetPlaceHolder("or paste the path of a PDF and press Enter, or drop the file here")
	u.pathEntry.OnSubmitted = u.openTyped
	u.refreshRecent()
	top := container.NewVBox(
		container.NewBorder(nil, nil, container.NewHBox(open, find), nil, u.recent),
		u.pathEntry,
		u.fileLabel,
		container.NewHBox(u.modes, widget.NewLabel("Unit"), u.unit),
		container.NewBorder(nil, nil, nil, container.NewHBox(savePreset, delPreset, reset), u.presets),
	)

	// preview
	u.prev = newPreview()
	u.prevBtn = widget.NewButtonWithIcon("", theme.NavigateBackIcon(), func() { u.prev.setSheet(u.prev.sheet - 1); u.sheetChanged() })
	u.nextBtn = widget.NewButtonWithIcon("", theme.NavigateNextIcon(), func() { u.prev.setSheet(u.prev.sheet + 1); u.sheetChanged() })
	u.sheetLabel = widget.NewLabel("")
	zoom := widget.NewSelect([]string{"Fit", "25%", "50%", "100%", "200%"}, func(z string) {
		if z == "Fit" {
			u.prev.setScale(0)
			return
		}
		pct, _ := strconv.Atoi(strings.TrimSuffix(z, "%"))
		u.prev.setScale(float64(pct) / 100)
	})
	zoom.SetSelected("Fit")
	u.prev.showThumbs = !u.store.HideThumbs
	pages := widget.NewCheck("Pages", func(on bool) {
		u.prev.showThumbs = on
		u.prev.Refresh()
		if u.store.HideThumbs == on {
			u.store.HideThumbs = !on
			u.saveStore()
		}
	})
	pages.SetChecked(!u.store.HideThumbs)
	sees := []string{"No see-through", "See-through, side turn", "See-through, head turn"}
	u.seeThrough = widget.NewSelect(sees, func(s string) {
		for i, v := range sees {
			if v == s && u.store.SeeThrough != i {
				u.store.SeeThrough = i
				u.saveStore()
			}
		}
		u.prev.see = u.store.SeeThrough
		u.prev.Refresh()
	})
	u.seeThrough.SetSelectedIndex(clamp(u.store.SeeThrough, seeNone, seeHead))
	u.seeThrough.Disable()
	// the label changes its text, so it has a place of its own
	bar := container.NewBorder(nil, nil, container.NewHBox(u.prevBtn, container.NewGridWrap(fyne.NewSize(170, 36), u.sheetLabel), u.nextBtn),
		container.NewHBox(pages, u.seeThrough, zoom))
	u.status = widget.NewLabel("")
	u.status.Wrapping = fyne.TextWrapWord
	u.useGuess = widget.NewButton("", func() {
		if u.guess == nil {
			return
		}
		s.Cols, s.Rows = strconv.Itoa(u.guess.Col), strconv.Itoa(u.guess.Row)
		u.pullAll()
		u.changed()
	})
	u.useGuess.Hide()
	right := container.NewBorder(bar, container.NewVBox(u.status, u.useGuess), nil, nil, container.NewScroll(u.prev))

	left := container.NewVScroll(u.acc)
	split := container.NewHSplit(left, right)
	split.Offset = 0.36

	// bottom: the equivalent command and the generate button
	// it is an entry so the text can be selected, what is typed in it is undone
	u.cmd = widget.NewEntry()
	u.cmd.OnChanged = func(t string) {
		if t != u.cmdText {
			u.cmd.SetText(u.cmdText)
		}
	}
	copyBtn := widget.NewButtonWithIcon("Copy command", theme.ContentCopyIcon(), func() {
		u.app.Clipboard().SetContent(u.cmd.Text)
	})
	u.genBtn = widget.NewButtonWithIcon("Generate PDF...", theme.DocumentCreateIcon(), u.generate)
	u.genBtn.Importance = widget.HighImportance
	u.genBtn.Disable()
	u.cancelBtn = widget.NewButton("Cancel", func() {
		if u.genCancel != nil {
			u.genCancel()
		}
	})
	u.cancelBtn.Hide()
	u.busy = widget.NewProgressBarInfinite()
	u.busy.Hide()
	bottom := container.NewVBox(
		container.NewBorder(nil, nil, widget.NewLabel("Same as:"), copyBtn, u.cmd),
		container.NewBorder(nil, nil, nil, container.NewHBox(u.cancelBtn, u.genBtn), u.busy),
	)

	u.applyMode()
	u.win.SetContent(container.NewBorder(top, bottom, nil, nil, split))
	u.showMessage("Open a PDF to see the imposition")
	u.refreshCommand()
}

// ---- modes

// applyMode shows the fields that make sense in the mode.
func (u *ui) applyMode() {
	s := u.s
	mode := s.Mode
	cols := item("Columns, rows", pair(u.cached("cols", &s.Cols, "auto"), u.cached("rows", &s.Rows, "auto")))
	clones := item("Clones across, down", pair(u.cached("clonex", &s.CloneX, "1"), u.cached("cloney", &s.CloneY, "1")))
	pads := item("Gap between clones", pair(u.cached("padx", &s.PadX, "0"), u.cached("pady", &s.PadY, "0")))

	grid := u.forms["grid"]
	pages := u.forms["pages"]
	flip := u.forms["flip"]
	title := map[string]string{}
	switch mode {
	case modeRepeat:
		grid.Items = []*widget.FormItem{clones, pads}
		pages.Items = []*widget.FormItem{item("Pages", u.cached("pages", &s.Pages, "all, ex: 1-4,7"))}
		flip.Items = nil
		title["grid"] = "Repeat"
	case modeBooklet:
		grid.Items = []*widget.FormItem{cols, clones, pads}
		pages.Items = []*widget.FormItem{item("Pages", u.cached("pages", &s.Pages, "all, ex: 1-8"))}
		flip.Items = []*widget.FormItem{
			item("Creep", u.cached("creep", &s.Creep, "0, up to the bleed")),
			item("Back sheets", container.NewHBox(u.cachedCheck("flip", "Flip", &s.Flip), u.cachedCheck("reverse", "Reverse", &s.Reverse))),
			item("Turn face sheets", u.cached("turn", &s.Turn, "degrees, ex: 180")),
			item("", u.cachedCheck("padblank", "Add blank pages to reach a multiple of 4", &s.PadBlank)),
		}
		title["grid"], title["flip"] = "Grid & clones", "Booklet"
	default:
		grid.Items = []*widget.FormItem{cols, clones, pads}
		pages.Items = []*widget.FormItem{
			item("Pages", u.cached("pages", &s.Pages, "all, ex: 1-4,7")),
			item("Flow", u.cached("flow", &s.Flow, "ex: 1,1,1,2,2,2")),
		}
		weld := u.cached("weld", &s.Weld, "1")
		duplex := u.cachedCheck("duplex", "Duplex", &s.Duplex)
		flip.Items = []*widget.FormItem{
			item("", duplex),
			item("Pages kept together", weld),
			item("Back sheets", container.NewHBox(u.cachedCheck("flip", "Flip", &s.Flip), u.cachedCheck("reverse", "Reverse", &s.Reverse))),
		}
		title["grid"], title["flip"] = "Grid & clones", "Duplex"
	}
	for _, f := range u.forms {
		f.Refresh()
	}
	u.acc.Items = []*widget.AccordionItem{
		widget.NewAccordionItem("Sheet", u.forms["sheet"]),
		widget.NewAccordionItem(title["grid"], grid),
		widget.NewAccordionItem("Pages", pages),
	}
	if len(flip.Items) > 0 {
		u.acc.Items = append(u.acc.Items, widget.NewAccordionItem(title["flip"], flip))
	}
	u.acc.Items = append(u.acc.Items, widget.NewAccordionItem("Cropmarks & bleed", u.forms["marks"]))
	u.acc.Refresh()
	u.acc.OpenAll()
}

// cached makes an entry the first time it is asked for, later it gives the same one:
// widgets that appear in some modes are kept, so they always follow the settings.
func (u *ui) cached(key string, p *string, placeholder string) fyne.CanvasObject {
	if u.cache == nil {
		u.cache = map[string]fyne.CanvasObject{}
	}
	if o, ok := u.cache[key]; ok {
		return o
	}
	e := u.entry(p, placeholder)
	u.cache[key] = e
	if key == "weld" && !u.s.Duplex {
		e.Disable()
	}
	return e
}

func (u *ui) cachedCheck(key, label string, p *bool) fyne.CanvasObject {
	if u.cache == nil {
		u.cache = map[string]fyne.CanvasObject{}
	}
	if o, ok := u.cache[key]; ok {
		return o
	}
	var after func()
	if key == "duplex" {
		// pages are kept together only in duplex
		after = func() {
			if w, ok := u.cache["weld"].(*widget.Entry); ok {
				if *p {
					w.Enable()
				} else {
					w.Disable()
				}
			}
		}
	}
	c := u.check(label, p, after)
	u.cache[key] = c
	return c
}
