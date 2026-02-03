package tray

import _ "embed"

//go:embed icon.ico
var iconData []byte

// GetIcon returns the embedded tray icon data
func GetIcon() []byte {
	return iconData
}
