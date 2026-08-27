package mapgeo

import (
	"math"
	"sort"
)

type Transform struct {
	Resolution float64
	OriginX    float64
	OriginY    float64
	Width      int
	Height     int
}

func (t Transform) WorldToPixel(x, y float64) (col, row float64) {
	col = (x - t.OriginX) / t.Resolution
	row = float64(t.Height) - (y-t.OriginY)/t.Resolution
	return col, row
}

func (t Transform) PixelToWorldCenter(col, row int) (x, y float64) {
	x = t.OriginX + (float64(col)+0.5)*t.Resolution
	y = t.OriginY + (float64(t.Height)-float64(row)-0.5)*t.Resolution
	return x, y
}

type Mask struct {
	W    int
	H    int
	Bits []bool
}

func NewMask(w, h int) *Mask {
	return &Mask{W: w, H: h, Bits: make([]bool, w*h)}
}

func (m *Mask) At(col, row int) bool {
	if m == nil || col < 0 || row < 0 || col >= m.W || row >= m.H {
		return false
	}
	return m.Bits[row*m.W+col]
}

func (m *Mask) Count() int {
	n := 0
	for _, b := range m.Bits {
		if b {
			n++
		}
	}
	return n
}

func (m *Mask) Set(col, row int) {
	if col < 0 || row < 0 || col >= m.W || row >= m.H {
		return
	}
	m.Bits[row*m.W+col] = true
}

func pointInPoly(px, py float64, poly []float64) bool {
	n := len(poly) / 2
	if n < 3 {
		return false
	}
	inside := false
	j := n - 1
	for i := 0; i < n; i++ {
		xi, yi := poly[2*i], poly[2*i+1]
		xj, yj := poly[2*j], poly[2*j+1]
		if (yi > py) != (yj > py) && px < (xj-xi)*(py-yi)/(yj-yi)+xi {
			inside = !inside
		}
		j = i
	}
	return inside
}

