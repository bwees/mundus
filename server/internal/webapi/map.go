package webapi

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-fuego/fuego"

	"github.com/bwees/mundus/server/internal/mapgeo"
	"github.com/bwees/mundus/server/internal/robotapi"
)

func registerMap(api *fuego.Server, d Deps) {
	fuego.Get(api, "/map", func(ctx fuego.ContextNoBody) (MapDTO, error) {
		return buildMap(d)
	}, fuego.OptionOperationID("getMap"))

	fuego.Put(api, "/map/room", func(ctx fuego.ContextWithBody[RenameRoomInput]) (OK, error) {
		body, err := ctx.Body()
		if err != nil {
			return OK{}, err
		}
		l, err := robotapi.ReadLabels(d.MapDir)
		if err != nil {
			return OK{}, err
		}
		found := false
		for i := range l.Data {
			if l.Data[i].ID == body.ID {
				l.Data[i].Name = body.Name
				found = true
			}
		}
		if !found {
			return OK{}, fmt.Errorf("room %q not found", body.ID)
		}
		return applyLabels(d, l)
	}, fuego.OptionOperationID("renameRoom"))

	fuego.Post(api, "/map/merge", func(ctx fuego.ContextWithBody[MergeRoomsInput]) (OK, error) {
		body, err := ctx.Body()
		if err != nil {
			return OK{}, err
		}
		return mergeRooms(d, body)
	}, fuego.OptionOperationID("mergeRooms"))

	fuego.Post(api, "/map/split", func(ctx fuego.ContextWithBody[SplitRoomInput]) (OK, error) {
		body, err := ctx.Body()
		if err != nil {
			return OK{}, err
		}
		return splitRoom(d, body)
	}, fuego.OptionOperationID("splitRoom"))

	fuego.Post(api, "/map/zone", func(ctx fuego.ContextWithBody[AddZoneInput]) (OK, error) {
		body, err := ctx.Body()
		if err != nil {
			return OK{}, err
		}
		if len(body.Geometry) < 6 {
			return OK{}, fmt.Errorf("zone needs at least 3 points")
		}
		m, err := robotapi.ReadMarkers(d.MapDir)
		if err != nil {
			return OK{}, err
		}
		id, err := robotapi.NextMarkerID(m, body.Kind)
		if err != nil {
			return OK{}, err
		}
		name := body.Name
		if name == "" {
			name = defaultZoneName(body.Kind)
		}
		m.Data = append(m.Data, robotapi.Marker{
			Geometry: body.Geometry, ID: id, Name: name, Type: "polygon",
			RotateAngle: []byte("null"),
		})
		return applyMarkers(d, m)
	}, fuego.OptionOperationID("addZone"))

	fuego.Post(api, "/map/zone/update", func(ctx fuego.ContextWithBody[UpdateZoneInput]) (OK, error) {
		body, err := ctx.Body()
		if err != nil {
			return OK{}, err
		}
		m, err := robotapi.ReadMarkers(d.MapDir)
		if err != nil {
			return OK{}, err
		}
		found := false
		for i := range m.Data {
			if m.Data[i].ID != body.ID {
				continue
			}
			found = true
			if len(body.Geometry) >= 6 {
				m.Data[i].Geometry = body.Geometry
			}
			if body.Name != "" {
				m.Data[i].Name = body.Name
			}
		}
		if !found {
			return OK{}, fmt.Errorf("zone %q not found", body.ID)
		}
		return applyMarkers(d, m)
	}, fuego.OptionOperationID("updateZone"))

	fuego.Post(api, "/map/zone/delete", func(ctx fuego.ContextWithBody[ZoneIDInput]) (OK, error) {
		body, err := ctx.Body()
		if err != nil {
			return OK{}, err
		}
		m, err := robotapi.ReadMarkers(d.MapDir)
		if err != nil {
			return OK{}, err
		}
		out := m.Data[:0]
		found := false
		for _, mk := range m.Data {
			if mk.ID == body.ID {
				found = true
				continue
			}
			out = append(out, mk)
		}
		if !found {
			return OK{}, fmt.Errorf("zone %q not found", body.ID)
		}
		m.Data = out
		return applyMarkers(d, m)
	}, fuego.OptionOperationID("deleteZone"))
}

