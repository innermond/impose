package impose

// boundaryMergeable tells whether the boundary right after position pos (0-based, out of n
// total cells grouped into clones of size group, group>=n or group<=0 meaning "one group")
// is a mergeable, bleed-shared boundary: always true inside a clone group, and true between
// clone groups only when there's no explicit pad there (a nonzero pad is a deliberate gap,
// so both sides keep their outer, full bleed instead).
func boundaryMergeable(pos, n, group int, pad float64) bool {
	if pos < 0 || pos >= n-1 {
		return false
	}
	if group > 0 && group < n && (pos+1)%group == 0 {
		return pad <= 0
	}
	return true
}

// gridInset gives how much of the bleed at axis position pos of n cells (grouped into
// clones of `group` size, `pad` apart) is clipped away on its near (left/top) and far
// (right/bottom) side when merge is on: half on every mergeable boundary (summing with the
// neighbor's half to one full bleed), 0 at the two outer edges, at a padded clone-group
// boundary, or when merge is off / there's only one cell.
func gridInset(pos, n, group int, pad, bleed float64, merge bool) (near, far float64) {
	if !merge || bleed <= 0 || n <= 1 {
		return 0, 0
	}
	half := bleed / 2
	if boundaryMergeable(pos-1, n, group, pad) {
		near = half
	}
	if boundaryMergeable(pos, n, group, pad) {
		far = half
	}
	return
}

// gridSpan gives the total extent of n cells of size w (grouped into clones of `group`
// size, `pad` apart) along an axis: each cell contributes w minus its insets, plus pad at
// every clone-group boundary that isn't merged away.
func gridSpan(n, group int, pad, w, bleed float64, merge bool) float64 {
	if n <= 0 {
		return 0
	}
	total := 0.0
	for i := 0; i < n; i++ {
		near, far := gridInset(i, n, group, pad, bleed, merge)
		total += w - near - far
		if i < n-1 && !boundaryMergeable(i, n, group, pad) {
			total += pad
		}
	}
	return total
}

// gridStarts gives the cumulative start position of every one of n cells sized w (grouped
// into clones of `group` size, `pad` apart) along an axis, from 0, saving bleed at every
// mergeable boundary exactly like gridSpan.
func gridStarts(n, group int, pad, w, bleed float64, merge bool) []float64 {
	starts := make([]float64, n)
	pos := 0.0
	for i := 0; i < n; i++ {
		starts[i] = pos
		near, far := gridInset(i, n, group, pad, bleed, merge)
		pos += w - near - far
		if i < n-1 && !boundaryMergeable(i, n, group, pad) {
			pos += pad
		}
	}
	return starts
}
