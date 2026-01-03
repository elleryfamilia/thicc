package config

import (
	"errors"
	"os"
	"path/filepath"

	homedir "github.com/mitchellh/go-homedir"
)

var ConfigDir string

// InitConfigDir finds the configuration directory for micro according to the XDG spec.
// If no directory is found, it creates one.
func InitConfigDir(flagConfigDir string) error {
	var e error

	// Check THICC_CONFIG_HOME first, fall back to MICRO_CONFIG_HOME for compatibility
	configHome := os.Getenv("THICC_CONFIG_HOME")
	if configHome == "" {
		configHome = os.Getenv("MICRO_CONFIG_HOME")
	}
	if configHome == "" {
		// No config home set, use XDG_CONFIG_HOME or default to ~/.config
		xdgHome := os.Getenv("XDG_CONFIG_HOME")
		if xdgHome == "" {
			home, err := homedir.Dir()
			if err != nil {
				return errors.New("Error finding your home directory\nCan't load config files: " + err.Error())
			}
			xdgHome = filepath.Join(home, ".config")
		}

		configHome = filepath.Join(xdgHome, "thicc")
	}
	ConfigDir = configHome

	if len(flagConfigDir) > 0 {
		if _, err := os.Stat(flagConfigDir); os.IsNotExist(err) {
			e = errors.New("Error: " + flagConfigDir + " does not exist. Defaulting to " + ConfigDir + ".")
		} else {
			ConfigDir = flagConfigDir
			return nil
		}
	}

	// Create micro config home directory if it does not exist
	// This creates parent directories and does nothing if it already exists
	err := os.MkdirAll(ConfigDir, os.ModePerm)
	if err != nil {
		return errors.New("Error creating configuration directory: " + err.Error())
	}

	return e
}
