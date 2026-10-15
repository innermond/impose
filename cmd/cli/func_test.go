package main

import (
	"reflect"
	"testing"
)

func TestIsFlag(t *testing.T) {
	for arg, want := range map[string]bool{
		"-f": true, "-angle": true, "--grid": true, "-grid=2x1": true,
		"-90": false, "-.5": false, "-": false, "": false, "2x1": false, "file.pdf": false, "x-": false,
	} {
		if got := isFlag(arg); got != want {
			t.Errorf("isFlag(%q) = %v want %v", arg, got, want)
		}
	}
}

func TestClivide(t *testing.T) {
	common := map[string]bool{"f": true, "angle": true, "pages": true}
	cli := []string{"-f", "in.pdf", "-angle", "-90", "-creep", "0.5", "-pages", "-2", "-angle=-45"}
	same, spec := clivide(cli, common)
	wantSame := []string{"-f", "in.pdf", "-angle", "-90", "-pages", "-2", "-angle=-45"}
	wantSpec := []string{"-creep", "0.5"}
	if !reflect.DeepEqual(same, wantSame) {
		t.Errorf("common: got %v want %v", same, wantSame)
	}
	if !reflect.DeepEqual(spec, wantSpec) {
		t.Errorf("specific: got %v want %v", spec, wantSpec)
	}
}

func TestParsex(t *testing.T) {
	for x, want := range map[string][2]int{"": {1, 1}, "2x1": {2, 1}, "10x3": {10, 3}} {
		col, row, err := parsex(x)
		if err != nil || col != want[0] || row != want[1] {
			t.Errorf("parsex(%q) = %d,%d,%v want %v", x, col, row, err, want)
		}
	}
	// the argument is parsed, whatever value the grid flag has
	grid = ""
	if col, row, err := parsex("3x2"); err != nil || col != 3 || row != 2 {
		t.Errorf("parsex with empty grid: %d,%d,%v", col, row, err)
	}
	for _, bad := range []string{"2", "axb", "2x", "2x3x4"} {
		if _, _, err := parsex(bad); err == nil {
			t.Errorf("parsex(%q) should fail", bad)
		}
	}
}

func TestUnitFactor(t *testing.T) {
	if f, err := unitFactor("in"); err != nil || f != 72 {
		t.Errorf("in: %v %v", f, err)
	}
	if f, _ := unitFactor("CM"); f != 10*mmPt() {
		t.Errorf("cm: %v", f)
	}
	if _, err := unitFactor("furlong"); err == nil {
		t.Error("unknown unit should fail")
	}
}

func mmPt() float64 {
	f, _ := unitFactor("mm")
	return f
}