func defaultZoneName(kind string) string {
	switch kind {
	case "carpet":
		return "Carpet"
	case "no_mop":
		return "No-Mop Zone"
	case "no_go":
		return "No-Go Zone"
	}
	return "Zone"
}

func applyMarkers(d Deps, m robotapi.Markers) (OK, error) {
	id, err := robotapi.ReadMapID(d.MapDir)
	if err != nil {
		return OK{}, err
	}
	if err := robotapi.BackupMarkers(d.MapDir); err != nil {
		return OK{}, fmt.Errorf("backup: %w", err)
	}
	if err := d.API.UpdateMapMarkers(m, id); err != nil {
		return OK{}, err
	}
	return OK{OK: true}, nil
}

func transformOf(d Deps) (mapgeo.Transform, error) {
	m, err := robotapi.ReadMapMeta(d.MapDir)
	if err != nil {
		return mapgeo.Transform{}, err
	}
	return mapgeo.Transform{
		Resolution: m.Resolution, OriginX: m.OriginX, OriginY: m.OriginY,
		Width: m.Width, Height: m.Height,
	}, nil
}

func buildMap(d Deps) (MapDTO, error) {
	l, err := robotapi.ReadLabels(d.MapDir)
	if err != nil {
		return MapDTO{}, err
	}
	m, err := robotapi.ReadMapMeta(d.MapDir)
	if err != nil {
		return MapDTO{}, err
	}
	png, err := robotapi.ReadMapPNG(d.MapDir)
	if err != nil {
		return MapDTO{}, err
	}
	rooms := make([]RoomGeomDTO, len(l.Data))
	for i, r := range l.Data {
		rooms[i] = RoomGeomDTO{ID: r.ID, Name: r.Name, ColorType: r.ColorType, Graph: r.Graph, Geometry: r.Geometry}
	}
	var zones []ZoneDTO
	if mk, err := robotapi.ReadMarkers(d.MapDir); err == nil {
		for _, z := range mk.Data {
			zones = append(zones, ZoneDTO{ID: z.ID, Name: z.Name, Kind: robotapi.MarkerKind(z.ID), Geometry: z.Geometry})
		}
	}
	return MapDTO{
		UUID: l.UUID, Resolution: m.Resolution, OriginX: m.OriginX, OriginY: m.OriginY,
		Width: m.Width, Height: m.Height,
		ImagePNG: "data:image/png;base64," + base64.StdEncoding.EncodeToString(png),
		Rooms:    rooms,
		Zones:    zones,
	}, nil
}

// applyLabels recomputes adjacency from the room polygons, snapshots the current
// map, and submits the new label set through funcID 1006.
func applyLabels(d Deps, l robotapi.Labels) (OK, error) {
	t, err := transformOf(d)
	if err != nil {
		return OK{}, err
	}
	masks := make([]*mapgeo.Mask, len(l.Data))
	for i, r := range l.Data {
		masks[i] = mapgeo.Rasterize(r.Geometry, t)
	}
	adj := mapgeo.BuildAdjacency(masks)
	for i := range l.Data {
		if adj[i] == nil {
			l.Data[i].Graph = []int{}
		} else {
			l.Data[i].Graph = adj[i]
		}
	}
	if err := robotapi.BackupLabels(d.MapDir); err != nil {
		return OK{}, fmt.Errorf("backup: %w", err)
	}
	if err := d.API.UpdateMapLabels(l); err != nil {
		return OK{}, err
	}
	if d.MQTT != nil {
		d.MQTT.RoomsChanged()
	}
	return OK{OK: true}, nil
}

