package gamepad

import "math"

type Vector struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type StickState struct {
	Position Vector `json:"position"`
	Pressed  bool   `json:"pressed"`
}

type TriggerState struct {
	Value float64 `json:"value"`
}

type ButtonState struct {
	A      bool `json:"a"`
	B      bool `json:"b"`
	X      bool `json:"x"`
	Y      bool `json:"y"`
	LB     bool `json:"lb"`
	RB     bool `json:"rb"`
	Select bool `json:"select"`
	Start  bool `json:"start"`
	Home   bool `json:"home"`
}

type DpadState struct {
	Up    bool `json:"up"`
	Down  bool `json:"down"`
	Left  bool `json:"left"`
	Right bool `json:"right"`
}

type SticksState struct {
	Left  StickState `json:"left"`
	Right StickState `json:"right"`
}

type TriggersState struct {
	LT TriggerState `json:"lt"`
	RT TriggerState `json:"rt"`
}

type GamepadState struct {
	Connected      bool          `json:"connected"`
	ControllerType string        `json:"controllerType"`
	Name           string        `json:"name"`
	Buttons        ButtonState   `json:"buttons"`
	Dpad           DpadState     `json:"dpad"`
	Sticks         SticksState   `json:"sticks"`
	Triggers       TriggersState `json:"triggers"`
}

type DeltaChanges struct {
	Connected      *bool          `json:"connected,omitempty"`
	ControllerType *string        `json:"controllerType,omitempty"`
	Name           *string        `json:"name,omitempty"`
	Buttons        *ButtonState   `json:"buttons,omitempty"`
	Dpad           *DpadState     `json:"dpad,omitempty"`
	Sticks         *SticksState   `json:"sticks,omitempty"`
	Triggers       *TriggersState `json:"triggers,omitempty"`
}

func (d *DeltaChanges) IsEmpty() bool {
	return d.Connected == nil &&
		d.ControllerType == nil &&
		d.Name == nil &&
		d.Buttons == nil &&
		d.Dpad == nil &&
		d.Sticks == nil &&
		d.Triggers == nil
}

const analogThreshold = 0.01

func floatEqual(a, b float64) bool {
	return math.Abs(a-b) < analogThreshold
}

func ComputeDelta(old, new_ GamepadState) *DeltaChanges {
	d := &DeltaChanges{}

	if old.Connected != new_.Connected {
		d.Connected = &new_.Connected
	}
	if old.ControllerType != new_.ControllerType {
		d.ControllerType = &new_.ControllerType
	}
	if old.Name != new_.Name {
		d.Name = &new_.Name
	}
	if old.Buttons != new_.Buttons {
		d.Buttons = &new_.Buttons
	}
	if old.Dpad != new_.Dpad {
		d.Dpad = &new_.Dpad
	}

	if !floatEqual(old.Sticks.Left.Position.X, new_.Sticks.Left.Position.X) ||
		!floatEqual(old.Sticks.Left.Position.Y, new_.Sticks.Left.Position.Y) ||
		old.Sticks.Left.Pressed != new_.Sticks.Left.Pressed ||
		!floatEqual(old.Sticks.Right.Position.X, new_.Sticks.Right.Position.X) ||
		!floatEqual(old.Sticks.Right.Position.Y, new_.Sticks.Right.Position.Y) ||
		old.Sticks.Right.Pressed != new_.Sticks.Right.Pressed {
		d.Sticks = &new_.Sticks
	}

	if !floatEqual(old.Triggers.LT.Value, new_.Triggers.LT.Value) ||
		!floatEqual(old.Triggers.RT.Value, new_.Triggers.RT.Value) {
		d.Triggers = &new_.Triggers
	}

	return d
}
