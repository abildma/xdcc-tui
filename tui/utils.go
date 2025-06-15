package tui

import (
	"os"
	"path/filepath"

)

// GetDownloadsDir returns the user's Downloads directory path
func GetDownloadsDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback to current directory if we can't get the home directory
		return "."
	}
	
	// Standard Downloads folder
	downloadsDir := filepath.Join(homeDir, "Downloads")
	
	// Check if the directory exists
	if _, err := os.Stat(downloadsDir); os.IsNotExist(err) {
		// Try to create it
		err = os.MkdirAll(downloadsDir, 0755)
		if err != nil {
			// Fallback to current directory if we can't create the Downloads directory
			return "."
		}
	}
	
	return downloadsDir
}


