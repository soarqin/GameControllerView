package gpvskin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// IOOverlay is the top-level Input Overlay JSON structure.
type IOOverlay struct {
	OverlayWidth  int         `json:"overlay_width"`
	OverlayHeight int         `json:"overlay_height"`
	Flags         int         `json:"flags"`
	DefaultWidth  int         `json:"default_width"`
	DefaultHeight int         `json:"default_height"`
	SpaceH        int         `json:"space_h"`
	SpaceV        int         `json:"space_v"`
	Version       int         `json:"version"`
	Elements      []IOElement `json:"elements"`
}

// IOElement is a single Input Overlay element JSON object.
// Extra fields (code, side, direction, trigger_mode, stick_radius) are
// marshalled only when non-zero / non-false by using *int / *bool pointers.
type IOElement struct {
	Type    int    `json:"type"`
	ID      string `json:"id"`
	Pos     [2]int `json:"pos"`
	Mapping [4]int `json:"mapping"`
	ZLevel  int    `json:"z_level"`

	// Type 2 (gamepad button): SDL2 button code.
	Code *int `json:"code,omitempty"`

	// Type 5 (analog stick): side + stick_radius.
	Side        *int `json:"side,omitempty"`
	StickRadius *int `json:"stick_radius,omitempty"`

	// Type 6 (trigger): side + direction + trigger_mode.
	TriggerSide *int  `json:"trigger_side,omitempty"`
	Direction   *int  `json:"direction,omitempty"`
	TriggerMode *bool `json:"trigger_mode,omitempty"`
}

// overlayFlags computes the IO flags bitmask from the skin's element list.
func overlayFlags(skin *SkinDefinition) int {
	flags := 4 // bit 2: is a gamepad
	for _, elem := range skin.Elements {
		if elem.Spec.IOType == IOAnalogStick {
			if elem.Spec.Side == SideLeft {
				flags |= 1
			} else {
				flags |= 2
			}
		}
	}
	return flags
}

// GenerateJSON builds an IOOverlay from the atlas entries and skin definition.
func GenerateJSON(skin *SkinDefinition, atlas *Atlas) *IOOverlay {
	overlay := &IOOverlay{
		OverlayWidth:  skin.OverlayWidth,
		OverlayHeight: skin.OverlayHeight,
		Flags:         overlayFlags(skin),
		DefaultWidth:  skin.OverlayWidth,
		DefaultHeight: skin.OverlayHeight,
		Version:       507,
	}

	for _, entry := range atlas.Entries {
		elem := entry.Element
		spec := elem.Spec

		ioElem := IOElement{
			Type:    int(spec.IOType),
			ID:      spec.Name,
			Pos:     [2]int{elem.ScreenX, elem.ScreenY},
			Mapping: [4]int{entry.AtlasU, entry.AtlasV, entry.AtlasW, entry.AtlasH},
			ZLevel:  spec.ZLevel,
		}

		switch spec.IOType {
		case IOGamepadBtn:
			code := spec.ButtonCode
			ioElem.Code = &code
		case IOAnalogStick:
			side := int(spec.Side)
			radius := spec.StickRadius
			if radius == 0 {
				radius = 22
			}
			ioElem.Side = &side
			ioElem.StickRadius = &radius
		case IOTrigger:
			side := int(spec.Side)
			dir := int(spec.Direction)
			// All GPV triggers use PressedOpacity atlas layout:
			//   frame0 = transparent (opacity:0 normal state)
			//   frame1 = sprite image (full opacity when pressed/held)
			// trigger_mode:false (progressive fill) is correct for this layout:
			//   the renderer draws frame0 as background (transparent),
			//   then clips frame1 proportionally to the trigger axis value.
			// trigger_mode:true would be binary (on/off) which loses analog feel.
			trigMode := false
			ioElem.TriggerSide = &side
			ioElem.Direction = &dir
			ioElem.TriggerMode = &trigMode
		}

		overlay.Elements = append(overlay.Elements, ioElem)
	}
	return overlay
}

// WriteOutput writes the atlas PNG and JSON config to the output directory.
// The output directory is created if it doesn't exist.
// Files are named <name>.json and <name>.png.
func WriteOutput(overlay *IOOverlay, atlas *Atlas, outDir, name string) error {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("create output dir %s: %w", outDir, err)
	}

	// Write PNG.
	pngPath := filepath.Join(outDir, name+".png")
	if err := SavePNG(atlas.Image, pngPath); err != nil {
		return err
	}

	// Write JSON.
	jsonPath := filepath.Join(outDir, name+".json")
	data, err := json.MarshalIndent(overlay, "", "    ")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}
	if err := os.WriteFile(jsonPath, data, 0644); err != nil {
		return fmt.Errorf("write JSON %s: %w", jsonPath, err)
	}

	return nil
}

// Convert is the high-level conversion pipeline:
//  1. Load CSS
//  2. Resolve elements from CSS
//  3. Download/rasterize images
//  4. Build texture atlas
//  5. Generate IO JSON
//  6. Write output files
func Convert(cssSource, skinClass, variant, outDir, svgTool string, scale float64) error {
	// 1. Load CSS.
	css, err := LoadCSS(cssSource)
	if err != nil {
		return fmt.Errorf("load CSS: %w", err)
	}

	// 2. Look up skin definition.
	skinDef := SkinByClass(skinClass)
	if skinDef == nil {
		if skinClass == "custom" {
			skinDef = &CustomSkinDef
		} else {
			return fmt.Errorf("unknown skin class %q; valid options: xbox, xbox-old, ps, ds4, nes, gc, n64, fpp, fight-stick, custom", skinClass)
		}
	}

	// 3. Resolve element positions and CSS properties.
	skin := ResolveElements(css, skinDef, variant)

	// 4. Set up image cache and download assets.
	cache, err := NewImageCache("", svgTool, scale)
	if err != nil {
		return err
	}

	// 5. Build atlas.
	atlas, err := BuildAtlas(skin, cache)
	if err != nil {
		return err
	}

	// 6. Generate JSON.
	overlay := GenerateJSON(skin, atlas)

	// Determine output name from outDir (last path component).
	name := filepath.Base(outDir)
	if name == "" || name == "." {
		name = skinClass
		if variant != "" {
			name += "-" + variant
		}
	}

	// 7. Write output.
	return WriteOutput(overlay, atlas, outDir, name)
}
