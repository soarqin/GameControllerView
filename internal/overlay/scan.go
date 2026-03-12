package overlay

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Entry represents a single available overlay configuration.
// For a primary overlay "dualsense", Name and URLPath are both "dualsense".
// For a variant "dualsense/compact", Name and URLPath are both "dualsense/compact".
type Entry struct {
	Name    string // display name shown in the tray menu
	URLPath string // value used in the ?overlay= URL parameter
}

// ScanDir scans the given overlays directory and returns all available overlay
// entries (primary configs and named variants).
//
// Directory layout expected:
//
//	overlaysDir/
//	  dualsense/
//	    dualsense.json   → Entry{"dualsense", "dualsense"}
//	    compact.json     → Entry{"dualsense/compact", "dualsense/compact"}
//	  xbox/
//	    xbox.json        → Entry{"xbox", "xbox"}
//
// Rules:
//   - Only immediate subdirectories of overlaysDir are considered.
//   - Within each subdirectory, only *.json files are enumerated.
//   - A JSON file whose base name (without extension) matches the parent
//     directory name is the primary config; all others are variants.
//   - If overlaysDir does not exist or cannot be read, an empty slice is
//     returned without error.
//
// The returned slice is sorted by Name.
func ScanDir(overlaysDir string) []Entry {
	topEntries, err := os.ReadDir(overlaysDir)
	if err != nil {
		// Directory absent or unreadable — not an error condition.
		return nil
	}

	var results []Entry

	for _, top := range topEntries {
		if !top.IsDir() {
			continue
		}
		dirName := top.Name()
		subDir := filepath.Join(overlaysDir, dirName)

		subEntries, err := os.ReadDir(subDir)
		if err != nil {
			continue
		}

		for _, sub := range subEntries {
			if sub.IsDir() {
				continue
			}
			fname := sub.Name()
			if !strings.HasSuffix(fname, ".json") {
				continue
			}
			jsonBase := strings.TrimSuffix(fname, ".json")

			var urlPath string
			if jsonBase == dirName {
				// Primary config: e.g. dualsense/dualsense.json → "dualsense"
				urlPath = dirName
			} else {
				// Variant config: e.g. dualsense/compact.json → "dualsense/compact"
				urlPath = dirName + "/" + jsonBase
			}

			results = append(results, Entry{
				Name:    urlPath,
				URLPath: urlPath,
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Name < results[j].Name
	})

	return results
}
