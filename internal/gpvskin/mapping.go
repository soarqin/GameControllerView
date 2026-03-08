package gpvskin

// ResolveElements looks up every element in the skin definition from the parsed
// CSS and returns a SkinDefinition with absolute screen positions.
//
// GPV DOM layout (fixed HTML template, CSS controls everything):
//
//	.controller.<skin>               ← root container (width/height from CSS)
//	  .triggers                      ← sub-container: absolute, left=X, top=0
//	    .trigger.left  (float:left)
//	    .trigger.right (float:right)
//	  .bumpers                       ← sub-container: absolute, left=X, top=Y
//	    .bumper.left  (float:left)
//	    .bumper.right (float:right)
//	  .arrows                        ← sub-container: absolute, left=X, top=Y
//	    .back   (float:left)
//	    .start  (float:right)
//	  .abxy                          ← sub-container: absolute, left=X, top=Y
//	    .button.a  (position:absolute inside)
//	    .button.b / .x / .y
//	  .sticks                        ← sub-container: absolute, left=X, top=Y
//	    .stick.left  (position:absolute inside)
//	    .stick.right
//	  .dpad                          ← sub-container: absolute, left=X, top=Y
//	    .face.up / .down / .left / .right (position:absolute inside)
//	  .quadrant / .meta / .touchpad / .fstick  ← direct children, position:absolute
func ResolveElements(css *ParsedCSS, skinDef *GPVSkinDef, variant string) *SkinDefinition {
	skinClass := skinDef.CSSClass

	// 1. Resolve controller container size.
	ctrlProps := lookupProps(css, [][]string{{"controller", skinClass}})
	if variant != "" {
		vp := lookupProps(css, [][]string{{"controller", skinClass, variant}})
		mergeRawProps(&ctrlProps, vp)
	}
	containerW := ctrlProps.Width
	containerH := ctrlProps.Height
	if containerW == 0 {
		containerW = 500
	}
	if containerH == 0 {
		containerH = 330
	}

	// 2. Resolve sub-container absolute positions.
	sc := resolveSubContainers(css, skinClass, variant)

	// 3. Resolve each element.
	var elements []SkinElement
	for _, spec := range skinDef.Elements {
		elem := resolveOneElement(css, skinClass, variant, spec, &sc, containerW, containerH)
		if elem != nil {
			elements = append(elements, *elem)
		}
	}

	return &SkinDefinition{
		Name:          skinClass,
		OverlayWidth:  containerW,
		OverlayHeight: containerH,
		Elements:      elements,
	}
}

// --------------------------------------------------------------------------
// Sub-container resolution
// --------------------------------------------------------------------------

// subContainerRects holds the pre-resolved absolute positions of GPV sub-containers.
type subContainerRects struct {
	triggers Rect
	bumpers  Rect
	arrows   Rect
	abxy     Rect
	sticks   Rect
	dpad     Rect
}

func resolveSubContainers(css *ParsedCSS, skinClass, variant string) subContainerRects {
	lk := func(childCls []string) Rect {
		p := lookupPropsMulti(css, skinClass, variant, [][]string{childCls})
		return Rect{X: p.Left, Y: p.Top, W: p.Width, H: p.Height}
	}
	return subContainerRects{
		triggers: lk([]string{"triggers"}),
		bumpers:  lk([]string{"bumpers"}),
		arrows:   lk([]string{"arrows"}),
		abxy:     lk([]string{"abxy"}),
		sticks:   lk([]string{"sticks"}),
		dpad:     lk([]string{"dpad"}),
	}
}

// --------------------------------------------------------------------------
// Layout types
// --------------------------------------------------------------------------

type layoutType int

const (
	layoutBackground  layoutType = iota // full controller background, pos (0,0)
	layoutFloatLeft                     // float:left in a sub-container
	layoutFloatRight                    // float:right in a sub-container
	layoutAbsInParent                   // position:absolute inside sub-container
	layoutDirectChild                   // position:absolute, direct child of .controller
)

type elementMetadata struct {
	// selectorPath is the CSS class chain BELOW .controller.<skin>.
	selectorPath [][]string
	layout       layoutType
	// parentRect is the resolved sub-container rect (nil for direct children).
	parentRect *Rect
}

// --------------------------------------------------------------------------
// Element metadata table
// --------------------------------------------------------------------------

