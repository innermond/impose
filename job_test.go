package impose

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"sync"
	"testing"

	"github.com/unidoc/unipdf/v3/creator"
	"github.com/unidoc/unipdf/v3/extractor"
	"github.com/unidoc/unipdf/v3/model"
)

// numbered makes a pdf with pages that have the text P1, P2, ...
func numbered(t *testing.T, n int) *Source {
	t.Helper()
	c := creator.New()
	c.SetPageSize(creator.PageSize{200, 300})
	for i := 1; i <= n; i++ {
		c.NewPage()
		p := c.NewParagraph(fmt.Sprintf("P%d", i))
		if err := c.Draw(p); err != nil {
			t.Fatal(err)
		}
	}
	var buf bytes.Buffer
	if err := c.Write(&buf); err != nil {
		t.Fatal(err)
	}
	src, err := OpenSource(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	return src
}

var pageText = regexp.MustCompile(`P(\d+)`)

// sheetsPages gives the sorted page numbers found on every sheet of a pdf
func sheetsPages(t *testing.T, pdf []byte) [][]int {
	t.Helper()
	rd, err := model.NewPdfReader(bytes.NewReader(pdf))
	if err != nil {
		t.Fatal(err)
	}
	n, _ := rd.GetNumPages()
	var out [][]int
	for i := 1; i <= n; i++ {
		page, err := rd.GetPage(i)
		if err != nil {
			t.Fatal(err)
		}
		ex, err := extractor.New(page)
		if err != nil {
			t.Fatal(err)
		}
		text, err := ex.ExtractText()
		if err != nil {
			t.Fatal(err)
		}
		pp := []int{}
		for _, m := range pageText.FindAllStringSubmatch(text, -1) {
			v, _ := strconv.Atoi(m[1])
			pp = append(pp, v)
		}
		slices.Sort(pp)
		out = append(out, pp)
	}
	return out
}

func planPages(l *Layout) [][]int {
	out := [][]int{}
	for _, s := range l.Sheets {
		pp := []int{}
		for _, p := range s.Placements {
			pp = append(pp, p.Page)
		}
		slices.Sort(pp)
		out = append(out, pp)
	}
	return out
}

func TestPlanMatchesRun(t *testing.T) {
	mm := creator.PPMM
	base := func() Options {
		o := DefaultOptions()
		o.Width, o.Height = 450*mm, 320*mm
		o.NoCropmark = true
		return o
	}
	tt := []struct {
		name  string
		pages int
		opts  func() Options
	}{
		{"grid", 4, func() Options { o := base(); o.Col, o.Row = 2, 1; return o }},
		{"pages", 4, func() Options { o := base(); o.Col, o.Row = 2, 1; o.Pages = "2-4"; return o }},
		{"flow", 4, func() Options {
			o := base()
			o.Col, o.Row = 3, 2
			o.Width, o.Height = 650*mm, 650*mm
			o.Flow = []int{1, 1, 1, 2, 2, 2}
			return o
		}},
		{"duplex", 8, func() Options { o := base(); o.Col, o.Row = 2, 1; o.Duplex = true; return o }},
		{"duplexflip", 8, func() Options { o := base(); o.Col, o.Row = 2, 1; o.Duplex, o.Flip = true, true; return o }},
		{"duplexweld", 8, func() Options {
			o := base()
			o.Col, o.Row, o.Duplex, o.Weld = 4, 1, true, 2
			o.Width = 900 * mm
			return o
		}},
		{"repeat", 2, func() Options {
			o := base()
			o.Mode = ModeRepeat
			o.CloneX, o.CloneY = 3, 1
			o.Width = 650 * mm
			return o
		}},
		{"repeatclones", 2, func() Options {
			o := base()
			o.Mode = ModeRepeat
			o.CloneX, o.CloneY, o.ClonePadX, o.ClonePadY = 2, 2, 5*mm, 5*mm
			o.Autopage = true
			return o
		}},
		{"booklet", 8, func() Options { o := base(); o.Mode = ModeBooklet; return o }},
		{"booklet4x1", 8, func() Options {
			o := base()
			o.Mode = ModeBooklet
			o.Col, o.Row = 4, 1
			o.Width = 900 * mm
			return o
		}},
		{"bookletcreep", 12, func() Options {
			o := base()
			o.Mode = ModeBooklet
			o.Creep, o.Bleedx = 0.5*mm, 1*mm
			return o
		}},
		{"angle", 2, func() Options { o := base(); o.Col, o.Row, o.Angle = 2, 1, 90; return o }},
		{"marks", 4, func() Options {
			o := base()
			o.NoCropmark, o.Col, o.Row, o.Left, o.Top = false, 2, 1, 12*mm, 12*mm
			return o
		}},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			src := numbered(t, tc.pages)
			o := tc.opts()
			var buf bytes.Buffer
			if err := Run(context.Background(), o, src, &buf, nil); err != nil {
				t.Fatal(err)
			}
			l, err := Plan(context.Background(), o, src)
			if err != nil {
				t.Fatal(err)
			}
			want := sheetsPages(t, buf.Bytes())
			got := planPages(l)
			if fmt.Sprint(got) != fmt.Sprint(want) {
				t.Errorf("plan %v, pdf has %v", got, want)
			}
			// the layout is inside the sheet
			for i, s := range l.Sheets {
				for _, p := range s.Placements {
					if p.X < -1e-6 || p.Y < -1e-6 || p.X+p.W > l.Width+1e-6 || p.Y+p.H > l.Height+1e-6 {
						t.Errorf("sheet %d page %d is out of the sheet: %+v in %vx%v", i, p.Page, p, l.Width, l.Height)
					}
				}
			}
		})
	}
}