func mergeRooms(d Deps, in MergeRoomsInput) (OK, error) {
	if len(in.IDs) < 2 {
		return OK{}, fmt.Errorf("merge needs at least two rooms")
	}
	l, err := robotapi.ReadLabels(d.MapDir)
	if err != nil {
		return OK{}, err
	}
	t, err := transformOf(d)
	if err != nil {
		return OK{}, err
	}
	sel := map[string]bool{}
	for _, id := range in.IDs {
		sel[id] = true
	}
	var picked []robotapi.RoomLabel
	var rest []robotapi.RoomLabel
	for _, r := range l.Data {
		if sel[r.ID] {
			picked = append(picked, r)
		} else {
			rest = append(rest, r)
		}
	}
	if len(picked) != len(in.IDs) {
		return OK{}, fmt.Errorf("some rooms not found")
	}
	masks := make([]*mapgeo.Mask, len(picked))
	for i, r := range picked {
		masks[i] = mapgeo.Rasterize(r.Geometry, t)
	}
	geom := mapgeo.Contour(mapgeo.Union(masks...), t)
	if len(geom) < 6 {
		return OK{}, fmt.Errorf("merge produced empty geometry")
	}
	survivor := picked[0]
	name := in.Name
	if name == "" {
		name = survivor.Name
	}
	merged := robotapi.RoomLabel{
		ColorType: survivor.ColorType, Geometry: geom, Graph: []int{},
		GroundMaterial: survivor.GroundMaterial, ID: survivor.ID, Name: name, Type: "polygon",
	}
	l.Data = append(rest, merged)
	return applyLabels(d, l)
}

func splitRoom(d Deps, in SplitRoomInput) (OK, error) {
	l, err := robotapi.ReadLabels(d.MapDir)
	if err != nil {
		return OK{}, err
	}
	t, err := transformOf(d)
	if err != nil {
		return OK{}, err
	}
	var target *robotapi.RoomLabel
	var rest []robotapi.RoomLabel
	for i := range l.Data {
		if l.Data[i].ID == in.ID {
			target = &l.Data[i]
		} else {
			rest = append(rest, l.Data[i])
		}
	}
	if target == nil {
		return OK{}, fmt.Errorf("room %q not found", in.ID)
	}
	left, right := mapgeo.SplitByLine(mapgeo.Rasterize(target.Geometry, t), in.Line[0], in.Line[1], in.Line[2], in.Line[3], t)
	lg := mapgeo.Contour(left, t)
	rg := mapgeo.Contour(right, t)
	if len(lg) < 6 || len(rg) < 6 {
		return OK{}, fmt.Errorf("split line does not divide the room into two parts")
	}
	keep := robotapi.RoomLabel{
		ColorType: target.ColorType, Geometry: lg, Graph: []int{},
		GroundMaterial: target.GroundMaterial, ID: target.ID, Name: target.Name, Type: "polygon",
	}
	newName := in.NewName
	if newName == "" {
		newName = target.Name + " 2"
	}
	created := robotapi.RoomLabel{
		ColorType: nextColor(target.ColorType), Geometry: rg, Graph: []int{},
		GroundMaterial: nil, ID: nextRoomID(l.Data), Name: newName, Type: "polygon",
	}
	l.Data = append(rest, keep, created)
	return applyLabels(d, l)
}

func nextRoomID(rooms []robotapi.RoomLabel) string {
	max := 0
	for _, r := range rooms {
		if n, ok := roomNum(r.ID); ok && n > max {
			max = n
		}
	}
	return fmt.Sprintf("ROOM_%03d", max+1)
}

func roomNum(id string) (int, bool) {
	s := strings.TrimPrefix(id, "ROOM_")
	if s == id {
		return 0, false
	}
	n, err := strconv.Atoi(s)
	return n, err == nil
}

// nextColor picks a palette index (0-4) different from the source room's.
func nextColor(source int) int {
	return (source + 1) % 5
}
