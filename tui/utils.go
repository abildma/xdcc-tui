package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"xdcc-tui/search"
)

// FormatSize formats file size in human-readable format
func FormatSize(size int64) string {
	if size < 0 {
		return "--"
	}

	var result string
	if size >= search.GigaByte {
		result = fmt.Sprintf("%.2fGB", float64(size)/float64(search.GigaByte))
	} else if size >= search.MegaByte {
		result = fmt.Sprintf("%.2fMB", float64(size)/float64(search.MegaByte))
	} else if size >= search.KiloByte {
		result = fmt.Sprintf("%.2fKB", float64(size)/float64(search.KiloByte))
	} else {
		result = fmt.Sprintf("%dB", size)
	}
	return result
}

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


