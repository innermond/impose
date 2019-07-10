package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path"

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

	flag.Parse()

	if fn == "" {
		return errors.New("pdf file required")
	}

	if fout == "" {
		ext := path.Ext(fn)
		fout = fn[:len(fn)-len(ext)] + ".imposition" + ext
	}

	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "centerx":
			centerx = true
		case "centery":
			centery = true
		case "left":
			left *= creator.PPMM
		case "right":
			right *= creator.PPMM
		case "top":
			top *= creator.PPMM
		case "bottom":
			bottom *= creator.PPMM
		case "width":
			width *= creator.PPMM
		case "height":
			height *= creator.PPMM
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
	bbox, err := page.GetMediaBox()
	//bbox, err := GetBleedBox(page)
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
		left = (available - wpages) * 0.5
		right = left
	}
	if centery {
		hpages := h
		available := media[1] - (top + bottom)
		for hpages < available {
			hpages += h
		}
		hpages -= h
		top = (available - hpages) * 0.5
		bottom = top
	}

	xpos = left
	ypos = top
	for i := 0; i < np; i++ {
		num := i + 1

		endx = xpos + float64(w)
		if endx > media[0]-right {
			ypos += float64(h)
			xpos = left
			endx = xpos + float64(w)
			endy = ypos + float64(h)
			if endy > media[1]-bottom {
				ypos = top
				endy = ypos + float64(h)
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
		p := c.NewParagraph(fmt.Sprintf("page %d", num))
		p.SetPos(bk.Width()*0.5, bk.Height()*0.5)
		bk.Draw(p)
		bk.SetPos(xpos, ypos)
		_ = c.Draw(bk)

		xpos = endx
		fmt.Println(num)
	}
	err = c.WriteToFile(fout)
	if err != nil {
		log.Fatal(err)
	}
}

// GetBleedBox gets the inheritable media box value, either from the page
// or a higher up page/pages struct.
func GetBleedBox(p *pdf.PdfPage) (*pdf.PdfRectangle, error) {
	if p.BleedBox != nil {
		return p.BleedBox, nil
	}

	node := p.Parent
	for node != nil {
		dict, ok := core.GetDict(node)
		if !ok {
			return nil, errors.New("invalid parent objects dictionary")
		}

		if obj := dict.Get("BleedBox"); obj != nil {
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

		node = dict.Get("Parent")
	}

	return nil, errors.New("bleed box not defined")
}
