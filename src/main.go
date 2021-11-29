package main

import (
	"flag"
	"fmt"
	"os"

	dealers "m/dealers"
	pblib "m/pblib"
	"m/setting"
	// register ws transport
)

const mainLOOP string = "mainloop"

func die(format string, v ...interface{}) {
	fmt.Fprintln(os.Stderr, fmt.Sprintf(format, v...))
	os.Exit(1)
}

func main() {

	const logkind string = "mainloop"
	logInfo := func(format string, a ...interface{}) {
		pblib.LogInfo(logkind, format, a...)
	}
	logTrace := func(format string, a ...interface{}) {
		pblib.LogTrace(logkind, format, a...)
	}
	logError := func(format string, a ...interface{}) {
		pblib.LogError(logkind, format, a...)
	}
	logDebug := func(format string, a ...interface{}) {
		pblib.LogDebug(logkind, format, a...)
	}
	logInfo("===========print test start ============")
	logInfo("test logInfo")
	logTrace("test logTrace")
	logError("test logError")
	logDebug("test logDebug")
	logInfo("===========print test end ============")
	pwdDir, _ := os.Getwd()
	logDebug("系统开始启动,路径:%s", pwdDir)

	//////////////////////////////////////////////

	var role string
	var port int
	var cfgpath string
	var mode string
	flag.StringVar(&role, "role", "", "角色(role):client|server")
	flag.StringVar(&mode, "mode", "download", "模式(mode):download|upload")
	flag.StringVar(&cfgpath, "c", "", "配置路径(cfgpath)")
	flag.IntVar(&port, "p", 5000, "端口(port)")
	flag.Parse()
	logInfo("软件版本V0.0.2")
	logInfo("配置文件:%s", cfgpath)

	switch role {
	case "server":
		{
			cfg := setting.GetServerConfig(cfgpath)
			logInfo("setting=%s", cfg.String())
			dealers.StartServer(cfg)
		}
		break
	case "client":
		{
			cfg := setting.GetClientConfig(cfgpath)
			logInfo("setting=%s", cfg.String())
			switch mode {
			case "download":
				{
					go dealers.StartViewWeb()
					dealers.StartDownloadClient(&cfg)
					for {

					}
				}
			case "upload":
				{
					dealers.StartUploadTask(&cfg)
					break
				}
			default:
				die("mode is illegal")
				break
			}
		}
		break
	default:
		die("role is illegal")
	}
	// fmt.Println(mode)
	// fmt.Println("hello world")
	// fmt.Println(flag.Args())
}
