package dealers

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	pblib "m/pblib"
	"m/setting"
	"os"
	"path"
	"strings"
	"time"

	"github.com/leekchan/timeutil"
	"github.com/schollz/progressbar/v3"
	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/protocol/req"
	"golang.org/x/time/rate"

	// register ws transport
	_ "go.nanomsg.org/mangos/v3/transport/ws"
)

func StartUploadTask(cfg *setting.GobalClientConf) {
	fmt.Println("上传模式")
	//过滤文件表达式
	FilePattern := cfg.FilePattern
	FilePatternList := []string{}
	if FilePattern != "" {
		FilePatternList = strings.Split(FilePattern, "|")
	}
	//形成文件过滤函数
	isExtVaildFunc := func(fullpath string) bool {
		extName := "*" + path.Ext(fullpath)
		if FilePattern != "" {
			if extName == ".tmp" {
				return false
			}
			flag := false
			for _, suffix := range FilePatternList {
				if suffix == extName || suffix == "*.*" {
					flag = true
					break
				}
			}
			return flag
		}
		return true
	}

	SrcDir := strings.TrimRight(cfg.LocalDir, "/")

	//不管如何先保证目录一定存在
	os.MkdirAll(SrcDir, 0777)

	proof := ProofInfo{
		PWD:  cfg.UserPwd,
		Node: cfg.ResNode,
	}

	nowQueue := pblib.NewCASQueue(1024 * 1024)
	oldQueue := pblib.NewCASQueue(1024 * 1024)
	//消费队列(优先消费今天的)

	go func() {
		for {
			if nowQueue.Quantity() > 0 {
				for {
					data, flag := nowQueue.Get()
					if flag == false {
						break
					}
					retVal := uploadfile(data.(string), &proof, cfg)
					if retVal == false {
						//如果没有成功则重新压入队列
						nowQueue.Put(data)
					}
				}
			}
			if nowQueue.Quantity() == 0 {
				data, flag := oldQueue.Get()
				if flag == true {
					retVal := uploadfile(data.(string), &proof, cfg)
					if retVal == false {
						//如果没有成功则重新压入队列
						oldQueue.Put(data)
					}
				}
			}
			//如果今天和历史的队列都是空的,那么延迟1s等待可消费队列
			if nowQueue.Quantity() == 0 && oldQueue.Quantity() == 0 {
				// time.Sleep(time.Second * 5)
			}
		}
	}()
	//生产队列
	go func() {
		//反复寻找今天的新产生的文件
		for {
			t := time.Now()
			//优先文件表达式
			PriorRuleStr := timeutil.Strftime(&t, cfg.PriorRule)
			PriorRuleStr = strings.TrimRight(PriorRuleStr, "/")
			lstRuleDirs := strings.Split(PriorRuleStr, "/")
			if PriorRuleStr == "" {
				lstRuleDirs = []string{}
			}
			lstRuleSize := len(lstRuleDirs)
			if nowQueue.Quantity() == 0 {
				//首先搜索本天全部提交
				pblib.SearchFiles(SrcDir,
					"",
					0,
					func(relativedir string, level int) string {
						//根据规则计算,跳跃目录
						appenddir := ""
						//优先模式
						for i := level; i < lstRuleSize; i++ {
							ruleDir := lstRuleDirs[i]
							if ruleDir == "<dirs>" {
								break
							} else {
								appenddir += ruleDir
								appenddir += "/"
							}
						}
						return appenddir
					},
					func(dir string, level int) bool {
						//默认情况是不乱跳过的
						return false
					},
					func(fullpath string, relativepath string) bool {
						if isExtVaildFunc(fullpath) == false {
							return true
						}
						//每次要传输非今日文件之前,一定要检查,今日传输队列是否为空
						nowQueue.Put(fullpath)
						return true
					})
			}
			if cfg.PriorRule != "" {
				//只有今天队列和历史队列都等于空了(减少搜索频率)，才开始搜索
				if nowQueue.Quantity() == 0 && oldQueue.Quantity() == 0 {
					// fmt.Println("oldQueue searchdir=", SrcDir)
					// fmt.Println("oldQueue lstRuleSize=", lstRuleSize)
					pblib.SearchFiles(SrcDir,
						"",
						0,
						func(relativedir string, level int) string {
							return ""
						},
						func(dir string, level int) bool {
							//排除当天模式
							if level < lstRuleSize {
								if lstRuleDirs[level] == dir {
									return true
								}
							}
							//默认情况是不乱跳过的
							return false
						},
						func(fullpath string, relativepath string) bool {
							if isExtVaildFunc(fullpath) == false {
								return true
							}
							oldQueue.Put(fullpath)
							//执行完一次就立马返回
							return false
						})
				}
			}
			time.Sleep(time.Millisecond * 10)
		}
	}()

	for {

	}

}

