package impose

import (
	"fmt"
	"log"
	"math"
	"strings"

	"github.com/unidoc/unipdf/v3/creator"
	"github.com/unidoc/unipdf/v3/model"
)

type Box struct {
	Width, Height            float64
	Top, Right, Bottom, Left float64
}

func (b *Box) AvailableWidth() float64 {
	return b.Width - (b.Left + b.Right)
}

func (b *Box) AvailableHeight() float64 {
	return b.Height - (b.Top + b.Bottom)
}

func (b *Box) Switch() {
	b.Width, b.Height = b.Height, b.Width
}

type BigBox struct {
	*Box
}
type SmallBox struct {
	*Box
	Angle float64
}

type Boxes struct {
	Big                      *BigBox
	Small                    *SmallBox
	Col, Row, CloneX, CloneY int
	Num                      int

	BleedX, BleedY float64

	Creator *creator.Creator
	Reader  *PdfReader

	Cropmark *creator.Block
	cropPage uint
	Outline  bool

	DeltaPos float64 // used to adjust / modify positions
}

func (bb *Boxes) NewSheet() {
	sheet := model.NewPdfPage()
	sheet.MediaBox = &model.PdfRectangle{Llx: 0, Lly: 0, Urx: bb.Big.Width, Ury: bb.Big.Height}
	bb.Creator.AddPage(sheet)
}

func (bb *Boxes) AdjustMarginCenteringAlongWidth() {
	wpages := 0.0
	available := bb.Big.AvailableWidth()
	i := 0
	for wpages < available {
		wpages += bb.Small.Width
		// sensible to grid
		if i == bb.Col*bb.CloneX {
			break
		}
		i++
	}
	wpages -= bb.Small.Width
	bb.Big.Left = (bb.Big.Width - wpages) * 0.5
	bb.Big.Right = bb.Big.Left
}

func (bb *Boxes) AdjustMarginCenteringAlongHeight() {
	hpages := 0.0
	available := bb.Big.AvailableHeight()
	i := 0
	for hpages < available {
		hpages += bb.Small.Height
		// sensible to grid
		if i == bb.Row*bb.CloneY {
			break
		}
		i++
	}
	hpages -= bb.Small.Height
	bb.Big.Top = (bb.Big.Height - hpages) * 0.5
	bb.Big.Bottom = bb.Big.Top
}

// threshold to compare aproximating floating values
const epsilon = 1e-9

func (bb *Boxes) EnoughWidth() bool {
	dif := bb.Big.AvailableWidth() - float64(bb.Col*bb.CloneX)*bb.Small.Width
	return dif > 0 || math.Abs(dif) < epsilon
}

func (bb *Boxes) EnoughHeight() bool {
	dif := bb.Big.AvailableHeight() - float64(bb.Row*bb.CloneY)*bb.Small.Height
	return dif > 0 || math.Abs(dif) < epsilon
}

func (bb *Boxes) ParseFlow(flow string) ([]int, error) {
	var (
		ff  []int
		err error
	)

	if flow != "" {
		ff, err = getFlowAsInts(strings.Split(flow, ","), bb.Col)
		if err != nil {
			log.Fatal(err)
		}
		if len(ff) != bb.Col {
			return ff, fmt.Errorf("number of flow elements should be equal with %d", bb.Col)
		}
	} else {
		for i := 1; i <= bb.Col; i++ {
			ff = append(ff, i)
		}
	}
	return ff, nil
}

func (bb *Boxes) GuessGrid() (col, row int) {
	var (
		stopCountingCol          bool
		xpos, ypos               = bb.Big.Left, bb.Big.Top
		endx, endy, peakx, peaky float64
	)

	for {
		if !stopCountingCol {
			col++
		}
		endx = xpos + float64(bb.Small.Width)
		peakx = bb.Big.AvailableWidth()
		//	float64 endx > peakx
		if math.Abs(endx-peakx) > epsilon {
			stopCountingCol = true
			xpos = bb.Big.Left
			ypos += float64(bb.Small.Height)
			endy = ypos + float64(bb.Small.Height)
			peaky = bb.Big.AvailableHeight()
			row++
			if math.Abs(endy-peaky) > epsilon {
				break
			}
		}
		xpos += float64(bb.Small.Width)
	}
	return
}

func (bb *Boxes) CreateCropmark(
	markw, markh,
	extw, exth,
	bleedx, bleedy float64,
	bookletMode bool,
	angled bool,
	croppage uint,
) {
	cropbk := &CropMarkBlock{bb.Small.Width, bb.Small.Height,
		bleedx, bleedy,
		bb.Col * bb.CloneX, bb.Row * bb.CloneY,
		extw, exth,
		markw, markh,
		bb,
	}
	bb.Cropmark = cropbk.Create(bookletMode, angled)
	bb.cropPage = croppage
}

func (bb *Boxes) DrawCropmark() {
	if bb.Cropmark != nil {
		bb.Creator.Draw(bb.Cropmark)
	}

}