func elementMeta(name string, sc *subContainerRects) *elementMetadata {
	switch name {
	case "background":
		return &elementMetadata{
			selectorPath: nil, // handled specially
			layout:       layoutBackground,
		}
	case "trigger-left":
		return &elementMetadata{selectorPath: [][]string{{"trigger", "left"}}, layout: layoutFloatLeft, parentRect: &sc.triggers}
	case "trigger-right":
		return &elementMetadata{selectorPath: [][]string{{"trigger", "right"}}, layout: layoutFloatRight, parentRect: &sc.triggers}
	case "trigger-button-left":
		return &elementMetadata{selectorPath: [][]string{{"trigger-button", "left"}}, layout: layoutFloatLeft, parentRect: &sc.triggers}
	case "trigger-button-right":
		return &elementMetadata{selectorPath: [][]string{{"trigger-button", "right"}}, layout: layoutFloatRight, parentRect: &sc.triggers}
	case "bumper-left":
		return &elementMetadata{selectorPath: [][]string{{"bumper", "left"}}, layout: layoutFloatLeft, parentRect: &sc.bumpers}
	case "bumper-right":
		return &elementMetadata{selectorPath: [][]string{{"bumper", "right"}}, layout: layoutFloatRight, parentRect: &sc.bumpers}
	case "back":
		return &elementMetadata{selectorPath: [][]string{{"back"}}, layout: layoutFloatLeft, parentRect: &sc.arrows}
	case "start":
		return &elementMetadata{selectorPath: [][]string{{"start"}}, layout: layoutFloatRight, parentRect: &sc.arrows}
	case "button-a":
		return &elementMetadata{selectorPath: [][]string{{"button", "a"}}, layout: layoutAbsInParent, parentRect: &sc.abxy}
	case "button-b":
		return &elementMetadata{selectorPath: [][]string{{"button", "b"}}, layout: layoutAbsInParent, parentRect: &sc.abxy}
	case "button-x":
		return &elementMetadata{selectorPath: [][]string{{"button", "x"}}, layout: layoutAbsInParent, parentRect: &sc.abxy}
	case "button-y":
		return &elementMetadata{selectorPath: [][]string{{"button", "y"}}, layout: layoutAbsInParent, parentRect: &sc.abxy}
	case "stick-left":
		return &elementMetadata{selectorPath: [][]string{{"stick", "left"}}, layout: layoutAbsInParent, parentRect: &sc.sticks}
	case "stick-right":
		return &elementMetadata{selectorPath: [][]string{{"stick", "right"}}, layout: layoutAbsInParent, parentRect: &sc.sticks}
	case "dpad-up":
		return &elementMetadata{selectorPath: [][]string{{"face", "up"}}, layout: layoutAbsInParent, parentRect: &sc.dpad}
	case "dpad-down":
		return &elementMetadata{selectorPath: [][]string{{"face", "down"}}, layout: layoutAbsInParent, parentRect: &sc.dpad}
	case "dpad-left":
		return &elementMetadata{selectorPath: [][]string{{"face", "left"}}, layout: layoutAbsInParent, parentRect: &sc.dpad}
	case "dpad-right":
		return &elementMetadata{selectorPath: [][]string{{"face", "right"}}, layout: layoutAbsInParent, parentRect: &sc.dpad}
	case "meta":
		return &elementMetadata{selectorPath: [][]string{{"meta"}}, layout: layoutDirectChild}
	case "touchpad":
		return &elementMetadata{selectorPath: [][]string{{"touchpad"}}, layout: layoutDirectChild}
	case "fstick":
		return &elementMetadata{selectorPath: [][]string{{"fstick"}}, layout: layoutDirectChild}
	case "quadrant":
		return &elementMetadata{selectorPath: [][]string{{"quadrant"}}, layout: layoutDirectChild}
	}
	return nil
}

// --------------------------------------------------------------------------
// Single-element resolver
// --------------------------------------------------------------------------

