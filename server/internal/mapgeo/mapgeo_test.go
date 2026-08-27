package mapgeo

import (
	"math"
	"testing"
)

func shoelace(poly []float64) float64 {
	n := len(poly) / 2
	area := 0.0
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		area += poly[2*i]*poly[2*j+1] - poly[2*j]*poly[2*i+1]
	}
	return math.Abs(area) / 2
}

func bbox(poly []float64) (minX, minY, maxX, maxY float64) {
	minX, minY = poly[0], poly[1]
	maxX, maxY = poly[0], poly[1]
	for i := 0; i < len(poly); i += 2 {
		minX = math.Min(minX, poly[i])
		maxX = math.Max(maxX, poly[i])
		minY = math.Min(minY, poly[i+1])
		maxY = math.Max(maxY, poly[i+1])
	}
	return
}

func square(cx, cy, half float64) []float64 {
	return []float64{
		cx - half, cy - half,
		cx + half, cy - half,
		cx + half, cy + half,
		cx - half, cy + half,
	}
}

func TestTransformRoundTrip(t *testing.T) {
	tr := Transform{Resolution: 0.05, OriginX: -4.525, OriginY: -2.25, Width: 210, Height: 219}
	cases := []struct{ col, row int }{
		{0, 0},
		{0, 218},
		{209, 0},
		{209, 218},
		{105, 110},
		{37, 200},
	}
	for _, c := range cases {
		x, y := tr.PixelToWorldCenter(c.col, c.row)
		fcol, frow := tr.WorldToPixel(x, y)
		gotCol := int(math.Floor(fcol))
		gotRow := int(math.Floor(frow))
		if gotCol != c.col || gotRow != c.row {
			t.Errorf("round trip pixel (%d,%d) -> world (%.4f,%.4f) -> pixel (%d,%d)",
				c.col, c.row, x, y, gotCol, gotRow)
		}
		if math.Abs(fcol-(float64(c.col)+0.5)) > 1e-9 {
			t.Errorf("col continuous mismatch: got %.9f want %.9f", fcol, float64(c.col)+0.5)
		}
		if math.Abs(frow-(float64(c.row)+0.5)) > 1e-9 {
			t.Errorf("row continuous mismatch: got %.9f want %.9f", frow, float64(c.row)+0.5)
		}
	}
}

func TestRasterizeSquareArea(t *testing.T) {
	tr := Transform{Resolution: 0.05, OriginX: 0, OriginY: 0, Width: 100, Height: 100}
	poly := square(2.5, 2.5, 1.0)
	m := Rasterize(poly, tr)
	wantArea := 2.0 * 2.0
	wantPixels := wantArea / (tr.Resolution * tr.Resolution)
	got := float64(m.Count())
	if math.Abs(got-wantPixels)/wantPixels > 0.02 {
		t.Errorf("rasterized count %.0f, want ~%.0f", got, wantPixels)
	}
}

func TestUnion(t *testing.T) {
	tr := Transform{Resolution: 0.05, OriginX: 0, OriginY: 0, Width: 200, Height: 100}
	a := Rasterize(square(1.0, 2.5, 0.5), tr)
	b := Rasterize(square(5.0, 2.5, 0.5), tr)
	disjoint := Union(a, b)
	if disjoint.Count() != a.Count()+b.Count() {
		t.Errorf("disjoint union %d, want %d", disjoint.Count(), a.Count()+b.Count())
	}

	c := Rasterize(square(1.0, 2.5, 0.5), tr)
	d := Rasterize(square(1.3, 2.5, 0.5), tr)
	overlap := Union(c, d)
	if overlap.Count() >= c.Count()+d.Count() {
		t.Errorf("overlapping union %d should be < %d", overlap.Count(), c.Count()+d.Count())
	}
	if overlap.Count() <= c.Count() {
		t.Errorf("overlapping union %d should exceed one square %d", overlap.Count(), c.Count())
	}
}

func TestSplitByLine(t *testing.T) {
	tr := Transform{Resolution: 0.05, OriginX: 0, OriginY: 0, Width: 100, Height: 100}
	poly := square(2.5, 2.5, 1.0)
	m := Rasterize(poly, tr)
	left, right := SplitByLine(m, 2.5, 0, 2.5, 5, tr)
	total := left.Count() + right.Count()
	if total != m.Count() {
		t.Errorf("split parts sum to %d, want %d", total, m.Count())
	}
	diff := math.Abs(float64(left.Count() - right.Count()))
	if diff/float64(m.Count()) > 0.05 {
		t.Errorf("halves unequal: left %d right %d", left.Count(), right.Count())
	}
}

