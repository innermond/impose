package main

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

func (bb *Box) AvailableWidth() float64 {
	return bb.Width - (bb.Left + bb.Right)
}

func (bb *Box) AvailableHeight() float64 {
	return bb.Height - (bb.Top + bb.Bottom)
}

func (bb *Box) Switch() {
	bb.Width, bb.Height = bb.Height, bb.Width
}

type BigBox struct {
	*Box
}
type SmallBox struct {
	*Box
}

type Boxes struct {
	Big      *BigBox
	Small    *SmallBox
	Col, Row int
	Num      int
}

func (bb *Boxes) AdjustMarginCenteringAlongWidth() {
	wpages := 0.0
	available := bb.Big.AvailableWidth()
	i := 0
	for wpages < available {
		wpages += bb.Small.Width
		// sensible to grid
		if i == bb.Col {
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
		if i == bb.Row {
			break
		}
		i++
	}
	hpages -= bb.Small.Height
	bb.Big.Top = (bb.Big.Height - hpages) * 0.5
	bb.Big.Bottom = bb.Big.Top
}

func (bb *Boxes) SwitchGrid() {
	bb.Col, bb.Row = bb.Row, bb.Col
	bb.Small.Switch()
}

// threshold to compare aproximating floating values
const epsilon = 1e-9

func (bb *Boxes) EnoughWidth() bool {
	dif := bb.Big.AvailableWidth() - float64(bb.Col)*bb.Small.Width
	return dif > 0 || math.Abs(dif) < epsilon
}

func (bb *Boxes) EnoughHeight() bool {
	dif := bb.Big.AvailableHeight() - float64(bb.Row)*bb.Small.Height
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

func (bb *Boxes) Impose(flow string, np int, angle float64, pxp []int, pdfReader *model.PdfReader, c *creator.Creator, cros2b *creator.Block, booklet bool, creep float64, outline bool, bleedx, bleedy float64) {
	// start imposition
	var (
		sheet, pg                  *model.PdfPage
		bk                         *creator.Block
		i, j                       int
		dt, step                   float64
		nextSheet                  bool
		xpos, ypos                 = left, top
		num                        int
		creepCount, nextSheetCount int
	)
	// proxy variables
	var (
		col, row   = bb.Col, bb.Row
		maxOnSheet = col * row
		left, top  = bb.Big.Left, bb.Big.Top
		w, h       = bb.Small.Width, bb.Small.Height
	)
	if booklet {
		step = creep / float64(np/4)
	}

	sheet = model.NewPdfPage()
	sheet.MediaBox = &model.PdfRectangle{0, 0, c.Width(), c.Height()}
	c.AddPage(sheet)
	nextSheetCount = 1

	// parse flow
	ff, err := bb.ParseFlow(flow)
	if err != nil {
		log.Fatal(err)
	}

	iload := 10
grid:
	for {
		for y := 0; y < row; y++ {
			for x := 0; x < len(ff); x++ {
				if i >= np {
					break grid
				}
				// take flow order into account
				num = ff[x] + j*col
				// num resulted larger than number of pages
				// place an empty space with the right wide
				if num > np {
					xpos += float64(w)
					continue
				}
				// get the page number from pages slice
				num = pxp[num-1]

				// check the need for a new page
				if i >= maxOnSheet {
					nextSheet = (maxOnSheet+i)%maxOnSheet == 0
				}
				if nextSheet {
					// put cropmarks on sheet
					c.Draw(cros2b)
					// initialize position
					ypos = top
					//c.NewPage()
					sheet = model.NewPdfPage()
					sheet.MediaBox = &model.PdfRectangle{0, 0, c.Width(), c.Height()}
					c.AddPage(sheet)
				}

				// count pages processed
				i++

				// what just a certain page?
				if samepage > 0 {
					num = samepage
				}

				// import page
				pg, err = pdfReader.GetPage(num)
				if err != nil {
					log.Fatal(err)
				}
				adjustMediaBox(pg, bleedx, bleedy)
				bk, err = creator.NewBlockFromPage(pg)
				if err != nil {
					log.Fatal(err)
				}

				// lay down imported page
				xposx, yposy := xpos, ypos
				if angle != 0.0 {
					bk.SetAngle(angle)
					if angle == -90.0 || angle == 270 {
						xposx += w
					}
					if angle == 90.0 || angle == -270 {
						yposy += h
					}
					if angle == -180 || angle == 180 {
						xposx += w
						yposy += h
					}
				}
				if !booklet {
					bk.Clip(0, 0, bk.Width(), bk.Height(), outline)
				} else {
					// time to check if block is front or verso
					if i > 2 && (i-1)%2 == 0 {
						if nextSheet {
							nextSheetCount++
							if nextSheetCount%2 == 0 {
								dt -= float64(creepCount) * step
							} else {
								dt += step
							}
							// reset counter
							creepCount = 0
						} else {
							creepCount++
							dt += step
						}
					}
					/*if i > 4 && (i-1)%4 == 0 {
						dt += step
					}*/
					direction := 0.0
					if i%2 == 0 {
						direction = -1.0
					} else {
						direction = 1.0
					}

					switch angle {
					case 0.0:
						xposx += direction * dt
						bk.Clip(-1*direction*dt, 0, bk.Width(), bk.Height(), outline)
					case -90, 270:
						yposy += direction * dt
						bk.Clip(-direction*dt, 0, bk.Width(), bk.Height(), outline)
					case 90, -270:
						yposy += direction * dt
						bk.Clip(direction*dt, 0, bk.Width(), bk.Height(), outline)
					case 180, -180:
						xposx += direction * dt
						bk.Clip(direction*dt, 0, bk.Width(), bk.Height(), outline)
					}
				}
				// layout page
				bk.SetPos(xposx, yposy)
				_ = c.Draw(bk)

				xpos += float64(w)
				if nextSheet {
					nextSheet = false
				}
			}
			ypos += float64(h)
			xpos = left
			j++
		}
		// the poor man's vizual indicator that something is happening
		fmt.Print(".")
		// clear all dots
		if iload%10 == 0 {
			fmt.Print("\r")
		}
		iload++
	}
	// put cropmarks for the last sheet
	c.Draw(cros2b)
	fmt.Print("\n")
}
