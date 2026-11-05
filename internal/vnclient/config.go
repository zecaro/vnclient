package vnclient

import (
	"flag"
	"os"
	"path/filepath"
)

// Config holds the runtime settings loaded from flags or the environment.
type Config struct {
	SSHAddr  string
	SSHUser  string
	SSHKey   string
	VNCPort  string
	VNCAddr  string
	VNCPass  string
	RemoteDL string
	Title    string
	StartCmd string
}

// LoadConfig reads the environment with defaults for the Hermes host.
func LoadConfig() Config {
	home, _ := os.UserHomeDir()
	return Config{
		SSHAddr:  env("SSH_ADDR", "10.0.0.120"),
		SSHUser:  env("SSH_USER", "ubuntu"),
		SSHKey:   env("SSH_KEY", filepath.Join(home, ".ssh", "id_ed25519")),
		VNCPort:  env("VNC_PORT", "5901"),
		VNCAddr:  os.Getenv("VNC_ADDR"),
		VNCPass:  os.Getenv("VNC_PASS"),
		RemoteDL: env("REMOTE_DL", "/home/ubuntu/Downloads"),
		Title:    env("VNC_TITLE", "vnclient"),
	}
}

// AddFlags registers the command line flags on top of the loaded config.
func AddFlags(fs *flag.FlagSet, cfg *Config) {
	fs.StringVar(&cfg.SSHAddr, "host", cfg.SSHAddr, "SSH host or user@host, optionally :port")
	fs.StringVar(&cfg.SSHUser, "user", cfg.SSHUser, "SSH user")
	fs.StringVar(&cfg.SSHKey, "key", cfg.SSHKey, "SSH private key")
	fs.StringVar(&cfg.VNCPort, "vnc-port", cfg.VNCPort, "VNC port on the SSH host")
	fs.StringVar(&cfg.VNCAddr, "vnc-addr", cfg.VNCAddr, "direct VNC address; skips SSH")
	fs.StringVar(&cfg.VNCPass, "vnc-pass", cfg.VNCPass, "VNC password; prompts when empty")
	fs.StringVar(&cfg.RemoteDL, "remote-dl", cfg.RemoteDL, "remote download folder")
	fs.StringVar(&cfg.Title, "title", cfg.Title, "window title")
	fs.StringVar(&cfg.StartCmd, "start-cmd", cfg.StartCmd, "command to run on the SSH host before connecting")
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