func resolveOneElement(
	css *ParsedCSS,
	skinClass, variant string,
	spec GPVElementSpec,
	sc *subContainerRects,
	containerW, containerH int,
) *SkinElement {
	meta := elementMeta(spec.Name, sc)
	if meta == nil {
		return nil
	}

	// Background element: uses the controller container itself.
	if meta.layout == layoutBackground {
		imgURL, cropX, cropY := resolveControllerBg(css, skinClass, variant)
		css2 := CSSProperties{
			Width:        containerW,
			Height:       containerH,
			ImageURL:     imgURL,
			CropX:        cropX,
			CropY:        cropY,
			PressedStyle: PressedNone,
		}
		return &SkinElement{Spec: &spec, CSS: css2, ScreenX: 0, ScreenY: 0}
	}

	// Resolve CSS for this element (merged: generic base class + specific + variant).
	p := resolveElementProps(css, skinClass, variant, meta.selectorPath)

	// Resolve pressed-state image/crop.
	pp := resolvePressedProps(css, skinClass, variant, meta.selectorPath)

	// Element width/height: fall back to parent container.
	w := p.Width
	h := p.Height
	if w == 0 && meta.parentRect != nil {
		w = meta.parentRect.W
	}
	if h == 0 && meta.parentRect != nil {
		h = meta.parentRect.H
	}
	// triggers container: height="100%" — use parent height for trigger.right which sets height:100%
	if h == 0 && meta.parentRect != nil {
		h = meta.parentRect.H
	}

	// Compute absolute screen position.
	var screenX, screenY int
	parent := meta.parentRect

	switch meta.layout {
	case layoutAbsInParent:
		screenX = parent.X + p.Left
		screenY = parent.Y + p.Top
		if p.HasRight {
			screenX = parent.X + parent.W - w - p.Right
		}
		if p.HasBottom {
			screenY = parent.Y + parent.H - h - p.Bottom
		}
	case layoutFloatLeft:
		screenX = parent.X
		screenY = parent.Y
	case layoutFloatRight:
		screenX = parent.X + parent.W - w
		screenY = parent.Y
	case layoutDirectChild:
		screenX = p.Left
		screenY = p.Top
		if p.HasRight {
			screenX = containerW - w - p.Right
		}
		if p.HasBottom {
			screenY = containerH - h - p.Bottom
		}
	}

	// Image URL: normal state.
	imgURL := p.ImageURL
	// For PressedOpacity elements, image may only be in pressed selector (e.g. ds4 .meta).
	if imgURL == "" {
		imgURL = pp.ImageURL
	}

	cropX := -p.BgPositionX
	cropY := -p.BgPositionY
	pressedCropX := -pp.BgPositionX
	pressedCropY := -pp.BgPositionY
	// If pressed selector has no position override, fall back to same as normal.
	if pp.ImageURL == "" && pp.BgPositionX == 0 && pp.BgPositionY == 0 {
		pressedCropX = cropX
		pressedCropY = cropY
	}

	css2 := CSSProperties{
		Width:        w,
		Height:       h,
		ImageURL:     imgURL,
		CropX:        cropX,
		CropY:        cropY,
		PressedCropX: pressedCropX,
		PressedCropY: pressedCropY,
		PressedStyle: spec.PressedStyle,
		MirrorX:      p.MirrorX,
	}

	return &SkinElement{
		Spec:    &spec,
		CSS:     css2,
		ScreenX: screenX,
		ScreenY: screenY,
	}
}

// resolveControllerBg returns the image URL for the controller background.
func resolveControllerBg(css *ParsedCSS, skinClass, variant string) (imgURL string, cropX, cropY int) {
	p := lookupProps(css, [][]string{{"controller", skinClass}})
	if variant != "" {
		vp := lookupProps(css, [][]string{{"controller", skinClass, variant}})
		mergeRawProps(&p, vp)
	}
	return p.ImageURL, -p.BgPositionX, -p.BgPositionY
}

// resolveElementProps resolves CSS for a path like [["button","a"]] under .controller.<skin>.
// It merges the generic base class (e.g. .button) with the specific class (e.g. .button.a).
func resolveElementProps(css *ParsedCSS, skinClass, variant string, selectorPath [][]string) rawCSSProps {
	// If the selector has compound classes (e.g. ["button","a"]), first fetch the
	// base class (e.g. ["button"]) so we inherit size, image etc, then override.
	var base rawCSSProps
	if len(selectorPath) == 1 && len(selectorPath[0]) >= 2 {
		baseCls := selectorPath[0][:1] // e.g. ["button"]
		base = lookupPropsMulti(css, skinClass, variant, [][]string{baseCls})
	}
	specific := lookupPropsMulti(css, skinClass, variant, selectorPath)
	mergeRawProps(&base, specific)
	return base
}

// resolvePressedProps fetches CSS from the pressed-state selector.
func resolvePressedProps(css *ParsedCSS, skinClass, variant string, selectorPath [][]string) rawCSSProps {
	// Add "pressed" to the last class set.
	pressedPath := make([][]string, len(selectorPath))
	for i, cls := range selectorPath {
		pressedPath[i] = append([]string{}, cls...)
	}
	last := len(pressedPath) - 1
	pressedPath[last] = append(pressedPath[last], "pressed")
	return lookupPropsMulti(css, skinClass, variant, pressedPath)
}

