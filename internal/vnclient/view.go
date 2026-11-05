package vnclient

import (
	"image"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
	"github.com/tenthirtyam/go-vnc"
)

// View is a custom widget that displays the VNC framebuffer.
type View struct {
	widget.BaseWidget
	img     *canvas.Image
	mu      sync.Mutex
	fb      *image.RGBA
	buttons vnc.ButtonMask

	// OnPointer receives pointer events in framebuffer coordinates.
	OnPointer func(uint16, uint16, vnc.ButtonMask)
}

// NewView returns a view with an initial 1280x800 buffer.
func NewView() *View {
	buf := image.NewRGBA(image.Rect(0, 0, 1280, 800))
	img := canvas.NewImageFromImage(buf)
	img.FillMode = canvas.ImageFillStretch
	img.ScaleMode = canvas.ImageScaleFastest
	v := &View{img: img, fb: buf}
	v.ExtendBaseWidget(v)
	return v
}

func (v *View) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(v.img)
}

func (v *View) setFB(src *image.RGBA) {
	fyne.DoAndWait(func() {
		v.mu.Lock()
		defer v.mu.Unlock()
		if v.fb == nil || v.fb.Rect != src.Rect {
			v.fb = image.NewRGBA(src.Rect)
			v.img.Image = v.fb
		}
		copy(v.fb.Pix, src.Pix)
		v.img.Refresh()
	})
}

func (v *View) clamp(x, y float32) (uint16, uint16) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.fb == nil {
		return 0, 0
	}
	w, h := v.Size().Width, v.Size().Height
	if w <= 0 || h <= 0 {
		return 0, 0
	}
	b := v.fb.Bounds()
	ix := int(float32(b.Dx()) * x / w)
	iy := int(float32(b.Dy()) * y / h)
	if ix < 0 {
		ix = 0
	}
	if iy < 0 {
		iy = 0
	}
	if ix >= b.Dx() {
		ix = b.Dx() - 1
	}
	if iy >= b.Dy() {
		iy = b.Dy() - 1
	}
	return uint16(ix), uint16(iy)
}

func (v *View) MouseDown(e *desktop.MouseEvent) {
	if v.OnPointer == nil {
		return
	}
	v.buttons |= maskFor(e.Button)
	x, y := v.clamp(e.Position.X, e.Position.Y)
	v.OnPointer(x, y, v.buttons)
}

func (v *View) MouseUp(e *desktop.MouseEvent) {
	if v.OnPointer == nil {
		return
	}
	v.buttons &^= maskFor(e.Button)
	x, y := v.clamp(e.Position.X, e.Position.Y)
	v.OnPointer(x, y, v.buttons)
}

func (v *View) MouseMoved(e *desktop.MouseEvent) {
	if v.OnPointer == nil {
		return
	}
	x, y := v.clamp(e.Position.X, e.Position.Y)
	v.OnPointer(x, y, v.buttons)
}

func (v *View) MouseIn(*desktop.MouseEvent) {}

func (v *View) MouseOut() {
	v.buttons = 0
}

func maskFor(b desktop.MouseButton) vnc.ButtonMask {
	switch b {
	case desktop.MouseButtonPrimary:
		return 1
	case desktop.MouseButtonTertiary:
		return 2
	case desktop.MouseButtonSecondary:
		return 4
	}
	return 0
}
