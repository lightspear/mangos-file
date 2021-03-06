package dealers

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	pblib "m/pblib"
	"m/setting"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/boltdb/bolt"
	"github.com/boltdb/boltd"
	"github.com/leekchan/timeutil"
	"github.com/schollz/progressbar/v3"
	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/protocol/req"
	"golang.org/x/time/rate"

	// register ws transport
	_ "go.nanomsg.org/mangos/v3/transport/ws"
)

type FindFileCallback func(string, int, *pblib.FileInfo, *ProofInfo)

var downloadsock *mangos.Socket = nil

func getsock_download(cfg *setting.GobalClientConf) *mangos.Socket {
	if downloadsock != nil {
		return downloadsock
	}
	sock, e := req.NewSocket()
	if e != nil {
		die("cannot make req socket: %v", e)
	}
	url := fmt.Sprintf("%s/download", cfg.WSServerUrl)
	sock.SetOption(mangos.OptionMaxReconnectTime, time.Second)
	if e = sock.Dial(url); e != nil {
		// die("cannot dial req url: %v", e)
		return getsock_download(cfg)
	}
	// Time for TCP connection set up
	sock.SetOption(mangos.OptionRecvDeadline, time.Second)
	sock.SetOption(mangos.OptionSendDeadline, time.Second)
	sock.SetOption(mangos.OptionBestEffort, true)
	downloadsock = &sock
	return downloadsock
}

func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

func deleteRemoteFile(remotefile string, proof *ProofInfo, cfg *setting.GobalClientConf) ClientCommonRespInfo {
	psock := getsock_common(cfg)

	sock := *psock

	doneCh := make(chan struct{})
	failCh := make(chan struct{})
	// doneCh <- struct{}{}
	var err error
	data := ClientCommonRespInfo{}
	go func() {

		s := DeleteFileCMD{
			Path: remotefile,
		}
		cmd := ClientCommonCMD{
			Cmd:   "deletefile",
			Proof: *proof,
			Body:  s,
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
			// ??????
		}

	}()

	select {
	case <-time.After(time.Second * 2):
		sock.Close()
		commonsock = nil
		fmt.Printf("time.After 2s\n")
		return deleteRemoteFile(remotefile, proof, cfg)
	case <-doneCh:
		return data
	case <-failCh:
		return deleteRemoteFile(remotefile, proof, cfg)
	}
}

