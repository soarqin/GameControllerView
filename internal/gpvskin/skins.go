package gpvskin

// GPVSkinDef describes a GamepadViewer skin: its CSS class name, available
// variants, and the ordered list of element specs.
type GPVSkinDef struct {
	// CSSClass is the skin class used in CSS selectors (e.g. "xbox", "ds4").
	CSSClass string
	// Variants are optional modifier classes (e.g. "white").
	Variants []string
	// Elements are the ordered element specs for this skin.
	// Background (type=IOTexture) must be first.
	Elements []GPVElementSpec
}

// standardButtonSpecs returns the face-button specs common to most skins.
func standardButtonSpecs() []GPVElementSpec {
	return []GPVElementSpec{
		{Name: "button-a", IOType: IOGamepadBtn, ButtonCode: 0, PressedStyle: PressedSprite, ZLevel: 1},
		{Name: "button-b", IOType: IOGamepadBtn, ButtonCode: 1, PressedStyle: PressedSprite, ZLevel: 1},
		{Name: "button-x", IOType: IOGamepadBtn, ButtonCode: 2, PressedStyle: PressedSprite, ZLevel: 1},
		{Name: "button-y", IOType: IOGamepadBtn, ButtonCode: 3, PressedStyle: PressedSprite, ZLevel: 1},
	}
}

// standardDpadSpecs returns the 4-direction dpad specs common to most skins.
// Most GPV skins use opacity-based pressed state for d-pad directions:
// the CSS sets .face { opacity: 0 } and .face.pressed { opacity: 1 }.
// This means frame0 in the atlas is transparent and frame1 is the sprite.
func standardDpadSpecs() []GPVElementSpec {
	return []GPVElementSpec{
		{Name: "dpad-up", IOType: IOGamepadBtn, ButtonCode: 11, PressedStyle: PressedOpacity, ZLevel: 1},
		{Name: "dpad-down", IOType: IOGamepadBtn, ButtonCode: 12, PressedStyle: PressedOpacity, ZLevel: 1},
		{Name: "dpad-left", IOType: IOGamepadBtn, ButtonCode: 13, PressedStyle: PressedOpacity, ZLevel: 1},
		{Name: "dpad-right", IOType: IOGamepadBtn, ButtonCode: 14, PressedStyle: PressedOpacity, ZLevel: 1},
	}
}

// AllSkins is the registry of all supported GamepadViewer skins.
var AllSkins = []GPVSkinDef{
	xboxOneSkin,
	xbox360Skin,
	ps3Skin,
	ds4Skin,
	nesSkin,
	gcSkin,
	n64Skin,
	fppSkin,
	fightStickSkin,
}

// SkinByClass finds a skin definition by CSS class name.
func SkinByClass(class string) *GPVSkinDef {
	for i := range AllSkins {
		if AllSkins[i].CSSClass == class {
			return &AllSkins[i]
		}
	}
	return nil
}

// --------------------------------------------------------------------------
// Xbox One skin ("xbox")
// --------------------------------------------------------------------------

