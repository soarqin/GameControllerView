package gpvskin

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
)

const (
	// atlasPad is the gap between sprite frames in the atlas (IO convention: 3px).
	atlasPad = 3
	// atlasMargin is the gap between distinct sprite cells in the atlas.
	atlasMargin = 2
)

// SpriteEntry holds an element and its allocated position in the atlas.
type SpriteEntry struct {
	Element *SkinElement
	// Atlas coordinates for frame 0 (normal state).
	AtlasU, AtlasV int
	// Per-frame sprite size.
	AtlasW, AtlasH int
}

// Atlas is a packed texture atlas.
type Atlas struct {
	Image   *image.NRGBA
	Entries []SpriteEntry
}

// BuildAtlas packs all skin element sprites into a single texture atlas PNG.
//
// IO sprite layout conventions (from input-overlay-format.md):
//
//	Button / Trigger / Stick:
//	  frame 0 (normal/released) at [u, v, w, h]
//	  frame 1 (pressed/active)  at [u, v+h+3, w, h]   (vertical, 3px gap)
//
//	GamepadID: 5 frames horizontal, each (w+3) apart
//	Dpad:      9 frames horizontal, each (w+3) apart
//
// GPV PressedStyle handling:
//
//	PressedSprite  — src image has normal and pressed sprites at different background-position
//	PressedOpacity — element is opacity:0 normally; pressed = visible sprite
//	                 → frame 0 = transparent, frame 1 = the visible sprite
//	PressedInvert  — same sprite, but inverted when pressed (fight-stick)
//	                 → frame 0 = sprite, frame 1 = sprite (invert applied at render time, not here)
//	PressedNone    — only one frame
func BuildAtlas(skin *SkinDefinition, cache *ImageCache) (*Atlas, error) {
	type atlasRect struct{ u, v, w, h int }

	type spriteWork struct {
		elem *SkinElement

		// Source image for normal state (may be nil if PressedOpacity with no normal image).
		normalImg  image.Image
		normalCrop image.Rectangle

		// Source image for pressed state (often same as normalImg, different crop).
		pressedImg  image.Image
		pressedCrop image.Rectangle

		frames  int       // number of IO frames: 1, 2, 5 (gamepad_id), or 9 (dpad)
		frameW  int       // width of one frame in atlas
		frameH  int       // height of one frame in atlas
		atlasW  int       // total width reserved in atlas (multi-frame elements are wider)
		atlasH  int       // total height reserved in atlas
		atlasAt atlasRect // assigned atlas position (set during packing)
	}

	var works []spriteWork

	for i := range skin.Elements {
		elem := &skin.Elements[i]
		css := elem.CSS
		spec := elem.Spec

		w := css.Width
		h := css.Height
		if w <= 0 || h <= 0 {
			continue
		}
		if css.ImageURL == "" {
			// No image source — skip (element has no visual representation).
			continue
		}

		srcImg, err := cache.Get(css.ImageURL)
		if err != nil {
			fmt.Printf("warning: skip %s: %v\n", spec.Name, err)
			continue
		}

		// CSS background-position uses positive values to shift the image right,
		// making the crop start at a negative coordinate.  When background-position
		// is positive, the sprite is actually measured from the RIGHT edge of the
		// image: cropX = imgWidth - elementWidth + bgPosX (which equals imgWidth + cropX
		// when cropX is already stored as -bgPosX).
		cropX := css.CropX
		cropY := css.CropY
		pressedCropX := css.PressedCropX
		pressedCropY := css.PressedCropY
		imgW := srcImg.Bounds().Dx()
		imgH := srcImg.Bounds().Dy()
		if cropX < 0 {
			cropX = imgW + cropX // imgW - |bgPosX|
		}
		if cropY < 0 {
			cropY = imgH + cropY
		}
		if pressedCropX < 0 {
			pressedCropX = imgW + pressedCropX
		}
		if pressedCropY < 0 {
			pressedCropY = imgH + pressedCropY
		}
		// Clamp to valid range.
		if cropX < 0 {
			cropX = 0
		}
		if cropY < 0 {
			cropY = 0
		}

		normalCrop := image.Rect(cropX, cropY, cropX+w, cropY+h)
		pressedCrop := image.Rect(pressedCropX, pressedCropY, pressedCropX+w, pressedCropY+h)

		// Determine frame layout.
		frames := 1
		atlasW := w
		atlasH := h

		switch spec.IOType {
		case IOTexture:
			frames = 1
		case IOGamepadID:
			// 5 frames horizontal.
			frames = 5
			atlasW = w*5 + atlasPad*4
		case IODpad:
			// 9 frames horizontal.
			frames = 9
			atlasW = w*9 + atlasPad*8
		default:
			if spec.PressedStyle != PressedNone {
				frames = 2
				atlasH = h*2 + atlasPad
			}
		}

		works = append(works, spriteWork{
			elem:        elem,
			normalImg:   srcImg,
			normalCrop:  normalCrop,
			pressedImg:  srcImg,
			pressedCrop: pressedCrop,
			frames:      frames,
			frameW:      w,
			frameH:      h,
			atlasW:      atlasW,
			atlasH:      atlasH,
		})
	}

	if len(works) == 0 {
		// Return a 1×1 transparent atlas so we don't fail on empty skins.
		img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
		return &Atlas{Image: img}, nil
	}

	// Bin-pack into atlas rows.
	const maxWidth = 4096
	curX, curY, rowH := 0, 0, 0
	for i := range works {
		sw := &works[i]
		if curX > 0 && curX+sw.atlasW+atlasMargin > maxWidth {
			curY += rowH + atlasMargin
			curX = 0
			rowH = 0
		}
		sw.atlasAt = atlasRect{u: curX, v: curY, w: sw.atlasW, h: sw.atlasH}
		curX += sw.atlasW + atlasMargin
		if sw.atlasH > rowH {
			rowH = sw.atlasH
		}
	}
	totalH := curY + rowH

	// Compute total width.
	totalW := 0
	for i := range works {
		sw := &works[i]
		if r := sw.atlasAt.u + sw.atlasAt.w; r > totalW {
			totalW = r
		}
	}
	if totalW < 1 {
		totalW = 1
	}
	if totalH < 1 {
		totalH = 1
	}

	// Allocate atlas image (transparent).
	atlasImg := image.NewNRGBA(image.Rect(0, 0, totalW, totalH))
	draw.Draw(atlasImg, atlasImg.Bounds(), image.Transparent, image.Point{}, draw.Src)

	var entries []SpriteEntry

	for i := range works {
		sw := &works[i]
		atU := sw.atlasAt.u
		atV := sw.atlasAt.v
		fw := sw.frameW
		fh := sw.frameH

		switch sw.elem.Spec.IOType {
		case IOGamepadID:
			// 5 horizontal frames — all identical (GPV only has one quadrant sprite).
			for f := 0; f < 5; f++ {
				dx := atU + f*(fw+atlasPad)
				drawCropped(atlasImg, sw.normalImg, sw.normalCrop, image.Pt(dx, atV), sw.elem.CSS.MirrorX)
			}

		case IODpad:
			// 9 horizontal frames — all use the neutral/center sprite.
			// The fight-stick joystick is a single sprite; direction-based rendering
			// happens at runtime, not in the atlas.
			for f := 0; f < 9; f++ {
				dx := atU + f*(fw+atlasPad)
				drawCropped(atlasImg, sw.normalImg, sw.normalCrop, image.Pt(dx, atV), sw.elem.CSS.MirrorX)
			}

		default:
			switch sw.elem.Spec.PressedStyle {
			case PressedNone:
				// Single frame only.
				drawCropped(atlasImg, sw.normalImg, sw.normalCrop, image.Pt(atU, atV), sw.elem.CSS.MirrorX)

			case PressedOpacity:
				// Normal state = invisible (opacity:0). Frame 0 stays transparent.
				// Pressed state = the sprite. Frame 1 = draw sprite at pressed crop.
				// (If normalCrop == pressedCrop, use normalCrop as the pressed sprite.)
				pressedY := atV + fh + atlasPad
				pressedSrc := sw.pressedImg
				pressedCrop := sw.pressedCrop
				// For elements where pressed image differs from normal:
				// normalCrop and pressedCrop may both have valid positions.
				// The "visible" sprite is whichever one has actual content.
				drawCropped(atlasImg, pressedSrc, pressedCrop, image.Pt(atU, pressedY), sw.elem.CSS.MirrorX)

			case PressedSprite:
				// Frame 0 = normal, Frame 1 = pressed (different background-position).
				drawCropped(atlasImg, sw.normalImg, sw.normalCrop, image.Pt(atU, atV), sw.elem.CSS.MirrorX)
				pressedY := atV + fh + atlasPad
				drawCropped(atlasImg, sw.pressedImg, sw.pressedCrop, image.Pt(atU, pressedY), sw.elem.CSS.MirrorX)

			case PressedInvert:
				// Same sprite for both frames; invert is applied at runtime.
				drawCropped(atlasImg, sw.normalImg, sw.normalCrop, image.Pt(atU, atV), sw.elem.CSS.MirrorX)
				pressedY := atV + fh + atlasPad
				drawCropped(atlasImg, sw.normalImg, sw.normalCrop, image.Pt(atU, pressedY), sw.elem.CSS.MirrorX)
			}
		}

		entries = append(entries, SpriteEntry{
			Element: sw.elem,
			AtlasU:  atU,
			AtlasV:  atV,
			AtlasW:  fw,
			AtlasH:  fh,
		})
	}

	return &Atlas{Image: atlasImg, Entries: entries}, nil
}

// drawCropped copies a cropped rectangle from src into dst at dstPt.
// If mirrorX is true the copied region is horizontally flipped.
func drawCropped(dst *image.NRGBA, src image.Image, crop image.Rectangle, dstPt image.Point, mirrorX bool) {
	if src == nil {
		return
	}
	w := crop.Dx()
	h := crop.Dy()
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			srcX := crop.Min.X + x
			srcY := crop.Min.Y + y
			if !image.Pt(srcX, srcY).In(src.Bounds()) {
				continue
			}
			c := src.At(srcX, srcY)
			dstX := dstPt.X + x
			if mirrorX {
				dstX = dstPt.X + (w - 1 - x)
			}
			dstY := dstPt.Y + y
			if image.Pt(dstX, dstY).In(dst.Bounds()) {
				r32, g32, b32, a32 := c.RGBA()
				dst.SetNRGBA(dstX, dstY, color.NRGBA{
					R: uint8(r32 >> 8),
					G: uint8(g32 >> 8),
					B: uint8(b32 >> 8),
					A: uint8(a32 >> 8),
				})
			}
		}
	}
}
