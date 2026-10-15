package impose

import (
	"context"
	"fmt"
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
	ClonePadX, ClonePadY     float64
	Num                      int

	Creator *creator.Creator
	Reader  *PdfReader

	Cropmark *creator.Block
	Outline  bool

	DeltaPos float64 // used to adjust / modify positions

	// Ctx stops the imposition when done, nil never stops it.
	Ctx context.Context
	// Dry only computes positions: nothing is read from the pdf or drawn.
	Dry bool
	// Sheets is the number of sheets laid down, known when the cycle ends.
	Sheets int
	// OnPlace, if set, is called for every page placed on a sheet with the sheet number, from 0.
	OnPlace func(sheet int, p Placement)
}

// ctxErr tells if the imposition has been stopped.
func (bb *Boxes) ctxErr() error {
	if bb.Ctx == nil {
		return nil
	}
	return bb.Ctx.Err()
}

func (bb *Boxes) NewSheet() {
	if bb.Dry {
		return
	}
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
	bb.Big.Left = -0.5*(bb.ClonePadX*float64(bb.CloneX-1)) + (bb.Big.Width-wpages)*0.5
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
	bb.Big.Top = -0.5*(bb.ClonePadY*float64(bb.CloneY-1)) + (bb.Big.Height-hpages)*0.5
	bb.Big.Bottom = bb.Big.Top
}

// threshold to compare aproximating floating values
const epsilon = 1e-9

func (bb *Boxes) EnoughWidth() bool {
	consumedWidth := float64(bb.Col*bb.CloneX) * bb.Small.Width
	consumedCloned := float64((bb.CloneX - 1)) * bb.ClonePadX
	consumed := consumedWidth + consumedCloned
	dif := bb.Big.AvailableWidth() - consumed
	return dif > 0.0 || math.Abs(dif) < epsilon
}

func (bb *Boxes) EnoughHeight() bool {
	consumedHeight := float64(bb.Row*bb.CloneY) * bb.Small.Height
	consumedCloned := float64((bb.CloneY - 1)) * bb.ClonePadY
	consumed := consumedHeight + consumedCloned
	dif := bb.Big.AvailableHeight() - consumed
	return dif > 0.0 || math.Abs(dif) < epsilon
}

func (bb *Boxes) ParseFlow(flow string) ([]int, error) {
	var (
		ff  []int
		err error
	)

	if flow != "" {
		ff, err = getFlowAsInts(strings.Split(flow, ","), bb.Col)
		if err != nil {
			return nil, err
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

// GuessGrid counts how many pages fit along the width and the height of the sheet,
// taking in account margins, clones and the pads between clones
func (bb *Boxes) GuessGrid() (col, row int) {
	fit := func(available, size, pad float64, clones int) int {
		clones = max(clones, 1)
		if size <= 0 {
			return 0
		}
		n := math.Floor((available - float64(clones-1)*pad + epsilon) / (float64(clones) * size))
		return max(int(n), 0)
	}
	col = fit(bb.Big.AvailableWidth(), bb.Small.Width, bb.ClonePadX, bb.CloneX)
	row = fit(bb.Big.AvailableHeight(), bb.Small.Height, bb.ClonePadY, bb.CloneY)
	return
}

func (bb *Boxes) CreateCropmark(
	markw, markh,
	extw, exth,
	bleedx, bleedy float64,
	bookletMode bool,
	angled bool,
) {
	bb.Cropmark = bb.newCropMarkBlock(markw, markh, extw, exth, bleedx, bleedy).Create(bookletMode, angled)
}

func (bb *Boxes) newCropMarkBlock(markw, markh, extw, exth, bleedx, bleedy float64) *CropMarkBlock {
	return &CropMarkBlock{bb.Small.Width, bb.Small.Height,
		bleedx, bleedy,
		bb.Col * bb.CloneX, bb.Row * bb.CloneY,
		extw, exth,
		markw, markh,
		bb,
	}
}

func (bb *Boxes) DrawCropmark() {
	if bb.Cropmark != nil && !bb.Dry {
		bb.Creator.Draw(bb.Cropmark)
	}

}
