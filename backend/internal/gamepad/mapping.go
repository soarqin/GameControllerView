package gamepad

import "math"

// AxisMapping defines how a raw axis index maps to a gamepad field.
type AxisMapping struct {
	Index     int32
	Target    string // "left_x", "left_y", "right_x", "right_y", "lt", "rt"
	IsTrigger bool
	Invert    bool
	// For triggers: raw range. Some devices use -32768..32767, others 0..32767.
	RawMin int16
	RawMax int16
}

// ButtonMapping defines how a raw button index maps to a gamepad button.
type ButtonMapping struct {
	Index  int32
	Target string // "a", "b", "x", "y", "lb", "rb", "select", "start", "home", "l3", "r3"
}

// DeviceMapping holds the complete mapping for a specific device type.
type DeviceMapping struct {
	Name    string
	Axes    []AxisMapping
	Buttons []ButtonMapping
	HasHat  bool
}

// NormalizeAxis converts a raw axis value (-32768..32767) to -1.0..1.0.
func NormalizeAxis(raw int16) float64 {
	v := float64(raw) / math.MaxInt16
	if v < -1.0 {
		v = -1.0
	}
	return v
}

// NormalizeTrigger converts a raw trigger value to 0.0..1.0.
func NormalizeTrigger(raw int16, rawMin, rawMax int16) float64 {
	if rawMax == rawMin {
		return 0
	}
	v := float64(raw-rawMin) / float64(rawMax-rawMin)
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	return v
}

// ApplyDeadzone returns 0 if the value is within the deadzone threshold.
func ApplyDeadzone(v float64, threshold float64) float64 {
	if math.Abs(v) < threshold {
		return 0
	}
	return v
}

// Built-in mappings for common controllers.

var xboxMapping = &DeviceMapping{
	Name: "xbox",
	Axes: []AxisMapping{
		{Index: 0, Target: "left_x"},
		{Index: 1, Target: "left_y", Invert: true},
		{Index: 2, Target: "right_x"},
		{Index: 3, Target: "right_y", Invert: true},
		{Index: 4, Target: "lt", IsTrigger: true, RawMin: -32768, RawMax: 32767},
		{Index: 5, Target: "rt", IsTrigger: true, RawMin: -32768, RawMax: 32767},
	},
	Buttons: []ButtonMapping{
		{Index: 0, Target: "a"},
		{Index: 1, Target: "b"},
		{Index: 2, Target: "x"},
		{Index: 3, Target: "y"},
		{Index: 4, Target: "lb"},
		{Index: 5, Target: "rb"},
		{Index: 6, Target: "select"},
		{Index: 7, Target: "start"},
		{Index: 8, Target: "l3"},
		{Index: 9, Target: "r3"},
		{Index: 10, Target: "home"},
	},
	HasHat: true,
}

var playstationMapping = &DeviceMapping{
	Name: "playstation",
	Axes: []AxisMapping{
		{Index: 0, Target: "left_x"},
		{Index: 1, Target: "left_y", Invert: true},
		{Index: 2, Target: "right_x"},
		{Index: 3, Target: "right_y", Invert: true},
		{Index: 4, Target: "lt", IsTrigger: true, RawMin: -32768, RawMax: 32767},
		{Index: 5, Target: "rt", IsTrigger: true, RawMin: -32768, RawMax: 32767},
	},
	Buttons: []ButtonMapping{
		{Index: 0, Target: "a"},      // Cross (×)
		{Index: 1, Target: "b"},      // Circle (○)
		{Index: 2, Target: "x"},      // Square (□)
		{Index: 3, Target: "y"},      // Triangle (△)
		{Index: 4, Target: "select"}, // Share / Create
		{Index: 5, Target: "home"},   // PS button
		{Index: 6, Target: "start"},  // Options
		{Index: 7, Target: "l3"},
		{Index: 8, Target: "r3"},
		{Index: 9, Target: "lb"},     // L1
		{Index: 10, Target: "rb"},    // R1
	},
	HasHat: true,
}

var switchProMapping = &DeviceMapping{
	Name: "switch_pro",
	Axes: []AxisMapping{
		{Index: 0, Target: "left_x"},
		{Index: 1, Target: "left_y", Invert: true},
		{Index: 2, Target: "right_x"},
		{Index: 3, Target: "right_y", Invert: true},
	},
	Buttons: []ButtonMapping{
		{Index: 0, Target: "a"},
		{Index: 1, Target: "b"},
		{Index: 2, Target: "x"},
		{Index: 3, Target: "y"},
		{Index: 4, Target: "lb"},
		{Index: 5, Target: "rb"},
		{Index: 6, Target: "select"},
		{Index: 7, Target: "start"},
		{Index: 8, Target: "l3"},
		{Index: 9, Target: "r3"},
		{Index: 10, Target: "home"},
	},
	HasHat: true,
}

var genericMapping = &DeviceMapping{
	Name: "generic",
	Axes: []AxisMapping{
		{Index: 0, Target: "left_x"},
		{Index: 1, Target: "left_y", Invert: true},
		{Index: 2, Target: "right_x"},
		{Index: 3, Target: "right_y", Invert: true},
		{Index: 4, Target: "lt", IsTrigger: true, RawMin: -32768, RawMax: 32767},
		{Index: 5, Target: "rt", IsTrigger: true, RawMin: -32768, RawMax: 32767},
	},
	Buttons: []ButtonMapping{
		{Index: 0, Target: "a"},
		{Index: 1, Target: "b"},
		{Index: 2, Target: "x"},
		{Index: 3, Target: "y"},
		{Index: 4, Target: "lb"},
		{Index: 5, Target: "rb"},
		{Index: 6, Target: "select"},
		{Index: 7, Target: "start"},
		{Index: 8, Target: "l3"},
		{Index: 9, Target: "r3"},
		{Index: 10, Target: "home"},
	},
	HasHat: true,
}

// Known vendor/product IDs.
type deviceKey struct {
	VendorID  uint16
	ProductID uint16
}

var knownDevices = map[deviceKey]*DeviceMapping{
	// Microsoft Xbox controllers
	{0x045E, 0x028E}: xboxMapping, // Xbox 360
	{0x045E, 0x02FF}: xboxMapping, // Xbox One
	{0x045E, 0x0B12}: xboxMapping, // Xbox Series X|S
	{0x045E, 0x0B13}: xboxMapping, // Xbox Series X|S (wireless)
	// Sony PlayStation controllers
	{0x054C, 0x0CE6}: playstationMapping, // DualSense
	{0x054C, 0x09CC}: playstationMapping, // DualShock 4 v2
	{0x054C, 0x05C4}: playstationMapping, // DualShock 4 v1
	// Nintendo Switch Pro Controller
	{0x057E, 0x2009}: switchProMapping,
}

// GetMapping returns the appropriate mapping for a device identified by vendor/product ID.
// Falls back to generic mapping if no specific mapping is found.
func GetMapping(vendorID, productID uint16) *DeviceMapping {
	key := deviceKey{VendorID: vendorID, ProductID: productID}
	if m, ok := knownDevices[key]; ok {
		return m
	}
	return genericMapping
}
