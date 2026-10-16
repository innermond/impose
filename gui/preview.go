package main

import (
	"fmt"
	"image"
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
	badgeFill    = color.NRGBA{214, 228, 255, 220}
	bleedShade   = color.NRGBA{0, 0, 0, 36}
	trimColor    = color.NRGBA{220, 0, 120, 255}
	backColor    = color.NRGBA{0, 150, 70, 255}
)

const (
	previewPad = 12
	// backTranslucency is how much of the back sheet does not show through the paper
	backTranslucency = 0.7
)

// see-through: how the sheet is turned to print its back
const (
	seeNone = iota
	// seeSide turns the sheet on its side edge, the back is mirrored left to right
	seeSide
	// seeHead turns the sheet head to foot, the back is mirrored top to bottom
	seeHead
)

// preview draws a sheet of the layout: sheet, margins, pages and cropmarks.
// The pages are rendered from the pdf when thumbs is set, boxes otherwise.
type preview struct {
	widget.BaseWidget
	layout *impose.Layout
	sheet  int
	// msg is shown instead of the sheet when there is no layout
	msg string
	// scale is pixels for a point, 0 fits the sheet in the widget
	scale float64

	thumbs     *thumbs
	showThumbs bool
	// pairs tells sheets go face then back, see is how the back shows through the face
	pairs bool
	see   int
}