var xboxOneSkin = GPVSkinDef{
	CSSClass: "xbox",
	Variants: []string{"white"},
	Elements: concat(
		[]GPVElementSpec{
			{Name: "background", IOType: IOTexture, PressedStyle: PressedNone, ZLevel: 0},
			// Triggers: CSS uses opacity:0 normally, JS sets element.style.opacity = triggerValue.
			// PressedOpacity atlas layout: frame0=transparent, frame1=sprite.
			// With trigger_mode:false (progressive), the renderer clips frame1 proportional
			// to the trigger value — correct analog fill effect.
			{Name: "trigger-left", IOType: IOTrigger, Side: SideLeft, Direction: DirUp, PressedStyle: PressedOpacity, ZLevel: 1},
			{Name: "trigger-right", IOType: IOTrigger, Side: SideRight, Direction: DirUp, PressedStyle: PressedOpacity, ZLevel: 1},
			// Bumpers (LB/RB)
			{Name: "bumper-left", IOType: IOGamepadBtn, ButtonCode: 9, PressedStyle: PressedOpacity, ZLevel: 1},
			{Name: "bumper-right", IOType: IOGamepadBtn, ButtonCode: 10, PressedStyle: PressedOpacity, ZLevel: 1},
			// Back (View) / Start (Menu) — share the same SVG, side by side
			{Name: "back", IOType: IOGamepadBtn, ButtonCode: 4, PressedStyle: PressedOpacity, ZLevel: 1},
			{Name: "start", IOType: IOGamepadBtn, ButtonCode: 6, PressedStyle: PressedOpacity, ZLevel: 1},
			// Xbox One has no separate guide/meta element in CSS — omitted.
			// Analog sticks
			{Name: "stick-left", IOType: IOAnalogStick, Side: SideLeft, StickRadius: 22, PressedStyle: PressedSprite, ZLevel: 1},
			{Name: "stick-right", IOType: IOAnalogStick, Side: SideRight, StickRadius: 22, PressedStyle: PressedSprite, ZLevel: 1},
			// Player indicator
			{Name: "quadrant", IOType: IOGamepadID, PressedStyle: PressedNone, ZLevel: 1},
		},
		standardButtonSpecs(),
		standardDpadSpecs(),
	),
}

// --------------------------------------------------------------------------
// Xbox 360 skin ("xbox-old")
// --------------------------------------------------------------------------

var xbox360Skin = GPVSkinDef{
	CSSClass: "xbox-old",
	Elements: concat(
		[]GPVElementSpec{
			{Name: "background", IOType: IOTexture, PressedStyle: PressedNone, ZLevel: 0},
			{Name: "trigger-left", IOType: IOTrigger, Side: SideLeft, Direction: DirDown, PressedStyle: PressedOpacity, ZLevel: 1},
			{Name: "trigger-right", IOType: IOTrigger, Side: SideRight, Direction: DirDown, PressedStyle: PressedOpacity, ZLevel: 1},
			{Name: "bumper-left", IOType: IOGamepadBtn, ButtonCode: 9, PressedStyle: PressedOpacity, ZLevel: 1},
			{Name: "bumper-right", IOType: IOGamepadBtn, ButtonCode: 10, PressedStyle: PressedOpacity, ZLevel: 1},
			{Name: "back", IOType: IOGamepadBtn, ButtonCode: 4, PressedStyle: PressedSprite, ZLevel: 1},
			{Name: "start", IOType: IOGamepadBtn, ButtonCode: 6, PressedStyle: PressedSprite, ZLevel: 1},
			{Name: "stick-left", IOType: IOAnalogStick, Side: SideLeft, StickRadius: 22, PressedStyle: PressedSprite, ZLevel: 1},
			{Name: "stick-right", IOType: IOAnalogStick, Side: SideRight, StickRadius: 22, PressedStyle: PressedSprite, ZLevel: 1},
			{Name: "quadrant", IOType: IOGamepadID, PressedStyle: PressedNone, ZLevel: 1},
		},
		standardButtonSpecs(),
		standardDpadSpecs(),
	),
}

// --------------------------------------------------------------------------
// PlayStation 3 skin ("ps")
// --------------------------------------------------------------------------