func Rasterize(poly []float64, t Transform) *Mask {
	m := NewMask(t.Width, t.Height)
	if len(poly) < 6 {
		return m
	}

	minX, minY := poly[0], poly[1]
	maxX, maxY := poly[0], poly[1]
	for i := 0; i < len(poly); i += 2 {
		minX = math.Min(minX, poly[i])
		maxX = math.Max(maxX, poly[i])
		minY = math.Min(minY, poly[i+1])
		maxY = math.Max(maxY, poly[i+1])
	}

	c0, r1 := t.WorldToPixel(minX, minY)
	c1, r0 := t.WorldToPixel(maxX, maxY)
	colLo := clamp(int(math.Floor(c0)), 0, t.Width-1)
	colHi := clamp(int(math.Ceil(c1)), 0, t.Width-1)
	rowLo := clamp(int(math.Floor(r0)), 0, t.Height-1)
	rowHi := clamp(int(math.Ceil(r1)), 0, t.Height-1)

	for row := rowLo; row <= rowHi; row++ {
		for col := colLo; col <= colHi; col++ {
			x, y := t.PixelToWorldCenter(col, row)
			if pointInPoly(x, y, poly) {
				m.Set(col, row)
			}
		}
	}
	return m
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func Union(masks ...*Mask) *Mask {
	var out *Mask
	for _, m := range masks {
		if m == nil {
			continue
		}
		if out == nil {
			out = NewMask(m.W, m.H)
		}
		if m.W != out.W || m.H != out.H {
			continue
		}
		for i, b := range m.Bits {
			if b {
				out.Bits[i] = true
			}
		}
	}
	return out
}

func SplitByLine(m *Mask, x1, y1, x2, y2 float64, t Transform) (left, right *Mask) {
	left = NewMask(m.W, m.H)
	right = NewMask(m.W, m.H)
	dx, dy := x2-x1, y2-y1
	for row := 0; row < m.H; row++ {
		for col := 0; col < m.W; col++ {
			if !m.At(col, row) {
				continue
			}
			x, y := t.PixelToWorldCenter(col, row)
			cross := dx*(y-y1) - dy*(x-x1)
			if cross >= 0 {
				left.Set(col, row)
			} else {
				right.Set(col, row)
			}
		}
	}
	return left, right
}

func Adjacent(a, b *Mask) bool {
	if a == nil || b == nil {
		return false
	}
	neighbors := [][2]int{{0, 0}, {1, 0}, {-1, 0}, {0, 1}, {0, -1}}
	for row := 0; row < a.H; row++ {
		for col := 0; col < a.W; col++ {
			if !a.At(col, row) {
				continue
			}
			for _, d := range neighbors {
				if b.At(col+d[0], row+d[1]) {
					return true
				}
			}
		}
	}
	return false
}

func BuildAdjacency(masks []*Mask) [][]int {
	out := make([][]int, len(masks))
	for i := range masks {
		var adj []int
		for j := range masks {
			if i == j {
				continue
			}
			if Adjacent(masks[i], masks[j]) {
				adj = append(adj, j)
			}
		}
		sort.Ints(adj)
		out[i] = adj
	}
	return out
}

func largestComponent(m *Mask) *Mask {
	labels := make([]int, m.W*m.H)
	best := NewMask(m.W, m.H)
	bestCount := 0
	cur := 0
	dirs := [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
	for r := 0; r < m.H; r++ {
		for c := 0; c < m.W; c++ {
			if !m.At(c, r) || labels[r*m.W+c] != 0 {
				continue
			}
			cur++
			stack := [][2]int{{c, r}}
			labels[r*m.W+c] = cur
			var cells [][2]int
			for len(stack) > 0 {
				p := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				cells = append(cells, p)
				for _, d := range dirs {
					nc, nr := p[0]+d[0], p[1]+d[1]
					if nc < 0 || nr < 0 || nc >= m.W || nr >= m.H {
						continue
					}
					if !m.At(nc, nr) || labels[nr*m.W+nc] != 0 {
						continue
					}
					labels[nr*m.W+nc] = cur
					stack = append(stack, [2]int{nc, nr})
				}
			}
			if len(cells) > bestCount {
				bestCount = len(cells)
				best = NewMask(m.W, m.H)
				for _, p := range cells {
					best.Set(p[0], p[1])
				}
			}
		}
	}
	return best
}

type corner struct{ c, r int }

type edge struct{ a, b corner }

func traceLoops(comp *Mask) [][]corner {
	var edges []edge
	for r := 0; r < comp.H; r++ {
		for c := 0; c < comp.W; c++ {
			if !comp.At(c, r) {
				continue
			}
			if !comp.At(c, r-1) {
				edges = append(edges, edge{corner{c, r}, corner{c + 1, r}})
			}
			if !comp.At(c+1, r) {
				edges = append(edges, edge{corner{c + 1, r}, corner{c + 1, r + 1}})
			}
			if !comp.At(c, r+1) {
				edges = append(edges, edge{corner{c + 1, r + 1}, corner{c, r + 1}})
			}
			if !comp.At(c-1, r) {
				edges = append(edges, edge{corner{c, r + 1}, corner{c, r}})
			}
		}
	}

	outgoing := map[corner][]int{}
	for i, e := range edges {
		outgoing[e.a] = append(outgoing[e.a], i)
	}
	used := make([]bool, len(edges))

	var loops [][]corner
	for i := range edges {
		if used[i] {
			continue
		}
		var loop []corner
		curr := i
		for curr >= 0 && !used[curr] {
			used[curr] = true
			loop = append(loop, edges[curr].a)
			next := -1
			for _, j := range outgoing[edges[curr].b] {
				if !used[j] {
					next = j
					break
				}
			}
			curr = next
		}
		if len(loop) >= 3 {
			loops = append(loops, loop)
		}
	}
	return loops
}

func cornerArea(loop []corner) float64 {
	area := 0.0
	n := len(loop)
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		area += float64(loop[i].c)*float64(loop[j].r) - float64(loop[j].c)*float64(loop[i].r)
	}
	return area / 2
}

func Contour(m *Mask, t Transform) []float64 {
	comp := largestComponent(m)
	loops := traceLoops(comp)
	if len(loops) == 0 {
		return nil
	}
	best := loops[0]
	bestArea := math.Abs(cornerArea(best))
	for _, l := range loops[1:] {
		a := math.Abs(cornerArea(l))
		if a > bestArea {
			bestArea = a
			best = l
		}
	}

	pts := make([][2]float64, len(best))
	for i, cn := range best {
		pts[i][0] = t.OriginX + float64(cn.c)*t.Resolution
		pts[i][1] = t.OriginY + float64(t.Height-cn.r)*t.Resolution
	}

	simp := simplifyClosed(pts, 0.75*t.Resolution)
	if len(simp) < 3 {
		simp = pts
	}

	out := make([]float64, 0, len(simp)*2)
	for _, p := range simp {
		out = append(out, p[0], p[1])
	}
	return out
}

func simplifyClosed(ring [][2]float64, eps float64) [][2]float64 {
	n := len(ring)
	if n < 3 {
		return ring
	}
	far, fd := 0, -1.0
	for i := 1; i < n; i++ {
		d := math.Hypot(ring[i][0]-ring[0][0], ring[i][1]-ring[0][1])
		if d > fd {
			fd = d
			far = i
		}
	}
	part1 := ring[:far+1]
	part2 := make([][2]float64, 0, n-far+1)
	part2 = append(part2, ring[far:]...)
	part2 = append(part2, ring[0])

	s1 := douglasPeucker(part1, eps)
	s2 := douglasPeucker(part2, eps)

	out := make([][2]float64, 0, len(s1)+len(s2))
	out = append(out, s1[:len(s1)-1]...)
	out = append(out, s2[:len(s2)-1]...)
	return out
}

func douglasPeucker(pts [][2]float64, eps float64) [][2]float64 {
	if len(pts) < 3 {
		return pts
	}
	a, b := pts[0], pts[len(pts)-1]
	dmax, idx := 0.0, 0
	for i := 1; i < len(pts)-1; i++ {
		d := perpDist(pts[i], a, b)
		if d > dmax {
			dmax = d
			idx = i
		}
	}
	if dmax > eps {
		left := douglasPeucker(pts[:idx+1], eps)
		right := douglasPeucker(pts[idx:], eps)
		return append(left[:len(left)-1], right...)
	}
	return [][2]float64{a, b}
}

func perpDist(p, a, b [2]float64) float64 {
	dx, dy := b[0]-a[0], b[1]-a[1]
	if dx == 0 && dy == 0 {
		return math.Hypot(p[0]-a[0], p[1]-a[1])
	}
	num := math.Abs(dy*p[0] - dx*p[1] + b[0]*a[1] - b[1]*a[0])
	return num / math.Hypot(dx, dy)
}
