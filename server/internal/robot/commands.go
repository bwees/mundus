package robot

// Every value is a cpp-tbox terminal command line sent verbatim to
// control_center on 127.0.0.1:50000. Node names and telemetry shapes were
// verified on a live S20 (W1106000) by walking `tree` and reading the `print`
// dumps; Tags mark how far
// that went: [C] confirmed on device (for probes, output shape observed);
// [A] node exists but args and safety behaviour are unverified.

type Commands struct {
	Explore         string // [C]
	WaterBaseCharge string // [C]
	MarkWaterBase   string // [C]
	FillHumidifier  string // [C]

	LabelsPath string // [C]
	ProbeMap   string // [C] rooms_set id fallback + map uuid
	// Both probes emit JSON that state.go parses: brain status carries
	// battery_percent/fan_level/curr_running_state/is_charging/is_on_base,
	// iot carries srv.state (WorkingStatus) and cloud_conn.is_connected.
	ProbeBrainStatus string // [C]
	ProbeIot         string // [C]
	// ProbeCleanMode returns the persisted clean mode, which is what the app
	// and funcID 1043 both write. [C]
	ProbeCleanMode string
}

func DefaultCommands() Commands {
	return Commands{
		Explore:         "/cc/iot/sim/explore",
		WaterBaseCharge: "/cc/iot/sim/water_base_charge",
		MarkWaterBase:   "/cc/iot/sim/mark_water_base",
		FillHumidifier:  "/cc/iot/sim/fill_hum",

		LabelsPath: "/data/control_center/current_map/labels.json",
		ProbeMap:   "/cc/db/print_map",

		ProbeBrainStatus: "/cc/brain/status/print",
		ProbeIot:         "/cc/iot/print",
		ProbeCleanMode:   "/cc/db/print_clean_mode",
	}
}
