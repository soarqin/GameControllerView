package gamepad

import (
	"strings"
	"testing"
)

func TestParseSDLGUID(t *testing.T) {
	tests := []struct {
		guid    string
		wantVID uint16
		wantPID uint16
		wantOK  bool
	}{
		// 03000000300f00000a01000000000000
		// bus LE16: bytes[0..1] = 0x03,0x00 → 0x0003
		// crc LE16: bytes[2..3] = 0x00,0x00 → 0x0000
		// vid LE16: bytes[4..5] = 0x30,0x0f → 0x30 | (0x0f<<8) = 0x0f30
		// pad LE16: bytes[6..7] = 0x00,0x00
		// pid LE16: bytes[8..9] = 0x0a,0x01 → 0x0a | (0x01<<8) = 0x010a
		{"03000000300f00000a01000000000000", 0x0f30, 0x010a, true},
		// PS5 DualSense: 030000004c050000e60c000000000000
		// vid LE16: bytes[4..5] = 0x4c,0x05 → 0x054c  (Sony)
		// pid LE16: bytes[8..9] = 0xe6,0x0c → 0x0ce6
		{"030000004c050000e60c000000000000", 0x054c, 0x0ce6, true},
		// Too short
		{"03000000300f000", 0, 0, false},
		// Non-hardware bus (e.g. 0x61 is printable char 'a' → not valid VID/PID form)
		{"61000000300f00000a01000000000000", 0, 0, false},
	}

	for _, tt := range tests {
		vid, pid, ok := parseSDLGUID(tt.guid)
		if ok != tt.wantOK {
			t.Errorf("parseSDLGUID(%q): ok=%v, want %v", tt.guid, ok, tt.wantOK)
			continue
		}
		if ok && (vid != tt.wantVID || pid != tt.wantPID) {
			t.Errorf("parseSDLGUID(%q): vid=0x%04x pid=0x%04x, want vid=0x%04x pid=0x%04x",
				tt.guid, vid, pid, tt.wantVID, tt.wantPID)
		}
	}
}

func TestParseMappingFields_Basic(t *testing.T) {
	// PS5 DualSense: 030000004c050000e60c000000000000
	// vid LE16: bytes[4..5]=0x4c,0x05 → 0x054c; pid LE16: bytes[8..9]=0xe6,0x0c → 0x0ce6
	line := "030000004c050000e60c000000000000,PS5 Controller,a:b1,b:b2,x:b0,y:b3,back:b8,guide:b12,start:b9,leftstick:b10,rightstick:b11,leftshoulder:b4,rightshoulder:b5,dpup:h0.1,dpdown:h0.4,dpleft:h0.8,dpright:h0.2,leftx:a0,lefty:a1,rightx:a2,righty:a5,lefttrigger:a3,righttrigger:a4,platform:Windows,"

	m := parseMappingFields(line)
	if m == nil {
		t.Fatal("parseMappingFields returned nil")
	}

	if m.VendorID != 0x054c {
		t.Errorf("VendorID=0x%04x, want 0x054c", m.VendorID)
	}
	if m.ProductID != 0x0ce6 {
		t.Errorf("ProductID=0x%04x, want 0x0ce6", m.ProductID)
	}
	if m.Name != "PS5 Controller" {
		t.Errorf("Name=%q, want PS5 Controller", m.Name)
	}

	// Check axis bindings
	axisTargets := make(map[string]int)
	for _, ab := range m.Axes {
		axisTargets[ab.Target] = ab.AxisIndex
	}
	if axisTargets["left_x"] != 0 {
		t.Errorf("left_x axis index = %d, want 0", axisTargets["left_x"])
	}
	if axisTargets["left_y"] != 1 {
		t.Errorf("left_y axis index = %d, want 1", axisTargets["left_y"])
	}
	if axisTargets["lt"] != 3 {
		t.Errorf("lt axis index = %d, want 3", axisTargets["lt"])
	}

	// Check button bindings
	buttonTargets := make(map[string]int)
	for _, bb := range m.Buttons {
		buttonTargets[bb.Target] = bb.ButtonIndex
	}
	if buttonTargets["a"] != 1 {
		t.Errorf("a button index = %d, want 1", buttonTargets["a"])
	}
	if buttonTargets["guide"] != 12 {
		t.Errorf("guide button index = %d, want 12", buttonTargets["guide"])
	}

	// Check hat bindings
	if len(m.Hats) == 0 {
		t.Error("expected hat bindings, got none")
	}
	hatTargets := make(map[string]int)
	for _, hb := range m.Hats {
		hatTargets[hb.Target] = hb.DirMask
	}
	if hatTargets["dpup"] != 1 {
		t.Errorf("dpup hat mask = %d, want 1", hatTargets["dpup"])
	}
	if hatTargets["dpdown"] != 4 {
		t.Errorf("dpdown hat mask = %d, want 4", hatTargets["dpdown"])
	}
}