var ps3Skin = GPVSkinDef{
	CSSClass: "ps",
	Variants: []string{"white"},
	Elements: concat(
		[]GPVElementSpec{
			{Name: "background", IOType: IOTexture, PressedStyle: PressedNone, ZLevel: 0},
			{Name: "trigger-left", IOType: IOTrigger, Side: SideLeft, Direction: DirUp, PressedStyle: PressedOpacity, ZLevel: 1},
			{Name: "trigger-right", IOType: IOTrigger, Side: SideRight, Direction: DirUp, PressedStyle: PressedOpacity, ZLevel: 1},
			{Name: "bumper-left", IOType: IOGamepadBtn, ButtonCode: 9, PressedStyle: PressedOpacity, ZLevel: 1},
			{Name: "bumper-right", IOType: IOGamepadBtn, ButtonCode: 10, PressedStyle: PressedOpacity, ZLevel: 1},
			{Name: "back", IOType: IOGamepadBtn, ButtonCode: 4, PressedStyle: PressedSprite, ZLevel: 1},
			{Name: "start", IOType: IOGamepadBtn, ButtonCode: 6, PressedStyle: PressedSprite, ZLevel: 1},
			{Name: "stick-left", IOType: IOAnalogStick, Side: SideLeft, StickRadius: 22, PressedStyle: PressedSprite, ZLevel: 1},
			{Name: "stick-right", IOType: IOAnalogStick, Side: SideRight, StickRadius: 22, PressedStyle: PressedSprite, ZLevel: 1},
			{Name: "quadrant", IOType: IOGamepadID, PressedStyle: PressedNone, ZLevel: 1},
		},
		standardButtonSpecs(),
		standardDpadSpecs(),
	),
}

// --------------------------------------------------------------------------
// DualShock 4 / PS4 skin ("ds4")
// --------------------------------------------------------------------------

var ds4Skin = GPVSkinDef{
	CSSClass: "ds4",
	Variants: []string{"white"},
	Elements: concat(
		[]GPVElementSpec{
			{Name: "background", IOType: IOTexture, PressedStyle: PressedNone, ZLevel: 0},
			{Name: "trigger-left", IOType: IOTrigger, Side: SideLeft, Direction: DirUp, PressedStyle: PressedOpacity, ZLevel: 1},
			{Name: "trigger-right", IOType: IOTrigger, Side: SideRight, Direction: DirUp, PressedStyle: PressedOpacity, ZLevel: 1},
			{Name: "bumper-left", IOType: IOGamepadBtn, ButtonCode: 9, PressedStyle: PressedOpacity, ZLevel: 1},
			{Name: "bumper-right", IOType: IOGamepadBtn, ButtonCode: 10, PressedStyle: PressedOpacity, ZLevel: 1},
			{Name: "back", IOType: IOGamepadBtn, ButtonCode: 4, PressedStyle: PressedOpacity, ZLevel: 1},
			{Name: "start", IOType: IOGamepadBtn, ButtonCode: 6, PressedStyle: PressedOpacity, ZLevel: 1},
			// DS4-specific buttons
			{Name: "touchpad", IOType: IOGamepadBtn, ButtonCode: 20, PressedStyle: PressedOpacity, ZLevel: 1},
			// Note: .meta (PS/Home button) is NOT mapped — GPV cannot detect the guide button via browser APIs.
			{Name: "stick-left", IOType: IOAnalogStick, Side: SideLeft, StickRadius: 22, PressedStyle: PressedSprite, ZLevel: 1},
			{Name: "stick-right", IOType: IOAnalogStick, Side: SideRight, StickRadius: 22, PressedStyle: PressedSprite, ZLevel: 1},
		},
		standardButtonSpecs(),
		standardDpadSpecs(),
	),
}

// --------------------------------------------------------------------------
// NES skin ("nes")
// --------------------------------------------------------------------------

var nesSkin = GPVSkinDef{
	CSSClass: "nes",
	Elements: []GPVElementSpec{
		{Name: "background", IOType: IOTexture, PressedStyle: PressedNone, ZLevel: 0},
		// NES only has A and B (X and Y are hidden via display:none)
		{Name: "button-a", IOType: IOGamepadBtn, ButtonCode: 0, PressedStyle: PressedSprite, ZLevel: 1},
		{Name: "button-b", IOType: IOGamepadBtn, ButtonCode: 1, PressedStyle: PressedSprite, ZLevel: 1},
		{Name: "back", IOType: IOGamepadBtn, ButtonCode: 4, PressedStyle: PressedSprite, ZLevel: 1},
		{Name: "start", IOType: IOGamepadBtn, ButtonCode: 6, PressedStyle: PressedSprite, ZLevel: 1},
		{Name: "dpad-up", IOType: IOGamepadBtn, ButtonCode: 11, PressedStyle: PressedSprite, ZLevel: 1},
		{Name: "dpad-down", IOType: IOGamepadBtn, ButtonCode: 12, PressedStyle: PressedSprite, ZLevel: 1},
		{Name: "dpad-left", IOType: IOGamepadBtn, ButtonCode: 13, PressedStyle: PressedSprite, ZLevel: 1},
		{Name: "dpad-right", IOType: IOGamepadBtn, ButtonCode: 14, PressedStyle: PressedSprite, ZLevel: 1},
	},
}

