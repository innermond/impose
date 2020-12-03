package impose

import (
	"errors"
	"io"
	"log"

	"github.com/unidoc/unipdf/v3/creator"
	"github.com/unidoc/unipdf/v3/model"
)

type PdfReader struct {
	*model.PdfReader
	pg *model.PdfPage
	// forced bleeds
	dx, dy float64
	// natural bleeds
	bx, by float64
}

func NewReader(f io.ReadSeeker, dx, dy float64) (*PdfReader, error) {
	r, err := model.NewPdfReader(f)
	if err != nil {
		return nil, err
	}
	return &PdfReader{r, nil, dx, dy, 0.0, 0.0}, nil
}

// the trimbox is the base of calculated mediabox
func (r *PdfReader) AdjustTrimBox(newbox *model.PdfRectangle) (*model.PdfRectangle, error) {
	if r.pg == nil {
		return nil, errors.New("No page. Need to call GetPage(num) before")
	}

	tbox, err := r.pg.GetBox("TrimBox")
	log.Printf("original trimbox: %v \n", tbox)
	// no trimbox
	if err != nil {
		// try cropbox
		cbox, err := r.pg.GetBox("CropBox")
		log.Printf("original cropbox: %v \n", cbox)
		if err == nil {
			tbox = cbox
		} else {
			// use original mediabox as trimbox
			tbox, err = r.pg.GetMediaBox()
			if err != nil {
				return nil, err
			}
		}
	}
	log.Printf("guessed tbox: %v \n", tbox)

	if newbox != nil {
		// lower point is relative to the estimated tbox
		tbox.Llx += newbox.Llx
		tbox.Lly += newbox.Lly
		// upper point is calculated as newbox's upper point contains the width & height desired
		tbox.Urx = tbox.Llx + newbox.Urx
		tbox.Ury = tbox.Lly + newbox.Ury
		log.Printf("forced tbox: %v \n", tbox)
	}

	r.pg.TrimBox = tbox

	return tbox, nil
}

func (r *PdfReader) AdjustMediaBox() (*model.PdfRectangle, error) {
	if r.pg == nil {
		return nil, errors.New("No page. Need to call GetPage(num) before")
	}

	//TODO force mediabox from trim + bleed
	tbox, err := r.pg.GetBox("TrimBox")
	if err != nil {
		return nil, err
	}

	// expand with bleedx and bleedy
	dx := r.dx
	dy := r.dy
	if r.dx == 0 {
		dx = 2 * creator.PPMM
	}
	if r.dy == 0 {
		dy = 2 * creator.PPMM
	}
	// MediaBox = TrimBox + 2*bleed
	mbox := &model.PdfRectangle{}
	mbox.Llx = tbox.Llx - dx
	mbox.Lly = tbox.Lly - dy
	mbox.Urx = tbox.Urx + dx
	mbox.Ury = tbox.Ury + dy
	// move to 0,0
	mbox.Urx -= mbox.Llx
	mbox.Llx = 0
	mbox.Ury -= mbox.Lly
	mbox.Lly = 0
	tbox.Llx = dx
	tbox.Urx = mbox.Urx - dx
	tbox.Lly = dy
	tbox.Ury = mbox.Ury - dy
	r.pg.MediaBox = mbox
	r.pg.TrimBox = tbox
	// if mediabox is smaller than trim + bleed computed enlarge
	// mediabox width smaller than adjusted mbox width
	/*if mediabox.Urx-mediabox.Llx <= mbox.Urx-mbox.Llx ||
		mediabox.Ury-mediabox.Lly <= mbox.Ury-mbox.Lly {
		// use mediabox
		mediabox = mbox
	}*/
	// adjust
	//r.pg.MediaBox = mediabox
	return mbox, nil
}

func (r *PdfReader) GetBleeds() (float64, float64) {
	return r.dx, r.dy
}

func (r *PdfReader) SetBleeds(bx, by float64) {
	r.dx, r.dy = bx, by
}

func (r *PdfReader) GetPage(num int) (*model.PdfPage, error) {
	var err error
	r.pg, err = r.PdfReader.GetPage(num)
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
	return creator.NewBlockFromPage(r.pg)
}
