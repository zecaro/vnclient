package vnclient

import (
	"image"
	"image/color"
	"image/draw"

	"github.com/tenthirtyam/go-vnc"
)

func toRGBA(c vnc.Color) color.RGBA {
	return color.RGBA{R: uint8(c.R), G: uint8(c.G), B: uint8(c.B), A: 255}
}

func fill(fb *image.RGBA, x0, y0, w, h int, c vnc.Color) {
	rgba := toRGBA(c)
	for y := range h {
		for x := range w {
			fb.SetRGBA(x0+x, y0+y, rgba)
		}
	}
}

func drawRect(fb *image.RGBA, r vnc.Rectangle) {
	x0, y0 := int(r.X), int(r.Y)
	w, h := int(r.Width), int(r.Height)
	switch enc := r.Enc.(type) {
	case *vnc.RawEncoding:
		for y := range h {
			for x := range w {
				fb.SetRGBA(x0+x, y0+y, toRGBA(enc.Colors[y*w+x]))
			}
		}
	case *vnc.HextileEncoding:
		tilesX := (w + 15) / 16
		for i, t := range enc.Tiles {
			tx := x0 + (i%tilesX)*16
			ty := y0 + (i/tilesX)*16
			tw, th := int(t.Width), int(t.Height)
			if len(t.Colors) > 0 {
				for y := range th {
					for x := range tw {
						fb.SetRGBA(tx+x, ty+y, toRGBA(t.Colors[y*tw+x]))
					}
				}
			} else {
				fill(fb, tx, ty, tw, th, t.Background)
				for _, s := range t.Subrectangles {
					fill(fb, tx+int(s.X), ty+int(s.Y), int(s.Width), int(s.Height), s.Color)
				}
			}
		}
	case *vnc.RREEncoding:
		fill(fb, x0, y0, w, h, enc.BackgroundColor)
		for _, s := range enc.Subrectangles {
			fill(fb, x0+int(s.X), y0+int(s.Y), int(s.Width), int(s.Height), s.Color)
		}
	case *vnc.CopyRectEncoding:
		src := image.Rect(int(enc.SrcX), int(enc.SrcY), int(enc.SrcX)+w, int(enc.SrcY)+h)
		draw.Draw(fb, image.Rect(x0, y0, x0+w, y0+h), fb, src.Min, draw.Src)
	}
}
