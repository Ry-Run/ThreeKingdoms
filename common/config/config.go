package config

import (
	"os"
	"path/filepath"
)

const configFile = "../conf/conf.yml"

var Conf Config

func Load(cfgName string) {
	curDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	configPath := filepath.Join(curDir, configFile)
	if cfgName != "" {
		configPath = filepath.Join(curDir, cfgName)
	}
	load(configPath)
}
