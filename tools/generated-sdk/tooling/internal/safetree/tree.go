package safetree

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

func Validate(root string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlinked generated entry is forbidden: %s", path)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return fmt.Errorf("non-regular generated entry is forbidden: %s", path)
		}
		return nil
	})
}

func ReadRegular(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("generated entry is not a regular file: %s", path)
	}
	return os.ReadFile(path)
}

func WriteNew(path string, data []byte, mode fs.FileMode) (resultErr error) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	defer func() {
		resultErr = errors.Join(resultErr, file.Close())
	}()
	if _, err := file.Write(data); err != nil {
		return err
	}
	return nil
}
