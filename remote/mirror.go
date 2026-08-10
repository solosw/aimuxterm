package remote

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type MirrorConfig struct {
	Host       string
	Port       int
	User       string
	Password   string
	RemotePath string
	CachePath  string
}


// SyncWorkspace mirrors a remote SSH directory into a local cache directory.
func SyncWorkspace(cfg MirrorConfig) error {
	if cfg.Port == 0 {
		cfg.Port = 22
	}
	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", cfg.Host, cfg.Port), &ssh.ClientConfig{
		User: cfg.User,
		Auth: []ssh.AuthMethod{ssh.Password(cfg.Password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout: 10 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("ssh dial: %w", err)
	}
	defer client.Close()

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return fmt.Errorf("sftp client: %w", err)
	}
	defer sftpClient.Close()

	remoteFiles, err := listRemoteFiles(sftpClient, cfg.RemotePath)
	if err != nil {
		return err
	}
	localFiles := listLocalHashes(cfg.CachePath)
	plan := BuildMirrorPlan(localFiles, remoteFiles)

	for _, p := range plan.WritePaths {
		if err := copyRemoteFile(sftpClient, cfg.RemotePath, cfg.CachePath, p); err != nil {
			return err
		}
	}
	for _, p := range plan.DeletePaths {
		_ = os.Remove(filepath.Join(cfg.CachePath, filepath.FromSlash(p)))
	}
	return nil
}

func listRemoteFiles(c *sftp.Client, root string) ([]fileMeta, error) {
	var out []fileMeta
	walker := c.Walk(root)
	for walker.Step() {
		if walker.Err() != nil {
			continue
		}
		stat := walker.Stat()
		if stat == nil || stat.IsDir() {
			continue
		}
		rel := strings.TrimPrefix(path.Clean(walker.Path()), path.Clean(root))
		rel = strings.TrimPrefix(rel, "/")
		if rel == "" {
			continue
		}
		f, err := c.Open(walker.Path())
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			return nil, err
		}
		out = append(out, fileMeta{Path: rel, Hash: hashBytes(data)})
	}
	return out, nil
}

func listLocalHashes(root string) map[string]string {
	out := map[string]string{}
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		out[filepath.ToSlash(rel)] = hashBytes(data)
		return nil
	})
	return out
}

func copyRemoteFile(c *sftp.Client, remoteRoot, localRoot, rel string) error {
	rp := path.Join(remoteRoot, rel)
	lf := filepath.Join(localRoot, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(lf), 0755); err != nil {
		return err
	}
	r, err := c.Open(rp)
	if err != nil {
		return err
	}
	defer r.Close()
	w, err := os.Create(lf)
	if err != nil {
		return err
	}
	defer w.Close()
	_, err = io.Copy(w, r)
	return err
}
