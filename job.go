package impose

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"sync"

	"github.com/innermond/pange"
	"github.com/unidoc/unipdf/v3/creator"
)

// Mode is the way pages are imposed.
type Mode int

const (
	// ModeGrid lays pages on a grid, in order or following Flow and Duplex.
	ModeGrid Mode = iota
	// ModeRepeat repeats every page on its own sheets, cloned Clone times.
	ModeRepeat
	// ModeBooklet lays pages for a folded booklet.
	ModeBooklet
)

// Options has everything an imposition needs. Lengths are in points.
type Options struct {
	Mode Mode

	// sheet
	Width, Height            float64
	Top, Right, Bottom, Left float64
	// Autopage calculates sheet size from content, Autopadding is added around.
	Autopage    bool
	Autopadding float64
	CenterX     bool
	CenterY     bool
	// Angle rotates every page, in degrees.
	Angle float64

	// grid, Col and Row 0 mean not known: booklet uses 2x1, grid mode gives a GridGuessError.
	Col, Row int
	// clones of the grid, 0 means 1.
	CloneX, CloneY       int
	ClonePadX, ClonePadY float64

	// Pages is a selection like 1-4,7, empty means all the pages.
	Pages string
	// Flow is 1-based, it can repeat pages, empty means as they are.
	Flow                  []int
	Duplex, Flip, Reverse bool
	// Weld is the length of a group of pages in duplex, 0 means 1.
	Weld int
	// Turn rotates pages on back sheets, in degrees.
	Turn float64

	// cropmarks; Offx and Offy are measured from the trim edge, bleed is subtracted from them.
	Bleedx, Bleedy float64
	Offx, Offy     float64
	Markw, Markh   float64
	NoCropmark     bool
	// CropmarkPages are the sheets that have cropmarks, counted from 0, empty means every sheet.
	CropmarkPages []int
	// Outline draws a rectangle around every page.
	Outline bool

	// Creep is the thickness of the paper in a booklet, limited by Bleedx by the caller.
	Creep float64
}

// DefaultOptions are the defaults of the command line, in points.
func DefaultOptions() Options {
	mm := creator.PPMM
	return Options{
		Width:       320 * mm,
		Height:      450 * mm,
		Autopadding: 2 * mm,
		CloneX:      1,
		CloneY:      1,
		Weld:        1,
		Offx:        2 * mm,
		Offy:        2 * mm,
		Markw:       5 * mm,
		Markh:       5 * mm,
	}
}

// GridGuessError is what is given when the grid is not known: how many pages fit on the sheet.
type GridGuessError struct {
	Col, Row int
}

func (e *GridGuessError) Error() string {
	if e.Col < 1 || e.Row < 1 {
		return "not even one page fits on the sheet"
	}
	return fmt.Sprintf("suggested grid %dx%d", e.Col, e.Row)
}

// Source is a pdf file to impose, it can be used for many impositions one at a time.
type Source struct {
	mu sync.Mutex
	rd *PdfReader
	// NumPages is the number of pages of the file.
	NumPages int
}

// OpenSource reads the pdf file.
func OpenSource(in io.ReadSeeker) (*Source, error) {
	rd, err := NewReader(in, 0, 0)
	if err != nil {
		return nil, err
	}
	np, err := rd.GetNumPages()
	if err != nil {
		return nil, err
	}
	return &Source{rd: rd, NumPages: np}, nil
}

// PageSize gives the size of the first page, all pages are assumed to have the same size,
// expanded with bleed from its trim box or crop box.
func (s *Source) PageSize(bleedx, bleedy float64) (w, h float64, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pageSize(bleedx, bleedy)
}

func (s *Source) pageSize(bleedx, bleedy float64) (w, h float64, err error) {
	s.rd.SetBleed(bleedx, bleedy)
	page, err := s.rd.GetPage(1)
	if err != nil {
		return 0, 0, err
	}
	bbox, err := page.GetMediaBox()
	if err != nil {
		return 0, 0, err
	}
	return bbox.Urx - bbox.Llx, bbox.Ury - bbox.Lly, nil
}

// job is an imposition ready to run
type job struct {
	o     Options
	bb    *Boxes
	pags  []int
	marks []int
	// bookletCrop tells cropmarks are made for a booklet
	bookletCrop bool
	extw, exth  float64
}

