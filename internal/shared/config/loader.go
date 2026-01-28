package config

import (
	"fmt"
	"log"
	"os"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

func load(configPath string) {
	if !fileExist(configPath) {
		panic(fmt.Sprintf("config file not exist, configPath=%v", configPath))
	}

	v := viper.New()
	v.SetConfigFile(configPath)
	// todo 确认 Conf 并发读取问题
	v.OnConfigChange(func(e fsnotify.Event) {
		log.Println("配置文件变更")
		err := v.Unmarshal(&Conf)
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
	err = v.Unmarshal(&Conf)
	if err != nil {
		panic(err)
	}
}

func fileExist(fileName string) bool {
	_, err := os.Stat(fileName)
	return err == nil
}