// --------------------------------------------------------------------------
// GameCube skin ("gc")
// --------------------------------------------------------------------------

var gcSkin = GPVSkinDef{
	CSSClass: "gc",
	Elements: concat(
		[]GPVElementSpec{
			{Name: "background", IOType: IOTexture, PressedStyle: PressedNone, ZLevel: 0},
			{Name: "trigger-left", IOType: IOTrigger, Side: SideLeft, Direction: DirUp, PressedStyle: PressedOpacity, ZLevel: 1},
			{Name: "trigger-right", IOType: IOTrigger, Side: SideRight, Direction: DirUp, PressedStyle: PressedOpacity, ZLevel: 1},
			// GC has no left bumper; right bumper = Z button
			{Name: "bumper-right", IOType: IOGamepadBtn, ButtonCode: 10, PressedStyle: PressedOpacity, ZLevel: 1},
			// GC start (large center button)
			{Name: "start", IOType: IOGamepadBtn, ButtonCode: 6, PressedStyle: PressedOpacity, ZLevel: 1},
			// GC sticks: left = main stick, right = C-stick
			{Name: "stick-left", IOType: IOAnalogStick, Side: SideLeft, StickRadius: 22, PressedStyle: PressedOpacity, ZLevel: 1},
			{Name: "stick-right", IOType: IOAnalogStick, Side: SideRight, StickRadius: 22, PressedStyle: PressedOpacity, ZLevel: 1},
		},
		// GC face buttons use opacity-based pressed (start at opacity:0)
		[]GPVElementSpec{
			{Name: "button-a", IOType: IOGamepadBtn, ButtonCode: 0, PressedStyle: PressedOpacity, ZLevel: 1},
			{Name: "button-b", IOType: IOGamepadBtn, ButtonCode: 1, PressedStyle: PressedOpacity, ZLevel: 1},
			{Name: "button-x", IOType: IOGamepadBtn, ButtonCode: 2, PressedStyle: PressedOpacity, ZLevel: 1},
			{Name: "button-y", IOType: IOGamepadBtn, ButtonCode: 3, PressedStyle: PressedOpacity, ZLevel: 1},
		},
		standardDpadSpecs(),
	),
}

// --------------------------------------------------------------------------
// Nintendo 64 skin ("n64")
// --------------------------------------------------------------------------

