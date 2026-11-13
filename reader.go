package impose

import (
	"errors"
	"io"

	"github.com/unidoc/unipdf/v3/creator"
	"github.com/unidoc/unipdf/v3/model"
)

type PdfReader struct {
	*model.PdfReader
	pg     *model.PdfPage
	dx, dy float64
	// tw, th override the detected trim size, 0 means keep it as detected.
	tw, th float64
	// orig keeps the mediabox of pages as they are in the file, before any adjustment
	orig map[int]model.PdfRectangle
}

func NewReader(f io.ReadSeeker, dx, dy float64) (*PdfReader, error) {
	r, err := model.NewPdfReader(f)
	if err != nil {
		return nil, err
	}
	return &PdfReader{PdfReader: r, dx: dx, dy: dy, orig: map[int]model.PdfRectangle{}}, nil
}

// SetBleed changes the bleed pages are expanded with, it works on pages got after it.
func (r *PdfReader) SetBleed(dx, dy float64) {
	r.dx, r.dy = dx, dy
}

// SetTrim overrides the detected trim size pages are expanded from, it works on pages got
// after it. w and h 0 means use the file's own TrimBox, CropBox or MediaBox as before.
func (r *PdfReader) SetTrim(w, h float64) {
	r.tw, r.th = w, h
}

// transformMediaBox calculates the new MediaBox coordinates after rotation
func transformMediaBox(llx, lly, urx, ury float64, rotation int64) (float64, float64, float64, float64) {
	// Calculate width and height
	width := urx - llx
	height := ury - lly
	// Calculate the center of the original MediaBox
	cx := llx + width/2
	cy := lly + height/2

	switch rotation {
	case 90:
		return cx - height/2, cy - width/2, cx + height/2, cy + width/2
	case 180:
		return cx - width/2, cy - height/2, cx + width/2, cy + height/2
	case 270:
		return cx - height/2, cy - width/2, cx + height/2, cy + width/2
	default:
		return llx, lly, urx, ury
	}
}

func (r *PdfReader) AdjustMediaBox(num int) error {
	if r.pg == nil {
		return errors.New("No page. Need to call GetPage(num) before")
	}
	// TODO involve BleedBox
	// the trim box is the page's final, physically cut-off size: TrimBox if the file
	// declares one, else CropBox, else the page's own original MediaBox (nothing more
	// specific is known, so the whole page is taken as the trim)
	tbox, err := r.pg.GetBox("TrimBox")
	if err != nil {
		cbox, err := r.pg.GetBox("CropBox")
		if err == nil {
			tbox = cbox
		} else {
			mediabox, ok := r.orig[num]
			if !ok {
				return errors.New("No mediabox")
			}
			tbox = &mediabox
		}
	}
	// an override replaces the detected trim box with one of the wanted size, centered
	// on the same spot, for files whose own boxes do not match the real, physical trim
	if r.tw > 0 && r.th > 0 {
		cx, cy := (tbox.Llx+tbox.Urx)/2, (tbox.Lly+tbox.Ury)/2
		tbox = &model.PdfRectangle{
			Llx: cx - r.tw/2, Urx: cx + r.tw/2,
			Lly: cy - r.th/2, Ury: cy + r.th/2,
		}
	}
	r.pg.TrimBox = tbox

	// MediaBox = TrimBox expanded outward by bleed on every edge
	mbox := &model.PdfRectangle{}
	llx := tbox.Llx - r.dx
	lly := tbox.Lly - r.dy
	urx := tbox.Urx + r.dx
	ury := tbox.Ury + r.dy

	//if r.pg.Rotate != nil {
	//		llx, lly, urx, ury = transformMediaBox(llx, lly, urx, ury, *r.pg.Rotate)
	//}

	mbox.Llx = llx
	mbox.Lly = lly
	mbox.Urx = urx
	mbox.Ury = ury

	r.pg.MediaBox = mbox
	return nil
}

func (r *PdfReader) GetPage(num int) (*model.PdfPage, error) {
	var err error
	r.pg, err = r.PdfReader.GetPage(num)
	if err != nil {
		return nil, err
	}
	// remember the mediabox of the file, to adjust it again with other bleed
	if _, ok := r.orig[num]; !ok {
		mb, err := r.pg.GetMediaBox()
		if err != nil {
			return nil, err
		}
		r.orig[num] = *mb
	}
	err = r.AdjustMediaBox(num)
	if err != nil {
		return nil, err
	}
	return r.pg, nil
}

func (r *PdfReader) BlockFromPage(num int) (*creator.Block, error) {
	_, err := r.GetPage(num)
	if err != nil {
		return nil, err
	}
	block, err := creator.NewBlockFromPage(r.pg)
	if err != nil {
		return nil, err
	}
	return block, nil
}
