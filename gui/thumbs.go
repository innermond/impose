package main

import (
	"container/list"
	"image"
	"math"
	"sync"

	"github.com/gen2brain/go-fitz"
	"github.com/innermond/impose"
)

const (
	// tiers of resolution: tier t has 0.25 * 2^t pixels for a point, from 18 to 288 dpi
	minTier = 0
	maxTier = 4
	// maxThumbSide is the longest side of a thumbnail, in pixels
	maxThumbSide = 2048
	// thumbCacheBytes is the memory the thumbnails can take
	thumbCacheBytes = 200 << 20
	thumbWorkers    = 2
)

// tierPPP is the pixels for a point of a tier
func tierPPP(t int) float64 {
	return 0.25 * math.Exp2(float64(t))
}

// tierFor is the lowest tier sharp enough for pixels-per-point ppp, for a page of w x h points,
// the page is not made bigger than maxThumbSide.
func tierFor(ppp, w, h float64) int {
	t := minTier
	for t < maxTier && tierPPP(t) < ppp {
		t++
	}
	for t > minTier && math.Max(w, h)*tierPPP(t) > maxThumbSide {
		t--
	}
	return t
}

// thumbKey is a page rendered with a bleed at a tier.
type thumbKey struct {
	page           int
	bleedx, bleedy float64
	tier           int
}

// orientKey is a thumbnail turned counter clockwise by quarter turns, then mirrored.
type orientKey struct {
	thumbKey
	turns            int
	mirrorX, mirrorY bool
}

type thumbEntry struct {
	key any
	img *image.RGBA
}

// thumbs renders pages of the pdf in background and keeps the last used ones.
type thumbs struct {
	mu  sync.Mutex
	src *impose.Source
	// gen changes with the source, renders of an older source are dropped
	gen   int
	lru   *list.List
	items map[any]*list.Element
	bytes int
	limit int
	// queue is waiting to be rendered, the first first; busy is being rendered
	queue   []thumbKey
	busy    map[thumbKey]bool
	failed  map[thumbKey]bool
	workers int

	render func(src *impose.Source, k thumbKey) (*image.RGBA, error)
	// onReady is called, from a worker, after a thumbnail is ready
	onReady func()
}

func newThumbs(onReady func()) *thumbs {
	t := &thumbs{limit: thumbCacheBytes, render: renderPage, onReady: onReady}
	t.reset()
	return t
}

func (t *thumbs) reset() {
	t.gen++
	t.lru = list.New()
	t.items = map[any]*list.Element{}
	t.bytes = 0
	t.queue = nil
	t.busy = map[thumbKey]bool{}
	t.failed = map[thumbKey]bool{}
}

// setSource forgets everything about the previous pdf.
func (t *thumbs) setSource(src *impose.Source) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.src = src
	t.reset()
}

// best gives the thumbnail of the key or, while it is not there, the one of another tier,
// the sharpest one. It is nil if there is none.
func (t *thumbs) best(k thumbKey) (img *image.RGBA, tier int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if img := t.getLocked(k); img != nil {
		return img, k.tier
	}
	for tier := maxTier; tier >= minTier; tier-- {
		o := k
		o.tier = tier
		if img := t.getLocked(o); img != nil {
			return img, tier
		}
	}
	return nil, 0
}

// oriented gives the thumbnail of the key turned and mirrored, nil if the thumbnail is not rendered.
func (t *thumbs) oriented(k thumbKey, turns int, mirrorX, mirrorY bool) *image.RGBA {
	img, tier := t.best(k)
	if img == nil {
		return nil
	}
	k.tier = tier
	turns = ((turns % 4) + 4) % 4
	if turns == 0 && !mirrorX && !mirrorY {
		return img
	}
	ok := orientKey{k, turns, mirrorX, mirrorY}
	t.mu.Lock()
	defer t.mu.Unlock()
	if o := t.getLocked(ok); o != nil {
		return o
	}
	o := orient(img, turns, mirrorX, mirrorY)
	t.putLocked(ok, o)
	return o
}

