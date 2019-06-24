package main

import (
	"errors"
	"flag"
	"log"
	"path"

	"github.com/jung-kurt/gofpdf"
	"github.com/phpdave11/gofpdi"
	rscpdf "rsc.io/pdf"
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
		}
	})

	return err
}

func main() {
	err := param()
	if err != nil {
		log.Fatal(err)
	}

	// set default page media
	media := gofpdf.SizeType{width, height}
	pdf := gofpdf.NewCustom(&gofpdf.InitType{UnitStr: unit, Size: media})

	pdf.SetCompression(false)

	// Margins of page
	pdf.SetMargins(left, top, right)
	pdf.SetAutoPageBreak(false, bottom)

	// get num pages and box size
	rf, err := rscpdf.Open(fn)
	if err != nil {
		log.Fatal(err)
	}
	np := rf.NumPage()
	if lessPagesNum > 0 && lessPagesNum < np {
		np = lessPagesNum
	}

	_, _, w, h, boxname, err := pageBox(rf, 1, "BleedBox", pdf.GetConversionRatio())
	if err != nil {
		log.Fatal(err)
	}

	pdf.AddPage()

	var fpdi = gofpdi.NewImporter()
	fpdi.SetSourceFile(fn)

	var xpos, ypos, endx, endy float64

	left, top, right, bottom := pdf.GetMargins()
	if centerx {
		wpages := w
		available := media.Wd - (left + right)
		for wpages < available {
			wpages += w
		}
		wpages -= w
		left = (available - wpages) * 0.5
		right = left
	}
	if centery {
		hpages := h
		available := media.Ht - (top + bottom)
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
		if endx > media.Wd-right {
			ypos += float64(h)
			xpos = left
			endx = xpos + float64(w)
			endy = ypos + float64(h)
			if endy > media.Ht-bottom {
				ypos = top
				endy = ypos + float64(h)
				pdf.AddPage()
			}
		}
		importPage(pdf, fpdi, boxname, num, xpos, ypos, w, 0.0)
		xpos = endx
	}

	err = pdf.OutputFileAndClose(fout)
	if err != nil {
		log.Fatal(err)
	}
}

func importPage(pdf *gofpdf.Fpdf, fpdi *gofpdi.Importer, boxname string, num int, xpos, ypos, w, h float64) {
	tplid := fpdi.ImportPage(num, "/"+boxname)
	// import template to page
	tplObjIDs := fpdi.PutFormXobjectsUnordered()
	pdf.ImportTemplates(tplObjIDs)
	imported := fpdi.GetImportedObjectsUnordered()
	pdf.ImportObjects(imported)
	importedObjPos := fpdi.GetImportedObjHashPos()
	pdf.ImportObjPos(importedObjPos)

	tplName, sx, sy, tx, ty := fpdi.UseTemplate(tplid, xpos, ypos, w, 0.0)
	pdf.UseImportedTemplate(tplName, sx, sy, tx, ty)
}

func pageBox(rf *rscpdf.Reader, inx int, tryboxname string, k float64) (x float64, y float64, w float64, h float64, boxname string, err error) {
	p := rf.Page(inx)
	boxname = tryboxname
	mediabox := p.V.Key(boxname)
	if mediabox.Len() == 0 {
		boxname = "MediaBox"
		mediabox = p.V.Key(boxname)
		if mediabox.Len() != 4 {
			err = errors.New("wrong box lenght")
			return
		}
	}

	x = mediabox.Index(0).Float64() / k
	y = mediabox.Index(1).Float64() / k
	w = mediabox.Index(2).Float64() / k
	h = mediabox.Index(3).Float64() / k

	return
}
