package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/innermond/impose"
)

func main() {
	err := param()
	deal(err)

	// profiling
	defer cpu()()

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

	if verbosity > 1 {
		log.Printf("read pdf %q", fn)
	}
	src, err := impose.OpenSource(f)
	fatal(err)

	opts := options()

	err = impose.Run(context.Background(), opts, src, os.Stdout, func(num int) {
		if verbosity > 1 {
			log.Println("Page ", num)
		}
	})
	fatal(err)

	// profilng
	mem()
}

// options gathers what the command line asked for
func options() impose.Options {
	o := impose.DefaultOptions()
	o.Mode = impose.ModeGrid
	if repeat {
		o.Mode = impose.ModeRepeat
	} else if bookletMode {
		o.Mode = impose.ModeBooklet
	}

	// an empty grid is not known, see impose.Options
	if grid != "" {
		var err error
		o.Col, o.Row, err = parsex(grid)
		fatal("grid: ", err)
	}
	var err error
	o.CloneX, o.CloneY, err = parsex(clone)
	fatal("clone: ", err)
	o.ClonePadX, o.ClonePadY = clonepadx, clonepady

	o.Pages = pages
	o.Flow = flowList()
	o.Duplex, o.Flip, o.Reverse = duplex, flip, reverse
	o.Weld, o.Turn = weld, turn
	o.Creep = creep

	o.Width, o.Height = width, height
	o.Top, o.Right, o.Bottom, o.Left = top, right, bottom, left
	o.Autopage, o.Autopadding = autopage, autopadding
	o.CenterX, o.CenterY = centerx, centery
	o.Angle = angle

	o.Bleedx, o.Bleedy = bleedx, bleedy
	o.Offx, o.Offy = offx, offy
	o.Markw, o.Markh = markw, markh
	o.NoCropmark = !showcropmark
	o.CropmarkPages = showcropmarkPages
	o.Outline = outline
	return o
}

// flowList parses the flow, pages are numbered from 1
func flowList() []int {
	if len(flow) < 1 {
		return []int{1}
	}
	flowArr := []int{}
	for _, val := range strings.Split(flow, ",") {
		inx, err := strconv.Atoi(strings.TrimSpace(val))
		if err != nil {
			log.Fatalf("flow: %v", err)
		}
		if inx < 1 {
			log.Fatalf("flow: %d is not valid, pages are numbered from 1", inx)
		}
		flowArr = append(flowArr, inx)
	}
	return flowArr
}
