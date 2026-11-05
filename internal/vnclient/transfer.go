package vnclient

import (
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

func sshClient(cfg Config) (*ssh.Client, error) {
	key, err := os.ReadFile(cfg.SSHKey)
	if err != nil {
		return nil, err
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, err
	}
	user := cfg.SSHUser
	host := cfg.SSHAddr
	if i := strings.LastIndex(host, "@"); i >= 0 {
		user = host[:i]
		host = host[i+1:]
	}
	return ssh.Dial("tcp", hostPort(host, "22"), &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	})
}

func hostPort(addr, def string) string {
	if _, _, err := net.SplitHostPort(addr); err == nil {
		return addr
	}
	return net.JoinHostPort(addr, def)
}

// Upload copies a local file to the configured remote download folder.
func Upload(cfg Config, local string) error {
	sc, err := sshClient(cfg)
	if err != nil {
		return err
	}
	defer sc.Close()
	cl, err := sftp.NewClient(sc)
	if err != nil {
		return err
	}
	defer cl.Close()
	if err := cl.MkdirAll(cfg.RemoteDL); err != nil {
		return err
	}
	src, err := os.Open(local)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := cl.Create(path.Join(cfg.RemoteDL, filepath.Base(local)))
	if err != nil {
		return err
	}
	defer dst.Close()
	_, err = io.Copy(dst, src)
	return err
}

// Download copies a remote file to the local Downloads folder.
func Download(cfg Config, remote string) error {
	sc, err := sshClient(cfg)
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
