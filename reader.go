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
	// orig keeps the mediabox of pages as they are in the file, before any adjustment
	orig map[int]model.PdfRectangle
}

func NewReader(f io.ReadSeeker, dx, dy float64) (*PdfReader, error) {
	r, err := model.NewPdfReader(f)
	if err != nil {
		return nil, err
	}
	return &PdfReader{r, nil, dx, dy, map[int]model.PdfRectangle{}}, nil
}

// SetBleed changes the bleed pages are expanded with, it works on pages got after it.
func (r *PdfReader) SetBleed(dx, dy float64) {
	r.dx, r.dy = dx, dy
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
	// adjust mediabox expanding from trim/crop box with bleed amounts but no more than actual mediabox
	// TrimBox is the final page
	tbox, err := r.pg.GetBox("TrimBox")
	// no trimbox
	if err != nil {
		cbox, err := r.pg.GetBox("CropBox")
		if err == nil {
			tbox = cbox
			r.pg.TrimBox = cbox
		} else {
			// no trimbox or cropbox
			// only mediabox so dont adjust
			return nil
		}
	}
	// MediaBox = TrimBox + bleed
	mbox := &model.PdfRectangle{}
	// expand with bleedx and bleedy
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

	// what?? we have at least a cropbox or a trimbox but not a mediabox???
	mediabox, ok := r.orig[num]
	if !ok {
		return errors.New("No mediabox")
	}

	// do not exceed unadjusted real mediabox
	// mediabox width smaller than adjusted mbox width
	if mediabox.Urx-mediabox.Llx < mbox.Urx-mbox.Llx ||
		mediabox.Ury-mediabox.Lly < mbox.Ury-mbox.Lly {
		// use mediabox
		mb := mediabox
		mbox = &mb
	}
	// adjust
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
