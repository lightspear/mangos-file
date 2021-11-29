package setting

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/dustin/go-humanize"
	"golang.org/x/time/rate"
)

type LimitRateClass struct {
	TimeRange string
	Start     int
	End       int
	Rate      int
}

type GobalClientConf struct {
	// 应用名称
	Title string
	// 实况日志
	GrpcLiveLogAddr string
	//
	WSServerUrl     string
	ResNode         string
	RemoteDir       string
	UserPwd         string
	LocalDir        string
	MoveDir         string
	Resume          int
	FilePattern     string
	PriorRule       string
	StartDay        string
	EndDay          string
	Completelogfile string
	ExecInterval    int
	SliceSize       int
	RecordDb        string
	DbViewHttpPort  int
	LimitRate       []LimitRateClass
}

var GClientConfig GobalClientConf

func (conf *GobalClientConf) String() string {
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

func test() {

	f, _ := humanize.ParseBigBytes("1ki")
	la := f.Int64()
	fmt.Println(la)
	start := time.Now()

	limit := rate.NewLimiter(1, 1)
	c := context.Background()
	for {
		limit.Wait(c)
		fmt.Println("log:event happen")
	}

	for {
		if limit.AllowN(time.Now(), 1) {
			fmt.Println("log:event happen")
		} else {
			// fmt.Println("log:event not allow")
		}
		// time.Sleep(time.Millisecond * 100)
	}
	fmt.Println(time.Since(start)) // output: 7.501262697s （初始桶内5个和每秒2个token）
	for {

	}
}

func GetClientConfig(filePath string) (cfg GobalClientConf) {

	// test()

	_, err := toml.DecodeFile(filePath, &cfg)
	if err != nil {
		panic(err)
	}

	if cfg.SliceSize == 0 {
		cfg.SliceSize = 128
	}

	for i, v := range cfg.LimitRate {

		arr := strings.Split(v.TimeRange, "~")
		if len(arr) != 2 {
			panic(errors.New("TimeRange格式不对"))
		}

		tStart, _ := time.Parse("15:04", arr[0])
		cfg.LimitRate[i].Start = tStart.Hour()*60 + tStart.Minute()
		tEnd, _ := time.Parse("15:04", arr[1])
		cfg.LimitRate[i].End = tEnd.Hour()*60 + tEnd.Minute()
		if cfg.LimitRate[i].End < cfg.LimitRate[i].Start {
			cfg.LimitRate[i].End = cfg.LimitRate[i].End + 24*60
		}
	}

	GClientConfig = cfg
	return cfg
}
