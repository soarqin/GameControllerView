package gamepad

import _ "embed"

// embeddedGameControllerDB is the bundled SDL_GameControllerDB database.
// It is used as a base when no external gamecontrollerdb.txt is present
// next to the executable.
//
//go:embed gamecontrollerdb.txt
var embeddedGameControllerDB []byte
