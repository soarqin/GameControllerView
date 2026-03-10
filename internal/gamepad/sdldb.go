package gamepad

// sdldb.go — SDL GameControllerDB parser
//
// Reads gamecontrollerdb.txt (SDL2 / SDL3 format) and builds a lookup table
// keyed by (vendorID, productID). Entries are platform-filtered: only
// "platform:Windows" lines are kept on Windows builds, but this file is
// platform-agnostic (the caller decides which platform string to accept).
//
// SDL GUID binary layout (16 bytes, stored as 32 hex chars):
//   bytes 0-1  : bus type  (little-endian uint16, e.g. 0x0003 = USB)
//   bytes 2-3  : CRC16 of device name (little-endian, can be 0)
//   bytes 4-5  : Vendor ID (little-endian uint16)
//   bytes 6-7  : 0x0000
//   bytes 8-9  : Product ID (little-endian uint16)
//   bytes 10-11: 0x0000
//   bytes 12-13: version
//   bytes 14-15: driver signature / driver data
//
// SDL mapping field syntax (comma-separated key:value pairs):
//   Button sources  b<n>            — 0-based button index
//   Axis sources    a<n>            — 0-based axis index (by report order)
//                   +a<n>           — positive half of axis
//                   -a<n>           — negative half of axis
//                   a<n>~           — axis inverted
//                   +a<n>~          — positive half, inverted (rare)
//   Hat source      h<n>.<mask>     — hat index n, direction mask (1/2/4/8)
//   Half-axis btn   +<target>:b<n>  — positive half of axis from button
//                   -<target>:b<n>  — negative half of axis from button
//
// Supported SDL target names → our semantic names:
//   a, b, x, y, back, start, guide, leftshoulder/lb, rightshoulder/rb,
//   lefttrigger/lt, righttrigger/rt,
//   leftstick/ls, rightstick/rs,
//   leftx, lefty, rightx, righty,
//   dpup, dpdown, dpleft, dpright,
//   touchpad, misc1, paddle1..4 (ignored beyond touchpad for now)

import (
	"bufio"
	"encoding/hex"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
)

// SDLAxisBinding describes how an SDL axis index binds to a gamepad semantic target.
type SDLAxisBinding struct {
	AxisIndex int    // 0-based axis index (by report order in value caps)
	Target    string // semantic name: "left_x", "left_y", "right_x", "right_y", "lt", "rt"
	Invert    bool   // axis value is inverted (~)
	HalfPos   bool   // only positive half maps to target (+ prefix)
	HalfNeg   bool   // only negative half maps to target (- prefix)
}

// SDLButtonBinding describes how a button index maps to a semantic target.
type SDLButtonBinding struct {
	ButtonIndex int    // 0-based button index
	Target      string // "a", "b", "x", "y", "lb", "rb", "lt", "rt", "back", "start", "guide", "ls", "rs", "touchpad"
}

// SDLHatBinding describes a hat-switch direction mapped to a dpad direction.
type SDLHatBinding struct {
	HatIndex int    // hat index (almost always 0)
	DirMask  int    // SDL dir mask: 1=up, 2=right, 4=down, 8=left
	Target   string // "dpup", "dpdown", "dpleft", "dpright"
}

// SDLAxisHalfButton describes an axis half-component driven by a button (e.g. N64 C-stick).
type SDLAxisHalfButton struct {
	Target    string // "right_x", "right_y", etc.
	Sign      int    // +1 or -1
	ButtonIdx int    // 0-based button index
}

// SDLMapping is the parsed representation of one gamecontrollerdb.txt entry.
type SDLMapping struct {
	GUID      string // 32-char hex GUID as in the file
	Name      string
	VendorID  uint16
	ProductID uint16

	// Axis bindings (from "leftx:a0", "lefttrigger:+a2", "lefty:a1~", etc.)
	Axes []SDLAxisBinding

	// Button bindings (from "a:b2", "dpdown:b11", etc.)
	Buttons []SDLButtonBinding

	// Hat bindings (from "dpdown:h0.4", etc.)
	Hats []SDLHatBinding

	// Half-axis from button (from "+rightx:b9", "-righty:b8", etc.)
	AxisHalfButtons []SDLAxisHalfButton
}

