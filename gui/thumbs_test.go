package main

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/gen2brain/go-fitz"
	"github.com/innermond/impose"
	"github.com/unidoc/unipdf/v3/creator"
	"github.com/unidoc/unipdf/v3/model"
)

// trimmedPDF makes a pdf of n pages of 200x300 with a trim box 10 points inside,
// each page has its number near the top left corner and a bar along its left edge.
func trimmedPDF(t *testing.T, n int) []byte {
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
		p.SetFontSize(30)
		p.SetPos(30, 30)
		bar := c.NewRectangle(10, 100, 20, 150)
		bar.SetFillColor(creator.ColorRGBFromHex("#000000"))
		for _, d := range []creator.Drawable{p, bar} {
			if err := c.Draw(d); err != nil {
				t.Fatal(err)
			}
		}
	}
	var buf bytes.Buffer
	if err := c.Write(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func trimmedSource(t *testing.T, n int) *impose.Source {
	t.Helper()
	src, err := impose.OpenSource(bytes.NewReader(trimmedPDF(t, n)))
	if err != nil {
		t.Fatal(err)
	}
	return src
}

// savePNG keeps an image to look at, when SHOT_DIR is set
func savePNG(t *testing.T, img image.Image, name string) {
	dir := os.Getenv("SHOT_DIR")
	if dir == "" {
		return
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".png"), buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestTierFor(t *testing.T) {
	cases := []struct {
		ppp, w, h float64
		want      int
	}{
		{0.1, 600, 800, 0},
		{0.25, 600, 800, 0},
		{0.3, 600, 800, 1},
		{1, 600, 800, 2},
		{1.5, 600, 800, 3},
		{9, 100, 100, 4},
		// 3000 points at 1 px for a point is too big
		{1, 3000, 600, 1},
	}
	for _, c := range cases {
		if got := tierFor(c.ppp, c.w, c.h); got != c.want {
			t.Errorf("tierFor(%v, %v, %v) = %d, want %d", c.ppp, c.w, c.h, got, c.want)
		}
	}
}

func TestOrient(t *testing.T) {
	// 2 wide, 3 tall, every pixel has its own red
	img := image.NewRGBA(image.Rect(0, 0, 2, 3))
	for y := range 3 {
		for x := range 2 {
			img.Set(x, y, color.RGBA{uint8(10*y + x), 0, 0, 255})
		}
	}
	red := func(o *image.RGBA, x, y int) int { return int(o.RGBAAt(x, y).R) }
	cases := []struct {
		turns  int
		mx, my bool
		w, h   int
		// what is in the top left and top right corners
		tl, tr int
	}{
		{0, false, false, 2, 3, 0, 1},
		// counter clockwise: the top right comes to the top left
		{1, false, false, 3, 2, 1, 21},
		{2, false, false, 2, 3, 21, 20},
		{3, false, false, 3, 2, 20, 0},
		{0, true, false, 2, 3, 1, 0},
		{0, false, true, 2, 3, 20, 21},
		{1, true, false, 3, 2, 21, 1},
	}
	for _, c := range cases {
		o := orient(img, c.turns, c.mx, c.my)
		if o.Bounds().Dx() != c.w || o.Bounds().Dy() != c.h {
			t.Errorf("%+v: size %v", c, o.Bounds())
			continue
		}
		if tl, tr := red(o, 0, 0), red(o, c.w-1, 0); tl != c.tl || tr != c.tr {
			t.Errorf("%+v: corners %d %d", c, tl, tr)
		}
	}
}

func TestThumbsKeepTheLastUsed(t *testing.T) {
	th := newThumbs(nil)
	th.limit = 3 * 4 * 10 * 10
	key := func(p int) thumbKey { return thumbKey{page: p, tier: 1} }
	for p := 1; p <= 3; p++ {
		th.putLocked(key(p), image.NewRGBA(image.Rect(0, 0, 10, 10)))
	}
	// 1 is used, so 2 is the oldest
	if img, _ := th.best(key(1)); img == nil {
		t.Fatal("1 is missing")
	}
	th.putLocked(key(4), image.NewRGBA(image.Rect(0, 0, 10, 10)))
	for p, want := range map[int]bool{1: true, 2: false, 3: true, 4: true} {
		if img, _ := th.best(key(p)); (img != nil) != want {
			t.Errorf("page %d kept %v, want %v", p, img != nil, want)
		}
	}
	if th.bytes != th.limit {
		t.Errorf("bytes %d, want %d", th.bytes, th.limit)
	}
	// another tier stands in while the wanted one is missing
	if img, tier := th.best(thumbKey{page: 3, tier: 4}); img == nil || tier != 1 {
		t.Errorf("best of another tier: %v %d", img != nil, tier)
	}
}

func TestThumbsRenderInBackground(t *testing.T) {
	var mu sync.Mutex
	rendered := []int{}
	ready := make(chan struct{}, 10)
	th := newThumbs(func() { ready <- struct{}{} })
	th.render = func(_ *impose.Source, k thumbKey) (*image.RGBA, error) {
		mu.Lock()
		rendered = append(rendered, k.page)
		mu.Unlock()
		if k.page == 3 {
			return nil, fmt.Errorf("broken")
		}
		return image.NewRGBA(image.Rect(0, 0, 4, 4)), nil
	}
	// nothing without a source
	th.want([]thumbKey{{page: 1}})
	if len(th.queue) != 0 {
		t.Fatal("queued without a source")
	}
	th.setSource(&impose.Source{NumPages: 3})
	th.want([]thumbKey{{page: 1}, {page: 2}, {page: 1}, {page: 3}})
	for range 2 {
		<-ready
	}
	waitFor(t, "the workers", func() bool {
		th.mu.Lock()
		defer th.mu.Unlock()
		return th.workers == 0
	})
	if img, _ := th.best(thumbKey{page: 2}); img == nil {
		t.Error("page 2 is not rendered")
	}
	// a page that failed is not tried again, a page that is there is not rendered again
	th.want([]thumbKey{{page: 3}, {page: 1}})
	if len(th.queue) != 0 {
		t.Errorf("queue %v", th.queue)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(rendered) != 3 {
		t.Errorf("rendered %v", rendered)
	}
	// another file forgets everything
	th.setSource(&impose.Source{NumPages: 3})
	if img, _ := th.best(thumbKey{page: 2}); img != nil {
		t.Error("page 2 of the previous file is kept")
	}
}

func TestRenderPageHasBleed(t *testing.T) {
	src := trimmedSource(t, 2)
	img, err := renderPage(src, thumbKey{page: 2, bleedx: 3, bleedy: 5, tier: 2})
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 186 || b.Dy() != 290 {
		t.Errorf("size %v, want 186x290", b)
	}
	savePNG(t, img, "page2")
}

// darkBox is the box of the dark pixels of img inside r
func darkBox(img image.Image, r image.Rectangle) image.Rectangle {
	box := image.Rectangle{}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			c := color.GrayModel.Convert(img.At(x, y)).(color.Gray)
			if c.Y < 100 {
				box = box.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return box
}

// TestPreviewMatchesOutput puts the thumbnails where the preview puts them and checks
// the result looks like the pdf made by the imposition.
func TestPreviewMatchesOutput(t *testing.T) {
	src := trimmedSource(t, 4)
	th := newThumbs(nil)
	th.setSource(src)
	for _, angle := range []float64{0, 90, 180, 270} {
		o := impose.DefaultOptions()
		o.Width, o.Height = 600, 600
		o.Col, o.Row = 2, 1
		o.Angle = angle
		o.Bleedx, o.Bleedy = 4, 4
		o.NoCropmark = true
		l, err := impose.Plan(context.Background(), o, src)
		if err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		if err := impose.Run(context.Background(), o, src, &out, nil); err != nil {
			t.Fatal(err)
		}
		doc, err := fitz.NewFromMemory(out.Bytes())
		if err != nil {
			t.Fatal(err)
		}
		want, err := doc.ImageDPI(0, 72)
		doc.Close()
		if err != nil {
			t.Fatal(err)
		}

		got := image.NewRGBA(want.Bounds())
		for i := range got.Pix {
			got.Pix[i] = 255
		}
		for _, pl := range l.Sheets[0].Placements {
			k := thumbKey{page: pl.Page, bleedx: l.Bleedx, bleedy: l.Bleedy, tier: 2}
			img, err := th.render(src, k)
			if err != nil {
				t.Fatal(err)
			}
			th.putLocked(k, img)
			turns, ok := quarterTurns(pl.Angle)
			if !ok {
				t.Fatalf("angle %v", pl.Angle)
			}
			o := th.oriented(k, turns, false, false)
			r := image.Rect(int(pl.X+0.5), int(pl.Y+0.5), int(pl.X+pl.W+0.5), int(pl.Y+pl.H+0.5))
			for y := r.Min.Y; y < r.Max.Y; y++ {
				for x := r.Min.X; x < r.Max.X; x++ {
					sx := (x - r.Min.X) * o.Bounds().Dx() / r.Dx()
					sy := (y - r.Min.Y) * o.Bounds().Dy() / r.Dy()
					got.Set(x, y, o.At(sx, sy))
				}
			}
			gb, wb := darkBox(got, r), darkBox(want, r)
			if gb.Empty() || abs(gb.Min.X-wb.Min.X) > 2 || abs(gb.Min.Y-wb.Min.Y) > 2 ||
				abs(gb.Max.X-wb.Max.X) > 2 || abs(gb.Max.Y-wb.Max.Y) > 2 {
				t.Errorf("angle %v page %d: preview %v, pdf %v", angle, pl.Page, gb, wb)
			}
		}
		savePNG(t, got, fmt.Sprintf("preview%v", angle))
		savePNG(t, want, fmt.Sprintf("output%v", angle))
	}
}

func abs(v int) int { return max(v, -v) }