func TestPlanErrors(t *testing.T) {
	src := numbered(t, 4)
	mm := creator.PPMM
	o := DefaultOptions()
	o.Width, o.Height = 450*mm, 320*mm

	var gg *GridGuessError
	_, err := Plan(context.Background(), o, src)
	if !errorAs(err, &gg) || gg.Col < 1 || gg.Row < 1 {
		t.Errorf("no grid should suggest one, got %v", err)
	}

	o.Col, o.Row = 9, 1
	if _, err := Plan(context.Background(), o, src); err == nil || err.Error() != "9 columns do not fit" {
		t.Errorf("got %v", err)
	}

	o.Mode, o.Col, o.Row = ModeBooklet, 0, 0
	o.Pages = "1-3"
	if _, err := Plan(context.Background(), o, src); err == nil || err.Error() != "3 is not divisible with 4" {
		t.Errorf("got %v", err)
	}
}

func TestPlanBookletPadBlank(t *testing.T) {
	mm := creator.PPMM
	o := DefaultOptions()
	o.Width, o.Height = 450*mm, 320*mm
	o.NoCropmark = true
	o.Mode = ModeBooklet

	tt := []struct {
		pages int
		sel   string
		want  string
	}{
		{3, "1-3", "[[1] [2 3]]"},
		{5, "1-5", "[[1] [2] [3] [4 5]]"},
		{4, "1-4", "[[1 4] [2 3]]"},
	}
	for _, tc := range tt {
		src := numbered(t, tc.pages)
		o.Pages, o.PadBlank = tc.sel, false
		if tc.pages%4 != 0 {
			if _, err := Plan(context.Background(), o, src); err == nil {
				t.Errorf("%d pages without padding should fail", tc.pages)
			}
		}
		o.PadBlank = true
		l, err := Plan(context.Background(), o, src)
		if err != nil {
			t.Fatal(err)
		}
		if got := fmt.Sprint(planPages(l)); got != tc.want {
			t.Errorf("%d pages: plan %v, want %v", tc.pages, got, tc.want)
		}
		var buf bytes.Buffer
		if err := Run(context.Background(), o, src, &buf, nil); err != nil {
			t.Fatal(err)
		}
		if got, want := fmt.Sprint(sheetsPages(t, buf.Bytes())), fmt.Sprint(planPages(l)); got != want {
			t.Errorf("%d pages: pdf has %v, plan %v", tc.pages, got, want)
		}
	}
}

func TestPlanCancel(t *testing.T) {
	src := numbered(t, 4)
	o := DefaultOptions()
	o.Col, o.Row = 2, 1
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Plan(ctx, o, src); err != context.Canceled {
		t.Errorf("got %v", err)
	}
}

