// Package gpvskin converts GamepadViewer CSS skins to Input Overlay format.
package gpvskin

// IOElementType mirrors the Input Overlay element_type enum.
type IOElementType int

const (
	IOTexture     IOElementType = 0
	IOKeyboard    IOElementType = 1
	IOGamepadBtn  IOElementType = 2
	IOMouseBtn    IOElementType = 3
	IOWheel       IOElementType = 4
	IOAnalogStick IOElementType = 5
	IOTrigger     IOElementType = 6
	IOGamepadID   IOElementType = 7
	IODpad        IOElementType = 8
	IOMouseMove   IOElementType = 9
)

// TriggerDir matches Input Overlay direction enum (DIR_UP=1, DIR_DOWN=2, ...).
type TriggerDir int

const (
	DirNone  TriggerDir = 0
	DirUp    TriggerDir = 1
	DirDown  TriggerDir = 2
	DirLeft  TriggerDir = 3
	DirRight TriggerDir = 4
)

// StickSide: 0=left, 1=right.
type StickSide int

const (
	SideLeft  StickSide = 0
	SideRight StickSide = 1
)

// PressedStyle describes how a GPV element shows its pressed state visually.
type PressedStyle int

const (
	PressedSprite  PressedStyle = iota // background-position shift (sprite swap)
	PressedOpacity                     // opacity 0→1 (element invisible when not pressed)
	PressedInvert                      // filter:invert (fight stick style)
	PressedNone                        // no pressed state (static textures)
)

// Rect is a pixel rectangle.
type Rect struct {
	X, Y, W, H int
}

// GPVElementSpec describes a fixed GamepadViewer DOM element and its mapping
// to Input Overlay semantics. Positions are absolute on the controller canvas.
type GPVElementSpec struct {
	// Name is a human-readable identifier (e.g. "trigger-left").
	Name string
	// IOType is the corresponding Input Overlay element type.
	IOType IOElementType
	// ButtonCode is the SDL2 gamepad button code (IOGamepadBtn only).
	ButtonCode int
	// Side is left/right (IOAnalogStick, IOTrigger).
	Side StickSide
	// Direction is the fill direction for progressive triggers.
	Direction TriggerDir
	// StickRadius is the max pixel offset for analog sticks.
	StickRadius int
	// PressedStyle controls how the pressed state is rendered.
	PressedStyle PressedStyle
	// ZLevel: 0=background, 1=interactive elements.
	ZLevel int
}

// CSSProperties holds resolved CSS properties for a single element after parsing.
type CSSProperties struct {
	// Position and size (absolute, already resolved relative to parent)
	Left, Top     int
	Width, Height int
	// Image source URL (resolved absolute URL or local path)
	ImageURL string
	// Crop within the source image for normal state: [x, y, w, h]
	// For elements that are a full-image background: all zeros → use full image.
	CropX, CropY int
	// Pressed-state crop within the same image
	PressedCropX, PressedCropY int
	// Whether element is hidden (display:none)
	Hidden bool
	// PressedStyle determined from CSS
	PressedStyle PressedStyle
	// IsSprite: whether the image needs cropping (has background-position)
	IsSprite bool
	// Float direction (for trigger/bumper layout)
	Float string // "left", "right", ""
	// Transform: rotateY(180deg) (for mirrored bumpers/triggers)
	MirrorX bool
}

// SkinElement is a fully resolved element ready for atlas packing.
type SkinElement struct {
	Spec *GPVElementSpec
	CSS  CSSProperties
	// Absolute screen position (resolved from parent containers)
	ScreenX, ScreenY int
}

// SkinDefinition is the fully parsed skin ready for conversion.
type SkinDefinition struct {
	Name          string
	OverlayWidth  int
	OverlayHeight int
	Elements      []SkinElement
}
