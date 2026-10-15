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
