// Package robotapi is a typed control layer over control_center's local
// funcRequest API (funcapi). Sending high-level funcIDs lets control_center run
// its full native choreography (cold-dock wake, exit_base, relocate, clean,
// post-work servicing, dock) — so we get identical behavior without
// reimplementing the lowlevel stack. funcID + param shapes are from
// ../switchbot-research/docs/replacement-re/ (command-surface.md,
// generated/funcid-params.md, generated/captured-samples.md).
package robotapi

import (
	"fmt"

	"github.com/bwees/mundus/server/internal/funcapi"
)

const (
	fnStartTask       = 1001 // params {"0":action, "1":detail}
	fnTaskControl     = 1009 // pause/resume/stop
	fnFindDevice      = 1019 // locate
	fnStartRecharge   = 1022 // dock
	fnSelfClean       = 1039 // wash/dry/stop-dry/dust
	fnUpdateCleanMode = 1043 // persistent clean mode
	fnUpdateLabels    = 1006 // room edit (rename/split/merge)
	fnUpdateMarkers   = 1007 // no-go / no-mop / carpet
)

// Self-clean actions (funcID 1039 param "0").
const (
	SelfCleanMopWash     = 1
	SelfCleanStartDrying = 2
	SelfCleanStopDrying  = 3
	SelfCleanDustCollect = 4
)

// Type is one of sweep | mop | sweep_mop | first_sweep_then_mop.
type CleanMode struct {
	Type       string `json:"type"`
	FanLevel   int    `json:"fan_level"`
	WaterLevel int    `json:"water_level"`
	Times      int    `json:"times"`
}

func DefaultCleanMode() CleanMode {
	return CleanMode{Type: "sweep", FanLevel: 2, WaterLevel: 1, Times: 1}
}

type API struct {
	local *funcapi.Local
}

func New(local *funcapi.Local) *API { return &API{local: local} }

func (a *API) call(funcID int, params map[string]any) error {
	_, err := a.local.Func(funcID, params)
	return err
}

func (a *API) CleanAll(mode CleanMode) error {
	return a.call(fnStartTask, map[string]any{
		"0": "clean_all",
		"1": map[string]any{"force_order": false, "mode": mode},
	})
}

// CleanRooms starts a per-room clean. force_order:true matches the vendor app.
func (a *API) CleanRooms(ids []string, mode CleanMode) error {
	if len(ids) == 0 {
		return fmt.Errorf("no rooms")
	}
	rooms := make([]map[string]any, len(ids))
	for i, id := range ids {
		rooms[i] = map[string]any{"room_id": id, "mode": mode}
	}
	return a.call(fnStartTask, map[string]any{
		"0": "clean_rooms",
		"1": map[string]any{"force_order": true, "mode": mode, "rooms": rooms},
	})
}

// CleanAreas starts a zone clean over polygons of [x,y] world-metre points.
func (a *API) CleanAreas(polys [][][2]float64, mode CleanMode) error {
	if len(polys) == 0 {
		return fmt.Errorf("no areas")
	}
	areas := make([]map[string]any, len(polys))
	for i, p := range polys {
		areas[i] = map[string]any{"polygon": p, "mode": mode}
	}
	return a.call(fnStartTask, map[string]any{
		"0": "clean_areas",
		"1": map[string]any{"force_order": false, "mode": mode, "areas": areas},
	})
}

func (a *API) CleanSpot(mode CleanMode) error {
	return a.call(fnStartTask, map[string]any{
		"0": "clean_here",
		"1": map[string]any{"mode": mode},
	})
}

func (a *API) Pause() error  { return a.call(fnTaskControl, map[string]any{"0": "pause"}) }
func (a *API) Resume() error { return a.call(fnTaskControl, map[string]any{"0": "resume"}) }
func (a *API) Stop() error   { return a.call(fnTaskControl, map[string]any{"0": "stop"}) }

func (a *API) Dock() error   { return a.call(fnStartRecharge, map[string]any{}) }
func (a *API) Locate() error { return a.call(fnFindDevice, map[string]any{}) }

// SetCleanMode persists the clean mode (funcID 1043; descriptor at key "0").
func (a *API) SetCleanMode(mode CleanMode) error {
	return a.call(fnUpdateCleanMode, map[string]any{"0": mode})
}

func (a *API) SelfClean(action int) error {
	return a.call(fnSelfClean, map[string]any{"0": action})
}
