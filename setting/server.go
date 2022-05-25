package setting

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/BurntSushi/toml"
)

type ResNode struct {
	RootDir    string
	UserName   string
	UserPwd    string
	DeleteFile int
}

type AppConf struct {
	// 应用名称
	Title string
	// 实况日志
	GrpcLiveLogAddr string
	//
	Port int
}

type GobalServerConf struct {
	App      AppConf
	ResNodes map[string]ResNode
}

var GServerConfig GobalServerConf

func (conf *GobalServerConf) String() string {
	b, err := json.Marshal(*conf)
	if err != nil {
		return fmt.Sprintf("%+v", *conf)
	}

	var out bytes.Buffer
	err = json.Indent(&out, b, "", "    ")
	if err != nil {
		return fmt.Sprintf("%+v", *conf)
	}
	return out.String()
}

func GetServerConfig(filePath string) (cfg GobalServerConf) {
	_, err := toml.DecodeFile(filePath, &cfg)
	if err != nil {
		panic(err)
	}
	GServerConfig = cfg
	return cfg
}
