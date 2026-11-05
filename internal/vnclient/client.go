package vnclient

import (
	"context"
	"fmt"
	"image"
	"net"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
	"github.com/tenthirtyam/go-vnc"
	"golang.org/x/crypto/ssh"
)

// Conn is a VNC connection that may own an SSH tunnel.
type Conn struct {
	*vnc.ClientConn
	ssh *ssh.Client
}

// Close closes the VNC connection and any SSH tunnel.
func (c *Conn) Close() error {
	err := c.ClientConn.Close()
	if c.ssh != nil {
		_ = c.ssh.Close()
	}
	return err
}

// Connect dials the VNC server directly or through SSH.
func Connect(cfg Config, ch chan vnc.ServerMessage) (*Conn, error) {
	var (
		conn net.Conn
		sc   *ssh.Client
		err  error
	)
	if cfg.VNCAddr != "" {
		conn, err = net.DialTimeout("tcp", cfg.VNCAddr, 10*time.Second)
	} else {
		sc, err = sshClient(cfg)
		if err == nil && cfg.StartCmd != "" {
			err = runRemote(sc, cfg.StartCmd)
		}
		if err == nil {
			conn, err = sc.Dial("tcp", net.JoinHostPort("127.0.0.1", cfg.VNCPort))
		}
	}
	if err != nil {
		if sc != nil {
			_ = sc.Close()
		}
		return nil, err
	}
	c, err := vnc.ClientWithContext(context.Background(), conn, &vnc.ClientConfig{
		Auth:            []vnc.ClientAuth{&vnc.PasswordAuth{Password: cfg.VNCPass}},
		ServerMessageCh: ch,
	})
	if err != nil {
		_ = conn.Close()
		if sc != nil {
			_ = sc.Close()
		}
		return nil, err
	}
	return &Conn{ClientConn: c, ssh: sc}, nil
}

func runRemote(sc *ssh.Client, cmd string) error {
	sess, err := sc.NewSession()
	if err != nil {
		return err
	}
	defer sess.Close()
	return sess.Run(cmd)
}

// Run applies the initial settings and processes server messages until the channel closes.
func Run(c *vnc.ClientConn, ch chan vnc.ServerMessage, view *View, status *widget.Label, clip fyne.Clipboard) error {
	encs := []vnc.Encoding{
		&vnc.RawEncoding{},
		&vnc.CopyRectEncoding{},
		&vnc.RREEncoding{},
		&vnc.HextileEncoding{},
		&vnc.CursorPseudoEncoding{},
		&vnc.DesktopSizePseudoEncoding{},
	}
	pf := &vnc.PixelFormat{
		BPP:        32,
		Depth:      24,
		BigEndian:  false,
		TrueColor:  true,
		RedMax:     255,
		GreenMax:   255,
		BlueMax:    255,
		RedShift:   16,
		GreenShift: 8,
		BlueShift:  0,
	}
	if err := c.SetPixelFormat(pf); err != nil {
		return err
	}
	if err := c.SetEncodings(encs); err != nil {
		return err
	}
	w, h := int(c.FrameBufferWidth), int(c.FrameBufferHeight)
	fb := image.NewRGBA(image.Rect(0, 0, w, h))
	view.setFB(fb)
	fyne.Do(func() { status.SetText(fmt.Sprintf("%s %dx%d", c.DesktopName, w, h)) })
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