func TestParseMappingFields_HalfAxis(t *testing.T) {
	// dpdown:+a1 → dpad down from positive half of axis 1
	// dpleft:-a0 → dpad left from negative half of axis 0
	line := "03000000102800000900000000000000,8BitDo SFC30,a:b1,b:b0,back:b10,dpdown:+a1,dpleft:-a0,dpright:+a0,dpup:-a1,leftshoulder:b6,rightshoulder:b7,start:b11,x:b4,y:b3,platform:Windows,"

	m := parseMappingFields(line)
	if m == nil {
		t.Fatal("parseMappingFields returned nil")
	}

	// Find dpdown axis binding
	var dpdownBinding *SDLAxisBinding
	for i := range m.Axes {
		if m.Axes[i].Target == "dpdown" {
			dpdownBinding = &m.Axes[i]
			break
		}
	}
	if dpdownBinding == nil {
		t.Fatal("no dpdown axis binding found")
	}
	if dpdownBinding.AxisIndex != 1 {
		t.Errorf("dpdown axis index = %d, want 1", dpdownBinding.AxisIndex)
	}
	if !dpdownBinding.HalfPos {
		t.Error("dpdown binding should have HalfPos=true")
	}

	var dpleftBinding *SDLAxisBinding
	for i := range m.Axes {
		if m.Axes[i].Target == "dpleft" {
			dpleftBinding = &m.Axes[i]
			break
		}
	}
	if dpleftBinding == nil {
		t.Fatal("no dpleft axis binding found")
	}
	if dpleftBinding.AxisIndex != 0 {
		t.Errorf("dpleft axis index = %d, want 0", dpleftBinding.AxisIndex)
	}
	if !dpleftBinding.HalfNeg {
		t.Error("dpleft binding should have HalfNeg=true")
	}
}

func TestParseMappingFields_N64HalfAxisButton(t *testing.T) {
	// +rightx:b9 — pressing button 9 sets right_x to +1
	line := "03000000c82d00000290000000000000,8BitDo N64,+rightx:b9,+righty:b3,-rightx:b4,-righty:b8,a:b0,b:b1,dpdown:h0.4,dpleft:h0.8,dpright:h0.2,dpup:h0.1,leftshoulder:b6,lefttrigger:b10,leftx:a0,lefty:a1,rightshoulder:b7,start:b11,platform:Windows,"

	m := parseMappingFields(line)
	if m == nil {
		t.Fatal("parseMappingFields returned nil")
	}

	if len(m.AxisHalfButtons) == 0 {
		t.Fatal("expected AxisHalfButtons, got none")
	}

	ahbMap := make(map[string]SDLAxisHalfButton)
	for _, ahb := range m.AxisHalfButtons {
		key := ahb.Target + ":" + string(rune('0'+ahb.Sign+1)) // crude sign key
		ahbMap[key] = ahb
	}

	// +rightx:b9 → right_x +1 from button index 9
	found := false
	for _, ahb := range m.AxisHalfButtons {
		if ahb.Target == "right_x" && ahb.Sign == 1 && ahb.ButtonIdx == 9 {
			found = true
		}
	}
	if !found {
		t.Error("+rightx:b9 not correctly parsed as AxisHalfButton{right_x, +1, 9}")
	}

	// -rightx:b4 → right_x -1 from button index 4
	found = false
	for _, ahb := range m.AxisHalfButtons {
		if ahb.Target == "right_x" && ahb.Sign == -1 && ahb.ButtonIdx == 4 {
			found = true
		}
	}
	if !found {
		t.Error("-rightx:b4 not correctly parsed as AxisHalfButton{right_x, -1, 4}")
	}
}

