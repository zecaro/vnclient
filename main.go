package main

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
	"github.com/pkg/sftp"
	"github.com/tenthirtyam/go-vnc"
	"golang.org/x/crypto/ssh"
)

var (
	vncAddr  = env("VNC_ADDR", "127.0.0.1:15901")
	vncPass  = os.Getenv("VNC_PASS")
	sshAddr  = env("SSH_ADDR", "10.0.0.120:22")
	sshUser  = env("SSH_USER", "ubuntu")
	remoteDL = env("REMOTE_DL", "/home/ubuntu/Downloads")
)

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

type vncView struct {
	widget.BaseWidget
	img *canvas.Image
	mu  sync.Mutex
	fb  *image.RGBA
	onPointer func(uint16, uint16, vnc.ButtonMask)
}

func newVNCView() *vncView {
	buf := image.NewRGBA(image.Rect(0, 0, 1280, 800))
	img := canvas.NewImageFromImage(buf)
	img.FillMode = canvas.ImageFillStretch
	img.ScaleMode = canvas.ImageScaleFastest
	return &vncView{img: img, fb: buf}
}

func (v *vncView) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(v.img)
}

func (v *vncView) setFB(src *image.RGBA) {
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

func (v *vncView) clamp(x, y float32) (uint16, uint16) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.fb == nil {
		return 0, 0
	}
	b := v.fb.Bounds()
	ix, iy := int(x), int(y)
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

func (v *vncView) MouseDown(e *desktop.MouseEvent) {
	if v.onPointer == nil {
		return
	}
	x, y := v.clamp(e.Position.X, e.Position.Y)
	v.onPointer(x, y, maskFor(e.Button))
}

func (v *vncView) MouseUp(e *desktop.MouseEvent) {
	if v.onPointer == nil {
		return
	}
	x, y := v.clamp(e.Position.X, e.Position.Y)
	v.onPointer(x, y, 0)
}

func (v *vncView) MouseMoved(e *desktop.MouseEvent) {
	if v.onPointer == nil {
		return
	}
	x, y := v.clamp(e.Position.X, e.Position.Y)
	v.onPointer(x, y, 0)
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

func toRGBA(c vnc.Color) color.RGBA {
	return color.RGBA{R: uint8(c.R >> 8), G: uint8(c.G >> 8), B: uint8(c.B >> 8), A: 255}
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
		yy := y0
		for _, t := range enc.Tiles {
			xx := x0
			tw, th := int(t.Width), int(t.Height)
			if len(t.Colors) > 0 {
				for y := range th {
					for x := range tw {
						fb.SetRGBA(xx+x, yy+y, toRGBA(t.Colors[y*tw+x]))
					}
				}
			} else {
				fill(fb, xx, yy, tw, th, t.Background)
				for _, s := range t.Subrectangles {
					fill(fb, xx+int(s.X), yy+int(s.Y), int(s.Width), int(s.Height), s.Color)
				}
			}
			xx += tw
			yy += th
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

func keysym(k fyne.KeyName) uint32 {
	switch k {
	case fyne.KeyReturn, fyne.KeyEnter:
		return 0xff0d
	case fyne.KeyBackspace:
		return 0xff08
	case fyne.KeyTab:
		return 0xff09
	case fyne.KeyEscape:
		return 0xff1b
	case fyne.KeyDelete:
		return 0xffff
	case fyne.KeyHome:
		return 0xff50
	case fyne.KeyEnd:
		return 0xff57
	case fyne.KeyPageUp:
		return 0xff55
	case fyne.KeyPageDown:
		return 0xff56
	case fyne.KeyLeft:
		return 0xff51
	case fyne.KeyUp:
		return 0xff52
	case fyne.KeyRight:
		return 0xff53
	case fyne.KeyDown:
		return 0xff54
	case desktop.KeyShiftLeft, desktop.KeyShiftRight:
		return 0xffe1
	case desktop.KeyControlLeft, desktop.KeyControlRight:
		return 0xffe3
	case desktop.KeyAltLeft, desktop.KeyAltRight:
		return 0xffe9
	case desktop.KeySuperLeft, desktop.KeySuperRight:
		return 0xffeb
	}
	return 0
}

func vncConnect(ch chan vnc.ServerMessage) (*vnc.ClientConn, error) {
	conn, err := net.DialTimeout("tcp", vncAddr, 10*time.Second)
	if err != nil {
		return nil, err
	}
	cfg := &vnc.ClientConfig{
		Auth:            []vnc.ClientAuth{&vnc.PasswordAuth{Password: vncPass}},
		ServerMessageCh: ch,
	}
	c, err := vnc.ClientWithContext(context.Background(), conn, cfg)
	if err != nil {
		conn.Close()
		return nil, err
	}
	return c, nil
}

func runVNC(c *vnc.ClientConn, ch chan vnc.ServerMessage, view *vncView, status *widget.Label, clip fyne.Clipboard) error {
	encs := []vnc.Encoding{
		&vnc.RawEncoding{},
		&vnc.CopyRectEncoding{},
		&vnc.RREEncoding{},
		&vnc.HextileEncoding{},
		&vnc.CursorPseudoEncoding{},
		&vnc.DesktopSizePseudoEncoding{},
	}
	if err := c.SetEncodings(encs); err != nil {
		return err
	}
	w, h := int(c.FrameBufferWidth), int(c.FrameBufferHeight)
	fb := image.NewRGBA(image.Rect(0, 0, w, h))
	view.setFB(fb)
	status.SetText(fmt.Sprintf("%s %dx%d", c.DesktopName, w, h))
	if err := c.FramebufferUpdateRequest(false, 0, 0, uint16(w), uint16(h)); err != nil {
		return err
	}

	var clipMu sync.Mutex
	var lastClip string
	go func() {
		t := time.NewTicker(500 * time.Millisecond)
		defer t.Stop()
		for range t.C {
			var s string
			fyne.DoAndWait(func() { s = clip.Content() })
			clipMu.Lock()
			changed := s != lastClip
			if changed {
				lastClip = s
			}
			clipMu.Unlock()
			if changed && s != "" {
				_ = c.CutText(s)
			}
		}
	}()

	for m := range ch {
		switch msg := m.(type) {
		case *vnc.FramebufferUpdateMessage:
			for _, r := range msg.Rectangles {
				drawRect(fb, r)
			}
			view.setFB(fb)
			_ = c.FramebufferUpdateRequest(true, 0, 0, uint16(w), uint16(h))
		case *vnc.ServerCutTextMessage:
			clipMu.Lock()
			lastClip = msg.Text
			clipMu.Unlock()
			fyne.Do(func() { clip.SetContent(msg.Text) })
		}
	}
	return nil
}

func sshClient() (*ssh.Client, error) {
	home, _ := os.UserHomeDir()
	keyPath := env("SSH_KEY", filepath.Join(home, ".ssh", "id_ed25519"))
	key, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, err
	}
	cfg := &ssh.ClientConfig{
		User:            sshUser,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}
	return ssh.Dial("tcp", sshAddr, cfg)
}

func upload(local string) error {
	sc, err := sshClient()
	if err != nil {
		return err
	}
	defer sc.Close()
	cl, err := sftp.NewClient(sc)
	if err != nil {
		return err
	}
	defer cl.Close()
	if err := cl.MkdirAll(remoteDL); err != nil {
		return err
	}
	src, err := os.Open(local)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := cl.Create(path.Join(remoteDL, filepath.Base(local)))
	if err != nil {
		return err
	}
	defer dst.Close()
	_, err = io.Copy(dst, src)
	return err
}

func download(remote string) error {
	sc, err := sshClient()
	if err != nil {
		return err
	}
	defer sc.Close()
	cl, err := sftp.NewClient(sc)
	if err != nil {
		return err
	}
	defer cl.Close()
	src, err := cl.Open(remote)
	if err != nil {
		return err
	}
	defer src.Close()
	home, _ := os.UserHomeDir()
	out := filepath.Join(home, "Downloads", path.Base(remote))
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	dst, err := os.Create(out)
	if err != nil {
		return err
	}
	defer dst.Close()
	_, err = io.Copy(dst, src)
	return err
}

func main() {
	a := app.New()
	w := a.NewWindow("Hermes VNC")
	w.Resize(fyne.NewSize(1280, 800))

	view := newVNCView()
	status := widget.NewLabel("conectando...")
	entry := widget.NewEntry()
	entry.SetPlaceHolder("/home/ubuntu/Downloads/arquivo")
	btn := widget.NewButton("Baixar", func() {
		name := entry.Text
		go func() {
			if err := download(name); err != nil {
				fyne.Do(func() { dialog.ShowError(err, w) })
				return
			}
			fyne.Do(func() { status.SetText("baixado: " + path.Base(name)) })
		}()
	})
	bottom := container.NewBorder(nil, nil, nil, btn, entry)
	w.SetContent(container.NewBorder(nil, container.NewVBox(bottom, status), nil, nil, view))

	w.SetOnDropped(func(_ fyne.Position, uris []fyne.URI) {
		for _, u := range uris {
			go func(p string) {
				if err := upload(p); err != nil {
					fyne.Do(func() { dialog.ShowError(err, w) })
					return
				}
				fyne.Do(func() { status.SetText("enviado: " + filepath.Base(p)) })
			}(u.Path())
		}
	})

	ch := make(chan vnc.ServerMessage, 64)
	var cMu sync.Mutex
	var c *vnc.ClientConn

	clip := a.Clipboard()
	go func() {
		cc, err := vncConnect(ch)
		if err != nil {
			fyne.Do(func() { status.SetText("erro: " + err.Error()) })
			return
		}
		cMu.Lock()
		c = cc
		cMu.Unlock()
		defer cc.Close()
		if err := runVNC(cc, ch, view, status, clip); err != nil {
			fyne.Do(func() { status.SetText("erro: " + err.Error()) })
		}
	}()

	view.onPointer = func(x, y uint16, mask vnc.ButtonMask) {
		cMu.Lock()
		defer cMu.Unlock()
		if c != nil {
			_ = c.PointerEvent(mask, x, y)
		}
	}
	w.Canvas().SetOnTypedRune(func(r rune) {
		cMu.Lock()
		defer cMu.Unlock()
		if c != nil {
			_ = c.KeyEvent(uint32(r), true)
			_ = c.KeyEvent(uint32(r), false)
		}
	})
	w.Canvas().SetOnTypedKey(func(e *fyne.KeyEvent) {
		ks := keysym(e.Name)
		if ks == 0 {
			return
		}
		cMu.Lock()
		defer cMu.Unlock()
		if c != nil {
			_ = c.KeyEvent(ks, true)
			_ = c.KeyEvent(ks, false)
		}
	})

	w.ShowAndRun()
}