func download(remotefile string, LocalDir string, fileinfo *pblib.FileInfo, proof *ProofInfo, cfg *setting.GobalClientConf) bool {

	//??????????????????,????????????128KB?????????,?????????????????????

	var step int = 1024 * cfg.SliceSize
	var offset int64 = 0

	// rel := strings.TrimPrefix(remotefile, cfg.RemoteDir)
	// fmt.Println("rel=", rel)
	localfile := path.Join(LocalDir, strings.TrimPrefix(remotefile, cfg.RemoteDir))
	//????????????
	localfileTmp := localfile + ".tmp"

	os.MkdirAll(path.Dir(localfile), 0644)
	localfilesize := pblib.GetFileSize(localfile)
	localfileTmpSize := pblib.GetFileSize(localfileTmp)

	if localfilesize >= fileinfo.Size {
		return false
	}

	//????????????????????????????????????,?????????????????????localfile
	if localfilesize >= 0 && localfilesize < fileinfo.Size {
		os.Remove(localfile)
	}
	//??????????????????????????????????????????????????????(???????????????????????????????????????????????????)
	if localfileTmpSize >= fileinfo.Size {
		os.Remove(localfileTmp)
		localfilesize = 0
	}

	//????????????????????????????????????
	if cfg.Resume == 1 && localfileTmpSize > 0 {
		offset = localfileTmpSize
	}
	var file *os.File
	//??????????????????????????????????????????????????????
	// fmt.Println("write to =", localfileTmp)
	// fmt.Println("offset=", offset)

	if offset == 0 {
		file, _ = os.OpenFile(localfileTmp, os.O_WRONLY|os.O_CREATE, 0755)
	} else {
		file, _ = os.OpenFile(localfileTmp, os.O_WRONLY|os.O_APPEND, 0755)
	}

	isDownloadOk := false

	defer func() {

		file.Close()
		if isDownloadOk == true {
			err := os.Rename(localfileTmp, localfile)
			if err == nil {
				pblib.LogInfo(mainLOOP, "[complete download] %s", localfile)
				os.Chtimes(localfile, time.UnixMilli(fileinfo.MTime), time.UnixMilli(fileinfo.MTime))
			}
		}

	}()
	doneCh := make(chan struct{})

	bar := progressbar.NewOptions(int(fileinfo.Size),
		// progressbar.OptionSetWriter(ansi.NewAnsiStdout()),
		// progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowBytes(true),
		progressbar.OptionShowCount(),

		progressbar.OptionSetWidth(10),
		progressbar.OptionSetDescription("[downloading]"),
		progressbar.OptionOnCompletion(func() {
			// isDownloadOk = true
			// doneCh <- struct{}{}
		}),
	)

	// io.MultiWriter()
	// writer := bufio.NewWriter(file)
	// file,_=os.OpenFile()
	// pblib.LogDebug(mainLOOP, "\n")
	pblib.LogDebug(mainLOOP, "[start download] %s", remotefile)
	//???????????????????????????128kb
	//
	var rateVal int = 0
	t := time.Now().Local()
	minutes := t.Hour()*60 + t.Minute()
	for _, v := range cfg.LimitRate {
		calcMin := minutes
		//?????????????????????????????????????????????,??????????????????????????????
		if calcMin < v.Start && v.End > 24*60 {
			calcMin = calcMin + 24*60
		}
		if v.Start < calcMin && calcMin < v.End {
			rateVal = v.Rate
			break
		}
	}

	// fmt.Println("rateVal=", rateVal)

	go func() {
		bar.Set64(offset)
		// bar.Reset()
		span := float64(step) / float64(rateVal*1024)
		var limiter *rate.Limiter = nil
		if rateVal > 0 {
			limiter = rate.NewLimiter(rate.Every(time.Millisecond*time.Duration(span*1000)), 1)
			//??????????????????
			limiter.Allow()
		}
		for {
			//?????????????????????,?????????????????????
			if limiter != nil && limiter.Allow() == false {
				continue
			}

			respInfo, _ := downloadSlice(remotefile, offset, step, proof, cfg)
			if respInfo.FileOffsetSize == 0 {
				isDownloadOk = true
				break
			} else {
				// fmt.Printf("write to FileBinary=%d\n", respInfo.FileOffsetSize)
				_, err := file.Write(respInfo.FileBinary)
				if err != nil {
					fmt.Printf("write to FileBinary,err=%v\n", err)
					break
				}
				bar.Add(respInfo.FileOffsetSize)
				// writer.Write(respInfo.FileBinary)
				// writer.Flush()
				// fmt.Printf("????????????:%.1f%%\n", (float64(offset) * 100 / float64(fileinfo.Size)))
			}
			offset += int64(min(step, respInfo.FileOffsetSize))
		}
		doneCh <- struct{}{}
	}()
	<-doneCh
	fmt.Println("isDownloadOk", isDownloadOk)
	// isDownloadOk = true
	// file.Close()
	// file = nil
	// isDownloadOk = true
	// fmt.Println("isDownloadOk=", isDownloadOk)
	// if isDownloadOk == true {
	// }
	return isDownloadOk
}

func downloadSlice(remotefile string, offset int64, downloadSize int, proof *ProofInfo, cfg *setting.GobalClientConf) (DownloadRespInfo, error) {

	psock := getsock_download(cfg)

	sock := *psock

	doneCh := make(chan struct{})
	failCh := make(chan struct{})
	sockErrCh := make(chan struct{})
	// doneCh <- struct{}{}
	var err error
	data := DownloadRespInfo{}
	go func() {
		cmd := DownloadCMD{
			RemoteFile:     remotefile,
			FileOffset:     offset,
			FileOffsetSize: downloadSize,
			Proof:          *proof,
		}
		buffers, _ := json.Marshal(cmd)
		if e := sock.Send(buffers); e != nil {
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
			}
			// fmt.Println("data=", data)
			// ??????
			doneCh <- struct{}{}
		}
	}()

	select {
	case <-time.After(time.Second * 2):
		fmt.Printf("time.After 2s\n")
		sock.Close()
		downloadsock = nil
		return downloadSlice(remotefile, offset, downloadSize, proof, cfg)
	case <-sockErrCh:
		sock.Close()
		downloadsock = nil
		return downloadSlice(remotefile, offset, downloadSize, proof, cfg)
	case <-doneCh:
		return data, nil
	case <-failCh:
		return downloadSlice(remotefile, offset, downloadSize, proof, cfg)
	}

}

