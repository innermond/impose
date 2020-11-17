package main

import (
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/innermond/impose"
	"github.com/innermond/pange"
	"github.com/unidoc/unipdf/v3/creator"
	"github.com/unidoc/unipdf/v3/model"
)

func main() {
	err := param()
	deal(err)

	// profiling
	cpu()

	// catch termination
	killChan := make(chan os.Signal, 1)
	signal.Notify(killChan, os.Interrupt)
	go func() {
		<-killChan
		if verbosity > 1 {
			log.Println("user interruption")
		}
		os.Exit(1)
	}()

	start := time.Now()
	defer func() {
		elapsed := time.Since(start)
		if verbosity > 0 {
			log.Printf("time taken %v\n", elapsed)
		}
	}()

	// Read the input pdf file.
	f, err := os.Open(fn)
	fatal(err)
	defer func() {
		f.Close()
		if verbosity > 2 {
			log.Printf("close file %q\n", fn)
		}
	}()

	// read first pdf page
	if verbosity > 1 {
		log.Printf("read pdf %q", fn)
	}
	pdf, err := impose.NewReader(f, bleedx, bleedy)
	fatal(err)

	// establish pages number
	np, err := pdf.GetNumPages()
	fatal(err)

	// from 1 to last
	if pages == "" {
		pages = fmt.Sprintf("1-%d", np)
	}
	// get pages for imposition
	sel := pange.Selection(pages)
	// all pages as a slice of ints
	pags, err := sel.Full()
	fatal(err)
	np = len(pags)

	// grid
	var (
		col int = 1
		row int = 1
	)

	col, row, err = parsex(grid)
	fatal("grid: ", err)

	// bookletMode need  certain grid and page order
	if bookletMode {
		if grid == "" {
			grid = "2x1"
			col, row = 2, 1
		}
	}

	if verbosity > 0 {
		log.Println("parsed parameters")
	}

	// assume all pages has the same dimensions as first one
	page, err := pdf.GetPage(1)
	fatal(err)

	bbox, err := page.GetMediaBox()
	fatal(err)

	if len(mediabox) != 0 {
		// force mediabox
		mbox := &model.PdfRectangle{}
		pp := mediabox
		switch len(pp) {
		case 1:
			mbox.Llx = pp[0] * creator.PPMM
			mbox.Lly = bbox.Lly
			mbox.Urx = bbox.Urx
			mbox.Ury = bbox.Ury
		case 2:
			mbox.Llx = pp[0] * creator.PPMM
			mbox.Lly = pp[1] * creator.PPMM
			mbox.Urx = bbox.Urx
			mbox.Ury = bbox.Ury
		case 3:
			mbox.Llx = pp[0] * creator.PPMM
			mbox.Lly = pp[1] * creator.PPMM
			mbox.Urx = pp[2] * creator.PPMM
			mbox.Ury = bbox.Ury
		case 4:
			mbox.Llx = pp[0] * creator.PPMM
			mbox.Lly = pp[1] * creator.PPMM
			mbox.Urx = pp[2] * creator.PPMM
			mbox.Ury = pp[3] * creator.PPMM
		}
		pdf.ForceMediaBox(mbox)
		err := pdf.AdjustMediaBox()
		fatal(err)
		bbox = mbox
	}

	if verbosity > 0 {
		log.Printf("mediabox: %v\n", bbox)
	}

	w := bbox.Urx - bbox.Llx
	h := bbox.Ury - bbox.Lly

	if verbosity > 0 {
		log.Printf("small box width: %v; small box height height: %v\n", w/creator.PPMM, h/creator.PPMM)
	}

	bx, by := pdf.GetNaturalBleeds()
	bx = math.Round(bx*100) / 100
	by = math.Round(by*100) / 100
	bx0 := math.Round(bleedx*100) / 100
	by0 := math.Round(bleedy*100) / 100
	if bx0 != bx || by0 != by {
		bxmm := math.Round(bx/creator.PPMM*100) / 100
		bymm := math.Round(by/creator.PPMM*100) / 100
		log.Printf("sugested bleed x: %v sugested bleed y: %v\n", bxmm, bymm)
	}
	// cropmarks adds extra to dimensions
	extw := offx + markw
	exth := offy + markh

	beSwitched := angle == 90.0 || angle == -90 || angle == 270 || angle == -270
	clonex, cloney := 1, 1
	clonex, cloney, err = parsex(clone)
	fatal("clone: ", err)
	if verbosity > 0 {
		log.Printf("clonex %v, cloney %v", clonex, cloney)
	}

	if autopage {
		width = left + float64(clonex)*float64(col)*w + right + 2*extw + 2*autopadding
		height = top + float64(cloney)*float64(row)*h + bottom + 2*exth + 2*autopadding
		if beSwitched {
			width = left + float64(clonex)*float64(col)*h + right + 2*extw + 2*autopadding
			height = top + float64(cloney)*float64(row)*w + bottom + 2*exth + 2*autopadding
		}
	}

	if verbosity > 0 {
		log.Printf("page width %v, page height %v", width/creator.PPMM, height/creator.PPMM)
	}

	// create a sheet page
	c := creator.New()
	c.SetPageSize(creator.PageSize{width, height})

	bigbox := &impose.BigBox{
		Box: &impose.Box{
			Width:  width,
			Height: height,
			Top:    top,
			Right:  right,
			Bottom: bottom,
			Left:   left,
		},
	}
	smallbox := &impose.SmallBox{Box: &impose.Box{Width: w, Height: h}, Angle: angle}
	bb := &impose.Boxes{
		Big:      bigbox,
		Small:    smallbox,
		Col:      col,
		Row:      row,
		CloneX:   clonex,
		CloneY:   cloney,
		Num:      np,
		Creator:  c,
		Reader:   pdf,
		Cropmark: nil,
		Outline:  outline,
		DeltaPos: 0.0,
	}

	angled := false
	if beSwitched {
		bb.Small.Switch()
		h, w = bb.Small.Height, bb.Small.Width
		angled = true
	}
	// centering by changing margins
	if centerx {
		bb.AdjustMarginCenteringAlongWidth()
	}
	if centery {
		bb.AdjustMarginCenteringAlongHeight()
	}
	// guess the grid
	if grid == "" {
		col, row := bb.GuessGrid()
		log.Fatalf("sugested grid %dx%d", col, row)
	}

	// start to lay down pages
	top, right, bottom, left = bb.Big.Top, bb.Big.Right, bb.Big.Bottom, bb.Big.Left
	col, row = bb.Col, bb.Row

	// check if media is enough
	if !bb.EnoughWidth() {
		log.Fatalf("%d columns do not fit", bb.Col)
	}
	if !bb.EnoughHeight() {
		log.Fatalf("%d rows do not fit", bb.Row)
	}

	if verbosity > 0 {
		log.Println("prepared boxes")
	}

	if cropmark >= 0 {
		bb.CreateCropmark(
			markw, markh,
			extw, exth,
			bleedx, bleedy,
			bookletMode,
			angled,
			cropmark,
		)
	}

	counter := make(chan int)
	if repeat {
		counter = bb.Repeat(pags, turn)
	} else if bookletMode {
		counter = bb.Booklet(
			pags,
			creep,
			flip, reverse, turn,
		)
	} else {
		flowArr := []int{}
		if len(flow) < 1 {
			flowArr = []int{0}
		} else {
			flowStr := strings.Split(flow, ",")
			for _, val := range flowStr {
				inx, err := strconv.Atoi(val)
				if err != nil {
					log.Fatal(err)
				}
				flowArr = append(flowArr, inx)
			}
		}
		counter = bb.Impose(
			pags,
			flowArr,
			weld,
			flip, reverse, turn, duplex,
		)
	}

	for {
		num, more := <-counter
		if more {
			if verbosity > 1 {
				log.Println("Page ", num)
			}
		} else {
			break
		}
	}

	err = c.Write(os.Stdout)
	fatal(err)

	// profilng
	mem()
}
