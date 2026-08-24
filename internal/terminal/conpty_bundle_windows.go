//go:build windows

package terminal

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

const conptyBundleVersion = "1.24.260710001"

type conptyBundleFile struct {
	path    string
	content []byte
}

func prepareConPTYBundle() (string, error) {
	files := bundledConPTYFiles()
	if len(files) == 0 {
		return "", errors.New("the bundled Windows terminal backend supports amd64 only")
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("locate user cache: %w", err)
	}
	root := filepath.Join(cache, "rrs", "conpty", conptyBundleVersion)
	if err := materializeConPTYBundle(root, files); err != nil {
		return "", err
	}
	return filepath.Join(root, "conpty.dll"), nil
}

func materializeConPTYBundle(root string, files []conptyBundleFile) error {
	for _, asset := range files {
		target := filepath.Join(root, filepath.FromSlash(asset.path))
		if err := ensureBundledFile(target, asset.content); err != nil {
			return fmt.Errorf("prepare Windows terminal backend %s: %w", asset.path, err)
		}
	}
	return nil
}

func ensureBundledFile(target string, content []byte) error {
	valid, err := fileMatches(target, content)
	if err == nil && valid {
		return nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	directory := filepath.Dir(target)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create cache directory: %w", err)
	}
	temporary, err := os.CreateTemp(directory, ".rrs-conpty-*")
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	temporaryName := temporary.Name()
	keepTemporary := true
	defer func() {
		if keepTemporary {
			_ = os.Remove(temporaryName)
		}
	}()

	if err := temporary.Chmod(0o700); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("set temporary file permissions: %w", err)
	}
	if _, err := temporary.Write(content); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write temporary file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync temporary file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary file: %w", err)
	}
	if err := replaceBundledFile(temporaryName, target); err != nil {
		valid, verifyErr := fileMatches(target, content)
		if verifyErr == nil && valid {
			return nil
		}
		return fmt.Errorf("install cached file: %w", err)
	}
	keepTemporary = false
	return nil
}

func replaceBundledFile(source, target string) error {
	sourcePointer, err := windows.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	targetPointer, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(
		sourcePointer,
		targetPointer,
		windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH,
	)
}

func fileMatches(path string, content []byte) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()

	hash := sha256.New()
	count, err := io.Copy(hash, file)
	if err != nil {
		return false, fmt.Errorf("hash cached file: %w", err)
	}
	want := sha256.Sum256(content)
	return count == int64(len(content)) && bytes.Equal(hash.Sum(nil), want[:]), nil
}
