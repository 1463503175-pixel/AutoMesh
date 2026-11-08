package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Config stores a small pointer to the actual workspace. Generated mesh reports,
// jobs, and logs remain in the selected data directory, not next to the EXE.
type workspaceConfig struct {
	Version       int    `json:"version"`
	DataDirectory string `json:"data_directory"`
}

func workspaceConfigCandidates() []string {
	executable, err := os.Executable()
	out := []string{}
	if err == nil {
		out = append(out, filepath.Join(filepath.Dir(executable), "automesh-settings.json"))
	}
	if dir := os.Getenv("APPDATA"); dir != "" {
		out = append(out, filepath.Join(dir, "AutoMesh2024R2", "automesh-settings.json"))
	}
	return out
}

func loadConfiguredDataDirectory() (string, bool, error) {
	for _, path := range workspaceConfigCandidates() {
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", false, fmt.Errorf("read storage settings %s: %w", path, err)
		}
		var cfg workspaceConfig
		if err := json.Unmarshal(data, &cfg); err != nil {
			return "", false, fmt.Errorf("invalid storage settings %s: %w", path, err)
		}
		if cfg.Version != 1 {
			return "", false, fmt.Errorf("unknown storage settings version at %s", path)
		}
		if err := validateDataDirectory(cfg.DataDirectory); err != nil {
			return "", false, fmt.Errorf("invalid saved data directory: %w", err)
		}
		return filepath.Clean(cfg.DataDirectory), true, nil
	}
	return "", false, nil
}

func validateDataDirectory(path string) error {
	if !filepath.IsAbs(path) || !asciiPath(path) {
		return errors.New("data directory must be an absolute ASCII path (e.g. D:\\AutoMeshData)")
	}
	cleaned := filepath.Clean(path)
	if cleaned == filepath.VolumeName(cleaned)+string(os.PathSeparator) || cleaned == string(os.PathSeparator) {
		return errors.New("drive root is not a valid data directory")
	}
	return nil
}

func saveConfiguredDataDirectory(dir string) (string, error) {
	if err := validateDataDirectory(dir); err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(workspaceConfig{Version: 1, DataDirectory: filepath.Clean(dir)}, "", "  ")
	if err != nil {
		return "", err
	}
	failures := []string{}
	for _, p := range workspaceConfigCandidates() {
		if err := writeAtomic(p, append(data, '\n')); err == nil {
			return p, nil
		} else {
			failures = append(failures, p+": "+err.Error())
		}
	}
	return "", errors.New("cannot save data location: " + strings.Join(failures, "; "))
}

// Check the entire source first. Require a new empty destination; never erase
// or mutate the source. Stage files, verify SHA-256, then atomically rename.
// A failed migration leaves the original workspace untouched.
func copyWorkspaceVerified(source, destination string) (int, error) {
	if err := validateDataDirectory(destination); err != nil {
		return 0, err
	}
	src, err := filepath.Abs(source)
	if err != nil {
		return 0, err
	}
	dst, err := filepath.Abs(destination)
	if err != nil {
		return 0, err
	}
	if src == dst {
		return 0, errors.New("source and destination are the same")
	}
	s, _ := filepath.EvalSymlinks(src)
	dParent, _ := filepath.EvalSymlinks(filepath.Dir(dst))
	dReal := filepath.Join(dParent, filepath.Base(dst))
	if s != "" && (strings.HasPrefix(strings.ToLower(dReal)+string(os.PathSeparator), strings.ToLower(s)+string(os.PathSeparator)) || strings.HasPrefix(strings.ToLower(s)+string(os.PathSeparator), strings.ToLower(dReal)+string(os.PathSeparator))) {
		return 0, errors.New("source and destination paths overlap")
	}
	if _, err := os.Lstat(dst); err == nil {
		return 0, fmt.Errorf("destination already exists; refusing to overwrite: %s", dst)
	} else if !os.IsNotExist(err) {
		return 0, err
	}
	type entry struct {
		Rel  string
		Hash [32]byte
		Mode os.FileMode
	}
	files := []entry{}
	dirs := []string{}
	err = filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, e := filepath.Rel(src, path)
		if e != nil {
			return e
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to migrate symlink: %s", path)
		}
		if info.IsDir() {
			dirs = append(dirs, rel)
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("refusing special file: %s", path)
		}
		h, e := hashFile(path)
		if e != nil {
			return e
		}
		files = append(files, entry{Rel: rel, Hash: h, Mode: info.Mode().Perm()})
		return nil
	})
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return 0, err
	}
	stage := fmt.Sprintf("%s.pending.%d", dst, time.Now().UnixNano())
	if err := os.Mkdir(stage, 0700); err != nil {
		return 0, err
	}
	defer os.RemoveAll(stage)
	for _, rel := range dirs {
		if rel != "." {
			if e := os.MkdirAll(filepath.Join(stage, rel), 0755); e != nil {
				return 0, e
			}
		}
	}
	for _, f := range files {
		from := filepath.Join(src, f.Rel)
		to := filepath.Join(stage, f.Rel)
		in, e := os.Open(from)
		if e != nil {
			return 0, e
		}
		out, e := os.OpenFile(to, os.O_WRONLY|os.O_CREATE|os.O_EXCL, f.Mode)
		if e != nil {
			in.Close()
			return 0, e
		}
		_, e = io.Copy(out, in)
		ie := in.Close()
		se := out.Sync()
		ce := out.Close()
		if e == nil {
			e = ie
		}
		if e == nil {
			e = se
		}
		if e == nil {
			e = ce
		}
		if e != nil {
			return 0, e
		}
		got, e := hashFile(to)
		if e != nil {
			return 0, e
		}
		if got != f.Hash {
			return 0, fmt.Errorf("file verification mismatch: %s", f.Rel)
		}
	}
	if _, err := os.Lstat(dst); err == nil {
		return 0, errors.New("destination appeared while copying")
	} else if !os.IsNotExist(err) {
		return 0, err
	}
	if err := os.Rename(stage, dst); err != nil {
		return 0, err
	}
	return len(files), nil
}
func hashFile(path string) ([32]byte, error) {
	var zero [32]byte
	f, e := os.Open(path)
	if e != nil {
		return zero, e
	}
	defer f.Close()
	h := sha256.New()
	if _, e = io.Copy(h, f); e != nil {
		return zero, e
	}
	var sum [32]byte
	copy(sum[:], h.Sum(nil))
	return sum, nil
}
