package app

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

var ErrHelp = errors.New("help requested")

type Config struct {
	ConfigDir        string            `json:"config_dir"`
	DataDir          string            `json:"data_dir"`
	CacheDir         string            `json:"cache_dir"`
	MaxArtifactBytes int64             `json:"max_artifact_bytes"`
	MaxBundleBytes   int64             `json:"max_bundle_bytes"`
	MaxBundleFiles   int               `json:"max_bundle_files"`
	Sources          map[string]string `json:"-"`
}

type fileConfig struct {
	DataDir          string `json:"data_dir"`
	CacheDir         string `json:"cache_dir"`
	MaxArtifactBytes int64  `json:"max_artifact_bytes"`
	MaxBundleBytes   int64  `json:"max_bundle_bytes"`
	MaxBundleFiles   int    `json:"max_bundle_files"`
}

func LoadConfig(args []string) (Config, error) {
	cfg, err := defaultConfig()
	if err != nil {
		return Config{}, err
	}

	if value := os.Getenv("MDBENCH_CONFIG_DIR"); value != "" {
		cfg.ConfigDir = value
		cfg.Sources["config_dir"] = "environment"
	}
	if value, ok := flagValue(args, "config-dir"); ok {
		cfg.ConfigDir = value
		cfg.Sources["config_dir"] = "flag"
	}

	if err := applyConfigFile(&cfg); err != nil {
		return Config{}, err
	}
	if err := applyEnvironment(&cfg); err != nil {
		return Config{}, err
	}
	if err := applyFlags(&cfg, args); err != nil {
		return Config{}, err
	}
	if cfg.MaxArtifactBytes <= 0 || cfg.MaxBundleBytes <= 0 || cfg.MaxBundleFiles <= 0 {
		return Config{}, errors.New("size and file limits must be positive")
	}
	return cfg, nil
}

func defaultConfig() (Config, error) {
	configBase, err := os.UserConfigDir()
	if err != nil {
		return Config{}, fmt.Errorf("resolve config directory: %w", err)
	}
	cacheBase, err := os.UserCacheDir()
	if err != nil {
		return Config{}, fmt.Errorf("resolve cache directory: %w", err)
	}
	dataBase := configBase
	if runtime.GOOS == "linux" {
		if value := os.Getenv("XDG_DATA_HOME"); value != "" {
			dataBase = value
		} else if home, homeErr := os.UserHomeDir(); homeErr == nil {
			dataBase = filepath.Join(home, ".local", "share")
		}
	}
	return Config{
		ConfigDir:        filepath.Join(configBase, "mdbench"),
		DataDir:          filepath.Join(dataBase, "mdbench"),
		CacheDir:         filepath.Join(cacheBase, "mdbench"),
		MaxArtifactBytes: 1 << 20,
		MaxBundleBytes:   8 << 20,
		MaxBundleFiles:   128,
		Sources: map[string]string{
			"config_dir": "default", "data_dir": "default", "cache_dir": "default",
			"max_artifact_bytes": "default", "max_bundle_bytes": "default", "max_bundle_files": "default",
		},
	}, nil
}

func applyConfigFile(cfg *Config) error {
	path := filepath.Join(cfg.ConfigDir, "config.json")
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open config %q: %w", path, err)
	}
	defer file.Close()

	var saved fileConfig
	decoder := json.NewDecoder(io.LimitReader(file, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&saved); err != nil {
		return fmt.Errorf("parse config %q: %w", path, err)
	}
	if saved.DataDir != "" {
		cfg.DataDir, cfg.Sources["data_dir"] = saved.DataDir, "config file"
	}
	if saved.CacheDir != "" {
		cfg.CacheDir, cfg.Sources["cache_dir"] = saved.CacheDir, "config file"
	}
	if saved.MaxArtifactBytes > 0 {
		cfg.MaxArtifactBytes, cfg.Sources["max_artifact_bytes"] = saved.MaxArtifactBytes, "config file"
	}
	if saved.MaxBundleBytes > 0 {
		cfg.MaxBundleBytes, cfg.Sources["max_bundle_bytes"] = saved.MaxBundleBytes, "config file"
	}
	if saved.MaxBundleFiles > 0 {
		cfg.MaxBundleFiles, cfg.Sources["max_bundle_files"] = saved.MaxBundleFiles, "config file"
	}
	return nil
}

func applyEnvironment(cfg *Config) error {
	if value := os.Getenv("MDBENCH_DATA_DIR"); value != "" {
		cfg.DataDir, cfg.Sources["data_dir"] = value, "environment"
	}
	if value := os.Getenv("MDBENCH_CACHE_DIR"); value != "" {
		cfg.CacheDir, cfg.Sources["cache_dir"] = value, "environment"
	}
	for name, target := range map[string]*int64{
		"MDBENCH_MAX_ARTIFACT_BYTES": &cfg.MaxArtifactBytes,
		"MDBENCH_MAX_BUNDLE_BYTES":   &cfg.MaxBundleBytes,
	} {
		if value := os.Getenv(name); value != "" {
			parsed, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return fmt.Errorf("parse %s: %w", name, err)
			}
			*target = parsed
			cfg.Sources[strings.ToLower(strings.TrimPrefix(name, "MDBENCH_"))] = "environment"
		}
	}
	if value := os.Getenv("MDBENCH_MAX_BUNDLE_FILES"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("parse MDBENCH_MAX_BUNDLE_FILES: %w", err)
		}
		cfg.MaxBundleFiles, cfg.Sources["max_bundle_files"] = parsed, "environment"
	}
	return nil
}

func applyFlags(cfg *Config, args []string) error {
	fs := flag.NewFlagSet("mdbench", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&cfg.ConfigDir, "config-dir", cfg.ConfigDir, "configuration directory")
	fs.StringVar(&cfg.DataDir, "data-dir", cfg.DataDir, "data directory")
	fs.StringVar(&cfg.CacheDir, "cache-dir", cfg.CacheDir, "cache directory")
	fs.Int64Var(&cfg.MaxArtifactBytes, "max-artifact-bytes", cfg.MaxArtifactBytes, "maximum Markdown bytes")
	fs.Int64Var(&cfg.MaxBundleBytes, "max-bundle-bytes", cfg.MaxBundleBytes, "maximum artifact bundle bytes")
	fs.IntVar(&cfg.MaxBundleFiles, "max-bundle-files", cfg.MaxBundleFiles, "maximum artifact bundle files")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return ErrHelp
		}
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	fs.Visit(func(f *flag.Flag) {
		cfg.Sources[strings.ReplaceAll(f.Name, "-", "_")] = "flag"
	})
	return nil
}

func flagValue(args []string, name string) (string, bool) {
	prefix := "--" + name + "="
	for index, arg := range args {
		if strings.HasPrefix(arg, prefix) {
			return strings.TrimPrefix(arg, prefix), true
		}
		if arg == "--"+name && index+1 < len(args) {
			return args[index+1], true
		}
	}
	return "", false
}

func Usage() string {
	return "mdbench [--config-dir PATH] [--data-dir PATH] [--cache-dir PATH]\n\nRun without a subcommand to open the terminal UI."
}
