//go:build windows

package gamepad

import (
	"testing"
)

// TestBuildAxisOrderSortsByUsage verifies that buildAxisOrder produces axes in
// usage-code order regardless of HID descriptor declaration order. SDL DB
// Windows entries with `0300xxxx` GUIDs assume DirectInput's usage-sorted axis
// enumeration (X, Y, Z, Rx, Ry, Rz). Without this sort, DualShock 4 / DualSense
// (which declare X, Y, Z, Rz, Rx, Ry — physical byte order) get axis indices
// 3/4/5 swapped, causing SDL's `lefttrigger:a3`, `righttrigger:a4`, `righty:a5`
// to map to wrong physical axes.
func TestBuildAxisOrderSortsByUsage(t *testing.T) {
	// Simulate the DualSense USB HID descriptor declaration order:
	// X, Y, Z, Rz, Rx, Ry, Hat. Each value cap declares one usage (IsRange=0).
	valueCaps := []hidpValueCaps{
		{UsagePage: usagePageGenericDesktop, UsageMin: hidUsageX, UsageMax: hidUsageX, LogicalMin: 0, LogicalMax: 255, BitSize: 8},
		{UsagePage: usagePageGenericDesktop, UsageMin: hidUsageY, UsageMax: hidUsageY, LogicalMin: 0, LogicalMax: 255, BitSize: 8},
		{UsagePage: usagePageGenericDesktop, UsageMin: hidUsageZ, UsageMax: hidUsageZ, LogicalMin: 0, LogicalMax: 255, BitSize: 8},
		{UsagePage: usagePageGenericDesktop, UsageMin: hidUsageRz, UsageMax: hidUsageRz, LogicalMin: 0, LogicalMax: 255, BitSize: 8},
		{UsagePage: usagePageGenericDesktop, UsageMin: hidUsageRx, UsageMax: hidUsageRx, LogicalMin: 0, LogicalMax: 255, BitSize: 8},
		{UsagePage: usagePageGenericDesktop, UsageMin: hidUsageRy, UsageMax: hidUsageRy, LogicalMin: 0, LogicalMax: 255, BitSize: 8},
		// Hat switch is excluded by buildAxisOrder.
		{UsagePage: usagePageGenericDesktop, UsageMin: hidUsageHat, UsageMax: hidUsageHat, LogicalMin: 0, LogicalMax: 7, BitSize: 4},
	}

	got := buildAxisOrder(valueCaps)

	// Expect 6 axes (hat excluded) in usage-sorted order: X, Y, Z, Rx, Ry, Rz.
	wantUsages := []uint16{hidUsageX, hidUsageY, hidUsageZ, hidUsageRx, hidUsageRy, hidUsageRz}
	if len(got) != len(wantUsages) {
		t.Fatalf("buildAxisOrder produced %d entries, want %d (hat should be excluded)", len(got), len(wantUsages))
	}
	for i, want := range wantUsages {
		if got[i].usage != want {
			t.Errorf("buildAxisOrder[%d].usage = 0x%02x, want 0x%02x", i, got[i].usage, want)
		}
	}

	// Sanity: SDL's `lefttrigger:a3` must now resolve to Rx (the physical L2),
	// `righttrigger:a4` to Ry (R2), `righty:a5` to Rz (right_y) — matching SDL
	// DB Windows entries for PS4/PS5.
	if got[3].usage != hidUsageRx {
		t.Errorf("axis 3 (lefttrigger target in SDL) = 0x%02x, want Rx (0x33)", got[3].usage)
	}
	if got[4].usage != hidUsageRy {
		t.Errorf("axis 4 (righttrigger target in SDL) = 0x%02x, want Ry (0x34)", got[4].usage)
	}
	if got[5].usage != hidUsageRz {
		t.Errorf("axis 5 (righty target in SDL) = 0x%02x, want Rz (0x35)", got[5].usage)
	}
}

// TestBuildAxisOrderStableForDuplicateUsages verifies that when two value caps
// declare the same (usagePage, usage) — e.g. an aliased axis — sort.SliceStable
// preserves declaration order so logicalMin/logicalMax from the first
// declaration are used. This guards against silent reordering of duplicate
// entries on weird descriptors.
func TestBuildAxisOrderStableForDuplicateUsages(t *testing.T) {
	valueCaps := []hidpValueCaps{
		{UsagePage: usagePageGenericDesktop, UsageMin: hidUsageX, UsageMax: hidUsageX, LogicalMin: 0, LogicalMax: 255, BitSize: 8},
		{UsagePage: usagePageGenericDesktop, UsageMin: hidUsageX, UsageMax: hidUsageX, LogicalMin: -32768, LogicalMax: 32767, BitSize: 16},
	}

	got := buildAxisOrder(valueCaps)
	if len(got) != 2 {
		t.Fatalf("got %d entries, want 2", len(got))
	}
	// First declaration (8-bit) must come first; SortStable preserves order.
	if got[0].logicalMax != 255 {
		t.Errorf("got[0].logicalMax = %d, want 255 (first declaration must come first)", got[0].logicalMax)
	}
	if got[1].logicalMax != 32767 {
		t.Errorf("got[1].logicalMax = %d, want 32767", got[1].logicalMax)
	}
}

// TestBuildAxisOrderHandlesUsageRanges verifies that range-style value caps
// (IsRange=1) are expanded into individual entries before sorting. This
// preserves the per-usage logical min/max when the caps declare a contiguous
// span of usages.
func TestBuildAxisOrderHandlesUsageRanges(t *testing.T) {
	// One value cap declaring usages 0x33..0x35 as a range (Rx, Ry, Rz).
	valueCaps := []hidpValueCaps{
		{UsagePage: usagePageGenericDesktop, UsageMin: hidUsageRx, UsageMax: hidUsageRz, IsRange: 1, LogicalMin: 0, LogicalMax: 1023, BitSize: 10},
	}

	got := buildAxisOrder(valueCaps)
	if len(got) != 3 {
		t.Fatalf("got %d entries, want 3 (range expansion)", len(got))
	}
	wantUsages := []uint16{hidUsageRx, hidUsageRy, hidUsageRz}
	for i, want := range wantUsages {
		if got[i].usage != want {
			t.Errorf("got[%d].usage = 0x%02x, want 0x%02x", i, got[i].usage, want)
		}
		if got[i].logicalMax != 1023 {
			t.Errorf("got[%d].logicalMax = %d, want 1023 (inherited from range cap)", i, got[i].logicalMax)
		}
	}
}
