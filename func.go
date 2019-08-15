package impose

import (
	"fmt"
	"strconv"
)

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
