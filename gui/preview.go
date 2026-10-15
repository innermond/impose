package main

import (
	"fmt"
	"image/color"
	"math"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/innermond/impose"
)

// colours of the paper stay the same in any theme
var (
	paperColor   = color.NRGBA{255, 255, 255, 255}
	paperBorder  = color.NRGBA{150, 150, 150, 255}
	marginColor  = color.NRGBA{235, 150, 150, 255}
	pageFill     = color.NRGBA{214, 228, 255, 255}
	pageStroke   = color.NRGBA{59, 111, 212, 255}
	pageText     = color.NRGBA{26, 43, 77, 255}
	pageTopColor = color.NRGBA{230, 120, 20, 255}
	markColor    = color.NRGBA{0, 0, 0, 255}
)

const previewPad = 12

// preview draws a sheet of the layout as boxes: sheet, margins, pages and cropmarks.
type preview struct {
	widget.BaseWidget
	layout *impose.Layout
	sheet  int
	// msg is shown instead of the sheet when there is no layout
	msg string
	// scale is pixels for a point, 0 fits the sheet in the widget
	scale float64
}

func newPreview() *preview {
	p := &preview{}
	p.ExtendBaseWidget(p)
	return p
}

// setLayout shows a layout, or the message if there is none.
func (p *preview) setLayout(l *impose.Layout, msg string) {
	p.layout, p.msg = l, msg
	p.sheet = clamp(p.sheet, 0, p.sheets()-1)
	p.Refresh()
}

func (p *preview) sheets() int {
	if p.layout == nil {
		return 0
	}
	return len(p.layout.Sheets)
}

func (p *preview) setSheet(i int) {
	p.sheet = clamp(i, 0, p.sheets()-1)
	p.Refresh()
}

func (p *preview) setScale(s float64) {
	p.scale = s
	p.Refresh()
	p.BaseWidget.Resize(p.Size())
}

func clamp(v, lo, hi int) int {
	return max(lo, min(v, hi))
}

func (p *preview) CreateRenderer() fyne.WidgetRenderer {
	return &previewRenderer{p: p}
}

type previewRenderer struct {
	p    *preview
	objs []fyne.CanvasObject
	size fyne.Size
}

func (r *previewRenderer) MinSize() fyne.Size {
	if l := r.p.layout; l != nil && r.p.scale > 0 {
		return fyne.NewSize(float32(l.Width*r.p.scale)+2*previewPad, float32(l.Height*r.p.scale)+2*previewPad)
	}
	return fyne.NewSize(240, 160)
}

func (r *previewRenderer) Layout(size fyne.Size) {
	r.size = size
	r.build()
}

func (r *previewRenderer) Refresh() {
	r.build()
	canvas.Refresh(r.p)
}

func (r *previewRenderer) Objects() []fyne.CanvasObject { return r.objs }
func (r *previewRenderer) Destroy()                     {}

// note puts a text in the middle
func (r *previewRenderer) note(msg string) {
	t := canvas.NewText(msg, theme.Color(theme.ColorNameDisabled))
	t.TextSize = 16
	t.Alignment = fyne.TextAlignCenter
	m := t.MinSize()
	t.Move(fyne.NewPos((r.size.Width-m.Width)/2, (r.size.Height-m.Height)/2))
	t.Resize(m)
	r.objs = append(r.objs, t)
}

