package robot

import "fmt"

// Robot is the terminal-command path: telemetry polling, room enumeration and
// the base-station actions with no funcRequest equivalent. Everything else goes
// through robotapi.
type Robot struct {
	c   *Client
	cmd Commands
}

func New(c *Client, cmd Commands) *Robot {
	return &Robot{c: c, cmd: cmd}
}

func (r *Robot) exec(cmd string) error {
	if cmd == "" {
		return fmt.Errorf("command not configured")
	}
	_, err := r.c.Exec(cmd)
	return err
}

func (r *Robot) Explore() error         { return r.exec(r.cmd.Explore) }
func (r *Robot) WaterBaseCharge() error { return r.exec(r.cmd.WaterBaseCharge) }
func (r *Robot) MarkWaterBase() error   { return r.exec(r.cmd.MarkWaterBase) }
func (r *Robot) FillHumidifier() error  { return r.exec(r.cmd.FillHumidifier) }

func (r *Robot) Raw(cmd string) (string, error) {
	return r.c.Exec(cmd)
}

// Poll reads current telemetry over the terminal. Every probe is best-effort;
// missing fields leave zero/unknown values rather than failing the whole poll.
// Battery, fan level and the human-readable run state come from the brain-status
// dump; the numeric WorkingStatus and cloud-connection flag come from the iot dump.
func (r *Robot) Poll() (State, error) {
	st := State{Battery: -1, HAState: "idle"}

	if out, err := r.c.Exec(r.cmd.ProbeBrainStatus); err == nil {
		if b, ok := parseBrainStatus(out); ok {
			if b.BatteryPercent != nil {
				st.Battery = *b.BatteryPercent
			}
			if b.FanLevel != nil {
				st.FanSpeed = FanLevelToHA(*b.FanLevel)
			}
			st.RunState = b.CurrRunningState
			st.Charging = b.IsCharging
			st.Docked = b.IsCharging || b.IsOnChargeBase || b.IsOnBase
			if b.IsFanFault {
				st.ErrorCode = 1
			}
		}
	}

	if out, err := r.c.Exec(r.cmd.ProbeIot); err == nil {
		if s, ok := parseIotState(out); ok {
			st.WorkingStatus = s.Srv.State
			st.CloudConnected = s.CloudConn.IsConnected
		}
	}

	st.HAState = r.resolveHAState(st)
	return st, nil
}

func (r *Robot) resolveHAState(st State) string {
	ha := "idle"
	if st.WorkingStatus != 0 {
		ha = workingStatusToHA(st.WorkingStatus)
	} else if st.RunState != "" {
		ha = runStateToHA(st.RunState)
	}
	if (st.Charging || st.Docked) && ha == "idle" {
		ha = "docked"
	}
	return ha
}