func TestPlanCropmarksFollowPages(t *testing.T) {
	src := numbered(t, 4)
	mm := creator.PPMM
	o := DefaultOptions()
	o.Width, o.Height = 450*mm, 320*mm
	o.Col, o.Row = 2, 1
	o.CropmarkPages = []int{1}
	l, err := Plan(context.Background(), o, src)
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Sheets) != 2 || len(l.Sheets[0].Marks) != 0 || len(l.Sheets[1].Marks) == 0 {
		t.Errorf("marks only on the second sheet, got %d and %d", len(l.Sheets[0].Marks), len(l.Sheets[1].Marks))
	}
	// 2 columns have 4 marks along the top and 2 along the left side, and as many on the opposite sides
	if n := len(l.Sheets[1].Marks); n != 2*(4+2) {
		t.Errorf("got %d marks", n)
	}
}

func TestSourceBleedIsNotCumulative(t *testing.T) {
	src := numbered(t, 1)
	w0, h0, err := src.PageSize(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	// no trim or crop box: bleed cannot change the page
	w1, h1, _ := src.PageSize(10, 10)
	w2, h2, _ := src.PageSize(0, 0)
	if w0 != w1 || h0 != h1 || w0 != w2 || h0 != h2 {
		t.Errorf("sizes changed %vx%v %vx%v %vx%v", w0, h0, w1, h1, w2, h2)
	}
}

func errorAs(err error, target **GridGuessError) bool {
	e, ok := err.(*GridGuessError)
	if ok {
		*target = e
	}
	return ok
}

func TestPlanMarksSitOnTrimEdges(t *testing.T) {
	src := numbered(t, 2)
	mm := creator.PPMM
	o := DefaultOptions()
	o.Width, o.Height = 450*mm, 320*mm
	o.Col, o.Row, o.Left, o.Top = 2, 1, 12*mm, 12*mm
	l, err := Plan(context.Background(), o, src)
	if err != nil {
		t.Fatal(err)
	}
	// vertical marks are on the edges of the 2 pages, horizontal ones on top and bottom of the grid
	xs, ys := map[float64]bool{}, map[float64]bool{}
	round := func(v float64) float64 { return float64(int(v*100+0.5)) / 100 }
	for _, m := range l.Sheets[0].Marks {
		if m.X1 == m.X2 {
			xs[round(m.X1)] = true
		} else {
			ys[round(m.Y1)] = true
		}
	}
	w, h := 200.0, 300.0
	for _, want := range []float64{o.Left, o.Left + w, o.Left + 2*w} {
		if !xs[round(want)] {
			t.Errorf("no vertical mark at %v, have %v", want, xs)
		}
	}
	for _, want := range []float64{o.Top, o.Top + h} {
		if !ys[round(want)] {
			t.Errorf("no horizontal mark at %v, have %v", want, ys)
		}
	}
}

// previews are planned while a pdf is being written, they share the source
func TestSourceSharedByPlanAndRun(t *testing.T) {
	src := numbered(t, 8)
	mm := creator.PPMM
	o := DefaultOptions()
	o.Width, o.Height = 450*mm, 320*mm
	o.Col, o.Row = 2, 1

	var wg sync.WaitGroup
	errs := make(chan error, 40)
	for i := 0; i < 4; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			var buf bytes.Buffer
			errs <- Run(context.Background(), o, src, &buf, nil)
		}()
		go func(i int) {
			defer wg.Done()
			p := o
			// the bleed changes between plans
			p.Bleedx, p.Bleedy = float64(i)*mm, float64(i)*mm
			for j := 0; j < 5; j++ {
				_, err := Plan(context.Background(), p, src)
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Error(err)
		}
	}
}