func (r *previewRenderer) build() {
	r.objs = r.objs[:0]
	l := r.p.layout
	if l == nil || len(l.Sheets) == 0 || l.Width <= 0 || l.Height <= 0 {
		r.note(r.p.msg)
		return
	}
	sheet := l.Sheets[clamp(r.p.sheet, 0, len(l.Sheets)-1)]

	// points to pixels
	scale := r.p.scale
	if scale <= 0 {
		scale = math.Min(float64(r.size.Width-2*previewPad)/l.Width, float64(r.size.Height-2*previewPad)/l.Height)
	}
	if scale <= 0 {
		return
	}
	// the sheet is in the middle when there is room
	ox := float64(r.size.Width)/2 - l.Width*scale/2
	oy := float64(r.size.Height)/2 - l.Height*scale/2
	ox, oy = math.Max(ox, previewPad), math.Max(oy, previewPad)
	pos := func(x, y float64) fyne.Position { return fyne.NewPos(float32(ox+x*scale), float32(oy+y*scale)) }
	dim := func(w, h float64) fyne.Size { return fyne.NewSize(float32(w*scale), float32(h*scale)) }
	add := func(o fyne.CanvasObject, x, y, w, h float64) {
		o.Move(pos(x, y))
		o.Resize(dim(w, h))
		r.objs = append(r.objs, o)
	}

	// paper
	paper := canvas.NewRectangle(paperColor)
	paper.StrokeColor, paper.StrokeWidth = paperBorder, 1
	add(paper, 0, 0, l.Width, l.Height)

	// margins
	if l.Left+l.Right+l.Top+l.Bottom > 0 {
		guide := canvas.NewRectangle(color.Transparent)
		guide.StrokeColor, guide.StrokeWidth = marginColor, 1
		add(guide, l.Left, l.Top, l.Width-l.Left-l.Right, l.Height-l.Top-l.Bottom)
	}

	// pages
	for _, pl := range sheet.Placements {
		box := canvas.NewRectangle(pageFill)
		box.StrokeColor, box.StrokeWidth = pageStroke, 1
		add(box, pl.X, pl.Y, pl.W, pl.H)
		r.pageLabel(pl, scale, pos(pl.X, pl.Y), dim(pl.W, pl.H))
	}
	if len(sheet.Placements) == 0 {
		blank := canvas.NewText("blank sheet", theme.Color(theme.ColorNameDisabled))
		blank.TextSize = 14
		m := blank.MinSize()
		blank.Move(pos(l.Width/2, l.Height/2).Subtract(fyne.NewPos(m.Width/2, m.Height/2)))
		blank.Resize(m)
		r.objs = append(r.objs, blank)
	}

	// cropmarks on top
	width := float32(math.Max(1, 0.4*impose.PointsPerMM*scale))
	for _, m := range sheet.Marks {
		ln := canvas.NewLine(markColor)
		ln.StrokeWidth = width
		ln.Position1, ln.Position2 = pos(m.X1, m.Y1), pos(m.X2, m.Y2)
		r.objs = append(r.objs, ln)
	}
}

// pageLabel writes the number of the page, with its angle, and marks the edge that is the top of the page.
func (r *previewRenderer) pageLabel(pl impose.Placement, scale float64, at fyne.Position, size fyne.Size) {
	label := fmt.Sprint(pl.Page)
	if pl.Angle != 0 {
		label += fmt.Sprintf(" (%s°)", formatNumber(pl.Angle))
	}
	t := canvas.NewText(label, pageText)
	t.TextSize = float32(math.Max(8, math.Min(float64(min(size.Width, size.Height))*0.28, 44)))
	m := t.MinSize()
	if m.Width > size.Width && size.Width > 0 {
		t.TextSize = float32(math.Max(6, float64(t.TextSize*size.Width/m.Width*0.9)))
		m = t.MinSize()
	}
	t.Move(fyne.NewPos(at.X+(size.Width-m.Width)/2, at.Y+(size.Height-m.Height)/2))
	t.Resize(m)
	r.objs = append(r.objs, t)

	// the top of the page, turned with the page: counter clockwise angles
	deg := math.Mod(math.Mod(pl.Angle, 360)+360, 360)
	var p1, p2 fyne.Position
	switch deg {
	case 0:
		p1, p2 = at, fyne.NewPos(at.X+size.Width, at.Y)
	case 90:
		p1, p2 = at, fyne.NewPos(at.X, at.Y+size.Height)
	case 180:
		p1, p2 = fyne.NewPos(at.X, at.Y+size.Height), fyne.NewPos(at.X+size.Width, at.Y+size.Height)
	case 270:
		p1, p2 = fyne.NewPos(at.X+size.Width, at.Y), fyne.NewPos(at.X+size.Width, at.Y+size.Height)
	default:
		return
	}
	top := canvas.NewLine(pageTopColor)
	top.StrokeWidth = 3
	top.Position1, top.Position2 = p1, p2
	r.objs = append(r.objs, top)
}
