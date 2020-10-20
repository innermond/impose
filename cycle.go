package impose

import (
	"log"
	"math"

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
func (bb *Boxes) Cycle(pxp []int, c chan int) {
	bb.CycleAdjusted(pxp, c, nil)
}

func (bb *Boxes) CycleAdjusted(pxp []int, c chan int, adjuster func(i int)) {
	var (
		err        error
		maxOnSheet = bb.Col * bb.Row
		xpos, ypos = bb.Big.Left, bb.Big.Top
		w, h       = bb.Small.Width, bb.Small.Height
		isNextSheet  bool
	)
	// start imposition
	bb.NewSheet()
	var (
		gridbk *creator.Block
		i          int // page counter
	)

  var gridCounter uint = 0
grid:
	for {
    // new empty row
    gridbk = creator.NewBlock(
      bb.Big.Width,
      bb.Big.Height,
    )

		for y := 0; y < bb.Row; y++ {
			xpos = bb.Big.Left
			for x := 0; x < bb.Col; x++ {
				// check the need for a new page
				if i >= maxOnSheet {
					isNextSheet = (maxOnSheet+i)%maxOnSheet == 0
				}
				if isNextSheet {
					// initialize position
          xpos = bb.Big.Left
					ypos = bb.Big.Top
					bb.NewSheet()
					isNextSheet = false
				}
        // skip blank pages
				if pxp[i] > 0 {
					if adjuster != nil {
						adjuster(i)
					}
					err = bb.BlockDrawPage(gridbk, pxp[i], xpos, ypos)
					if err != nil {
						log.Fatal(err)
					}
				}
				// count elements processed
				i++
				// signal page drawing
				c <- i
				xpos += w

        // overflow num pages?
				if i >= bb.Num {
          gridCounter++
          bb.putRow(gridbk)
          if bb.cropPage == 0 || gridCounter == bb.cropPage {
            bb.DrawCropmark()
          }
					break grid
				}
			}
      bb.putRow(gridbk)
			ypos += h
			xpos = bb.Big.Left
		}
    gridCounter++
    if bb.cropPage == 0 || gridCounter == bb.cropPage {
      bb.DrawCropmark()
    }
	}
	close(c)
}

func (bb *Boxes) putRow(gridbk *creator.Block) {
  for j := 0; j < bb.CloneY; j++ {
    for i := 0; i < bb.CloneX; i++ {
      var xk = float64(i)*float64(bb.Col)*bb.Small.Width 
      var yk = float64(j)*float64(bb.Row)*bb.Small.Height
      gridbk.SetPos(xk, yk)
      bb.Creator.Draw(gridbk)
    }
  }
}

func (bb *Boxes) BlockDrawPage(block *creator.Block, num int, xpos, ypos float64) error {
	var (
		err   error
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
		//xposx += w
		xposx += dt
		bk.Clip(0, -dt, bk.Width(), bk.Height(), bb.Outline)
	case 90, -270:
		//yposy += h
		xposx += dt
		bk.Clip(0, dt, bk.Width(), bk.Height(), bb.Outline)
	case 180, -180:
		//xposx += w
		//yposy += h
		xposx += dt
		bk.Clip(dt, 0, bk.Width(), bk.Height(), bb.Outline)
  default:
		xposx += dt
		bk.Clip(dt, 0, bk.Width(), bk.Height(), bb.Outline)
	}
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
