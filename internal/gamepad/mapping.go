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
	Target string // "a", "b", "x", "y", "lb", "rb", "back", "start", "guide", "ls", "rs"
}

// DeviceMapping holds the complete mapping for a specific device type.
type DeviceMapping struct {
	Name    string
	Axes    []AxisMapping
	Buttons []ButtonMapping
	HasHat  bool

	// HID-specific fields (used when the device is accessed via Raw Input HID,
	// not XInput). If HIDAxes is nil, a generic default axis assignment is used.
	// Keys are HID usage codes (e.g. 0x30 = X, 0x31 = Y, 0x32 = Z, etc.).
	// Values are semantic target names matching the XInput conventions.
	HIDAxes map[uint16]string // HID usage → "left_x"/"left_y"/"right_x"/"right_y"/"lt"/"rt"

	// HIDButtons maps 1-based HID button usage numbers to button target names.
	// If nil, buttons are assigned in order: a, b, x, y, lb, rb, back, start, guide, ls, rs.
	HIDButtons map[uint16]string // 1-based button usage → "a"/"b"/"x"/"y"/"lb"/"rb"/etc.
}

// normalizeAxis converts a raw axis value (-32768..32767) to -1.0..1.0.
func normalizeAxis(raw int16) float64 {
	v := float64(raw) / math.MaxInt16
	if v < -1.0 {
		v = -1.0
	}
	return v
}

// normalizeTrigger converts a raw trigger value to 0.0..1.0.
func normalizeTrigger(raw int16, rawMin, rawMax int16) float64 {
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

// applyDeadzone returns 0 if the value is within the deadzone threshold.
func applyDeadzone(v float64, threshold float64) float64 {
	if math.Abs(v) < threshold {
		return 0
	}
	return v
}

// deviceKey is the map key for vendor/product ID pairs.
type deviceKey struct {
	VendorID  uint16
	ProductID uint16
}

// GetMapping returns the appropriate mapping for a device identified by vendor/product ID.
// Falls back to generic mapping if no specific mapping is found.
func GetMapping(vendorID, productID uint16) *DeviceMapping {
	key := deviceKey{VendorID: vendorID, ProductID: productID}
	if m, ok := knownDevices[key]; ok {
		return m
	}
	return xboxMapping
}