func TestContourRectangle(t *testing.T) {
	tr := Transform{Resolution: 0.05, OriginX: 0, OriginY: 0, Width: 200, Height: 200}
	rect := []float64{
		1.0, 1.0,
		4.0, 1.0,
		4.0, 3.0,
		1.0, 3.0,
	}
	m := Rasterize(rect, tr)
	poly := Contour(m, tr)
	if len(poly) < 6 {
		t.Fatalf("contour returned %d floats, want >= 6", len(poly))
	}

	rMinX, rMinY, rMaxX, rMaxY := bbox(rect)
	cMinX, cMinY, cMaxX, cMaxY := bbox(poly)
	tol := tr.Resolution + 1e-9
	if math.Abs(rMinX-cMinX) > tol || math.Abs(rMinY-cMinY) > tol ||
		math.Abs(rMaxX-cMaxX) > tol || math.Abs(rMaxY-cMaxY) > tol {
		t.Errorf("bbox mismatch: rect (%.3f,%.3f,%.3f,%.3f) contour (%.3f,%.3f,%.3f,%.3f)",
			rMinX, rMinY, rMaxX, rMaxY, cMinX, cMinY, cMaxX, cMaxY)
	}

	rectArea := shoelace(rect)
	contArea := shoelace(poly)
	if math.Abs(contArea-rectArea)/rectArea > 0.10 {
		t.Errorf("contour area %.4f vs rect area %.4f (>10%%)", contArea, rectArea)
	}
}

func TestAdjacent(t *testing.T) {
	tr := Transform{Resolution: 0.05, OriginX: 0, OriginY: 0, Width: 300, Height: 100}
	a := Rasterize(square(1.0, 2.5, 0.5), tr)
	touching := Rasterize(square(2.0, 2.5, 0.5), tr)
	gapped := Rasterize(square(5.0, 2.5, 0.5), tr)

	if !Adjacent(a, touching) {
		t.Error("adjacent squares sharing an edge should be Adjacent")
	}
	if Adjacent(a, gapped) {
		t.Error("separated squares should not be Adjacent")
	}
}

func TestBuildAdjacency(t *testing.T) {
	tr := Transform{Resolution: 0.05, OriginX: 0, OriginY: 0, Width: 400, Height: 100}
	m0 := Rasterize(square(1.0, 2.5, 0.5), tr)
	m1 := Rasterize(square(2.0, 2.5, 0.5), tr)
	m2 := Rasterize(square(3.0, 2.5, 0.5), tr)

	adj := BuildAdjacency([]*Mask{m0, m1, m2})
	want := [][]int{{1}, {0, 2}, {1}}
	if len(adj) != len(want) {
		t.Fatalf("adjacency length %d, want %d", len(adj), len(want))
	}
	for i := range want {
		if len(adj[i]) != len(want[i]) {
			t.Fatalf("adj[%d]=%v, want %v", i, adj[i], want[i])
		}
		for j := range want[i] {
			if adj[i][j] != want[i][j] {
				t.Fatalf("adj[%d]=%v, want %v", i, adj[i], want[i])
			}
		}
	}
}

func TestRealisticFixture(t *testing.T) {
	tr := Transform{Resolution: 0.05, OriginX: -4.525, OriginY: -2.25, Width: 210, Height: 219}
	room := []float64{
		1.35, 4.425, 1.4, 4.375, 2.4, 4.375, 2.4, 3.775, 2.45, 3.725, 2.55, 3.725,
		2.55, 3.375, 2.7, 3.275, 2.7, 3.175, 2.5, 3.025, 2.3, 3.025, 2.15, 2.875,
		2.3, 2.775, 2.3, 2.575, 2.5, 2.475, 2.5, 0.875, 3.05, 0.775, 3.05, -1.125,
		2.5, -1.075, 2.5, -0.575, 1.85, -0.575, 1.7, -0.675, 1.7, -0.425, 0.15, -0.725,
		0.1, -1.075, -0.4, -0.925, -0.4, -0.775, -0.25, -0.575, -0.65, -0.525, -0.65, 0.625,
		-0.05, 0.575, 0.0, 0.225, 0.575, -0.2, 0.625, -0.6, 0.625, 1.325, -0.05, 1.375,
		0.0, 1.425, 2.175, 0.2, 2.225, 0.25, 2.225, 2.775, 2.975, 0.0, 3.025, -0.3,
		2.875, -0.55, 2.875, -0.6, 3.825, -0.2, 3.925, -0.15, 4.275, -0.1, 4.275, 0.45,
		3.825, 0.5, 3.825, 1.05, 4.375, 1.15, 4.425, 1.35,
	}

	m := Rasterize(room, tr)
	if m.Count() == 0 {
		t.Fatal("fixture rasterized to zero pixels")
	}
	pixelArea := float64(m.Count()) * tr.Resolution * tr.Resolution

	poly := Contour(m, tr)
	if len(poly) < 6 {
		t.Fatalf("contour returned %d floats, want >= 6", len(poly))
	}
	contArea := shoelace(poly)

	rel := math.Abs(contArea-pixelArea) / pixelArea
	t.Logf("pixel area=%.4f m^2 (%d px), contour area=%.4f m^2, rel diff=%.3f",
		pixelArea, m.Count(), contArea, rel)
	if rel > 0.12 {
		t.Errorf("contour area %.4f vs rasterized area %.4f differ by %.1f%% (>12%%)",
			contArea, pixelArea, rel*100)
	}
}