//下面是和mangos消息中间件通信的代码

var uploadsock *mangos.Socket = nil

func getsock_upload(cfg *setting.GobalClientConf) *mangos.Socket {
	if uploadsock != nil {
		return uploadsock
	}
	sock, e := req.NewSocket()
	if e != nil {
		die("cannot make req socket: %v", e)
	}
	url := fmt.Sprintf("%s/upload", cfg.WSServerUrl)
	sock.SetOption(mangos.OptionMaxReconnectTime, time.Second)
	if e = sock.Dial(url); e != nil {
		// die("cannot dial req url: %v", e)
		return getsock_upload(cfg)
	}
	// Time for TCP connection set up
	sock.SetOption(mangos.OptionRecvDeadline, time.Second)
	sock.SetOption(mangos.OptionSendDeadline, time.Second)

	uploadsock = &sock
	return uploadsock
}

var checkfilesock *mangos.Socket = nil

func getsock_checkfile(cfg *setting.GobalClientConf) *mangos.Socket {
	if checkfilesock != nil {
		return checkfilesock
	}

	sock, e := req.NewSocket()
	if e != nil {
		die("cannot make req socket: %v", e)
	}
	url := fmt.Sprintf("%s/checkfile", cfg.WSServerUrl)
	if e = sock.Dial(url); e != nil {
		// die("cannot dial req url: %v", e)
		return getsock_checkfile(cfg)
	}
	// Time for TCP connection set up
	sock.SetOption(mangos.OptionRecvDeadline, time.Second)
	sock.SetOption(mangos.OptionSendDeadline, time.Second)
	checkfilesock = &sock
	return checkfilesock
}

func uploadfile(fullpath string, proof *ProofInfo, cfg *setting.GobalClientConf) bool {
	fileInfo := pblib.FileInfo{}
	retFlag, _ := pblib.GetFileInfo(fullpath, &fileInfo)
	if retFlag == false {
		return true
	}

	psock := getsock_checkfile(cfg)
	sock := *psock

	relativePath := path.Join(cfg.RemoteDir, strings.TrimPrefix(fullpath, cfg.LocalDir))

	doneCh := make(chan struct{})
	failCh := make(chan struct{})

	data := CheckFileRespInfo{}
	cmd := CheckFileCMD{
		Path:     relativePath,
		FileInfo: fileInfo,
		Proof:    *proof,
	}
	// fmt.Println("fullpath=", fullpath)

	jsonbytes, _ := json.Marshal(cmd)

	// fmt.Println("jsonbytes=", string(jsonbytes))
	go func() {
		if e := sock.Send(jsonbytes); e != nil {
			fmt.Printf("Cannot send req: %v\n", e)
			failCh <- struct{}{}
		}

		if m, e := sock.Recv(); e != nil {
			fmt.Printf("Cannot recv reply: %v\n", e)
			failCh <- struct{}{}
		} else {
			dec := gob.NewDecoder(bytes.NewBuffer(m))
			err := dec.Decode(&data)
			if err != nil {
				failCh <- struct{}{}
			}
			// 成功
			doneCh <- struct{}{}
		}

	}()

	select {
	case <-time.After(time.Second * 2):
		fmt.Printf("time.After 2s\n")
		sock.Close()
		checkfilesock = nil
		return uploadfile(fullpath, proof, cfg)
	case <-doneCh:
		upload(fullpath, data.FileOffset, proof, cfg)
	case <-failCh:
		return uploadfile(fullpath, proof, cfg)
	}
	// fmt.Println("fullpath=", fullpath)
	// os.Remove(fullpath)
	return true
}