// trimmed makes a pdf of n pages of 200x300 with a trim box 10 points inside the media box.
func trimmed(t *testing.T, n int) *Source {
	t.Helper()
	c := creator.New()
	for i := 1; i <= n; i++ {
		page := model.NewPdfPage()
		page.MediaBox = &model.PdfRectangle{Urx: 200, Ury: 300}
		page.TrimBox = &model.PdfRectangle{Llx: 10, Lly: 10, Urx: 190, Ury: 290}
		if err := c.AddPage(page); err != nil {
			t.Fatal(err)
		}
		p := c.NewParagraph(fmt.Sprintf("P%d", i))
		p.SetPos(50, 50)
		if err := c.Draw(p); err != nil {
			t.Fatal(err)
		}
	}
	var buf bytes.Buffer
	if err := c.Write(&buf); err != nil {
		t.Fatal(err)
	}
	src, err := OpenSource(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	return src
}

func TestSourcePagePDF(t *testing.T) {
	src := trimmed(t, 3)
	for _, bleed := range []float64{0, 5} {
		want := 180 + 2*bleed
		pdf, err := src.PagePDF(2, bleed, bleed)
		if err != nil {
			t.Fatal(err)
		}
		one, err := OpenSource(bytes.NewReader(pdf))
		if err != nil {
			t.Fatal(err)
		}
		if one.NumPages != 1 {
			t.Fatalf("bleed %v: %d pages", bleed, one.NumPages)
		}
		w, h, err := one.PageSize(0, 0)
		if err != nil {
			t.Fatal(err)
		}
		if w != want || h != want+100 {
			t.Errorf("bleed %v: page %vx%v, want %vx%v", bleed, w, h, want, want+100)
		}
		if got := sheetsPages(t, pdf); len(got) != 1 || !slices.Equal(got[0], []int{2}) {
			t.Errorf("bleed %v: pages %v, want [[2]]", bleed, got)
		}
	}
	// the bleed of the preview does not stay with the source
	w, _, err := src.PageSize(0, 0)
	if err != nil || w != 180 {
		t.Errorf("page width %v after preview, want 180 (%v)", w, err)
	}
	if _, err := src.PagePDF(4, 0, 0); err == nil {
		t.Error("no error for a page out of the file")
	}
}

func TestPlanHasBleed(t *testing.T) {
	o := DefaultOptions()
	o.Col, o.Row = 1, 1
	o.Bleedx, o.Bleedy = 3, 4
	l, err := Plan(context.Background(), o, trimmed(t, 1))
	if err != nil {
		t.Fatal(err)
	}
	if l.Bleedx != 3 || l.Bleedy != 4 {
		t.Errorf("bleed %v %v, want 3 4", l.Bleedx, l.Bleedy)
	}
}

func TestPlanFlowWinsOverPageCount(t *testing.T) {
	mm := creator.PPMM
	src := numbered(t, 2)
	for _, duplex := range []bool{false, true} {
		o := DefaultOptions()
		o.Width, o.Height = 900*mm, 320*mm
		o.NoCropmark = true
		o.Col, o.Row = 4, 1
		o.Flow, o.Duplex = []int{1, 1, 2, 2}, duplex
		if duplex {
			o.PadBlank = false
		}
		if _, err := Plan(context.Background(), o, src); err != nil {
			t.Errorf("duplex=%v: %v", duplex, err)
		}
	}
	// without a flow the guard still applies
	o := DefaultOptions()
	o.Width, o.Height = 900*mm, 320*mm
	o.NoCropmark = true
	o.Col, o.Row, o.Duplex = 4, 1, true
	if _, err := Plan(context.Background(), o, src); err == nil {
		t.Error("duplex without flow or padblank should fail")
	}
}

func TestPlanFlowAppliedToSourcePages(t *testing.T) {
	mm := creator.PPMM
	src := numbered(t, 2)
	for _, weld := range []int{1, 2} {
		for _, duplex := range []bool{false, true} {
			o := DefaultOptions()
			o.Width, o.Height = 900*mm, 320*mm
			o.NoCropmark = true
			o.Col, o.Row = 2, 1
			o.Flow, o.Duplex, o.Weld = []int{1, 1, 2, 2}, duplex, weld
			l, err := Plan(context.Background(), o, src)
			if err != nil {
				t.Fatalf("duplex=%v weld=%d: %v", duplex, weld, err)
			}
			count := map[int]int{}
			for _, pp := range planPages(l) {
				for _, p := range pp {
					count[p]++
				}
			}
			if count[1] != 2 || count[2] != 2 {
				t.Errorf("duplex=%v weld=%d: pages used %v, want 1 and 2 twice each", duplex, weld, count)
			}
		}
	}
}
