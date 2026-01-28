package config

import (
	"os"
	"path/filepath"
)

const defaultConfigRelPath = "configs/conf.yml"

var Conf Config

func Load(cfgName string) {
	curDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	// 约定：
	// 1) 传入 cfgName（相对/绝对路径）则优先使用；
	// 2) 否则从当前目录开始向上查找 `configs/conf.yml`。
	if cfgName != "" {
		if filepath.IsAbs(cfgName) {
			load(cfgName)
			return
		}
		load(filepath.Join(curDir, cfgName))
		return
	}

	load(findConfigUpward(curDir))
}

func findConfigUpward(startDir string) string {
	dir := startDir
	for {
		candidate := filepath.Join(dir, defaultConfigRelPath)
		if fileExist(candidate) {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			panic("config file not exist, searched configs/conf.yml from: " + startDir)
		}
		dir = parent
	}
}