func startDownloadTask(searchdir string, level int, cfg *setting.GobalClientConf, priorInfo *PriorInfo, fileCallback FindFileCallback) {

	// fmt.Println("searchdir=", searchdir)

	proof := ProofInfo{
		PWD:  cfg.UserPwd,
		Node: cfg.ResNode,
	}
	//??????????????????

	searchdir += "/"
	relativedir := strings.Replace(searchdir, cfg.RemoteDir, "", 1)
	if priorInfo != nil && priorInfo.SkipFunc != nil {
		searchdir += priorInfo.SkipFunc(relativedir, level)
	}

	psock := getsock_common(cfg)

	sock := *psock

	doneCh := make(chan struct{})
	failCh := make(chan struct{})
	var err error
	data := ClientCommonRespInfo{}

	cmd := ClientCommonCMD{
		Cmd:   "list",
		Proof: proof,
		Body: ListDirCMD{
			Dir: strings.TrimRight(searchdir, "/"),
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
			fmt.Printf("Cannot send req: %v\n", e)
			failCh <- struct{}{}
		}

		if m, e := sock.Recv(); e != nil {
			fmt.Printf("Cannot recv reply: %v\n", e)
			failCh <- struct{}{}
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
			// ??????
		}
	}()

	select {
	case <-time.After(time.Second * 2):
		fmt.Printf("list time.After 2s\n")
		commonsock = nil
		startDownloadTask(searchdir, level, cfg, priorInfo, fileCallback)
	case <-doneCh:
		{
			listFiles := []pblib.FileInfo{}
			json.Unmarshal([]byte(data.Body), &listFiles)
			// fmt.Println(listFiles)
			for _, v := range listFiles {
				if v.IsDir {
					startDownloadTask(path.Join(searchdir, v.Name), level+1, cfg, priorInfo, fileCallback)
				} else {
					remotefile := path.Join(searchdir, v.Name)
					if fileCallback != nil {
						fileCallback(remotefile, level, &v, &proof)
					}
				}
			}
		}
	case <-failCh:
		startDownloadTask(searchdir, level, cfg, priorInfo, fileCallback)
	}

}

func StartDownloadClient(cfg *setting.GobalClientConf) {
	logInfo := func(format string, a ...interface{}) {
		pblib.LogInfo(mainLOOP, format, a...)
	}
	logError := func(format string, a ...interface{}) {
		pblib.LogError(mainLOOP, format, a...)
	}
	logInfo("setting=%s", cfg.String())
MAIN:
	d1, _ := time.ParseDuration("24h")
	var StartDay time.Time
	var EndDay time.Time
	var err1 error
	if cfg.StartDay != "" {
		StartDay, err1 = time.Parse("2006-01-02", cfg.StartDay)
		if err1 != nil {
			logError("StartDay Parse Faild")
		}
	}
	if cfg.EndDay != "" {
		EndDay, err1 = time.Parse("2006-01-02", cfg.EndDay)
		if err1 != nil {
			logError("EndDay Parse Faild")
			return
		}
	}
	CurrentDate := StartDay.Add(-d1)
	TotalDownload := int64(0)
	for {
		today := time.Now()
		//?????????????????????
		if cfg.StartDay != "" {
			CurrentDate = CurrentDate.Add(d1)
			if today.Format("2006-01-02") == CurrentDate.Format("2006-01-02") {
				break
			}
			if cfg.EndDay != "" && EndDay.Format("2006-01-02") == CurrentDate.Format("2006-01-02") {
				break
			}
		} else {
			//????????????????????????????????????????????????
			CurrentDate = today
		}

		//?????????????????????????????????
		var logfile string = ""
		if cfg.Completelogfile != "" {
			logfile = timeutil.Strftime(&CurrentDate, cfg.Completelogfile)
		}
		if pblib.PathExists2(logfile) == true {
			continue
		}
		// fmt.Println("CurrentDate=", CurrentDate.Format("2006-01-02"))
		var priorInfo *PriorInfo = nil
		//???????????????????????????????????????????????????
		if cfg.PriorRule != "" {
			// var t time.Time
			// //?????????????????????
			// if cfg.StartDay != "" {
			// 	t = CurrentDate
			// } else {
			// 	t = today
			// }
			PriorRuleStr := timeutil.Strftime(&CurrentDate, cfg.PriorRule)
			PriorRuleStr = strings.TrimRight(PriorRuleStr, "/")
			lstRuleDirs := strings.Split(PriorRuleStr, "/")
			if PriorRuleStr == "" {
				lstRuleDirs = []string{}
			}
			lstRuleSize := len(lstRuleDirs)
			// fmt.Println("PriorRuleStr=", PriorRuleStr)
			priorInfo = &PriorInfo{
				DownloadCount: 0,
				SkipFunc: func(relativedir string, level int) string {
					//??????????????????,????????????
					appenddir := ""
					if cfg.PriorRule == "" {
						goto END
					}
					//????????????
					for i := level; i < lstRuleSize; i++ {
						ruleDir := lstRuleDirs[i]
						if ruleDir == "<dirs>" {
							break
						} else {
							appenddir += ruleDir
							appenddir += "/"
						}
					}
				END:
					// fmt.Println("appenddir=", appenddir)
					return appenddir
				},
			}
		} else {
			priorInfo = &PriorInfo{
				DownloadCount: 0,
				SkipFunc:      nil,
			}
		}

		//????????????????????????
		total, _ := readRecord(CurrentDate.Format("2006-01-02"))
		if total > 0 {
			priorInfo.DownloadCount = total
		}

		startDownloadTask(cfg.RemoteDir, 0, cfg, priorInfo, func(remotefile string, level int, v *pblib.FileInfo, proof *ProofInfo) {
			fileExt := path.Ext(v.Name)
			//???????????????????????????????????????
			if fileExt == ".tmp" {
				return
			}

			localdir := timeutil.Strftime(&CurrentDate, cfg.LocalDir)
			if cfg.StartDay != "" && priorInfo.SkipFunc == nil {
				//?????????????????????????????????????????????????????????????????????
				if time.UnixMilli(v.MTime).Format("2006-01-02") != CurrentDate.Format("2006-01-02") {
					return
				}
			}

			flag := download(remotefile, localdir, v, proof, cfg)
			//[????????????]????????????????????????????????????????????????
			if flag == true {
				priorInfo.DownloadCount++
				//???????????????????????????(?????????????????????????????????????????????????????????)
				if cfg.StartDay != "" {
					writeRecord(CurrentDate.Format("2006-01-02"), priorInfo.DownloadCount)
				}
				//???????????????????????????,???????????????????????????????????????
				if cfg.IsDelete == 1 {

					//fmt.Println("try delete remotefile=", remotefile)
					ret := deleteRemoteFile(remotefile, proof, cfg)
					if ret.Status == 0 {
						pblib.LogTrace(mainLOOP, "[delete remote file ok]")
					}

				}

			}
		})

		if priorInfo.DownloadCount > 0 {
			TotalDownload += priorInfo.DownloadCount

			// logfile := timeutil.Strftime(&CurrentDate, cfg.Completelogfile)
			fmt.Printf("[CurrentDate:%s]:%d\n", CurrentDate.Format("2006-01-02"), priorInfo.DownloadCount)

		}

		// ??????????????????
		if logfile != "" {
			os.MkdirAll(path.Dir(logfile), os.ModePerm)
			ioutil.WriteFile(logfile, []byte(fmt.Sprintf("%d", priorInfo.DownloadCount)), 0777)
		}

		//?????????????????????????????????????????????????????????
		if cfg.StartDay == "" {
			break
		}

		// time.Sleep(time.Second)
	}
	logInfo("??????????????????,????????????:%d", TotalDownload)
	if cfg.ExecInterval > 0 {
		time.Sleep(time.Duration(cfg.ExecInterval) * time.Second)
		goto MAIN
	}
}

