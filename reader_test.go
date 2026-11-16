package impose

import (
	"bytes"
	"testing"

	"github.com/unidoc/unipdf/v3/creator"
	"github.com/unidoc/unipdf/v3/model"
)

func TestSuggestedBleedFromOriginalMediaBox(t *testing.T) {
	// trimmed's real TrimBox is 180x280, centered inside its 200x300 MediaBox: a 10pt
	// margin on every side, with no BleedBox declared.
	src := trimmed(t, 1)
	dx, dy, err := src.SuggestedBleed(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if dx != 10 || dy != 10 {
		t.Errorf("suggested bleed %vx%v, want 10x10", dx, dy)
	}
}

func TestSuggestedBleedFromBleedBox(t *testing.T) {
	c := creator.New()
	page := model.NewPdfPage()
	page.MediaBox = &model.PdfRectangle{Urx: 200, Ury: 300}
	page.TrimBox = &model.PdfRectangle{Llx: 10, Lly: 10, Urx: 190, Ury: 290}
	// an asymmetric BleedBox: 3pt near the left, 7 near the right, 4 top and bottom.
	page.BleedBox = &model.PdfRectangle{Llx: 7, Lly: 6, Urx: 197, Ury: 294}
	if err := c.AddPage(page); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := c.Write(&buf); err != nil {
		t.Fatal(err)
	}
	src, err := OpenSource(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	dx, dy, err := src.SuggestedBleed(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	// the tighter side wins: min(10-7, 197-190) = 3, min(10-6, 294-290) = 4
	if dx != 3 || dy != 4 {
		t.Errorf("suggested bleed %vx%v, want 3x4", dx, dy)
	}
}

func TestSuggestedBleedNoMargin(t *testing.T) {
	// numbered's pages have no TrimBox of their own, so the trim falls back to the whole
	// MediaBox: there is no margin left to suggest a bleed from.
	src := numbered(t, 1)
	dx, dy, err := src.SuggestedBleed(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if dx != 0 || dy != 0 {
		t.Errorf("suggested bleed %vx%v, want 0x0", dx, dy)
	}
}
