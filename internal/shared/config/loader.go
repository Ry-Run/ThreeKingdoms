package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

func Load[T any](cfgName string, c *T) {
	curDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	// 约定：
	// 1) 传入 cfgName（相对/绝对路径）则优先使用；
	// 2) 否则从当前目录开始向上查找 `configs/conf.yml`。
	if cfgName != "" {
		if filepath.IsAbs(cfgName) {
			loadConf(cfgName, c)
			return
		}
		loadConf(filepath.Join(curDir, cfgName), c)
		return
	}

	loadConf(findConfigUpward(curDir, cfgName), c)
}

func findConfigUpward(startDir string, defaultPath string) string {
	dir := startDir
	for {
		candidate := filepath.Join(dir, defaultPath)
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

func loadConf[T any](configPath string, c *T) {
	if !fileExist(configPath) {
		panic(fmt.Sprintf("config file not exist, configPath=%v", configPath))
	}

	v := viper.New()
	v.SetConfigFile(configPath)
	// todo 确认 Conf 并发读取问题
	v.OnConfigChange(func(e fsnotify.Event) {
		log.Println("配置文件变更")
		err := v.Unmarshal(c)
		if err != nil {
			panic(fmt.Errorf("viper unmarshal change config data: cast exception, err=%v \n", err))
		}
	})
	v.WatchConfig()
	// 加载配置
	err := v.ReadInConfig()
	if err != nil {
		panic(err)
	}
	err = v.Unmarshal(c)
	if err != nil {
		panic(err)
	}
}

func fileExist(fileName string) bool {
	_, err := os.Stat(fileName)
	return err == nil
}
