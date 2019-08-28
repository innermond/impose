package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

func isFlag(fg string) bool {
	is := len(fg) > 1 && fg[0] == '-' && fg[len(fg)-1] != '-'
	return is
}

// for cli divide a string slice, probably an os.Args[],
// into two string slices as they were separated command line args
func clivide(cli []string, comflags map[string]bool) (common, specific []string) {
	// extract comflags
	i := 0
	keep := true
	l := len(cli)
	for i < l {
		// current element
		curr := cli[i]
		// is flag?
		if isFlag(curr) {
			var fg string
			if eq := strings.IndexByte(curr[1:], '='); eq != -1 {
				fg = curr[1 : eq+1]
			} else {
				fg = curr[1:]
			}
			fg = strings.Trim(fg, "-")
			if _, ok := comflags[fg]; ok {
				common = append(common, curr)
				// next value can be an arg that belongs to curr element
				keep = true
			} else {
				specific = append(specific, curr)
				keep = false
			}
		}
		// dont go beyond cli edge, we assure can get the next element
		if i+1 >= l {
			break
		}
		// next element can be flag or arg
		next := cli[i+1]
		// is flag? then jump to processing flags code area
		if isFlag(next) {
			i++
			continue
		} else {
			if keep {
				// value flag kept
				common = append(common, next)
			} else {
				specific = append(specific, next)
			}
		}
		i++
	}
	return
}

func parsex(x string) (col, row int, err error) {
	// default value
	if grid == "" {
		return 1, 1, nil
	}

	// parse grid; has form like 2x1, at minimum 3 chars
	if len(x) < 3 || !strings.Contains(x, "x") {
		err = fmt.Errorf("invalid grid value %q", x)
		return
	}

	colrow := strings.Split(x, "x")
	if len(colrow) != 2 {
		err = errors.New("grid length invalid")
		return
	}

	col, err = strconv.Atoi(colrow[0])
	if err != nil {
		return
	}
	row, err = strconv.Atoi(colrow[1])
	if err != nil {
		return
	}

	return
}