func newJob(ctx context.Context, o Options, numPages int, w, h float64, rd *PdfReader, dry bool) (*job, error) {
	// from 1 to last
	pages := o.Pages
	if pages == "" {
		pages = fmt.Sprintf("1-%d", numPages)
	}
	pags, err := pange.Selection(pages).Full()
	if err != nil {
		return nil, fmt.Errorf("pages: %w", err)
	}
	np := len(pags)

	// grid
	col, row := o.Col, o.Row
	guess := false
	switch o.Mode {
	case ModeRepeat:
		col, row = 1, 1
	case ModeBooklet:
		if col == 0 && row == 0 {
			col, row = 2, 1
		}
	default:
		guess = col == 0 && row == 0
		if guess {
			col, row = 1, 1
		}
	}
	if col < 1 || row < 1 {
		return nil, fmt.Errorf("invalid grid %dx%d", col, row)
	}
	clonex, cloney := max(o.CloneX, 1), max(o.CloneY, 1)

	// last edge is further inside mediabox by bleed amount
	// adjust offsets otherwise they will be aware only by media edge
	offx := o.Offx - o.Bleedx
	offy := o.Offy - o.Bleedy
	// cropmarks adds extra to dimensions
	extw := offx + o.Markw
	exth := offy + o.Markh

	angle := o.Angle
	beSwitched := angle == 90.0 || angle == -90 || angle == 270 || angle == -270

	width, height := o.Width, o.Height
	if o.Autopage {
		autopadding := o.Autopadding
		clonepadxWidth := o.ClonePadX * float64(clonex-1)
		clonepadyHeight := o.ClonePadY * float64(cloney-1)
		pw, ph := w, h
		if beSwitched {
			pw, ph = h, w
		}
		width = o.Left + clonepadxWidth + float64(clonex)*float64(col)*pw + o.Right + 2*extw + 2*autopadding
		height = o.Top + clonepadyHeight + float64(cloney)*float64(row)*ph + o.Bottom + 2*exth + 2*autopadding
	}
	if width <= 0 || height <= 0 {
		return nil, errors.New("sheet size is needed")
	}

	// create a sheet page
	c := creator.New()
	c.SetPageSize(creator.PageSize{width, height})

	bb := &Boxes{
		Big: &BigBox{Box: &Box{
			Width:  width,
			Height: height,
			Top:    o.Top,
			Right:  o.Right,
			Bottom: o.Bottom,
			Left:   o.Left,
		}},
		Small:     &SmallBox{Box: &Box{Width: w, Height: h}, Angle: angle},
		Col:       col,
		Row:       row,
		CloneX:    clonex,
		CloneY:    cloney,
		ClonePadX: o.ClonePadX,
		ClonePadY: o.ClonePadY,
		Num:       np,
		Creator:   c,
		Reader:    rd,
		Cropmark:  nil,
		Outline:   o.Outline,
		DeltaPos:  0.0,
		Ctx:       ctx,
		Dry:       dry,
	}
	angled := false
	if beSwitched {
		bb.Small.Switch()
		angled = true
	}

	// guess the grid, before centering that depends on a known grid
	if guess {
		gc, gr := bb.GuessGrid()
		return nil, &GridGuessError{gc, gr}
	}
	// centering by changing margins
	if o.CenterX {
		bb.AdjustMarginCenteringAlongWidth()
	}
	if o.CenterY {
		bb.AdjustMarginCenteringAlongHeight()
	}

	// check if media is enough
	if !bb.EnoughWidth() {
		return nil, fmt.Errorf("%d columns do not fit", bb.Col)
	}
	if !bb.EnoughHeight() {
		return nil, fmt.Errorf("%d rows do not fit", bb.Row)
	}

	j := &job{o: o, bb: bb, pags: pags, bookletCrop: o.Mode == ModeBooklet, extw: extw, exth: exth}
	if o.NoCropmark {
		j.marks = nil
	} else {
		if len(o.CropmarkPages) > 0 {
			j.marks = slices.Clone(o.CropmarkPages)
		}
		if !dry {
			bb.CreateCropmark(o.Markw, o.Markh, extw, exth, o.Bleedx, o.Bleedy, j.bookletCrop, angled)
		}
	}
	return j, nil
}

// cycle lays down all the pages
func (j *job) cycle(progress func(int)) error {
	o, bb := j.o, j.bb
	switch o.Mode {
	case ModeRepeat:
		return bb.Repeat(j.pags, o.Turn, j.marks, progress)
	case ModeBooklet:
		return bb.Booklet(j.pags, o.Creep, o.Flip, o.Reverse, o.Turn, j.marks, progress)
	}
	flow := o.Flow
	if len(flow) == 0 {
		flow = []int{1}
	}
	return bb.Impose(j.pags, flow, max(o.Weld, 1), o.Flip, o.Reverse, o.Turn, o.Duplex, j.marks, progress)
}

// Run imposes the pages of src and writes the pdf to out. progress, if set, is called with the
// count of elements processed. ctx stops the imposition.
func Run(ctx context.Context, o Options, src *Source, out io.Writer, progress func(int)) error {
	src.mu.Lock()
	defer src.mu.Unlock()

	w, h, err := src.pageSize(o.Bleedx, o.Bleedy)
	if err != nil {
		return err
	}
	j, err := newJob(ctx, o, src.NumPages, w, h, src.rd, false)
	if err != nil {
		return err
	}
	if err := j.cycle(progress); err != nil {
		return err
	}
	return j.bb.Creator.Write(out)
}

// Plan works out where every page goes without drawing any pdf, it is fast and fit for previews.
func Plan(ctx context.Context, o Options, src *Source) (*Layout, error) {
	w, h, err := src.PageSize(o.Bleedx, o.Bleedy)
	if err != nil {
		return nil, err
	}
	j, err := newJob(ctx, o, src.NumPages, w, h, nil, true)
	if err != nil {
		return nil, err
	}
	var sheets []Sheet
	j.bb.OnPlace = func(sheet int, p Placement) {
		for len(sheets) <= sheet {
			sheets = append(sheets, Sheet{})
		}
		sheets[sheet].Placements = append(sheets[sheet].Placements, p)
	}
	if err := j.cycle(nil); err != nil {
		return nil, err
	}
	// sheets without any page, like blank duplex backs
	for len(sheets) < j.bb.Sheets {
		sheets = append(sheets, Sheet{})
	}
	if !o.NoCropmark {
		marks := j.bb.newCropMarkBlock(o.Markw, o.Markh, j.extw, j.exth, o.Bleedx, o.Bleedy).SheetLines(j.bookletCrop)
		for i := range sheets {
			if j.marks == nil || slices.Contains(j.marks, i) {
				sheets[i].Marks = marks
			}
		}
	}
	big := j.bb.Big
	return &Layout{
		Width: big.Width, Height: big.Height,
		Top: big.Top, Right: big.Right, Bottom: big.Bottom, Left: big.Left,
		Col: j.bb.Col, Row: j.bb.Row, CloneX: j.bb.CloneX, CloneY: j.bb.CloneY,
		Sheets: sheets,
	}, nil
}
