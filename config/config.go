package config

import (
	"io/ioutil"
	"os"
	"path/filepath"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

var (
	// Default is the default config
	Default = Config{
		Server: Server{
			Host: "127.0.0.1",
			Port: "8113",
		},
	}
)

type (
	Server struct {
		Host string `yaml:"host" json:"host"`
		Port string `yaml:"port" json:"port"`
	}
	Database struct {
		DBName   string `yaml:"dbname" json:"dbname"`
		UserName string `yaml:"username" json:"username"`
		Password string `yaml:"password" json:"password"`
		Host     string `yaml:"host" json:"host"`
		Port     string `yaml:"port" json:"port"`
	}
	Iotex struct {
		ChainEndPoint string `yaml:"chainEndPoint" json:"chainEndPoint"`
	}
	Config struct {
		Server   Server   `yaml:"server" json:"server"`
		Database Database `yaml:"database" json:"database"`
		Iotex    Iotex    `yaml:"iotex" json:"iotex"`
		//Index    indexservice.Config `yaml:"indexer" json:"indexer"`
	}
)

func New(path string) (cfg *Config, err error) {
	body, err := ioutil.ReadFile(path)
	if err != nil {
		return cfg, errors.Wrap(err, "failed to read config content")
	}
	cfg = &Default
	if err = yaml.Unmarshal(body, cfg); err != nil {
		return cfg, errors.Wrap(err, "failed to unmarshal config to struct")
	}
	return
}

var (
	// File names from which we attempt to read configuration.
	DefaultConfigFiles = []string{"config.yml", "config.yaml"}

	// Launchd doesn't set root env variables, so there is default
	DefaultConfigDirs = []string{"~/.iotex-analytics", "/usr/local/etc/iotex-analytics", "/etc/iotex-analytics"}
)

// FileExists checks to see if a file exist at the provided path.
func FileExists(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			// ignore missing files
			return false, nil
		}
		return false, err
	}
	f.Close()
	return true, nil
}

// FindDefaultConfigPath returns the first path that contains a config file.
// If none of the combination of DefaultConfigDirs and DefaultConfigFiles
// contains a config file, return empty string.
func FindDefaultConfigPath() string {
	for _, configDir := range DefaultConfigDirs {
		for _, configFile := range DefaultConfigFiles {
			dirPath, err := homedir.Expand(configDir)
			if err != nil {
				continue
			}
			path := filepath.Join(dirPath, configFile)
			if ok, _ := FileExists(path); ok {
				return path
			}
		}
	}
	return ""
}
