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
}

func NewReader(f io.ReadSeeker, dx, dy float64) (*PdfReader, error) {
	r, err := model.NewPdfReader(f)
	if err != nil {
		return nil, err
	}
	return &PdfReader{r, nil, dx, dy}, nil
}

func (r *PdfReader) AdjustMediaBox() error {
	if r.pg == nil {
		return errors.New("No page. Need to call GetPage(num) before")
	}
	// adjust mediabox expanding from trim/crop box with bleed amounts but no more than actual mediabox
	// TrimBox is the final page
	tbox, err := r.pg.GetBox("TrimBox")
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
	mbox.Llx = tbox.Llx - r.dx
	mbox.Lly = tbox.Lly - r.dy
	mbox.Urx = tbox.Urx + r.dx
	mbox.Ury = tbox.Ury + r.dy

	mediabox, err := r.pg.GetMediaBox()
	// what?? we have at least a cropbox or a trimbox but not a mediabox???
	if err != nil {
		return err
	}

	// do not exceed unadjusted real mediabox
	// mediabox width smaller than adjusted mbox width
	if mediabox.Urx-mediabox.Llx < mbox.Urx-mbox.Llx ||
		mediabox.Ury-mediabox.Lly < mbox.Ury-mbox.Lly {
		// use mediabox
		mbox = mediabox
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
	err = r.AdjustMediaBox()
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