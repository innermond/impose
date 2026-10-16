package impose

import (
	"math"
	"slices"

	"github.com/unidoc/unipdf/v3/creator"
)

func (bb *Boxes) Rotator(turn float64) func(int) {
	bb.Small.Angle += turn
	//var isBack, isFace, stilBack, stilFace bool
	return func(i int) {
		if turn != 0.0 /*&& i >= bb.Col*bb.Row*/ {
			// is on a duplex page ? all 2th page is duplex
			/*zero := int(math.Ceil(float64(i+1)/float64(bb.Col*bb.Row))) % 2

			isBack = zero == 0
			isFace = !isBack

			if !stilBack && isBack {
				stilBack = true
				stilFace = false
				bb.Small.Angle -= turn
			} else if !stilFace && isFace {
				stilBack = false
				stilFace = true
				bb.Small.Angle += turn
			}*/
		}
	}
}

func (bb *Boxes) Adjuster(turn float64, creepx []int, multiplier float64) func(int) {
	var isBack, isFace, stilBack, stilFace bool
	return func(i int) {
		if turn != 0.0 /*&& i >= bb.Col*bb.Row*/ {
			// is on a duplex page ? all 2th page is duplex
			zero := int(math.Ceil(float64(i+1)/float64(bb.Col*bb.Row))) % 2

			isBack = zero == 0
			isFace = !isBack

			if !stilBack && isBack {
				stilBack = true
				stilFace = false
				bb.Small.Angle -= turn
			} else if !stilFace && isFace {
				stilBack = false
				stilFace = true
				bb.Small.Angle += turn
			}
		}
		bb.DeltaPos = float64(creepx[i]) / multiplier
	}
}

// proxy func
func (bb *Boxes) Cycle(pxp []int, progress func(int), showcropmarkPages []int) (int, error) {
	return bb.CycleAdjusted(pxp, progress, nil, showcropmarkPages)
}

// CycleAdjusted lays down pages sheet after sheet and gives back the number of the last sheet, from 0.
// progress, if set, is called with the count of elements processed.
func (bb *Boxes) CycleAdjusted(pxp []int, progress func(int), adjuster func(i int), showcropmarkPages []int) (int, error) {
	var (
		err        error
		maxOnSheet = bb.Col * bb.Row
		xpos, ypos = bb.Big.Left, bb.Big.Top
		w, h       = bb.Small.Width, bb.Small.Height
		i          int
		nextSheet  bool
	)
	// start imposition
	sheetNum := 0
	bb.NewSheet()
	var (
		rowbk *creator.Block
	)
grid:
	for {
		for y := 0; y < bb.Row; y++ {
			if !bb.Dry {
				rowbk = creator.NewBlock(
					bb.Big.Width,
					bb.Small.Height,
				)
			}
			xpos = bb.Big.Left
			for x := 0; x < bb.Col; x++ {
				if err = bb.ctxErr(); err != nil {
					return sheetNum, err
				}
				if i >= bb.Num {
					bb.putRow(rowbk)
					if showcropmarkPages == nil || slices.Contains(showcropmarkPages, sheetNum) {
						bb.DrawCropmark()
					}
					break grid
				}
				// check the need for a new page
				if i >= maxOnSheet {
					nextSheet = (maxOnSheet+i)%maxOnSheet == 0
				}
				if nextSheet {
					// put cropmarks on sheet
					if showcropmarkPages == nil || slices.Contains(showcropmarkPages, sheetNum) {
						bb.DrawCropmark()
					}
					// initialize position
					xpos = bb.Big.Left
					ypos = bb.Big.Top
					bb.NewSheet()
					sheetNum++
					nextSheet = false
				}
				if pxp[i] > 0 {
					if adjuster != nil {
						adjuster(i)
					}
					if bb.OnPlace != nil {
						for _, p := range bb.placements(pxp[i], xpos, ypos) {
							bb.OnPlace(sheetNum, p)
						}
					}
					if !bb.Dry {
						err = bb.BlockDrawPage(rowbk, pxp[i], xpos, ypos)
						if err != nil {
							return sheetNum, err
						}
					}
				}
				// count elements processed
				i++
				if progress != nil {
					progress(i)
				}
				xpos += float64(w)
			}
			bb.putRow(rowbk)
			ypos += float64(h)
			xpos = bb.Big.Left
		}
	}
	bb.Sheets = sheetNum + 1
	return sheetNum, nil
}