// sdlTargetToSemantic converts SDL target field names to our internal semantic names.
// Returns ("", false) for targets we do not support.
func sdlTargetToSemantic(sdlTarget string) (string, bool) {
	switch sdlTarget {
	case "a":
		return "a", true
	case "b":
		return "b", true
	case "x":
		return "x", true
	case "y":
		return "y", true
	case "back":
		return "back", true
	case "start":
		return "start", true
	case "guide":
		return "guide", true
	case "leftshoulder":
		return "lb", true
	case "rightshoulder":
		return "rb", true
	case "lefttrigger":
		return "lt", true
	case "righttrigger":
		return "rt", true
	case "leftstick":
		return "ls", true
	case "rightstick":
		return "rs", true
	case "leftx":
		return "left_x", true
	case "lefty":
		return "left_y", true
	case "rightx":
		return "right_x", true
	case "righty":
		return "right_y", true
	case "dpup":
		return "dpup", true
	case "dpdown":
		return "dpdown", true
	case "dpleft":
		return "dpleft", true
	case "dpright":
		return "dpright", true
	case "touchpad":
		return "touchpad", true
	case "misc1":
		return "capture", true
	// paddle1-4: map to guide/touchpad/capture for now (rarely used)
	case "paddle1", "paddle2", "paddle3", "paddle4":
		return "", false // unsupported
	default:
		return "", false
	}
}

// parseSDLGUID extracts VendorID and ProductID from a 32-hex-char SDL GUID string.
// Returns (0, 0, false) if the GUID does not have the standard VID/PID layout.
func parseSDLGUID(guidStr string) (vendorID, productID uint16, ok bool) {
	if len(guidStr) != 32 {
		return 0, 0, false
	}
	raw, err := hex.DecodeString(guidStr)
	if err != nil || len(raw) != 16 {
		return 0, 0, false
	}
	// bus is stored as little-endian uint16 at bytes 0-1
	bus := uint16(raw[0]) | uint16(raw[1])<<8
	// Standard VID/PID layout: bus < 0x20 (printable char) means hardware bus,
	// and bytes 6-7 must be 0x0000, bytes 10-11 must be 0x0000.
	if bus >= 0x20 {
		return 0, 0, false
	}
	// bytes 6-7 must be 0
	if raw[6] != 0 || raw[7] != 0 {
		return 0, 0, false
	}
	vid := uint16(raw[4]) | uint16(raw[5])<<8
	pid := uint16(raw[8]) | uint16(raw[9])<<8
	return vid, pid, true
}

