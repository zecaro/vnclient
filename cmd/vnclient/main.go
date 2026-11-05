package main

import (
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/tenthirtyam/go-vnc"
	"golang.org/x/term"

	"github.com/zecaro/vnclient/internal/vnclient"
)

func main() {
	cfg := vnclient.LoadConfig()
	vnclient.AddFlags(flag.CommandLine, &cfg)
	flag.Parse()
	if cfg.VNCPass == "" {
		fmt.Fprint(os.Stderr, "VNC password: ")
		pw, err := term.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr)
		cfg.VNCPass = string(pw)
	}

	a := app.New()
	w := a.NewWindow(cfg.Title)
	w.Resize(fyne.NewSize(1280, 800))

	view := vnclient.NewView()
	status := widget.NewLabel("conectando...")
	entry := widget.NewEntry()
	entry.SetPlaceHolder("/home/ubuntu/Downloads/arquivo")
	btn := widget.NewButton("Baixar", func() {
		name := entry.Text
		go func() {
			if err := vnclient.Download(cfg, name); err != nil {
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
				if err := vnclient.Upload(cfg, p); err != nil {
					fyne.Do(func() { dialog.ShowError(err, w) })
					return
				}
				fyne.Do(func() { status.SetText("enviado: " + filepath.Base(p)) })
			}(u.Path())
		}
	})

	ch := make(chan vnc.ServerMessage, 64)
	var cMu sync.Mutex
	var c *vnclient.Conn

	clip := a.Clipboard()
	go func() {
		cc, err := vnclient.Connect(cfg, ch)
		if err != nil {
			fyne.Do(func() { status.SetText("erro: " + err.Error()) })
			return
		}
		cMu.Lock()
		c = cc
		cMu.Unlock()
		defer cc.Close()
		if err := vnclient.Run(cc.ClientConn, ch, view, status, clip); err != nil {
			fyne.Do(func() { status.SetText("erro: " + err.Error()) })
		}
	}()

	view.OnPointer = func(x, y uint16, mask vnc.ButtonMask) {
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
		ks := vnclient.Keysym(e.Name)
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