func upload(fullpath string, fileoffset int64, proof *ProofInfo, cfg *setting.GobalClientConf) bool {
	//如果文件不存在直接返回
	fi := pblib.FileInfo{}
	retFlag, _ := pblib.GetFileInfo(fullpath, &fi)

	if retFlag == false {
		return true
	}

	// fmt.Printf("fullpath=%s,offset=%d\n", fullpath, fileoffset)
	var step int = 1024 * cfg.SliceSize
	remotefile := strings.TrimPrefix(fullpath, cfg.LocalDir)
	remotefile = path.Join(cfg.RemoteDir, remotefile)

	file, err := os.Open(fullpath)
	if err != nil {
		return true
	}

	isUploadOk := false
	defer func() {
		file.Close()
		if isUploadOk == true {
			if cfg.MoveDir != "" && cfg.MoveDir != "del" {
				destFile := path.Join(cfg.MoveDir, strings.TrimPrefix(fullpath, cfg.LocalDir))
				os.MkdirAll(path.Dir(destFile), os.ModePerm)
				err := os.Rename(fullpath, destFile)
				if err != nil {
					fmt.Println("localfile move err=", err)
				} else {
					pblib.LogInfo(mainLOOP, "[localfile move to] %s", destFile)
				}
			} else if cfg.MoveDir == "del" {
				os.Remove(fullpath)
			}
		}
		// time.Sleep(time.Hour)
	}()

	doneCh := make(chan struct{})
	bar := progressbar.NewOptions(int(fi.Size),
		// progressbar.OptionSetWriter(ansi.NewAnsiStdout()),
		// progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowBytes(true),
		progressbar.OptionShowCount(),

		progressbar.OptionSetWidth(10),
		progressbar.OptionSetDescription("[uploading]"),
		progressbar.OptionOnCompletion(func() {
			// isDownloadOk = true
			// doneCh <- struct{}{}
		}),
	)

	//假设速率限制是每秒128kb
	//
	var rateVal int = 0
	t := time.Now().Local()
	minutes := t.Hour()*60 + t.Minute()
	for _, v := range cfg.LimitRate {
		calcMin := minutes
		//如果计算时间比最小的时间还要小,且范围是跨过了一天的
		if calcMin < v.Start && v.End > 24*60 {
			calcMin = calcMin + 24*60
		}
		if v.Start < calcMin && calcMin < v.End {
			rateVal = v.Rate
			break
		}
	}

	pblib.LogDebug(mainLOOP, "[start upload] %s", remotefile)
	if rateVal > 0 {
		pblib.LogDebug(mainLOOP, "[rateVal] %dkb/s", rateVal)
	}

	go func() {
		fmt.Println("fileoffset=", fileoffset)
		file.Seek(fileoffset, os.SEEK_SET)
		offset := fileoffset
		bar.Set64(offset)

		//限速器
		span := float64(step) / float64(rateVal*1024)
		var limiter *rate.Limiter = nil
		if rateVal > 0 {
			limiter = rate.NewLimiter(rate.Every(time.Millisecond*time.Duration(span*1000)), 1)
			//立马用掉余量
			limiter.Allow()
		}
		for {

			//如果限速器存在,这里开始玩限速
			if limiter != nil && limiter.Allow() == false {
				continue
			}

			b := make([]byte, step)
			n, _ := file.Read(b)
			uploadbinary := b[:n]
			a := uploadSlice(remotefile, offset, uploadbinary, fi, proof, cfg)
			bar.Add(n)
			// fmt.Println("a.FileOffset=", a.FileOffset)
			// fmt.Println("fi.Size=", fi.Size)

			if a.FileOffset == fi.Size {
				//这样就代表上传完成了
				isUploadOk = true
				break
			}
			offset = a.FileOffset
		}
		doneCh <- struct{}{}
	}()
	<-doneCh
	fmt.Println("isUploadOk", isUploadOk)

	//上传后移动走
	return isUploadOk
}

func uploadSlice(remotefile string, offset int64, uploadbytes []byte, fi pblib.FileInfo, proof *ProofInfo, cfg *setting.GobalClientConf) UploadRespInfo {

	psock := getsock_upload(cfg)

	sock := *psock

	doneCh := make(chan struct{})
	failCh := make(chan struct{})
	// doneCh <- struct{}{}
	var err error
	data := UploadRespInfo{}
	go func() {
		cmd := UploadCMD{
			RemoteFile: remotefile,
			FileOffset: offset,
			FileInfo:   fi,
			FileBinary: uploadbytes,
			Proof:      *proof,
		}
		var b bytes.Buffer
		enc := gob.NewEncoder(&b)
		err = enc.Encode(cmd)
		if err != nil {
			failCh <- struct{}{}
		}

		if e := sock.Send(b.Bytes()); e != nil {
			fmt.Printf("Cannot send req: %v\n", e)
			failCh <- struct{}{}
		}

		if m, e := sock.Recv(); e != nil {
			fmt.Printf("Cannot recv reply: %v\n", e)
			failCh <- struct{}{}
		} else {
			dec := gob.NewDecoder(bytes.NewBuffer(m))
			err = dec.Decode(&data)
			if err != nil {
				failCh <- struct{}{}
			} else if data.Status != 0 {
				fmt.Printf("reply:%s\n", data.Msg)
				failCh <- struct{}{}
			} else {
				doneCh <- struct{}{}
			}

			// fmt.Println("data=", data)
			// 成功
		}
	}()

	select {
	case <-time.After(time.Second * 2):
		fmt.Printf("time.After 2s\n")
		sock.Close()
		uploadsock = nil
		return uploadSlice(remotefile, offset, uploadbytes, fi, proof, cfg)
	case <-doneCh:
		return data
	case <-failCh:
		return uploadSlice(remotefile, offset, uploadbytes, fi, proof, cfg)
	}

}
