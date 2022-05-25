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

var uploadok_count int64 = 0

func StartUploadTask(cfg *setting.GobalClientConf) {
	fmt.Println("上传模式")

	if cfg.IsDelete == 0 && cfg.MoveDir == "" {
		fmt.Println("不能同时IsDelete==false且MoveDir==空")
		return
	}

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
	// uploadok_count := 0
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
		time.Sleep(time.Second)
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
	// sock.SetOption(mangos.OptionRetryTime, 5)

	uploadsock = &sock
	return uploadsock
}

// var checkfilesock *mangos.Socket = nil

// func getsock_checkfile(cfg *setting.GobalClientConf) *mangos.Socket {
// 	if checkfilesock != nil {
// 		return checkfilesock
// 	}

// 	sock, e := req.NewSocket()
// 	if e != nil {
// 		die("cannot make req socket: %v", e)
// 	}
// 	url := fmt.Sprintf("%s/checkfile", cfg.WSServerUrl)
// 	if e = sock.Dial(url); e != nil {
// 		// die("cannot dial req url: %v", e)
// 		return getsock_checkfile(cfg)
// 	}
// 	// Time for TCP connection set up
// 	sock.SetOption(mangos.OptionRecvDeadline, time.Second)
// 	sock.SetOption(mangos.OptionSendDeadline, time.Second)
// 	sock.SetOption(mangos.OptionBestEffort, true)
// 	checkfilesock = &sock
// 	return checkfilesock
// }

func uploadfile(fullpath string, proof *ProofInfo, cfg *setting.GobalClientConf) bool {
	fileInfo := pblib.FileInfo{}
	retFlag, _ := pblib.GetFileInfo(fullpath, &fileInfo)
	if retFlag == false {
		return true
	}

	psock := getsock_common(cfg)
	sock := *psock

	relativePath := path.Join(cfg.RemoteDir, strings.TrimPrefix(fullpath, cfg.LocalDir))

	doneCh := make(chan struct{})
	failCh := make(chan struct{})
	sockErrCh := make(chan struct{})

	var err error
	data := ClientCommonRespInfo{}

	cmd := ClientCommonCMD{
		Cmd:   "checkfile",
		Proof: *proof,
		Body: CheckFileCMD{
			Path:     relativePath,
			FileInfo: fileInfo,
		},
	}

	go func() {
		var b bytes.Buffer
		enc := gob.NewEncoder(&b)
		err = enc.Encode(cmd)
		if err != nil {
			failCh <- struct{}{}
		}

		if e := sock.Send(b.Bytes()); e != nil {
			fmt.Printf("uploadfile Cannot send req: %v\n", e)
			sockErrCh <- struct{}{}
		}

		if m, e := sock.Recv(); e != nil {
			fmt.Printf("uploadfile Cannot recv reply: %v\n", e)
			sockErrCh <- struct{}{}
		} else {

			err = json.Unmarshal(m, &data)
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
		fmt.Printf("uploadfile time.After 2s\n")
		sock.Close()
		commonsock = nil
		return uploadfile(fullpath, proof, cfg)
	case <-sockErrCh:
		sock.Close()
		commonsock = nil
		// fmt.Printf("sockErrCh restart uploadfile\n")
		return uploadfile(fullpath, proof, cfg)
	case <-doneCh:
		resp := CheckFileRespInfo{}
		json.Unmarshal([]byte(data.Body), &resp)

		isUploadOk := upload(fullpath, resp.FileOffset, proof, cfg)
		if isUploadOk == false {
			sock.Close()
			commonsock = nil
			return uploadfile(fullpath, proof, cfg)
		}
	case <-failCh:
		// sock.Close()
		// checkfilesock = nil
		return uploadfile(fullpath, proof, cfg)
	}
	// fmt.Println("fullpath=", fullpath)
	// os.Remove(fullpath)
	return false
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
			if cfg.MoveDir != "" {
				localdir := strings.TrimPrefix(cfg.LocalDir, "./")
				// fmt.Println("fullpath=", fullpath)
				// fmt.Println("localdir", localdir)
				destFile := path.Join(cfg.MoveDir, strings.TrimPrefix(fullpath, localdir))
				os.MkdirAll(path.Dir(destFile), os.ModePerm)
				err := os.Rename(fullpath, destFile)
				if err != nil {
					fmt.Println("localfile move err=", err)
				} else {
					uploadok_count++
					pblib.LogInfo(mainLOOP, "[localfile move to] %s,uploadok_count:%d", destFile, uploadok_count)

				}
			} else if cfg.IsDelete == 1 {
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
	//开始计算是否在限速时段,这个时段限速是多少
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
			// fmt.Printf("上传大小%d,:%d\n", offset, len(uploadbinary))
			// time.Sleep(time.Second)

			a := uploadSlice(remotefile, offset, uploadbinary, fi, proof, cfg)
			if a.Status == 1 {
				//
				isUploadOk = false
				break
			}
			// time.Sleep(time.Second * 2)

			bar.Add(n)
			// fmt.Println("a.FileOffset=", a.FileOffset)
			// fmt.Println("fi.Size=", fi.Size)

			if a.FileOffset == fi.Size {
				//这样就代表上传完成了
				isUploadOk = true
				break
			}
			// fmt.Println("")
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
	sockErrCh := make(chan struct{})
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
		// fmt.Printf("restart sock.Send:\n")
		if e := sock.Send(b.Bytes()); e != nil {
			fmt.Printf("uploadSlice Cannot send req: %v\n", e)
			sockErrCh <- struct{}{}
		}

		// fmt.Printf("restart sock.Recv:\n")
		if m, e := sock.Recv(); e != nil {
			fmt.Printf("uploadSlice Cannot recv reply: %v\n", e)
			// fmt.Printf("time.After 2s\n")
			// time.Sleep(time.Millisecond * 1000 * 2)
			sockErrCh <- struct{}{}
		} else {

			dec := gob.NewDecoder(bytes.NewBuffer(m))
			err = dec.Decode(&data)
			// fmt.Printf("sock.Recv:%v\n", data)
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
		fmt.Printf("uploadSlice time.After 2s\n")
		sock.Close()
		uploadsock = nil
		return uploadSlice(remotefile, offset, uploadbytes, fi, proof, cfg)
	case <-sockErrCh:
		sock.Close()
		uploadsock = nil
		data.Status = 1
		return data
	case <-doneCh:
		//fmt.Printf("sock doneCh:%v\n", data)
		return data
	case <-failCh:
		sock.Close()
		uploadsock = nil
		data.Status = 1
		return data
		// sock.Close()
		// uploadsock = nil
		// fmt.Printf("sock failCh:%v\n", data)
		// return uploadSlice(remotefile, offset, uploadbytes, fi, proof, cfg)
	}

}