// parseMappingFields parses the comma-separated field:value pairs from an SDL mapping string.
// mappingStr is the raw line from the file (including GUID and name at the front).
// Returns the parsed SDLMapping or nil on error.
func parseMappingFields(line string) *SDLMapping {
	// Format: <guid>,<name>,<field:value>,...,platform:<platform>,
	parts := strings.Split(line, ",")
	if len(parts) < 3 {
		return nil
	}

	guid := strings.TrimSpace(parts[0])
	name := strings.TrimSpace(parts[1])

	vid, pid, ok := parseSDLGUID(guid)
	if !ok {
		return nil
	}

	m := &SDLMapping{
		GUID:      guid,
		Name:      name,
		VendorID:  vid,
		ProductID: pid,
	}

	// Parse field:value pairs starting at parts[2].
	for i := 2; i < len(parts); i++ {
		field := strings.TrimSpace(parts[i])
		if field == "" {
			continue
		}

		// Skip meta fields.
		if strings.HasPrefix(field, "platform:") ||
			strings.HasPrefix(field, "crc:") ||
			strings.HasPrefix(field, "type:") ||
			strings.HasPrefix(field, "face:") ||
			strings.HasPrefix(field, "hint:") ||
			strings.HasPrefix(field, "sdk>=:") ||
			strings.HasPrefix(field, "sdk<=:") {
			continue
		}

		// Check for half-axis-from-button prefix: "+rightx:b9" or "-righty:b8"
		axisSign := 0
		remaining := field
		if strings.HasPrefix(field, "+") {
			axisSign = +1
			remaining = field[1:]
		} else if strings.HasPrefix(field, "-") {
			axisSign = -1
			remaining = field[1:]
		}

		colonIdx := strings.IndexByte(remaining, ':')
		if colonIdx < 0 {
			continue
		}
		sdlTarget := remaining[:colonIdx]
		src := remaining[colonIdx+1:]

		semantic, supported := sdlTargetToSemantic(sdlTarget)

		// Half-axis-from-button: the target name is an axis target, but the source is a button.
		// Example: "+rightx:b9" — pressing button 9 pushes right_x to +1.
		if axisSign != 0 && strings.HasPrefix(src, "b") {
			btnIdx, err := strconv.Atoi(src[1:])
			if err == nil && supported {
				m.AxisHalfButtons = append(m.AxisHalfButtons, SDLAxisHalfButton{
					Target:    semantic,
					Sign:      axisSign,
					ButtonIdx: btnIdx,
				})
			}
			continue
		}

		if !supported {
			continue
		}

		switch {
		case strings.HasPrefix(src, "b"):
			// Button source
			idx, err := strconv.Atoi(src[1:])
			if err != nil {
				continue
			}
			// Dpad directions mapped from buttons are stored as buttons with the dpad target name.
			// The caller handles converting dpup/dpdown/dpleft/dpright button presses to DpadState.
			m.Buttons = append(m.Buttons, SDLButtonBinding{
				ButtonIndex: idx,
				Target:      semantic,
			})

		case strings.HasPrefix(src, "h"):
			// Hat source: h<n>.<mask>
			dotIdx := strings.IndexByte(src, '.')
			if dotIdx < 0 {
				continue
			}
			hatIdx, err1 := strconv.Atoi(src[1:dotIdx])
			dirMask, err2 := strconv.Atoi(src[dotIdx+1:])
			if err1 != nil || err2 != nil {
				continue
			}
			m.Hats = append(m.Hats, SDLHatBinding{
				HatIndex: hatIdx,
				DirMask:  dirMask,
				Target:   semantic,
			})

		case strings.HasPrefix(src, "+a") || strings.HasPrefix(src, "-a"):
			// Half-axis source: +a<n> or -a<n>, optional ~ suffix.
			isPos := src[0] == '+'
			rest := src[2:] // strip "+a" or "-a"
			invert := strings.HasSuffix(rest, "~")
			if invert {
				rest = rest[:len(rest)-1]
			}
			idx, err := strconv.Atoi(rest)
			if err != nil {
				continue
			}
			binding := SDLAxisBinding{
				AxisIndex: idx,
				Target:    semantic,
				Invert:    invert,
				HalfPos:   isPos,
				HalfNeg:   !isPos,
			}
			m.Axes = append(m.Axes, binding)

		case strings.HasPrefix(src, "a"):
			// Full axis source: a<n> with optional ~ suffix.
			rest := src[1:]
			invert := strings.HasSuffix(rest, "~")
			if invert {
				rest = rest[:len(rest)-1]
			}
			idx, err := strconv.Atoi(rest)
			if err != nil {
				continue
			}
			// Also handle axisSign from the outer prefix (e.g. "+leftstick:-a2" is unusual but possible)
			halfPos := axisSign > 0
			halfNeg := axisSign < 0
			m.Axes = append(m.Axes, SDLAxisBinding{
				AxisIndex: idx,
				Target:    semantic,
				Invert:    invert,
				HalfPos:   halfPos,
				HalfNeg:   halfNeg,
			})
		}
	}

	return m
}

// LoadSDLMappingsFromFile reads a gamecontrollerdb.txt file and returns all
// parsed mappings for the given platform (e.g. "Windows").
// Lines that do not match the platform are skipped.
func LoadSDLMappingsFromFile(path, platform string) (map[deviceKey]*SDLMapping, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return LoadSDLMappingsFromReader(f, platform)
}

// LoadSDLMappingsFromReader reads gamecontrollerdb entries from any io.Reader.
func LoadSDLMappingsFromReader(r io.Reader, platform string) (map[deviceKey]*SDLMapping, error) {
	result := make(map[deviceKey]*SDLMapping)
	wantPlatform := "platform:" + platform + ","

	scanner := bufio.NewScanner(r)
	// Some DBs have very long lines (lots of bindings). Use a larger buffer.
	scanner.Buffer(make([]byte, 64*1024), 64*1024)

	lineNum := 0
	loaded := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Check platform filter.
		if !strings.Contains(line, wantPlatform) {
			continue
		}

		m := parseMappingFields(line)
		if m == nil {
			log.Printf("sdldb: failed to parse line %d: %.80s...", lineNum, line)
			continue
		}
		if m.VendorID == 0 && m.ProductID == 0 {
			// Skip generic wildcard entries (e.g. bus-based catch-all).
			continue
		}

		key := deviceKey{VendorID: m.VendorID, ProductID: m.ProductID}
		result[key] = m
		loaded++
	}
	if err := scanner.Err(); err != nil {
		return result, err
	}
	log.Printf("sdldb: loaded %d mappings for platform %q", loaded, platform)
	return result, nil
}
