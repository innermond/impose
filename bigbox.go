package main

import (
	"fmt"
	"log"
	"strings"
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
}

func (bb *Boxes) AdjustMarginCenteringAlongWidth() {
	wpages := bb.Small.Width
	available := bb.Big.AvailableWidth()
	i := 0
	for wpages < available {
		wpages += bb.Small.Width
		// sensible to grid
		i++
		if i == bb.Col {
			break
		}
	}
	wpages -= bb.Small.Width
	bb.Big.Left = (bb.Big.Width - wpages) * 0.5
	bb.Big.Right = bb.Big.Left
}

func (bb *Boxes) AdjustMarginCenteringAlongHeight() {
	hpages := bb.Small.Height
	available := bb.Big.AvailableHeight()
	i := 0
	for hpages < available {
		hpages += bb.Small.Height
		// sensible to grid
		i++
		if i == bb.Row {
			break
		}
	}
	hpages -= bb.Small.Height
	bb.Big.Top = (bb.Big.Height - hpages) * 0.5
	bb.Big.Bottom = bb.Big.Top
}

func (bb *Boxes) SwitchGrid() {
	bb.Col, bb.Row = bb.Row, bb.Col
	bb.Small.Switch()
}

func (bb *Boxes) EnoughWidth() bool {
	return bb.Big.AvailableWidth() > float64(bb.Col)*bb.Small.Width
}

func (bb *Boxes) EnoughHeight() bool {
	return bb.Big.AvailableHeight() > float64(bb.Row)*bb.Small.Height
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
		endx = floor63(xpos + float64(bb.Small.Width))
		peakx = floor63(bb.Big.AvailableWidth())
		if endx > peakx {
			stopCountingCol = true
			xpos = bb.Big.Left
			ypos += float64(bb.Small.Height)
			endy = floor63(ypos + float64(bb.Small.Height))
			peaky = floor63(bb.Big.AvailableHeight())
			row++
			if endy > peaky {
				break
			}
		}
		xpos += float64(bb.Small.Width)
	}
	return
}