var n64Skin = GPVSkinDef{
	CSSClass: "n64",
	Elements: concat(
		[]GPVElementSpec{
			{Name: "background", IOType: IOTexture, PressedStyle: PressedNone, ZLevel: 0},
			{Name: "trigger-left", IOType: IOTrigger, Side: SideLeft, Direction: DirUp, PressedStyle: PressedOpacity, ZLevel: 1},
			// N64 right trigger = Z button (analog)
			{Name: "trigger-right", IOType: IOTrigger, Side: SideRight, Direction: DirUp, PressedStyle: PressedOpacity, ZLevel: 1},
			{Name: "bumper-left", IOType: IOGamepadBtn, ButtonCode: 9, PressedStyle: PressedOpacity, ZLevel: 1},
			{Name: "bumper-right", IOType: IOGamepadBtn, ButtonCode: 10, PressedStyle: PressedOpacity, ZLevel: 1},
			{Name: "back", IOType: IOGamepadBtn, ButtonCode: 4, PressedStyle: PressedOpacity, ZLevel: 1},
			{Name: "start", IOType: IOGamepadBtn, ButtonCode: 6, PressedStyle: PressedOpacity, ZLevel: 1},
			// Note: .meta (N64 Home button equivalent) is NOT mapped — GPV cannot detect the guide button via browser APIs.
			{Name: "stick-left", IOType: IOAnalogStick, Side: SideLeft, StickRadius: 22, PressedStyle: PressedOpacity, ZLevel: 1},
		},
		[]GPVElementSpec{
			{Name: "button-a", IOType: IOGamepadBtn, ButtonCode: 0, PressedStyle: PressedOpacity, ZLevel: 1},
			{Name: "button-b", IOType: IOGamepadBtn, ButtonCode: 1, PressedStyle: PressedOpacity, ZLevel: 1},
			{Name: "button-x", IOType: IOGamepadBtn, ButtonCode: 2, PressedStyle: PressedOpacity, ZLevel: 1},
			{Name: "button-y", IOType: IOGamepadBtn, ButtonCode: 3, PressedStyle: PressedOpacity, ZLevel: 1},
		},
		standardDpadSpecs(),
	),
}

// --------------------------------------------------------------------------
// FightPad Pro skin ("fpp")
// --------------------------------------------------------------------------

var fppSkin = GPVSkinDef{
	CSSClass: "fpp",
	Elements: concat(
		[]GPVElementSpec{
			{Name: "background", IOType: IOTexture, PressedStyle: PressedNone, ZLevel: 0},
			{Name: "trigger-left", IOType: IOTrigger, Side: SideLeft, Direction: DirUp, PressedStyle: PressedOpacity, ZLevel: 1},
			{Name: "trigger-right", IOType: IOTrigger, Side: SideRight, Direction: DirUp, PressedStyle: PressedOpacity, ZLevel: 1},
			{Name: "bumper-left", IOType: IOGamepadBtn, ButtonCode: 9, PressedStyle: PressedOpacity, ZLevel: 1},
			{Name: "bumper-right", IOType: IOGamepadBtn, ButtonCode: 10, PressedStyle: PressedOpacity, ZLevel: 1},
			{Name: "back", IOType: IOGamepadBtn, ButtonCode: 4, PressedStyle: PressedOpacity, ZLevel: 1},
			{Name: "start", IOType: IOGamepadBtn, ButtonCode: 6, PressedStyle: PressedOpacity, ZLevel: 1},
			{Name: "stick-left", IOType: IOAnalogStick, Side: SideLeft, StickRadius: 22, PressedStyle: PressedSprite, ZLevel: 1},
			{Name: "quadrant", IOType: IOGamepadID, PressedStyle: PressedNone, ZLevel: 1},
		},
		[]GPVElementSpec{
			{Name: "button-a", IOType: IOGamepadBtn, ButtonCode: 0, PressedStyle: PressedOpacity, ZLevel: 1},
			{Name: "button-b", IOType: IOGamepadBtn, ButtonCode: 1, PressedStyle: PressedOpacity, ZLevel: 1},
			{Name: "button-x", IOType: IOGamepadBtn, ButtonCode: 2, PressedStyle: PressedOpacity, ZLevel: 1},
			{Name: "button-y", IOType: IOGamepadBtn, ButtonCode: 3, PressedStyle: PressedOpacity, ZLevel: 1},
		},
		standardDpadSpecs(),
	),
}

// --------------------------------------------------------------------------
// Fight Stick / Arcade Stick skin ("fight-stick")
// --------------------------------------------------------------------------

