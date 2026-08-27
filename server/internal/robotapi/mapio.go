package robotapi

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// RoomLabel mirrors one entry of map_server's labels.json. Geometry is a flat
// [x,y,x,y,…] polygon in world metres; Graph holds adjacency as indices into
// the Data slice.
type RoomLabel struct {
	ColorType      int       `json:"colorType"`
	Geometry       []float64 `json:"geometry"`
	Graph          []int     `json:"graph"`
	GroundMaterial any       `json:"groundMaterial"`
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Type           string    `json:"type"`
}

// Labels is the whole-map room set. Every edit (rename/split/merge) resubmits
// the complete set; the device diffs and persists it. UUID must equal the
// active map id or funcID 1006 rejects with result code 1.
type Labels struct {
	Data    []RoomLabel `json:"data"`
	UUID    string      `json:"uuid"`
	Version string      `json:"version"`
}

// Marker mirrors one entry of map_server's markers.json. Zone type is encoded
// in the ID prefix (there is no separate type field; Type is "polygon" for
// zones and "pose" for the dock). Geometry is a flat [x,y,…] rectangle for
// zones and [x,y,theta] for the base pose.
type Marker struct {
	Geometry    []float64       `json:"geometry"`
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Type        string          `json:"type"`
	RotateAngle json.RawMessage `json:"rotateAngle,omitempty"`
}

// Markers is the whole marker set for the active map (no uuid/version on disk).
type Markers struct {
	Data []Marker `json:"data"`
}

// Marker id prefixes, mirroring the vendor's on-device conventions.
const (
	prefixCarpet = "BLACK_CARPET"
	prefixNoMop  = "NOWASH"
	prefixNoGo   = "PROHIBIT"
	prefixBase   = "COMBINED_BASE"
)

func MarkerKind(id string) string {
	switch {
	case strings.HasPrefix(id, prefixCarpet):
		return "carpet"
	case strings.HasPrefix(id, prefixNoMop):
		return "no_mop"
	case strings.HasPrefix(id, prefixNoGo):
		return "no_go"
	case strings.HasPrefix(id, prefixBase):
		return "base"
	default:
		return "other"
	}
}

func KindPrefix(kind string) (string, bool) {
	switch kind {
	case "carpet":
		return prefixCarpet, true
	case "no_mop":
		return prefixNoMop, true
	case "no_go":
		return prefixNoGo, true
	default:
		return "", false
	}
}

// MapMeta is the occupancy-grid transform (from map.json) plus raster size.
type MapMeta struct {
	Resolution float64 `json:"resolution"`
	OriginX    float64 `json:"origin_x"`
	OriginY    float64 `json:"origin_y"`
	Width      int     `json:"width"`
	Height     int     `json:"height"`
}

func labelsPath(mapDir string) string { return filepath.Join(mapDir, "labels.json") }
func backupPath(mapDir string) string { return filepath.Join(mapDir, "labels.json.mundus.bak") }

func ReadLabels(mapDir string) (Labels, error) {
	var l Labels
	data, err := os.ReadFile(labelsPath(mapDir))
	if err != nil {
		return l, err
	}
	if err := json.Unmarshal(data, &l); err != nil {
		return l, err
	}
	for i := range l.Data {
		if l.Data[i].Graph == nil {
			l.Data[i].Graph = []int{}
		}
	}
	return l, nil
}

// UpdateMapLabels submits the whole label set via funcID 1006. Param "0" is the
// labels JSON as a string, param "1" is the target (active) map id.
func (a *API) UpdateMapLabels(l Labels) error {
	if l.UUID == "" {
		return fmt.Errorf("labels missing uuid")
	}
	body, err := json.Marshal(l)
	if err != nil {
		return err
	}
	return a.call(fnUpdateLabels, map[string]any{"0": string(body), "1": l.UUID})
}

// Each call overwrites the previous backup. There is no restore endpoint:
// recover by copying the .bak back over labels.json on the device.
func BackupLabels(mapDir string) error {
	data, err := os.ReadFile(labelsPath(mapDir))
	if err != nil {
		return err
	}
	return os.WriteFile(backupPath(mapDir), data, 0o644)
}

func ReadMapID(mapDir string) (string, error) {
	var info struct {
		UUID string `json:"uuid"`
	}
	data, err := os.ReadFile(filepath.Join(mapDir, "mapinfo.json"))
	if err != nil {
		return "", err
	}
	if err := json.Unmarshal(data, &info); err != nil {
		return "", err
	}
	return info.UUID, nil
}