// want queues the keys to render, in order, in place of what was waiting before.
func (t *thumbs) want(keys []thumbKey) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.src == nil {
		return
	}
	t.queue = t.queue[:0]
	seen := map[thumbKey]bool{}
	for _, k := range keys {
		if seen[k] || t.busy[k] || t.failed[k] || t.items[k] != nil {
			continue
		}
		seen[k] = true
		t.queue = append(t.queue, k)
	}
	for t.workers < thumbWorkers && t.workers < len(t.queue) {
		t.workers++
		go t.work()
	}
}

func (t *thumbs) work() {
	t.mu.Lock()
	for len(t.queue) > 0 {
		k := t.queue[0]
		t.queue = t.queue[1:]
		src, gen := t.src, t.gen
		t.busy[k] = true
		t.mu.Unlock()

		img, err := t.render(src, k)

		t.mu.Lock()
		if gen != t.gen {
			continue
		}
		delete(t.busy, k)
		if err != nil {
			t.failed[k] = true
			continue
		}
		t.putLocked(k, img)
		if t.onReady != nil {
			t.mu.Unlock()
			t.onReady()
			t.mu.Lock()
		}
	}
	t.workers--
	t.mu.Unlock()
}

func (t *thumbs) getLocked(k any) *image.RGBA {
	e := t.items[k]
	if e == nil {
		return nil
	}
	t.lru.MoveToFront(e)
	return e.Value.(*thumbEntry).img
}

func (t *thumbs) putLocked(k any, img *image.RGBA) {
	if e := t.items[k]; e != nil {
		t.bytes -= len(e.Value.(*thumbEntry).img.Pix)
		t.lru.Remove(e)
	}
	t.items[k] = t.lru.PushFront(&thumbEntry{k, img})
	t.bytes += len(img.Pix)
	// the newest one stays even if it is bigger than the limit
	for t.bytes > t.limit && t.lru.Len() > 1 {
		e := t.lru.Back()
		old := e.Value.(*thumbEntry)
		t.lru.Remove(e)
		delete(t.items, old.key)
		t.bytes -= len(old.img.Pix)
	}
}

// renderPage draws a page as it is cut by the imposition, with its bleed.
func renderPage(src *impose.Source, k thumbKey) (*image.RGBA, error) {
	pdf, err := src.PagePDF(k.page, k.bleedx, k.bleedy)
	if err != nil {
		return nil, err
	}
	doc, err := fitz.NewFromMemory(pdf)
	if err != nil {
		return nil, err
	}
	defer doc.Close()
	return doc.ImageDPI(0, 72*tierPPP(k.tier))
}

// orient turns img counter clockwise by quarter turns, then mirrors it left to right and top to bottom.
func orient(img *image.RGBA, turns int, mirrorX, mirrorY bool) *image.RGBA {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	ow, oh := w, h
	if turns%2 == 1 {
		ow, oh = h, w
	}
	out := image.NewRGBA(image.Rect(0, 0, ow, oh))
	for y := range h {
		for x := range w {
			var nx, ny int
			switch turns {
			case 0:
				nx, ny = x, y
			case 1:
				nx, ny = y, w-1-x
			case 2:
				nx, ny = w-1-x, h-1-y
			case 3:
				nx, ny = h-1-y, x
			}
			if mirrorX {
				nx = ow - 1 - nx
			}
			if mirrorY {
				ny = oh - 1 - ny
			}
			si := img.PixOffset(b.Min.X+x, b.Min.Y+y)
			di := out.PixOffset(nx, ny)
			copy(out.Pix[di:di+4], img.Pix[si:si+4])
		}
	}
	return out
}

// quarterTurns is an angle in quarter turns counter clockwise, from 0 to 3, it is not ok
// for angles that are not a multiple of 90 degrees.
func quarterTurns(angle float64) (int, bool) {
	q := math.Round(angle / 90)
	if math.Abs(angle/90-q) > 1e-9 {
		return 0, false
	}
	return ((int(q) % 4) + 4) % 4, true
}