// placements gives where the page of the cell, with its clones, is on the sheet
func (bb *Boxes) placements(num int, xpos, ypos float64) []Placement {
	w, h := bb.Small.Width, bb.Small.Height
	angle := bb.Small.Angle
	// creeping moves the page along the axis of its rotation
	dx, dy := bb.DeltaPos, 0.0
	switch angle {
	case -90, 270, 90, -270:
		dx, dy = 0, -bb.DeltaPos
	}
	out := make([]Placement, 0, bb.CloneX*bb.CloneY)
	for j := 0; j < bb.CloneY; j++ {
		for i := 0; i < bb.CloneX; i++ {
			xk := float64(i)*float64(bb.Col)*w + bb.ClonePadX*float64(i)
			yk := float64(j)*float64(bb.Row)*h + bb.ClonePadY*float64(j)
			out = append(out, Placement{Page: num, X: xpos + xk + dx, Y: ypos + yk + dy, W: w, H: h, Angle: angle})
		}
	}
	return out
}

func (bb *Boxes) putRow(rowbk *creator.Block) {
	if bb.Dry {
		return
	}
	for j := 0; j < bb.CloneY; j++ {
		for i := 0; i < bb.CloneX; i++ {
			var xk = float64(i) * float64(bb.Col) * bb.Small.Width
			var yk = float64(j) * float64(bb.Row) * bb.Small.Height
			padx, pady := 0.0, 0.0
			if i > 0 {
				padx = float64(bb.ClonePadX) * float64(i)
			}
			if j > 0 {
				pady = float64(bb.ClonePadY) * float64(j)
			}
			xk += padx
			yk += pady
			rowbk.SetPos(xk, yk)
			bb.Creator.Draw(rowbk)
		}
	}
}

func (bb *Boxes) BlockDrawPage(block *creator.Block, num int, xpos, ypos float64) error {
	var (
		err error
		//w, h  = bb.Small.Width, bb.Small.Height
		angle = bb.Small.Angle
		dt    = bb.DeltaPos

		bk *creator.Block
	)

	bk, err = bb.Reader.BlockFromPage(num)
	if err != nil {
		return err
	}

	// lay down imported page
	xposx, yposy := xpos, ypos

	if angle != 0 {
		bk.SetAngle(angle - bk.Angle())
	}
	dd := bk.Width() - bk.Height()
	// a page turned by a half turn keeps its width and height, only quarter turns need centering
	if math.Mod(bk.Angle(), 180) == 0 {
		dd = 0
	}
	if bk.Angle() != 0 {
		xposx -= 0.5*dd + dt
		yposy += 0.5*dd - dt
	}
	//angle += bk.Angle()
	//log.Println(dt, dd)
	// bk is top left corner oriented by framework choice
	// Clip is bottom right oriented by pdf specification
	// angle is counter clock wise, so -90 is clock wise
	// do the math!!!
	switch angle {
	case 0.0:
		bk.Clip(-1*dt, 0, bk.Width(), bk.Height(), bb.Outline)
	case -90, 270:
		bk.Clip(0, -dt, bk.Width(), bk.Height(), bb.Outline)
	case 90, -270:
		bk.Clip(0, dt, bk.Width(), bk.Height(), bb.Outline)
	case 180, -180:
		bk.Clip(dt, 0, bk.Width(), bk.Height(), bb.Outline)
	default:
		bk.Clip(dt, 0, bk.Width(), bk.Height(), bb.Outline)
	}
	xposx += dt
	// layout page
	bk.SetPos(xposx, yposy)
	_ = block.Draw(bk)

	return err
}

func (bb *Boxes) DrawPage(num int, xpos, ypos float64) error {
	var (
		err   error
		w, h  = bb.Small.Width, bb.Small.Height
		angle = bb.Small.Angle
		dt    = bb.DeltaPos

		bk *creator.Block
	)

	bk, err = bb.Reader.BlockFromPage(num)
	if err != nil {
		return err
	}

	// lay down imported page
	xposx, yposy := xpos, ypos
	bk.SetAngle(angle)
	// bk is top left corner oriented by framework choice
	// Clip is bottom right oriented by pdf specification
	// angle is counter clock wise, so -90 is clock wise
	// do the math!!!
	switch angle {
	case 0.0:
		xposx += dt
		bk.Clip(-1*dt, 0, bk.Width(), bk.Height(), bb.Outline)
	case -90, 270:
		xposx += w
		xposx += dt
		bk.Clip(0, -dt, bk.Width(), bk.Height(), bb.Outline)
	case 90, -270:
		yposy += h
		xposx += dt
		bk.Clip(0, dt, bk.Width(), bk.Height(), bb.Outline)
	case 180, -180:
		xposx += w
		yposy += h
		xposx += dt
		bk.Clip(dt, 0, bk.Width(), bk.Height(), bb.Outline)
	}
	// layout page
	bk.SetPos(xposx, yposy)
	_ = bb.Creator.Draw(bk)

	return err
}
