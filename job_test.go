package impose

import (
	"bytes"
	"context"
	"fmt"
	"math"
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
		{"mergebleed", 4, func() Options {
			o := base()
			o.Col, o.Row = 2, 2
			o.Bleedx, o.Bleedy = 2*mm, 2*mm
			o.MergeBleed = true
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
	w0, h0, err := src.PageSize(0, 0, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	// no trim or crop box: the whole page is the trim, bleed expands it outward
	w1, h1, _ := src.PageSize(10, 10, 0, 0)
	if w1 != w0+20 || h1 != h0+20 {
		t.Errorf("bleed did not expand the page: %vx%v -> %vx%v", w0, h0, w1, h1)
	}
	// re-applying with no bleed goes back to the original size, it is not cumulative
	w2, h2, _ := src.PageSize(0, 0, 0, 0)
	if w0 != w2 || h0 != h2 {
		t.Errorf("sizes changed %vx%v -> %vx%v", w0, h0, w2, h2)
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

// MergeBleed halves the bleed at an internal boundary on each neighboring card, so the
// two trim-edge marks either side of that boundary end up exactly one bleed apart, not two.
func TestPlanMergeBleedMarksAtMergedTrim(t *testing.T) {
	src := numbered(t, 3)
	mm := creator.PPMM
	o := DefaultOptions()
	o.Width, o.Height = 650*mm, 320*mm
	o.Col, o.Row, o.Left, o.Top = 3, 1, 12*mm, 12*mm
	o.Bleedx, o.Bleedy = 2*mm, 2*mm
	o.MergeBleed = true
	l, err := Plan(context.Background(), o, src)
	if err != nil {
		t.Fatal(err)
	}
	// vertical marks appear twice, once near the top and once near the bottom (SheetLines
	// mirrors a duplicate); collapse those pairs by rounding to a few decimals.
	round := func(v float64) float64 { return float64(int(v*1e6+0.5)) / 1e6 }
	seen := map[float64]bool{}
	for _, m := range l.Sheets[0].Marks {
		if m.X1 == m.X2 {
			seen[round(m.X1)] = true
		}
	}
	xs := make([]float64, 0, len(seen))
	for x := range seen {
		xs = append(xs, x)
	}
	slices.Sort(xs)
	// 3 columns give 6 distinct vertical trim positions: outer-left, a pair at each of
	// the 2 internal boundaries, outer-right.
	if len(xs) != 6 {
		t.Fatalf("want 6 vertical marks, got %d: %v", len(xs), xs)
	}
	for _, pair := range [][2]int{{1, 2}, {3, 4}} {
		if gap := xs[pair[1]] - xs[pair[0]]; math.Abs(gap-o.Bleedx) > 1e-6 {
			t.Errorf("internal mark gap %v, want %v (one bleed, not two)", gap, o.Bleedx)
		}
	}
}

// MergeBleed halves the bleed at internal grid boundaries, leaves outer edges alone, and
// never creates a gap or overlap between neighboring cards.
func TestPlanMergeBleed(t *testing.T) {
	mm := creator.PPMM
	base := func(merge bool) Options {
		o := DefaultOptions()
		o.Width, o.Height = 450*mm, 320*mm
		o.NoCropmark = true
		o.Col, o.Row = 3, 1
		o.Bleedx, o.Bleedy = 2*mm, 2*mm
		o.MergeBleed = merge
		return o
	}
	src := numbered(t, 3)

	plain, err := Plan(context.Background(), base(false), src)
	if err != nil {
		t.Fatal(err)
	}
	merged, err := Plan(context.Background(), base(true), src)
	if err != nil {
		t.Fatal(err)
	}
	byX := func(l *Layout) []Placement {
		pp := slices.Clone(l.Sheets[0].Placements)
		slices.SortFunc(pp, func(a, b Placement) int {
			switch {
			case a.X < b.X:
				return -1
			case a.X > b.X:
				return 1
			default:
				return 0
			}
		})
		return pp
	}
	pq, pp := byX(plain), byX(merged)
	if len(pq) != 3 || len(pp) != 3 {
		t.Fatalf("want 3 placements, got %d without merge, %d with", len(pq), len(pp))
	}

	// the grid starts at the same place (the left outer bleed is untouched); it ends
	// earlier than without merging, since 2 internal boundaries each saved one bleed
	if math.Abs(pp[0].X-pq[0].X) > 1e-6 {
		t.Errorf("left outer edge moved: got %v want %v", pp[0].X, pq[0].X)
	}
	saved := (pq[2].X + pq[2].W) - (pp[2].X + pp[2].W)
	if want := 2 * 2 * mm; math.Abs(saved-want) > 1e-6 {
		t.Errorf("grid span shrank by %v, want %v (2 internal boundaries x one bleed)", saved, want)
	}
	// placements are contiguous: no gap, no overlap
	for i := 0; i+1 < len(pp); i++ {
		if got, want := pp[i].X+pp[i].W, pp[i+1].X; math.Abs(got-want) > 1e-6 {
			t.Errorf("gap/overlap between placements %d and %d: %v vs %v", i, i+1, got, want)
		}
	}
	// the two edge cells each lose half a bleed (one internal side), the middle cell
	// loses a full bleed (two internal sides)
	half := mm // half of the 2mm bleed
	if got, want := pq[0].W-pp[0].W, half; math.Abs(got-want) > 1e-6 {
		t.Errorf("left cell width shrank by %v, want %v", got, want)
	}
	if got, want := pq[1].W-pp[1].W, 2*half; math.Abs(got-want) > 1e-6 {
		t.Errorf("middle cell width shrank by %v, want %v", got, want)
	}
	if got, want := pq[2].W-pp[2].W, half; math.Abs(got-want) > 1e-6 {
		t.Errorf("right cell width shrank by %v, want %v", got, want)
	}
	// NearX/FarX record exactly what was clipped, for consumers (the GUI preview) that
	// need to know which side of a placement is a merged, half-bleed edge
	wantInsets := [][2]float64{{0, half}, {half, half}, {half, 0}}
	for i, p := range pp {
		if p.NearX != wantInsets[i][0] || p.FarX != wantInsets[i][1] {
			t.Errorf("placement %d: NearX=%v FarX=%v, want %v", i, p.NearX, p.FarX, wantInsets[i])
		}
		if p.NearY != 0 || p.FarY != 0 {
			t.Errorf("placement %d: NearY=%v FarY=%v, want 0,0 (single row)", i, p.NearY, p.FarY)
		}
	}
	for _, p := range pq {
		if p.NearX != 0 || p.FarX != 0 {
			t.Errorf("without merge, NearX=%v FarX=%v, want 0,0", p.NearX, p.FarX)
		}
	}
}

// MergeBleed merges across clone boundaries too, exactly like grid columns, when there is
// no explicit gap between clones (ClonePadX/Y == 0).
func TestPlanMergeBleedAcrossClones(t *testing.T) {
	mm := creator.PPMM
	o := DefaultOptions()
	o.Width, o.Height = 450*mm, 320*mm
	o.NoCropmark = true
	o.Mode = ModeRepeat
	o.CloneX, o.CloneY = 3, 1
	o.Bleedx, o.Bleedy = 2*mm, 2*mm
	o.MergeBleed = true
	src := numbered(t, 1)

	l, err := Plan(context.Background(), o, src)
	if err != nil {
		t.Fatal(err)
	}
	pp := slices.Clone(l.Sheets[0].Placements)
	slices.SortFunc(pp, func(a, b Placement) int {
		switch {
		case a.X < b.X:
			return -1
		case a.X > b.X:
			return 1
		default:
			return 0
		}
	})
	if len(pp) != 3 {
		t.Fatalf("want 3 clones, got %d", len(pp))
	}
	half := mm
	wantInsets := [][2]float64{{0, half}, {half, half}, {half, 0}}
	for i, p := range pp {
		if p.NearX != wantInsets[i][0] || p.FarX != wantInsets[i][1] {
			t.Errorf("clone %d: NearX=%v FarX=%v, want %v", i, p.NearX, p.FarX, wantInsets[i])
		}
	}
	for i := 0; i+1 < len(pp); i++ {
		if got, want := pp[i].X+pp[i].W, pp[i+1].X; math.Abs(got-want) > 1e-6 {
			t.Errorf("gap/overlap between clones %d and %d: %v vs %v", i, i+1, got, want)
		}
	}
}

// An explicit, nonzero ClonePadX means a deliberate gap between clones, so that boundary
// must not merge even when MergeBleed is on: both clones keep their full, outer bleed there.
func TestPlanMergeBleedClonePadKeepsOuterEdges(t *testing.T) {
	mm := creator.PPMM
	o := DefaultOptions()
	o.Width, o.Height = 450*mm, 320*mm
	o.NoCropmark = true
	o.Mode = ModeRepeat
	o.CloneX, o.CloneY = 2, 1
	o.ClonePadX = 10 * mm
	o.Bleedx, o.Bleedy = 2*mm, 2*mm
	o.MergeBleed = true
	src := numbered(t, 1)

	l, err := Plan(context.Background(), o, src)
	if err != nil {
		t.Fatal(err)
	}
	pp := slices.Clone(l.Sheets[0].Placements)
	slices.SortFunc(pp, func(a, b Placement) int {
		switch {
		case a.X < b.X:
			return -1
		case a.X > b.X:
			return 1
		default:
			return 0
		}
	})
	if len(pp) != 2 {
		t.Fatalf("want 2 clones, got %d", len(pp))
	}
	for i, p := range pp {
		if p.NearX != 0 || p.FarX != 0 {
			t.Errorf("clone %d: NearX=%v FarX=%v, want 0,0 (padded clone boundary keeps full bleed)", i, p.NearX, p.FarX)
		}
	}
	if gap := pp[1].X - (pp[0].X + pp[0].W); math.Abs(gap-10*mm) > 1e-6 {
		t.Errorf("gap between clones %v, want the clone pad %v", gap, 10*mm)
	}
}

// Confirms the real PDF content, not just the computed geometry: clone copies are drawn
// directly (not through the shared, cheaply-restamped row block) when MergeBleed is on, so
// each one can get its own clip; this checks that path actually produces the right clips.
func TestRunMergeBleedClipsAcrossClonesFromTheFarSide(t *testing.T) {
	src := numbered(t, 1)
	mm := creator.PPMM
	o := DefaultOptions()
	o.Width, o.Height = 450*mm, 320*mm
	o.NoCropmark = true
	o.Mode = ModeRepeat
	o.CloneX, o.CloneY = 2, 1
	o.Bleedx, o.Bleedy = 2*mm, 2*mm
	o.MergeBleed = true

	var out bytes.Buffer
	if err := Run(context.Background(), o, src, &out, nil); err != nil {
		t.Fatal(err)
	}
	r, err := model.NewPdfReader(bytes.NewReader(out.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	page, err := r.GetPage(1)
	if err != nil {
		t.Fatal(err)
	}
	cs, err := page.GetAllContentStreams()
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`1 0 0 1 (-?[\d.]+) -?[\d.]+ cm\s*\nq\s*\nq\s*\n(-?[\d.]+) [\d.-]+ ([\d.]+) [\d.]+ re\s*\nW\s*\nn`)
	matches := re.FindAllStringSubmatch(cs, -1)
	half := mm
	// the source page (via numbered) has no TrimBox, so the whole 200x300 MediaBox is the
	// trim; the full tile width is trim(200) + 2*bleedx
	fullW := 200 + 2*2*mm
	type hit struct{ absX, clipX float64 }
	var hits []hit
	for _, m := range matches {
		absX, _ := strconv.ParseFloat(m[1], 64)
		clipX, _ := strconv.ParseFloat(m[2], 64)
		w, _ := strconv.ParseFloat(m[3], 64)
		if math.Abs(w-(fullW-half)) > 0.05 {
			continue
		}
		hits = append(hits, hit{absX, clipX})
	}
	if len(hits) != 2 {
		t.Fatalf("want 2 clone clip rectangles, got %d", len(hits))
	}
	left, right := hits[0], hits[1]
	if left.absX > right.absX {
		left, right = right, left
	}
	if math.Abs(left.clipX-0) > 0.05 {
		t.Errorf("left clone (outer left) must be clipped from its far/right side only: clip x-offset %v, want 0", left.clipX)
	}
	if math.Abs(right.clipX-half) > 0.05 {
		t.Errorf("right clone (outer right) must be clipped from its near/left side only: clip x-offset %v, want %v (half a bleed)", right.clipX, half)
	}
}

// CenterX must center the grid using its actual, merge-shrunk span, not the larger span
// it would have without MergeBleed, otherwise the left/right margins come out uneven.
func TestPlanMergeBleedCenters(t *testing.T) {
	mm := creator.PPMM
	o := DefaultOptions()
	o.Width, o.Height = 450*mm, 320*mm
	o.NoCropmark = true
	o.Col, o.Row = 3, 1
	o.Bleedx, o.Bleedy = 2*mm, 2*mm
	o.MergeBleed = true
	o.CenterX = true
	src := numbered(t, 3)

	l, err := Plan(context.Background(), o, src)
	if err != nil {
		t.Fatal(err)
	}
	pp := l.Sheets[0].Placements
	left, right := pp[0].X, 0.0
	for _, p := range pp {
		if p.X < left {
			left = p.X
		}
		if p.X+p.W > right {
			right = p.X + p.W
		}
	}
	leftMargin, rightMargin := left, l.Width-right
	if math.Abs(leftMargin-rightMargin) > 1e-6 {
		t.Errorf("not centered: left margin %v, right margin %v", leftMargin, rightMargin)
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
		pdf, err := src.PagePDF(2, bleed, bleed, 0, 0, 0, 0, 0, 0)
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
		w, h, err := one.PageSize(0, 0, 0, 0)
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
	w, _, err := src.PageSize(0, 0, 0, 0)
	if err != nil || w != 180 {
		t.Errorf("page width %v after preview, want 180 (%v)", w, err)
	}
	if _, err := src.PagePDF(4, 0, 0, 0, 0, 0, 0, 0, 0); err == nil {
		t.Error("no error for a page out of the file")
	}
}

// Row 0 is always the sheet's outer (top) edge, so its drawn content must only ever be
// clipped on its far (bottom, shared-with-row-1) side, never its near (top) side. This
// guards the actual drawn PDF content, not just the computed Placement geometry (which
// was already correct): Clip's y is the block's own native, bottom-up PDF coordinate,
// unlike SetPos's top-down y, and conflating them clipped row 0's content from the wrong
// (top) side, shifting what's visible down by half a bleed — invisible to text-position
// extraction (Tj/Td coordinates aren't affected by a separate clip operator), so this
// reads the clip rectangle's own "x y w h re W n" operator out of the content stream.
func TestRunMergeBleedClipsOuterRowFromTheFarSide(t *testing.T) {
	c := creator.New()
	for range 2 {
		page := model.NewPdfPage()
		page.MediaBox = &model.PdfRectangle{Urx: 200, Ury: 300}
		page.TrimBox = &model.PdfRectangle{Llx: 10, Lly: 10, Urx: 190, Ury: 290}
		if err := c.AddPage(page); err != nil {
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

	mm := creator.PPMM
	o := DefaultOptions()
	o.Width, o.Height = 250*mm, 250*mm
	o.NoCropmark = true
	o.Col, o.Row = 1, 2
	o.Bleedx, o.Bleedy = 3*mm, 3*mm
	o.MergeBleed = true

	var out bytes.Buffer
	if err := Run(context.Background(), o, src, &out, nil); err != nil {
		t.Fatal(err)
	}
	r, err := model.NewPdfReader(bytes.NewReader(out.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	page, err := r.GetPage(1)
	if err != nil {
		t.Fatal(err)
	}
	cs, err := page.GetAllContentStreams()
	if err != nil {
		t.Fatal(err)
	}
	// each placement is drawn straight to the sheet behind its own "1 0 0 1 0 <absY> cm"
	// (its absolute position on the sheet, pdf-native: higher means closer to the sheet's
	// own top) immediately followed by its clip rectangle: capture the two together so the
	// clip offset can be checked against the specific row it belongs to, not just "some row".
	re := regexp.MustCompile(`1 0 0 1 0 (-?[\d.]+) cm\s*\nq\s*\nq\s*\n[\d.-]+ (-?[\d.]+) [\d.]+ ([\d.]+) re\s*\nW\s*\nn`)
	matches := re.FindAllStringSubmatch(cs, -1)
	// the block's full (unclipped) height is trim(280) + 2*bleed(2*3mm); half a bleed
	fullH := 280 + 2*3*mm
	half := 1.5 * mm
	type hit struct{ absY, clipY float64 }
	var hits []hit
	for _, m := range matches {
		absY, _ := strconv.ParseFloat(m[1], 64)
		clipY, _ := strconv.ParseFloat(m[2], 64)
		h, _ := strconv.ParseFloat(m[3], 64)
		if math.Abs(h-(fullH-half)) > 0.05 {
			continue // not one of our two row clips
		}
		hits = append(hits, hit{absY, clipY})
	}
	if len(hits) != 2 {
		t.Fatalf("want 2 row clip rectangles, got %d", len(hits))
	}
	// row 0 (the sheet's own top) has the higher absolute (pdf-native) position
	row0, row1 := hits[0], hits[1]
	if row0.absY < row1.absY {
		row0, row1 = row1, row0
	}
	if math.Abs(row0.clipY-half) > 0.05 {
		t.Errorf("row 0 (outer top) must be clipped from its far/bottom side only: clip y-offset %v, want %v (half a bleed)", row0.clipY, half)
	}
	if math.Abs(row1.clipY-0) > 0.05 {
		t.Errorf("row 1 (outer bottom) must be clipped from its near/top side only: clip y-offset %v, want 0", row1.clipY)
	}
}

func TestSourceTrimOverride(t *testing.T) {
	// trimmed's real TrimBox is 180x280, centered at (100, 150) within its 200x300 MediaBox
	src := trimmed(t, 1)
	src.rd.SetTrim(150, 250)
	src.rd.SetBleed(10, 6)
	page, err := src.rd.GetPage(1)
	if err != nil {
		t.Fatal(err)
	}
	tb := page.TrimBox
	if w, h := tb.Urx-tb.Llx, tb.Ury-tb.Lly; w != 150 || h != 250 {
		t.Errorf("trim box %vx%v, want 150x250", w, h)
	}
	if cx, cy := (tb.Llx+tb.Urx)/2, (tb.Lly+tb.Ury)/2; cx != 100 || cy != 150 {
		t.Errorf("trim box centered at %v,%v, want 100,150", cx, cy)
	}
	mb := page.MediaBox
	if w, h := mb.Urx-mb.Llx, mb.Ury-mb.Lly; w != 150+2*10 || h != 250+2*6 {
		t.Errorf("media box %vx%v, want %vx%v", w, h, 150+2*10, 250+2*6)
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