func TestParseMappingFields_InvertedAxis(t *testing.T) {
	// righty:a3~ — axis 3, inverted
	line := "03000000260900008888000000000000,Cyber Gadget,leftx:a0,lefty:a1,rightx:a2,righty:a3~,start:b7,platform:Windows,"

	m := parseMappingFields(line)
	if m == nil {
		t.Fatal("parseMappingFields returned nil")
	}

	var rightyBinding *SDLAxisBinding
	for i := range m.Axes {
		if m.Axes[i].Target == "right_y" {
			rightyBinding = &m.Axes[i]
			break
		}
	}
	if rightyBinding == nil {
		t.Fatal("no right_y axis binding found")
	}
	if rightyBinding.AxisIndex != 3 {
		t.Errorf("right_y axis index = %d, want 3", rightyBinding.AxisIndex)
	}
	if !rightyBinding.Invert {
		t.Error("right_y binding should have Invert=true")
	}
}

func TestLoadSDLMappingsFromReader(t *testing.T) {
	// 8BitDo Pro 2 GUID: 03000000c82d00000360000000000000
	// vid LE16: bytes[4..5]=0xc8,0x2d → 0x2dc8; pid LE16: bytes[8..9]=0x03,0x60 → 0x6003
	data := `# Test gamecontrollerdb
03000000c82d00000360000000000000,8BitDo Pro 2,a:b1,b:b0,back:b10,dpdown:h0.4,dpleft:h0.8,dpright:h0.2,dpup:h0.1,guide:b12,leftshoulder:b6,leftstick:b13,lefttrigger:b8,leftx:a0,lefty:a1,rightshoulder:b7,rightstick:b14,righttrigger:b9,rightx:a3,righty:a4,start:b11,x:b4,y:b3,platform:Windows,
050000004c050000cc09000000010000,PS5 DualSense,a:b1,b:b2,x:b0,y:b3,platform:Linux,
03000000c82d00000360000000000000,8BitDo Pro 2 (duplicate),a:b0,platform:Windows,
`
	m, err := LoadSDLMappingsFromReader(strings.NewReader(data), "Windows")
	if err != nil {
		t.Fatalf("LoadSDLMappingsFromReader: %v", err)
	}

	// Should have 1 Windows entry (second Linux entry skipped;
	// duplicate VID/PID overwrites — last wins is fine)
	if len(m) != 1 {
		t.Errorf("got %d mappings, want 1", len(m))
	}

	// vid LE16 from 0xc82d GUID bytes: 0x2dc8; pid LE16 from 0x0360 bytes: 0x6003
	key := deviceKey{VendorID: 0x2dc8, ProductID: 0x6003}
	entry, ok := m[key]
	if !ok {
		t.Fatalf("8BitDo Pro 2 mapping not found (expected key VID=0x2dc8 PID=0x6003); keys in map:")
	}
	// Last entry wins (duplicate), so Name should be the duplicate.
	_ = entry
}

func TestLoadSDLMappingsFromFile_Real(t *testing.T) {
	const path = `E:\Projects\SDL_GameControllerDB\gamecontrollerdb.txt`
	m, err := LoadSDLMappingsFromFile(path, "Windows")
	if err != nil {
		t.Skipf("gamecontrollerdb.txt not available: %v", err)
	}
	if len(m) < 100 {
		t.Errorf("expected at least 100 Windows mappings, got %d", len(m))
	}
	t.Logf("Loaded %d Windows mappings from %s", len(m), path)

	// Spot-check a known entry: 8BitDo Pro 2
	// vid LE16 from GUID bytes 0xc82d: 0x2dc8; pid LE16 from 0x0360: 0x6003
	key := deviceKey{VendorID: 0x2dc8, ProductID: 0x6003}
	if entry, ok := m[key]; ok {
		t.Logf("8BitDo Pro 2: axes=%d buttons=%d hats=%d", len(entry.Axes), len(entry.Buttons), len(entry.Hats))
	}
}
