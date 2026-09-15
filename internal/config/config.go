package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
)

type Config struct {
	Port   int  `json:"port,omitempty"`
	Remote bool `json:"remote,omitempty"`
}

func Path(configDir string) string {
	return filepath.Join(configDir, "config.json")
}

func Read(configDir string) (*Config, error) {
	c, err := go_pkg_filesystem.ReadJSON[Config](Path(configDir))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Config{}, nil
		}
		return nil, err
	}
	return &c, nil
}

func write(configDir string, c *Config) error {
	if err := go_pkg_filesystem.WriteJSON(Path(configDir), c, true); err != nil {
		return fmt.Errorf("go_pkg_filesystem.WriteJSON: %w", err)
	}
	return nil
}

func SetPort(configDir string, port int) error {
	c, err := Read(configDir)
	if err != nil {
		return err
	}
	c.Port = port
	return write(configDir, c)
}

func SetRemote(configDir string, enabled bool) error {
	c, err := Read(configDir)
	if err != nil {
		return err
	}
	c.Remote = enabled
	return write(configDir, c)
}

func ClearPort(configDir string) error {
	c, err := Read(configDir)
	if err != nil {
		return err
	}
	c.Port = 0
	return write(configDir, c)
}