func newPreview() *preview {
	p := &preview{showThumbs: true}
	p.thumbs = newThumbs(func() { fyne.Do(p.Refresh) })
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

// partner is the sheet printed on the other side of the shown one, -1 if none.
func (p *preview) partner() int {
	if !p.pairs || p.see == seeNone {
		return -1
	}
	if o := p.sheet ^ 1; o < p.sheets() {
		return o
	}
	return -1
}

func clamp(v, lo, hi int) int {
	return max(lo, min(v, hi))
}

func (p *preview) CreateRenderer() fyne.WidgetRenderer {
	return &previewRenderer{p: p, imgs: map[imgKey]*canvas.Image{}}
}

// imgKey is a picture shown as it is or through the paper
type imgKey struct {
	img     *image.RGBA
	through bool
}

type previewRenderer struct {
	p    *preview
	objs []fyne.CanvasObject
	size fyne.Size
	// imgs are kept from a build to the next, so their textures are not made again
	imgs map[imgKey]*canvas.Image
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

// pixelScale is how many pixels of the screen make a unit of the window
func (r *previewRenderer) pixelScale() float64 {
	if app := fyne.CurrentApp(); app != nil {
		if c := app.Driver().CanvasForObject(r.p); c != nil && c.Scale() > 0 {
			return float64(c.Scale())
		}
	}
	return 1
}

// thumbKey is the thumbnail of a placement sharp enough for ppp pixels for a point
func thumbKeyFor(l *impose.Layout, pl impose.Placement, turns int, ppp float64) thumbKey {
	w, h := pl.W, pl.H
	if turns%2 == 1 {
		w, h = h, w
	}
	return thumbKey{page: pl.Page, bleedx: l.Bleedx, bleedy: l.Bleedy, tier: tierFor(ppp, w, h)}
}

// image gives the picture of a placement, turned and mirrored as it lies on the sheet, nil if
// there is none yet; the key is added to want so it is rendered.
func (r *previewRenderer) image(pl impose.Placement, ppp float64, mirrorX, mirrorY, through bool, want *[]thumbKey) *canvas.Image {
	p := r.p
	if !p.showThumbs || pl.Page < 1 {
		return nil
	}
	turns, ok := quarterTurns(pl.Angle)
	if !ok {
		return nil
	}
	k := thumbKeyFor(p.layout, pl, turns, ppp)
	*want = append(*want, k)
	img := p.thumbs.oriented(k, turns, mirrorX, mirrorY)
	if img == nil {
		return nil
	}
	ik := imgKey{img, through}
	ci := r.imgs[ik]
	if ci == nil {
		ci = canvas.NewImageFromImage(img)
		ci.FillMode = canvas.ImageFillStretch
		ci.ScaleMode = canvas.ImageScaleSmooth
		if through {
			ci.Translucency = backTranslucency
		}
	}
	return ci
}

func (r *previewRenderer) build() {
	r.objs = r.objs[:0]
	used := map[imgKey]*canvas.Image{}
	defer func() { r.imgs = used }()
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
	ppp := scale * r.pixelScale()
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
	var want []thumbKey
	addImage := func(ci *canvas.Image, through bool, x, y, w, h float64) {
		used[imgKey{ci.Image.(*image.RGBA), through}] = ci
		add(ci, x, y, w, h)
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

	// pages: their pictures, or boxes until they are rendered
	shown := make([]bool, len(sheet.Placements))
	for i, pl := range sheet.Placements {
		if ci := r.image(pl, ppp, false, false, false, &want); ci != nil {
			addImage(ci, false, pl.X, pl.Y, pl.W, pl.H)
			shown[i] = true
			continue
		}
		box := canvas.NewRectangle(pageFill)
		add(box, pl.X, pl.Y, pl.W, pl.H)
	}

	// the back sheet seen through the paper, as it lies when the sheet is turned
	if o := r.p.partner(); o >= 0 {
		mirrorX, mirrorY := r.p.see == seeSide, r.p.see == seeHead
		for _, pl := range l.Sheets[o].Placements {
			x, y := pl.X, pl.Y
			if mirrorX {
				x = l.Width - pl.X - pl.W
			}
			if mirrorY {
				y = l.Height - pl.Y - pl.H
			}
			if ci := r.image(pl, ppp, mirrorX, mirrorY, true, &want); ci != nil {
				addImage(ci, true, x, y, pl.W, pl.H)
			}
			r.dashedRect(pos(x, y), dim(pl.W, pl.H), backColor)
		}
	}

	// bleed and the trim edge
	for _, pl := range sheet.Placements {
		r.trim(l, pl, pos, dim)
	}

	// outlines and numbers on top of the pictures
	for i, pl := range sheet.Placements {
		box := canvas.NewRectangle(color.Transparent)
		box.StrokeColor, box.StrokeWidth = pageStroke, 1
		add(box, pl.X, pl.Y, pl.W, pl.H)
		r.pageLabel(pl, pos(pl.X, pl.Y), dim(pl.W, pl.H), shown[i])
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

	if r.p.showThumbs {
		// the next sheets are rendered ahead, after the shown one
		for _, s := range []int{r.p.sheet + 1, r.p.sheet - 1} {
			if s < 0 || s >= len(l.Sheets) {
				continue
			}
			for _, pl := range l.Sheets[s].Placements {
				if turns, ok := quarterTurns(pl.Angle); ok && pl.Page > 0 {
					want = append(want, thumbKeyFor(l, pl, turns, ppp))
				}
			}
		}
		r.p.thumbs.want(want)
	}
}

// trim shades the bleed of a placement and draws its trim edge.
func (r *previewRenderer) trim(l *impose.Layout, pl impose.Placement, pos func(x, y float64) fyne.Position, dim func(w, h float64) fyne.Size) {
	bx, by := l.Bleedx, l.Bleedy
	if bx <= 0 && by <= 0 {
		return
	}
	if turns, ok := quarterTurns(pl.Angle); ok && turns%2 == 1 {
		bx, by = by, bx
	}
	if 2*bx >= pl.W || 2*by >= pl.H {
		return
	}
	shade := func(x, y, w, h float64) {
		s := canvas.NewRectangle(bleedShade)
		s.Move(pos(x, y))
		s.Resize(dim(w, h))
		r.objs = append(r.objs, s)
	}
	shade(pl.X, pl.Y, pl.W, by)
	shade(pl.X, pl.Y+pl.H-by, pl.W, by)
	shade(pl.X, pl.Y+by, bx, pl.H-2*by)
	shade(pl.X+pl.W-bx, pl.Y+by, bx, pl.H-2*by)
	r.dashedRect(pos(pl.X+bx, pl.Y+by), dim(pl.W-2*bx, pl.H-2*by), trimColor)
}

// dashedRect draws the edges of a rectangle with dashes.
func (r *previewRenderer) dashedRect(at fyne.Position, size fyne.Size, c color.Color) {
	tl, br := at, at.Add(size)
	tr, bl := fyne.NewPos(br.X, tl.Y), fyne.NewPos(tl.X, br.Y)
	r.dashed(tl, tr, c)
	r.dashed(tr, br, c)
	r.dashed(bl, br, c)
	r.dashed(tl, bl, c)
}

// dashed draws a line made of dashes.
func (r *previewRenderer) dashed(p1, p2 fyne.Position, c color.Color) {
	const dash, gap = 5, 3
	dx, dy := p2.X-p1.X, p2.Y-p1.Y
	n := float32(math.Hypot(float64(dx), float64(dy)))
	if n == 0 {
		return
	}
	ux, uy := dx/n, dy/n
	for d := float32(0); d < n; d += dash + gap {
		e := min(d+dash, n)
		ln := canvas.NewLine(c)
		ln.StrokeWidth = 1
		ln.Position1 = fyne.NewPos(p1.X+ux*d, p1.Y+uy*d)
		ln.Position2 = fyne.NewPos(p1.X+ux*e, p1.Y+uy*e)
		r.objs = append(r.objs, ln)
	}
}

// pageLabel writes the number of the page, with its angle, and marks the edge that is the top of the page.
// Over a picture of the page the number is small, in a corner, so the page can be seen.
func (r *previewRenderer) pageLabel(pl impose.Placement, at fyne.Position, size fyne.Size, small bool) {
	label := fmt.Sprint(pl.Page)
	if pl.Angle != 0 {
		label += fmt.Sprintf(" (%s°)", formatNumber(pl.Angle))
	}
	t := canvas.NewText(label, pageText)
	if small {
		t.TextSize = 11
		m := t.MinSize()
		if m.Width+4 <= size.Width && m.Height+4 <= size.Height {
			bg := canvas.NewRectangle(badgeFill)
			bg.Move(at.Add(fyne.NewPos(2, 2)))
			bg.Resize(m.Add(fyne.NewSize(4, 0)))
			t.Move(at.Add(fyne.NewPos(4, 2)))
			t.Resize(m)
			r.objs = append(r.objs, bg, t)
		}
	} else {
		t.TextSize = float32(math.Max(8, math.Min(float64(min(size.Width, size.Height))*0.28, 44)))
		m := t.MinSize()
		if m.Width > size.Width && size.Width > 0 {
			t.TextSize = float32(math.Max(6, float64(t.TextSize*size.Width/m.Width*0.9)))
			m = t.MinSize()
		}
		t.Move(fyne.NewPos(at.X+(size.Width-m.Width)/2, at.Y+(size.Height-m.Height)/2))
		t.Resize(m)
		r.objs = append(r.objs, t)
	}

	// the top of the page, turned with the page: counter clockwise angles
	turns, ok := quarterTurns(pl.Angle)
	if !ok {
		return
	}
	var p1, p2 fyne.Position
	switch turns {
	case 0:
		p1, p2 = at, fyne.NewPos(at.X+size.Width, at.Y)
	case 1:
		p1, p2 = at, fyne.NewPos(at.X, at.Y+size.Height)
	case 2:
		p1, p2 = fyne.NewPos(at.X, at.Y+size.Height), fyne.NewPos(at.X+size.Width, at.Y+size.Height)
	case 3:
		p1, p2 = fyne.NewPos(at.X+size.Width, at.Y), fyne.NewPos(at.X+size.Width, at.Y+size.Height)
	}
	top := canvas.NewLine(pageTopColor)
	top.StrokeWidth = 3
	top.Position1, top.Position2 = p1, p2
	r.objs = append(r.objs, top)
}
