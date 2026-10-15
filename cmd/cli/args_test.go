package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/innermond/impose"
	"github.com/unidoc/unipdf/v3/creator"
)

// the command line made by Options.CLIArgs has to give the very pdf that impose.Run gives
func TestCLIArgsRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the command")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "impose")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}

	// source pdf, 12 pages
	c := creator.New()
	c.SetPageSize(creator.PageSize{200, 300})
	for i := 1; i <= 12; i++ {
		c.NewPage()
		if err := c.Draw(c.NewParagraph(fmt.Sprintf("P%d", i))); err != nil {
			t.Fatal(err)
		}
	}
	pdf := filepath.Join(dir, "in.pdf")
	f, err := os.Create(pdf)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Write(f); err != nil {
		t.Fatal(err)
	}
	f.Close()
	src, err := os.Open(pdf)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	source, err := impose.OpenSource(src)
	if err != nil {
		t.Fatal(err)
	}

	mm := creator.PPMM
	base := func() impose.Options {
		o := impose.DefaultOptions()
		o.Width, o.Height = 450*mm, 320*mm
		return o
	}
	tt := map[string]impose.Options{}
	set := func(name string, f func(o *impose.Options)) {
		o := base()
		f(&o)
		tt[name] = o
	}
	set("grid", func(o *impose.Options) { o.Col, o.Row = 2, 1 })
	set("pages", func(o *impose.Options) { o.Col, o.Row, o.Pages, o.NoCropmark = 2, 1, "2-4,7", true })
	set("autopage", func(o *impose.Options) { o.Col, o.Row, o.Autopage, o.Autopadding = 2, 1, true, 5*mm })
	set("autopageNoMarks", func(o *impose.Options) {
		o.Col, o.Row, o.Autopage, o.NoCropmark, o.Offx, o.Offy, o.Markw, o.Markh = 2, 1, true, true, 4*mm, 4*mm, 6*mm, 9*mm
	})
	set("margins", func(o *impose.Options) {
		o.Col, o.Row, o.Left, o.Top, o.Right, o.Bottom = 2, 1, 5*mm, 7*mm, 3*mm, 9*mm
	})
	set("center", func(o *impose.Options) { o.Col, o.Row, o.CenterX, o.CenterY = 2, 1, true, true })
	set("centerx", func(o *impose.Options) { o.Col, o.Row, o.CenterX = 2, 1, true })
	set("marks", func(o *impose.Options) {
		o.Col, o.Row, o.Left, o.Top = 2, 1, 14*mm, 14*mm
		o.Bleedx, o.Bleedy, o.Offx, o.Offy, o.Markw, o.Markh = 2*mm, 4*mm, 3*mm, 5*mm, 6*mm, 8*mm
		o.CropmarkPages = []int{1}
	})
	set("angle", func(o *impose.Options) { o.Col, o.Row, o.Angle, o.Pages = 2, 1, -90, "1-2" })
	set("clone", func(o *impose.Options) {
		o.Col, o.Row, o.CloneX, o.CloneY, o.ClonePadX, o.ClonePadY, o.Pages = 1, 1, 2, 2, 5*mm, 7*mm, "1-2"
		o.Autopage = true
	})
	set("flow", func(o *impose.Options) {
		o.Col, o.Row, o.Flow, o.Width, o.Height, o.NoCropmark = 3, 2, []int{1, 1, 1, 2, 2, 2}, 650*mm, 650*mm, true
	})
	set("duplex", func(o *impose.Options) {
		o.Col, o.Row, o.Duplex, o.Flip, o.Reverse, o.Turn = 2, 1, true, true, true, 180
	})
	set("duplexweld", func(o *impose.Options) {
		o.Col, o.Row, o.Duplex, o.Weld, o.Width = 4, 1, true, 2, 900*mm
	})
	set("repeat", func(o *impose.Options) {
		o.Mode, o.CloneX, o.CloneY, o.ClonePadX, o.ClonePadY, o.Pages, o.Autopage = impose.ModeRepeat, 2, 2, 5*mm, 5*mm, "1-2", true
	})
	set("repeatturn", func(o *impose.Options) {
		o.Mode, o.CloneX, o.Turn, o.Pages, o.Autopage, o.NoCropmark = impose.ModeRepeat, 2, 90, "1-2", true, true
	})
	set("booklet", func(o *impose.Options) { o.Mode = impose.ModeBooklet })
	set("booklet4x1", func(o *impose.Options) { o.Mode, o.Col, o.Row, o.Width = impose.ModeBooklet, 4, 1, 900*mm })
	set("bookletCreep", func(o *impose.Options) {
		o.Mode, o.Creep, o.Bleedx, o.Bleedy, o.Markh, o.Left, o.Top = impose.ModeBooklet, 0.5*mm, 1*mm, 1*mm, 12*mm, 20*mm, 20*mm
	})

	// the pdf has its date and id, and the library prints a banner
	volatile := regexp.MustCompile(`(?m)/ID \[[^\]]*\]|/CreationDate \([^)]*\)|/ModDate \([^)]*\)`)
	for name, o := range tt {
		t.Run(name, func(t *testing.T) {
			var want bytes.Buffer
			if err := impose.Run(context.Background(), o, source, &want, nil); err != nil {
				t.Fatalf("run: %v", err)
			}
			args := o.CLIArgs(pdf)
			cmd := exec.Command(bin, args...)
			var got, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &got, &stderr
			if err := cmd.Run(); err != nil {
				t.Fatalf("%v: %v\n%s", args, err, stderr.String())
			}
			// the library may print its banner before the pdf
			b := got.Bytes()
			if i := bytes.Index(b, []byte("%PDF")); i > 0 {
				b = b[i:]
			}
			a := volatile.ReplaceAll(b, nil)
			w := volatile.ReplaceAll(want.Bytes(), nil)
			if !bytes.Equal(a, w) {
				t.Errorf("pdf differs for %v (%d bytes against %d)", args, len(a), len(w))
			}
		})
	}
}