var fightStickSkin = GPVSkinDef{
	CSSClass: "fight-stick",
	Elements: []GPVElementSpec{
		{Name: "background", IOType: IOTexture, PressedStyle: PressedNone, ZLevel: 0},
		// Fight stick uses trigger-button (digital), not analog triggers
		{Name: "trigger-button-left", IOType: IOGamepadBtn, ButtonCode: 6, PressedStyle: PressedInvert, ZLevel: 1},
		{Name: "trigger-button-right", IOType: IOGamepadBtn, ButtonCode: 7, PressedStyle: PressedInvert, ZLevel: 1},
		{Name: "bumper-left", IOType: IOGamepadBtn, ButtonCode: 9, PressedStyle: PressedInvert, ZLevel: 1},
		{Name: "bumper-right", IOType: IOGamepadBtn, ButtonCode: 10, PressedStyle: PressedInvert, ZLevel: 1},
		{Name: "back", IOType: IOGamepadBtn, ButtonCode: 4, PressedStyle: PressedInvert, ZLevel: 1},
		{Name: "start", IOType: IOGamepadBtn, ButtonCode: 6, PressedStyle: PressedInvert, ZLevel: 1},
		// Fight stick joystick = dpad type 8 (8 directions sprite strip)
		{Name: "fstick", IOType: IODpad, PressedStyle: PressedNone, ZLevel: 1},
		// Face buttons use invert-style pressed
		{Name: "button-a", IOType: IOGamepadBtn, ButtonCode: 0, PressedStyle: PressedInvert, ZLevel: 1},
		{Name: "button-b", IOType: IOGamepadBtn, ButtonCode: 1, PressedStyle: PressedInvert, ZLevel: 1},
		{Name: "button-x", IOType: IOGamepadBtn, ButtonCode: 2, PressedStyle: PressedInvert, ZLevel: 1},
		{Name: "button-y", IOType: IOGamepadBtn, ButtonCode: 3, PressedStyle: PressedInvert, ZLevel: 1},
	},
}

// --------------------------------------------------------------------------
// Custom skin ("custom") — used when loading external CSS with ?css= parameter.
// The custom skin uses ".custom" as its CSS class and mirrors the PS3 layout.
// --------------------------------------------------------------------------

var CustomSkinDef = GPVSkinDef{
	CSSClass: "custom",
	Elements: concat(
		[]GPVElementSpec{
			{Name: "background", IOType: IOTexture, PressedStyle: PressedNone, ZLevel: 0},
			{Name: "trigger-left", IOType: IOTrigger, Side: SideLeft, Direction: DirUp, PressedStyle: PressedOpacity, ZLevel: 1},
			{Name: "trigger-right", IOType: IOTrigger, Side: SideRight, Direction: DirUp, PressedStyle: PressedOpacity, ZLevel: 1},
			{Name: "bumper-left", IOType: IOGamepadBtn, ButtonCode: 9, PressedStyle: PressedOpacity, ZLevel: 1},
			{Name: "bumper-right", IOType: IOGamepadBtn, ButtonCode: 10, PressedStyle: PressedOpacity, ZLevel: 1},
			{Name: "back", IOType: IOGamepadBtn, ButtonCode: 4, PressedStyle: PressedSprite, ZLevel: 1},
			{Name: "start", IOType: IOGamepadBtn, ButtonCode: 6, PressedStyle: PressedSprite, ZLevel: 1},
			{Name: "touchpad", IOType: IOGamepadBtn, ButtonCode: 20, PressedStyle: PressedOpacity, ZLevel: 1},
			// Note: .meta (PS/Home button) is NOT mapped — GPV cannot detect the guide button via browser APIs.
			{Name: "stick-left", IOType: IOAnalogStick, Side: SideLeft, StickRadius: 22, PressedStyle: PressedSprite, ZLevel: 1},
			{Name: "stick-right", IOType: IOAnalogStick, Side: SideRight, StickRadius: 22, PressedStyle: PressedSprite, ZLevel: 1},
			{Name: "quadrant", IOType: IOGamepadID, PressedStyle: PressedNone, ZLevel: 1},
		},
		standardButtonSpecs(),
		standardDpadSpecs(),
	),
}

// concat merges multiple slices of GPVElementSpec.
func concat(slices ...[]GPVElementSpec) []GPVElementSpec {
	var out []GPVElementSpec
	for _, s := range slices {
		out = append(out, s...)
	}
	return out
}
