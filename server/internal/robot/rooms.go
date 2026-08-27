package robot

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type Room struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type labelsFile struct {
	Data []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"data"`
	UUID string `json:"uuid"`
}

func (r *Robot) Rooms() ([]Room, error) {
	data, err := os.ReadFile(r.cmd.LabelsPath)
	if err != nil {
		return r.roomsFromDB()
	}
	var lf labelsFile
	if err := json.Unmarshal(data, &lf); err != nil {
		return nil, fmt.Errorf("parse labels: %w", err)
	}
	out := make([]Room, 0, len(lf.Data))
	for _, d := range lf.Data {
		if !strings.HasPrefix(d.ID, "ROOM_") {
			continue
		}
		name := d.Name
		if name == "" {
			name = d.ID
		}
		out = append(out, Room{ID: d.ID, Name: name})
	}
	if len(out) == 0 {
		return r.roomsFromDB()
	}
	return out, nil
}

func (r *Robot) roomsFromDB() ([]Room, error) {
	out, err := r.c.Exec(r.cmd.ProbeMap)
	if err != nil {
		return nil, err
	}
	var m struct {
		RoomsSet []string `json:"rooms_set"`
	}
	if !decodeJSON(out, &m) {
		return nil, fmt.Errorf("parse print_map")
	}
	rooms := make([]Room, 0, len(m.RoomsSet))
	for _, id := range m.RoomsSet {
		rooms = append(rooms, Room{ID: id, Name: id})
	}
	return rooms, nil
}
