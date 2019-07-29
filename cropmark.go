package main

import "github.com/unidoc/unipdf/v3/creator"

type CropMarkBlock struct {
	w, h, bleedx, bleedy float64
	col, row             int
	extw, exth           float64
	c                    *creator.Creator
}

func (bk *CropMarkBlock) Create() *creator.Block {
	w, h, bleedx, bleedy, col, row, extw, exth := bk.w, bk.h, bk.bleedx, bk.bleedy, bk.col, bk.row, bk.extw, bk.exth
	c := bk.c
	// extended to enncompass cropmarks
	cros2bw := float64(col)*w + 2*extw
	cros2bh := float64(row)*h + 2*exth
	// create cropmarks block
	crosb := creator.NewBlock(cros2bw, cros2bh)
	crosb.SetPos(0.0, 0.0)

	// the width used for cropmark
	lw := 0.4 * creator.PPMM // points

	// create top line of cropmarks and left line
	for y := 0; y < row; y++ {
		for x := 0; x < col; x++ {
			if y == 0 {
				// top line with space for cropmark
				l := c.NewLine(float64(x)*w+bleedx-0.5*lw+extw, 0, float64(x)*w+bleedx-0.5*lw+extw, markh)
				l.SetLineWidth(lw)
				crosb.Draw(l)
				l = c.NewLine(float64(x+1)*w-bleedx-0.5*lw+extw, 0, float64(x+1)*w-bleedx-0.5*lw+extw, markh)
				l.SetLineWidth(lw)

				crosb.Draw(l)
			}
		}
		// left line with space for cropmark
		l := c.NewLine(0, float64(y)*h+bleedy+0.5*lw+exth, markw, float64(y)*h+bleedy+0.5*lw+exth)
		l.SetLineWidth(lw)
		crosb.Draw(l)
		l = c.NewLine(0, float64(y+1)*h-bleedy+0.5*lw+exth, markw, float64(y+1)*h-bleedy+0.5*lw+exth)
		l.SetLineWidth(lw)
		crosb.Draw(l)
	}

	// use the half of cropmarks block created and a rotated duplicate of it
	// to get a fully cropmarks block
	cros2b := creator.NewBlock(cros2bw, cros2bh)
	// place with cropmarks outside - offset backward with their sizes extw and exth
	cros2b.SetPos(left-extw, top-exth)
	cros2b.Draw(crosb)
	//rect := c.NewRectangle(0.0, 0.0, cros2bw, cros2bh)
	//rect.SetBorderColor(creator.ColorBlack)
	//cros2b.Draw(rect)
	crosb.SetAngle(-180)
	crosb.SetPos(cros2bw, cros2bh)
	cros2b.Draw(crosb)

	return cros2b
}