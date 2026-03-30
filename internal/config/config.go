package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Config holds all application configuration.
type Config struct {
	Addr             string  `mapstructure:"addr"`
	PollRate         int     `mapstructure:"poll-rate"`
	Deadzone         float64 `mapstructure:"deadzone"`
	MouseSensitivity float64 `mapstructure:"mouse-sens"`
	OverlayDir       string  `mapstructure:"overlay-dir"`
	SDLDBPath        string  `mapstructure:"sdl-db"`
	LogLevel         string  `mapstructure:"log-level"`
}

// Load parses CLI flags, reads an optional TOML config file (inputview.toml
// next to the executable), and returns a validated Config.
//
// exeDir is the directory containing the executable; used to locate
// inputview.toml. Pass "." when running from a source tree.
func Load(exeDir string) (Config, error) {
	// --- 1. Define flags ---
	flags := pflag.NewFlagSet("inputview", pflag.ContinueOnError)
	flags.String("addr", ":8080", "HTTP listen address")
	flags.Int("poll-rate", 16, "Gamepad/keyboard poll rate in milliseconds (~60 Hz)")
	flags.Float64("deadzone", 0.05, "Analog stick deadzone, range 0.0-1.0")
	flags.Float64("mouse-sens", 500.0, "Mouse movement sensitivity divisor (lower = more sensitive)")
	flags.String("overlay-dir", "overlays", "Directory containing Input Overlay presets (relative to executable)")
	flags.String("sdl-db", "gamecontrollerdb.txt", "SDL GameControllerDB filename (relative to executable)")
	flags.String("log-level", "info", "Log level: debug, info, warn, error")

	// --- 2. Parse flags ---
	if err := flags.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			os.Exit(0)
		}
		return Config{}, err
	}

	// --- 3. Set viper defaults ---
	v := viper.New()
	v.SetDefault("addr", ":8080")
	v.SetDefault("poll-rate", 16)
	v.SetDefault("deadzone", 0.05)
	v.SetDefault("mouse-sens", 500.0)
	v.SetDefault("overlay-dir", "overlays")
	v.SetDefault("sdl-db", "gamecontrollerdb.txt")
	v.SetDefault("log-level", "info")

	// --- 4. Configure TOML config file location ---
	v.SetConfigName("inputview")
	v.AddConfigPath(exeDir)
	v.AddConfigPath(".")

	// --- 5. Read config file (optional) ---
	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			fmt.Fprintf(os.Stderr, "warning: could not read config file: %v\n", err)
		}
	}

	// --- 6. Bind CLI flags (override config file) ---
	if err := v.BindPFlags(flags); err != nil {
		return Config{}, err
	}

	// --- 7. Unmarshal into struct ---
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return Config{}, err
	}

	// --- 8. Validate ---
	if cfg.Deadzone < 0.0 || cfg.Deadzone > 1.0 {
		return Config{}, fmt.Errorf("deadzone must be in [0.0, 1.0], got %f", cfg.Deadzone)
	}
	if cfg.PollRate < 1 {
		return Config{}, fmt.Errorf("poll-rate must be >= 1, got %d", cfg.PollRate)
	}
	if cfg.MouseSensitivity <= 0 {
		return Config{}, fmt.Errorf("mouse-sens must be > 0, got %f", cfg.MouseSensitivity)
	}
	switch cfg.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return Config{}, fmt.Errorf("log-level must be one of debug/info/warn/error, got %q", cfg.LogLevel)
	}

	return cfg, nil
}
