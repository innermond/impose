package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/unidoc/unipdf/v3/core"
	"github.com/unidoc/unipdf/v3/creator"
	pdf "github.com/unidoc/unipdf/v3/model"
)

var (
	fn                       string
	fout                     string
	width                    float64
	height                   float64
	unit                     string
	top, left, bottom, right float64
	centerx, centery         bool
	lessPagesNum             int
	grid                     string
	flow                     string
)

func param() error {
	var err error

	flag.StringVar(&fn, "f", "", "source pdf file")
	flag.StringVar(&fout, "o", "", "imposition pdf file")
	flag.Float64Var(&width, "width", 320.0, "imposition sheet width")
	flag.Float64Var(&height, "height", 450.0, "imposition sheet height")
	flag.StringVar(&unit, "unit", "mm", "unit of measurements")
	flag.Float64Var(&top, "top", 5.0, "top margin")
	flag.Float64Var(&left, "left", 5.0, "left margin")
	flag.Float64Var(&bottom, "bottom", 5.0, "bottom margin")
	flag.Float64Var(&right, "right", 5.0, "right margin")
	flag.BoolVar(&centerx, "centerx", false, "center along sheet width")
	flag.BoolVar(&centery, "centery", false, "center along sheet height")
	flag.IntVar(&lessPagesNum, "less", 0, "number of pages to be subject of imposition")
	flag.StringVar(&grid, "grid", "", "imposition layout columns x  rows. ex: 2x3")
	flag.StringVar(&flow, "flow", "", "it works along with grid flag. how pages are ordered on every row, normali they are flowing from 1 to col, but that can be changed, ex: 4,2,1,3")

	flag.Parse()

	if fn == "" {
		return errors.New("pdf file required")
	}

	if fout == "" {
		ext := path.Ext(fn)
		fout = fn[:len(fn)-len(ext)] + ".imposition" + ext
	}

	left *= creator.PPMM
	right *= creator.PPMM
	top *= creator.PPMM
	bottom *= creator.PPMM
	width *= creator.PPMM
	height *= creator.PPMM

	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "centerx":
			centerx = true
		case "centery":
			centery = true
		}
	})

	return err
}

