# vnclient

Minimal VNC client with clipboard sync, drag-and-drop upload, and file download over SFTP.

Built with [Fyne](https://fyne.io) and [go-vnc](https://github.com/tenthirtyam/go-vnc).

## Build

```sh
go build -o vnclient ./cmd/vnclient
```

## Run

```sh
./vnclient -host 10.0.0.120 -user ubuntu -vnc-pass secret
```

Flags: `-host` (accepts `user@host`), `-user`, `-key`, `-vnc-port`, `-vnc-pass`, `-vnc-addr`, `-remote-dl`, `-start-cmd`, `-title`. Without `-vnc-pass` the client prompts. `-vnc-addr` skips SSH and connects directly. `-start-cmd` runs on the SSH host before the VNC dial.

The window shows the remote screen. The toolbar has `Enviar` (upload to the remote downloads folder) and `Baixar` (download the last file to `~/Downloads`). You can also drop files onto the window to upload them.

## Configuration

| Variable | Default | Description |
| --- | --- | --- |
| `VNC_ADDR` | `127.0.0.1:15901` | VNC server address |
| `VNC_PASS` | empty | VNC password |
| `SSH_ADDR` | `10.0.0.120:22` | SFTP server address |
| `SSH_USER` | `ubuntu` | SFTP user |
| `SSH_KEY` | `~/.ssh/id_ed25519` | SSH private key |
| `REMOTE_DL` | `/home/ubuntu/Downloads` | Remote upload folder |

## Notes

Encodings supported: Raw, Hextile, RRE, and CopyRect. The client requests a 32-bit true color framebuffer.

## License

MIT