// --------------------------------------------------------------------------
// CSS property lookup
// --------------------------------------------------------------------------

// rawCSSProps is a fully-parsed set of CSS properties for one element.
type rawCSSProps struct {
	Left, Top, Right, Bottom int
	Width, Height            int
	HasRight, HasBottom      bool
	ImageURL                 string
	BgPositionX, BgPositionY int
	Float                    string
	MirrorX                  bool
	Hidden                   bool
}

// lookupProps fetches and parses CSS properties for a full class-path.
func lookupProps(css *ParsedCSS, classpath [][]string) rawCSSProps {
	raw := css.Lookup(classpath)
	return parseRawProps(raw, css.BaseURL)
}

// lookupPropsMulti builds a full class-path for .controller.<skin>[.<variant>] + selectorPath
// and returns merged CSS properties.
func lookupPropsMulti(css *ParsedCSS, skinClass, variant string, selectorPath [][]string) rawCSSProps {
	containerCls := []string{"controller", skinClass}
	var cp [][]string
	cp = append(cp, containerCls)
	cp = append(cp, selectorPath...)
	p := lookupProps(css, cp)

	if variant != "" {
		varCls := []string{"controller", skinClass, variant}
		var vcp [][]string
		vcp = append(vcp, varCls)
		vcp = append(vcp, selectorPath...)
		vp := lookupProps(css, vcp)
		mergeRawProps(&p, vp)
	}
	return p
}

// parseRawProps converts a raw CSS property map to rawCSSProps.
func parseRawProps(raw map[string]string, baseURL string) rawCSSProps {
	var p rawCSSProps

	p.Left = ParsePx(raw["left"])
	p.Top = ParsePx(raw["top"])
	p.Width = ParsePx(raw["width"])
	p.Height = ParsePx(raw["height"])

	if v, ok := raw["right"]; ok && v != "" && v != "auto" {
		p.Right = ParsePx(v)
		p.HasRight = true
	}
	if v, ok := raw["bottom"]; ok && v != "" && v != "auto" {
		p.Bottom = ParsePx(v)
		p.HasBottom = true
	}
	if d, ok := raw["display"]; ok {
		p.Hidden = ParseDisplay(d)
	}
	if f, ok := raw["float"]; ok {
		p.Float = ParseFloat(f)
	}
	t := raw["transform"]
	if t == "" {
		t = raw["-webkit-transform"]
	}
	if t != "" {
		p.MirrorX = ParseTransform(t)
	}

	// Background image and position.
	imgURL := ""
	bpx, bpy := 0, 0

	if bg, ok := raw["background"]; ok && bg != "" {
		u, x, y := ParseBackground(bg, baseURL)
		imgURL = u
		bpx = x
		bpy = y
	}
	if bi, ok := raw["background-image"]; ok && bi != "" {
		u := ParseBackgroundImage(bi)
		if u != "" {
			imgURL = ResolveURL(u, baseURL)
		}
	}
	if bp, ok := raw["background-position"]; ok && bp != "" {
		x, y := ParseBackgroundPosition(bp)
		bpx = x
		bpy = y
	}
	if v, ok := raw["background-position-x"]; ok && v != "" {
		bpx = ParsePx(v)
	}
	if v, ok := raw["background-position-y"]; ok && v != "" {
		bpy = ParsePx(v)
	}

	p.ImageURL = imgURL
	p.BgPositionX = bpx
	p.BgPositionY = bpy
	return p
}

// mergeRawProps merges non-zero/non-empty fields from override into base.
func mergeRawProps(base *rawCSSProps, override rawCSSProps) {
	if override.Left != 0 {
		base.Left = override.Left
	}
	if override.Top != 0 {
		base.Top = override.Top
	}
	if override.Width != 0 {
		base.Width = override.Width
	}
	if override.Height != 0 {
		base.Height = override.Height
	}
	if override.HasRight {
		base.Right = override.Right
		base.HasRight = true
	}
	if override.HasBottom {
		base.Bottom = override.Bottom
		base.HasBottom = true
	}
	if override.Float != "" {
		base.Float = override.Float
	}
	if override.MirrorX {
		base.MirrorX = true
	}
	if override.ImageURL != "" {
		base.ImageURL = override.ImageURL
	}
	if override.BgPositionX != 0 {
		base.BgPositionX = override.BgPositionX
	}
	if override.BgPositionY != 0 {
		base.BgPositionY = override.BgPositionY
	}
}