func main() {

	err := param()
	if err != nil {
		log.Fatal(err)
	}

	// Read the input pdf file.
	f, err := os.Open(fn)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	pdfReader, err := pdf.NewPdfReader(f)
	if err != nil {
		log.Fatal(err)
	}

	np, err := pdfReader.GetNumPages()
	if err != nil {
		log.Fatal(err)
	}

	if lessPagesNum > 0 && lessPagesNum < np {
		np = lessPagesNum
	}

	// set default page media
	c := creator.New()
	media := creator.PageSize{width, height}
	c.SetPageSize(media)

	// Margins of page
	c.SetPageMargins(left, right, top, bottom)

	page, err := pdfReader.GetPage(1)
	if err != nil {
		log.Fatal(err)
	}
	//bbox, err := page.GetMediaBox()
	//bbox, err := GetBox(page, "BleedBox")
	bbox, err := page.GetBox("MediaBox")
	if err != nil {
		log.Fatal(err)
	}
	w := bbox.Urx - bbox.Llx
	h := bbox.Ury - bbox.Lly

	c.NewPage()

	var xpos, ypos, endx, endy float64

	if centerx {
		wpages := w
		available := media[0] - (left + right)
		for wpages < available {
			wpages += w
		}
		wpages -= w
		left = (width - wpages) * 0.5
		right = left
	}
	if centery {
		hpages := h
		fmt.Println(top, ":", bottom)
		available := media[1] - (top + bottom)
		lvl := 0
		for hpages < available {
			hpages += h
			lvl++
		}
		hpages -= h
		top = (height - hpages) * 0.5
		bottom = top
		fmt.Printf("lvl: %d hpages: %v h: %v top %v bottom %v height %v media[1] %v", lvl, hpages, h, top, bottom, height, media[1])
	}

	xpos = left
	ypos = top
	fmt.Println(xpos, ":", ypos)
	// natural flow
	if grid == "" {
		for i := 0; i < np; i++ {
			num := i + 1

			endx = xpos + float64(w)
			if endx > media[0]-right {
				xpos = left
				ypos += float64(h)
				endy = ypos + float64(h)
				if endy > media[1]-bottom {
					ypos = top
					xpos = left
					c.NewPage()
				}
			}
			pg, err := pdfReader.GetPage(num)
			if err != nil {
				log.Fatal(err)
			}
			bk, err := creator.NewBlockFromPage(pg)
			if err != nil {
				log.Fatal(err)
			}
			bk.SetPos(xpos, ypos)
			_ = c.Draw(bk)

			xpos += float64(w)
			fmt.Println(num)
		}
	} else {
		// parse grid
		colrow := strings.Split(grid, "x")
		if len(colrow) != 2 {
			log.Fatal(errors.New("grid length invalid"))
		}

		col, err := strconv.Atoi(colrow[0])
		if err != nil {
			log.Fatal(err)
		}
		row, err := strconv.Atoi(colrow[1])
		if err != nil {
			log.Fatal(err)
		}
		// parse flow
		var ff []int
		if flow != "" {
			ff, err = getFlowAsInts(strings.Split(flow, ","), col)
			if err != nil {
				log.Fatal(err)
			}
			if len(ff) != col {
				log.Fatal(fmt.Errorf("flow should be equal with %d", col))
			}
		} else {
			for i := 1; i <= col; i++ {
				ff = append(ff, i)
			}
		}

		var nextPage bool
		var maxOnPage = col * row
		var i, j int
	grid:
		for {
			for y := 0; y < row; y++ {
				for x := 0; x < len(ff); x++ {
					if i >= np {
						break grid
					}
					num := ff[x] + j*col
					if i >= maxOnPage {
						nextPage = (maxOnPage+i)%maxOnPage == 0
					}
					if nextPage {
						ypos = top
						c.NewPage()
						nextPage = false
					}
					i++
					pg, err := pdfReader.GetPage(num)
					if err != nil {
						log.Fatal(err)
					}
					bk, err := creator.NewBlockFromPageBox(pg, bbox)
					if err != nil {
						log.Fatal(err)
					}
					bk.SetPos(xpos, ypos)
					_ = c.Draw(bk)

					xpos += float64(w)
					fmt.Println(num)
				}
				ypos += float64(h)
				xpos = left
				j++
			}
		}
	}
	err = c.WriteToFile(fout)
	if err != nil {
		log.Fatal(err)
	}
}

// GetBox gets the inheritable media box value, either from the page
// or a higher up page/pages struct.
func GetBox(p *pdf.PdfPage, boxname string) (box *pdf.PdfRectangle, err error) {

	switch boxname {
	case "MediaBox":
		box = p.MediaBox
	case "BleedBox":
		box = p.BleedBox
	case "TrimBox":
		box = p.TrimBox
	case "CropBox":
		box = p.CropBox
	case "ArtBox":
		box = p.ArtBox
	default:
		err = fmt.Errorf("invalid box name %s", boxname)
		return
	}

	if box != nil {
		return box, nil
	}

	node := p.Parent
	for node != nil {
		dict, ok := core.GetDict(node)
		if !ok {
			return nil, errors.New("invalid parent objects dictionary")
		}

		if obj := dict.Get(core.PdfObjectName(boxname)); obj != nil {
			arr, ok := obj.(*core.PdfObjectArray)
			if !ok {
				return nil, errors.New("invalid media box")
			}
			rect, err := pdf.NewPdfRectangle(*arr)

			if err != nil {
				return nil, err
			}

			return rect, nil
		}

		node = dict.Get(core.PdfObjectName("Parent"))
	}

	return nil, fmt.Errorf("box %s not defined", boxname)
}

func getFlowAsInts(ss []string, max int) (list []int, err error) {
	keys := make(map[int]bool)
	var (
		i int
		e string
	)
	for _, e = range ss {
		i, err = strconv.Atoi(e)
		if err != nil {
			return
		}
		if i > max {
			err = fmt.Errorf("max flow is %d element %d unacceptable", max, i)
			return
		}
		if _, ok := keys[i]; !ok {
			keys[i] = true
			list = append(list, i)
		}
	}
	return
}
