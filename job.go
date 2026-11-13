package impose

import (
	"bytes"
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
	// It wins over the page count: pages it asks for and the source lacks are blanks.
	Flow                  []int
	Duplex, Flip, Reverse bool
	// Weld is the length of a group of pages in duplex, 0 means 1.
	Weld int
	// Turn rotates pages on back sheets, in degrees.
	Turn float64
	// PadBlank fills with blank pages at the end: a booklet up to a multiple of 4, a duplex grid up to an even number of sheets.
	PadBlank bool

	// TrimWidth, TrimHeight override the detected trim size, centered on the detected one.
	// 0 keeps the file's own TrimBox, CropBox or MediaBox.
	TrimWidth, TrimHeight float64

	// MergeBleed halves the bleed at every internal grid boundary between neighboring
	// cards, so the two halves sum to one full bleed; outer edges keep their full value.
	MergeBleed bool

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
func (s *Source) PageSize(bleedx, bleedy, trimw, trimh float64) (w, h float64, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pageSize(bleedx, bleedy, trimw, trimh)
}

func (s *Source) pageSize(bleedx, bleedy, trimw, trimh float64) (w, h float64, err error) {
	s.rd.SetTrim(trimw, trimh)
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

// PageBoxes are the sizes, in points, of the first page's boxes after SetBleed(bleedx, bleedy).
// TrimBox is always set (reader.go's AdjustMediaBox falls back to CropBox, then to the page's
// own original MediaBox, when the file does not declare a TrimBox). HasCrop is false when the
// file does not declare a CropBox at all.
type PageBoxes struct {
	MediaW, MediaH float64
	TrimW, TrimH   float64
	CropW, CropH   float64
	HasCrop        bool
}

// Boxes gives the MediaBox, TrimBox and CropBox of the first page, for display: it does not
// affect, and is not affected by, the imposition itself.
func (s *Source) Boxes(bleedx, bleedy, trimw, trimh float64) (PageBoxes, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rd.SetTrim(trimw, trimh)
	s.rd.SetBleed(bleedx, bleedy)
	page, err := s.rd.GetPage(1)
	if err != nil {
		return PageBoxes{}, err
	}
	mb, err := page.GetMediaBox()
	if err != nil {
		return PageBoxes{}, err
	}
	pb := PageBoxes{MediaW: mb.Urx - mb.Llx, MediaH: mb.Ury - mb.Lly}
	if tb := page.TrimBox; tb != nil {
		pb.TrimW, pb.TrimH = tb.Urx-tb.Llx, tb.Ury-tb.Lly
	}
	if cb, err := page.GetBox("CropBox"); err == nil {
		pb.CropW, pb.CropH, pb.HasCrop = cb.Urx-cb.Llx, cb.Ury-cb.Lly, true
	}
	return pb, nil
}

// PagePDF makes a pdf of one page of the file, num counts from 1, cut as the imposition cuts it:
// its trim or crop box expanded with bleed. It is fit for rendering a preview of the page.
// nearx, farx, neary, fary clip away bleed on each side, as MergeBleed would for a placement;
// 0 keeps the page at its full, symmetric bleed.
func (s *Source) PagePDF(num int, bleedx, bleedy, trimw, trimh, nearx, farx, neary, fary float64) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if num < 1 || num > s.NumPages {
		return nil, fmt.Errorf("page %d out of 1-%d", num, s.NumPages)
	}
	s.rd.SetTrim(trimw, trimh)
	s.rd.SetBleed(bleedx, bleedy)
	bk, err := s.rd.BlockFromPage(num)
	if err != nil {
		return nil, err
	}
	// the page is upright, as its size is measured
	bk.SetAngle(0)
	w, h := bk.Width()-nearx-farx, bk.Height()-neary-fary
	// Clip's y is the block's own native, bottom-up PDF axis, unlike SetPos's top-down y:
	// the near (top) amount is skipped by SetPos, the far (bottom) amount by Clip's offset.
	bk.Clip(nearx, fary, w, h, false)
	bk.SetPos(-nearx, -neary)
	c := creator.New()
	c.SetPageSize(creator.PageSize{w, h})
	c.NewPage()
	if err := c.Draw(bk); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := c.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
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
	// merging bleed shrinks the grid's pitch, so it only makes sense for a plain
	// Col x Row grid (possibly cloned): booklet pairs pages across a fold, and a rotated
	// page's own axes no longer line up with the sheet's.
	// 180/-180 is excluded too: Clip's y-axis convention (bottom-up, unlike SetPos's
	// top-down) makes the correct near/far mapping for a half-turned page unverified.
	mergeBleed := o.MergeBleed && o.Mode != ModeBooklet && angle == 0

	width, height := o.Width, o.Height
	if o.Autopage {
		autopadding := o.Autopadding
		pw, ph := w, h
		bleedx, bleedy := o.Bleedx, o.Bleedy
		if beSwitched {
			pw, ph = h, w
			bleedx, bleedy = bleedy, bleedx
		}
		pageswidth := gridSpan(col*clonex, col, o.ClonePadX, pw, bleedx, mergeBleed)
		pagesheight := gridSpan(row*cloney, row, o.ClonePadY, ph, bleedy, mergeBleed)
		width = o.Left + pageswidth + o.Right + 2*extw + 2*autopadding
		height = o.Top + pagesheight + o.Bottom + 2*exth + 2*autopadding
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
		Small:      &SmallBox{Box: &Box{Width: w, Height: h}, Angle: angle},
		Col:        col,
		Row:        row,
		CloneX:     clonex,
		CloneY:     cloney,
		ClonePadX:  o.ClonePadX,
		ClonePadY:  o.ClonePadY,
		Bleedx:     o.Bleedx,
		Bleedy:     o.Bleedy,
		MergeBleed: mergeBleed,
		Num:        np,
		Creator:    c,
		Reader:     rd,
		Cropmark:   nil,
		Outline:    o.Outline,
		DeltaPos:   0.0,
		Ctx:        ctx,
		Dry:        dry,
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
		return bb.Booklet(j.pags, o.Creep, o.Flip, o.Reverse, o.PadBlank, o.Turn, j.marks, progress)
	}
	flow := o.Flow
	if len(flow) == 0 {
		flow = []int{1}
	}
	// an explicit flow defines the layout, the pages it lacks are blanks
	pad := o.PadBlank || len(o.Flow) > 0
	return bb.Impose(j.pags, flow, max(o.Weld, 1), o.Flip, o.Reverse, o.Turn, o.Duplex, pad, j.marks, progress)
}

// Run imposes the pages of src and writes the pdf to out. progress, if set, is called with the
// count of elements processed. ctx stops the imposition.
func Run(ctx context.Context, o Options, src *Source, out io.Writer, progress func(int)) error {
	src.mu.Lock()
	defer src.mu.Unlock()

	w, h, err := src.pageSize(o.Bleedx, o.Bleedy, o.TrimWidth, o.TrimHeight)
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
	w, h, err := src.PageSize(o.Bleedx, o.Bleedy, o.TrimWidth, o.TrimHeight)
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
		Bleedx: o.Bleedx, Bleedy: o.Bleedy,
		TrimWidth: o.TrimWidth, TrimHeight: o.TrimHeight,
		Col: j.bb.Col, Row: j.bb.Row, CloneX: j.bb.CloneX, CloneY: j.bb.CloneY,
		Sheets: sheets,
	}, nil
}
