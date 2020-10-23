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
  // natural bleeds
  bx, by float64

  mbox *model.PdfRectangle 
}

func NewReader(f io.ReadSeeker, dx, dy float64) (*PdfReader, error) {
	r, err := model.NewPdfReader(f)
	if err != nil {
		return nil, err
	}
	return &PdfReader{r, nil, dx, dy, 0.0, 0.0, nil}, nil
}

func (r *PdfReader) ForceMediaBox(forcedbox *model.PdfRectangle) {
  r.mbox = forcedbox
}

func (r *PdfReader) AdjustMediaBox() error {
	if r.pg == nil {
		return errors.New("No page. Need to call GetPage(num) before")
	}

  //TODO force mediabox from trim + bleed
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
	// MediaBox = TrimBox + 2*bleed
	mbox := &model.PdfRectangle{}
	// expand with bleedx and bleedy
	mbox.Llx = tbox.Llx - r.dx
	mbox.Lly = tbox.Lly - r.dy
	mbox.Urx = tbox.Urx + r.dx
	mbox.Ury = tbox.Ury + r.dy
  
// use forced mediabox
  if r.mbox != nil {
    r.pg.MediaBox = r.mbox
  }
	mediabox, err := r.pg.GetMediaBox()
	// what?? we have at least a cropbox or a trimbox but not a mediabox???
	if err != nil {
		return err
	}

	// if mediabox is smaller than trim + bleed computed enlarge
	// mediabox width smaller than adjusted mbox width
	if mediabox.Urx-mediabox.Llx < mbox.Urx-mbox.Llx ||
		mediabox.Ury-mediabox.Lly < mbox.Ury-mbox.Lly {
		// use mediabox
		mediabox = mbox
	}
	// adjust
	r.pg.MediaBox = mediabox
  // bleed
  r.bx, r.by = tbox.Llx - mediabox.Llx, tbox.Lly - mediabox.Lly
	return nil
}

func (r *PdfReader) GetNaturalBleeds() (float64, float64) {
  return r.bx, r.by
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