func markersPath(mapDir string) string { return filepath.Join(mapDir, "markers.json") }
func markersBackupPath(mapDir string) string {
	return filepath.Join(mapDir, "markers.json.mundus.bak")
}

func ReadMarkers(mapDir string) (Markers, error) {
	var m Markers
	data, err := os.ReadFile(markersPath(mapDir))
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return m, err
	}
	return m, nil
}

// UpdateMapMarkers submits the whole marker set via funcID 1007. Param "0" is
// the markers JSON string, param "1" the active map id (for the map-id guard).
func (a *API) UpdateMapMarkers(m Markers, mapID string) error {
	if mapID == "" {
		return fmt.Errorf("markers update missing map id")
	}
	body, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return a.call(fnUpdateMarkers, map[string]any{"0": string(body), "1": mapID})
}

func BackupMarkers(mapDir string) error {
	data, err := os.ReadFile(markersPath(mapDir))
	if err != nil {
		return err
	}
	return os.WriteFile(markersBackupPath(mapDir), data, 0o644)
}

func NextMarkerID(m Markers, kind string) (string, error) {
	prefix, ok := KindPrefix(kind)
	if !ok {
		return "", fmt.Errorf("unknown zone kind %q", kind)
	}
	max := -1
	for _, mk := range m.Data {
		if !strings.HasPrefix(mk.ID, prefix+"_") {
			continue
		}
		if n, err := strconv.Atoi(strings.TrimPrefix(mk.ID, prefix+"_")); err == nil && n > max {
			max = n
		}
	}
	return fmt.Sprintf("%s_%03d", prefix, max+1), nil
}

// ReadMapMeta reads map.json (resolution/origin) and the PGM header (size).
func ReadMapMeta(mapDir string) (MapMeta, error) {
	var m MapMeta
	var raw struct {
		Resolution float64   `json:"resolution"`
		Origin     []float64 `json:"origin"`
		Image      string    `json:"image"`
	}
	data, err := os.ReadFile(filepath.Join(mapDir, "map.json"))
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return m, err
	}
	if len(raw.Origin) < 2 {
		return m, fmt.Errorf("map.json origin malformed")
	}
	m.Resolution = raw.Resolution
	m.OriginX, m.OriginY = raw.Origin[0], raw.Origin[1]
	img := raw.Image
	if img == "" {
		img = "map.pgm"
	}
	w, h, _, err := readPGM(filepath.Join(mapDir, img))
	if err != nil {
		return m, err
	}
	m.Width, m.Height = w, h
	return m, nil
}

func ReadMapPNG(mapDir string) ([]byte, error) {
	w, h, pix, err := readPGM(filepath.Join(mapDir, "map.pgm"))
	if err != nil {
		return nil, err
	}
	img := image.NewGray(image.Rect(0, 0, w, h))
	copy(img.Pix, pix)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// readPGM parses a binary (P5) portable graymap: header tokens then W*H bytes.
func readPGM(path string) (w, h int, pix []byte, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, nil, err
	}
	defer f.Close()
	r := bufio.NewReader(f)
	magic, err := readToken(r)
	if err != nil {
		return 0, 0, nil, err
	}
	if magic != "P5" {
		return 0, 0, nil, fmt.Errorf("unsupported pgm magic %q", magic)
	}
	wt, err := readToken(r)
	if err != nil {
		return 0, 0, nil, err
	}
	ht, err := readToken(r)
	if err != nil {
		return 0, 0, nil, err
	}
	mt, err := readToken(r)
	if err != nil {
		return 0, 0, nil, err
	}
	w, _ = strconv.Atoi(wt)
	h, _ = strconv.Atoi(ht)
	maxval, _ := strconv.Atoi(mt)
	if w <= 0 || h <= 0 || maxval != 255 {
		return 0, 0, nil, fmt.Errorf("bad pgm header %dx%d max %d", w, h, maxval)
	}
	pix = make([]byte, w*h)
	if _, err := io.ReadFull(r, pix); err != nil {
		return 0, 0, nil, err
	}
	return w, h, pix, nil
}

// readToken reads one whitespace-delimited header token, skipping # comments.
func readToken(r *bufio.Reader) (string, error) {
	var b []byte
	for {
		c, err := r.ReadByte()
		if err != nil {
			return "", err
		}
		if c == '#' {
			for c != '\n' {
				if c, err = r.ReadByte(); err != nil {
					return "", err
				}
			}
			continue
		}
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			if len(b) > 0 {
				return string(b), nil
			}
			continue
		}
		b = append(b, c)
	}
}
