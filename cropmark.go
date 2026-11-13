package impose

import (
	"github.com/unidoc/unipdf/v3/creator"
)

type CropMarkBlock struct {
	w, h, bleedx, bleedy float64
	col, row             int
	extw, exth           float64
	markw, markh         float64
	father               *Boxes
}

// lines gives the lines of the half of cropmarks, as they are drawn in the cropmarks block
func (bk *CropMarkBlock) lines(bookletMode bool) []Line {
	w, h, bleedx, bleedy, col, row, extw, exth, markw, markh := bk.w, bk.h, bk.bleedx, bk.bleedy, bk.col, bk.row, bk.extw, bk.exth, bk.markw, bk.markh
	lw := bk.lineWidth()
	cx, cy := bk.father.CloneX, bk.father.CloneY
	cpx, cpy := bk.father.ClonePadX, bk.father.ClonePadY

	var ll []Line
	if bookletMode {
		// draw only ends
		for cx > 0 {
			x := float64(cx-1)*cpx + float64(cx)*2*w - bleedx - 0.5*lw + extw
			ll = append(ll, Line{x, 0, x, markh})
			x = float64(cx-1)*cpx + float64(cx-1)*2*w + bleedx - 0.5*lw + extw
			ll = append(ll, Line{x, 0, x, markh})
			cx--
		}
		for cy > 0 {
			y := float64(cy-1)*cpy + float64(cy)*h + exth - bleedy + 0.5*lw
			ll = append(ll, Line{0, y, markw, y})
			y = float64(cy-1)*cpy + float64(cy-1)*h + bleedy + 0.5*lw + exth
			ll = append(ll, Line{0, y, markw, y})
			cy--
		}
		return ll
	}
	// col and row count cells over all clones, marks follow every clone with its pad
	group, grouprow := bk.father.Col, bk.father.Row
	merge := bk.father.MergeBleed
	xStarts := gridStarts(col, group, cpx, w, bleedx, merge)
	yStarts := gridStarts(row, grouprow, cpy, h, bleedy, merge)
	for x := 0; x < col; x++ {
		near, _ := gridInset(x, col, group, cpx, bleedx, merge)
		// top line with space for cropmark
		x1 := xStarts[x] + bleedx - near - 0.5*lw + extw
		ll = append(ll, Line{x1, 0, x1, markh})
		x2 := xStarts[x] + w - near - bleedx - 0.5*lw + extw
		ll = append(ll, Line{x2, 0, x2, markh})
	}
	for y := 0; y < row; y++ {
		near, _ := gridInset(y, row, grouprow, cpy, bleedy, merge)
		// left line with space for cropmark
		y1 := yStarts[y] + bleedy - near + 0.5*lw + exth
		ll = append(ll, Line{0, y1, markw, y1})
		y2 := yStarts[y] + h - near - bleedy + 0.5*lw + exth
		ll = append(ll, Line{0, y2, markw, y2})
	}
	return ll
}

// lineWidth is the width used for cropmark, in points
func (bk *CropMarkBlock) lineWidth() float64 {
	return 0.4 * creator.PPMM
}

// size of the block that encompasses the cropmarks
func (bk *CropMarkBlock) size() (w, h float64) {
	f := bk.father
	w = gridSpan(bk.col, f.Col, f.ClonePadX, bk.w, bk.bleedx, f.MergeBleed) + 2*bk.extw
	h = gridSpan(bk.row, f.Row, f.ClonePadY, bk.h, bk.bleedy, f.MergeBleed) + 2*bk.exth
	return
}

// SheetLines gives all the cropmarks lines as they are on the sheet, from its top left corner:
// the half of cropmarks and its duplicate rotated with 180 degrees.
// Lines are centered on their stroke, so they sit on the trim edges of pages.
func (bk *CropMarkBlock) SheetLines(bookletMode bool) []Line {
	cros2bw, cros2bh := bk.size()
	// the block is placed with cropmarks outside - offset backward with their sizes extw and exth
	ox, oy := bk.father.Big.Left-bk.extw, bk.father.Big.Top-bk.exth
	half := bk.lines(bookletMode)
	// a stroke is drawn from the coordinate of its line, not centered on it
	lw := bk.lineWidth()
	for i, l := range half {
		if l.X1 == l.X2 {
			half[i].X1, half[i].X2 = l.X1+0.5*lw, l.X2+0.5*lw
		} else {
			half[i].Y1, half[i].Y2 = l.Y1-0.5*lw, l.Y2-0.5*lw
		}
	}
	out := make([]Line, 0, 2*len(half))
	for _, l := range half {
		out = append(out, Line{ox + l.X1, oy + l.Y1, ox + l.X2, oy + l.Y2})
	}
	for _, l := range half {
		out = append(out, Line{ox + cros2bw - l.X1, oy + cros2bh - l.Y1, ox + cros2bw - l.X2, oy + cros2bh - l.Y2})
	}
	return out
}

func (bk *CropMarkBlock) Create(bookletMode, angled bool) *creator.Block {
	extw, exth := bk.extw, bk.exth
	c := bk.father.Creator
	// extended to enncompass cropmarks
	cros2bw, cros2bh := bk.size()
	// create cropmarks block
	crosb := creator.NewBlock(cros2bw, cros2bh)
	crosb.SetPos(0.0, 0.0)

	lw := bk.lineWidth()
	for _, ln := range bk.lines(bookletMode) {
		l := c.NewLine(ln.X1, ln.Y1, ln.X2, ln.Y2)
		l.SetLineWidth(lw)
		crosb.Draw(l)
	}
	// use the half of cropmarks block created and a rotated duplicate of it
	// to get a fully cropmarks block
	cros2b := creator.NewBlock(cros2bw, cros2bh)
	// place with cropmarks outside - offset backward with their sizes extw and exth
	cros2b.SetPos(bk.father.Big.Left-extw, bk.father.Big.Top-exth)
	cros2b.Draw(crosb)
	//rect := c.NewRectangle(0.0, 0.0, cros2bw, cros2bh)
	//rect.SetBorderColor(creator.ColorBlack)
	//cros2b.Draw(rect)
	crosb.SetPos(-1*cros2bw, -1*cros2bh)
	crosb.SetAngle(-180)
	crosb.SetPos(0, 0)
	cros2b.Draw(crosb)

	return cros2b
}