func writeRecord(day string, count int64) error {
	if statisticsDB == nil {
		return nil
	}
	db := statisticsDB

	//?????????
	err := db.Update(func(tx *bolt.Tx) error {
		//?????????????????????????????????
		b := tx.Bucket([]byte("DayStatistics"))
		if b == nil {
			//?????????"MyBucket"??????
			_, err := tx.CreateBucket([]byte("DayStatistics"))
			if err != nil {
				//???????????????????????????????????????
				return nil
			}
		} else {
			total := strconv.FormatInt(count, 10)

			err := b.Put([]byte(day), []byte(total))
			if err != nil {
				log.Panic("??????????????????......")
			}
		}
		//???????????????nil
		return nil
	})
	return err
}

func readRecord(day string) (int64, error) {
	if statisticsDB == nil {
		return 0, nil
	}
	db := statisticsDB
	total := int64(0)

	//?????????
	err := db.View(func(tx *bolt.Tx) error {
		//?????????????????????????????????
		b := tx.Bucket([]byte("DayStatistics"))
		if b == nil {
			//?????????"MyBucket"??????
			_, err := tx.CreateBucket([]byte("DayStatistics"))
			if err != nil {
				//???????????????????????????????????????
				return err
			}
		} else {
			count := b.Get([]byte(day))
			total, _ = strconv.ParseInt(string(count), 10, 64)
		}
		//???????????????nil
		return nil
	})
	return total, err
}

var statisticsDB *bolt.DB

func StartViewWeb() {
	if setting.GClientConfig.RecordDb == "" {
		return
	}
	os.MkdirAll(path.Dir(setting.GClientConfig.RecordDb), os.ModePerm)
	db, err := bolt.Open(setting.GClientConfig.RecordDb, 0600, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	statisticsDB = db
	if setting.GClientConfig.DbViewHttpPort > 0 {
		http.Handle("/", boltd.NewHandler(db))
		port := fmt.Sprintf(":%d", setting.GClientConfig.DbViewHttpPort)
		go func() {
			log.Fatal(http.ListenAndServe(port, nil))
		}()
	}

	for {

	}
}

// func Test() {

// 	writeRecord("2020-10-10", 789)
// 	count, _ := readRecord("2020-10-10")
// 	fmt.Println("count=", count)

// 	listenDB()

// 	for {

// 	}
// }
