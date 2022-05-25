package dealers

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	pblib "m/pblib"
	"m/setting"
	"net/http"
	"os"
	"path"
	"time"

	// register ws transport
	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/protocol/rep"
	"go.nanomsg.org/mangos/v3/transport/ws"
	_ "go.nanomsg.org/mangos/v3/transport/ws"
)

type ReqHandler func(sock mangos.Socket)

//增加响应前缀
func addReqHandler(mux *http.ServeMux, port int, routeUrl string, reqhandler ReqHandler) {
	sock, _ := rep.NewSocket()
	url := fmt.Sprintf("ws://127.0.0.1:%d/", port)
	if l, e := sock.NewListener(url, nil); e != nil {
		die("bad listener: %v", e)
	} else if h, e := l.GetOption(ws.OptionWebSocketHandler); e != nil {
		die("bad handler: %v", e)
	} else {
		mux.Handle(routeUrl, h.(http.Handler))
		l.Listen()
	}
	go reqhandler(sock)
	pblib.LogTrace(mainLOOP, "listen:%s", routeUrl)
}

//读取文件临时信息
func readTmpInfoFile(targetInfoTmp string, checkfileinfo *CheckFileRespInfo) error {
	if pblib.FileExist2(targetInfoTmp) {
		buffers, _ := ioutil.ReadFile(targetInfoTmp)
		if len(buffers) == 0 {
			os.Remove(targetInfoTmp)
			return errors.New("targetInfoTmp读取失败")
		}
		//解析文件信息
		r1 := json.Unmarshal(buffers, checkfileinfo)
		if r1 != nil {
			os.Remove(targetInfoTmp)
			return errors.New("targetInfoTmp 解析失败")
		}
	}
	return nil
}

//写入文件临时信息
func writeTmpInfoFile(targetInfoTmp string, checkfileinfo *CheckFileRespInfo) error {
	buffer, err := json.Marshal(checkfileinfo)
	ioutil.WriteFile(targetInfoTmp, buffer, 0777)
	return err
}

func StartServer(cfg setting.GobalServerConf) {

	port := cfg.App.Port

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

	mux := http.NewServeMux()
	mux.HandleFunc("/static", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "STATIC")
	})

	logInfo("handler:%s", "/static")
	//增加应答复用路由命令/download(json)
	addReqHandler(mux, port, "/download", func(sock mangos.Socket) {

		relyError := func(format string, a ...interface{}) {
			str := fmt.Sprintf(format, a...)
			respST := DownloadRespInfo{
				Status: 1,
				Msg:    str,
			}
			var b bytes.Buffer
			enc := gob.NewEncoder(&b)
			err := enc.Encode(respST)
			if err != nil {
				fmt.Println(err)
			}
			if e := sock.Send(b.Bytes()); e != nil {
				logError("Cannot get request: %v", e)
			}
		}

		relyInfo := func(resp *DownloadRespInfo) {

			var b bytes.Buffer
			enc := gob.NewEncoder(&b)
			err := enc.Encode(resp)
			if err != nil {
				fmt.Println(err)
			}
			if e := sock.Send(b.Bytes()); e != nil {
				logError("Cannot get request: %v", e)
			}
		}

		for {
			// don't care about the content of received message
			buffer, e := sock.Recv()
			if e != nil {
				logError("Cannot get request: %v", e)
				continue
			}

			downloadCMD := DownloadCMD{}
			json.Unmarshal(buffer, &downloadCMD)
			// var listFiles []pblib.FileInfo
			//
			item, ok := cfg.ResNodes[downloadCMD.Proof.Node]
			if !ok {
				relyError("node:%s is not exist", downloadCMD.Proof.Node)
				continue
			}
			if item.UserPwd != downloadCMD.Proof.PWD {
				relyError("node PWD is not match")
				continue
			}

			func() {
				curfilePath := path.Join(item.RootDir, downloadCMD.RemoteFile)
				// fmt.Println("curfilePath=", curfilePath)
				file, _ := os.OpenFile(curfilePath, os.O_RDONLY, 0777)
				defer file.Close()
				bs := make([]byte, downloadCMD.FileOffsetSize)
				file.Seek(downloadCMD.FileOffset, os.SEEK_SET)
				n, _ := file.Read(bs)
				respST := DownloadRespInfo{
					Msg:            "ok",
					FileOffset:     downloadCMD.FileOffset,
					FileOffsetSize: n,
					FileBinary:     bs[:n],
				}
				relyInfo(&respST)
			}()

			//
		}
	})

	//增加应答复用路由命令/upload
	addReqHandler(mux, port, "/upload", func(sock mangos.Socket) {

		relyError := func(format string, a ...interface{}) {
			str := fmt.Sprintf(format, a...)
			respST := DownloadRespInfo{
				Status: 1,
				Msg:    str,
			}
			var b bytes.Buffer
			enc := gob.NewEncoder(&b)
			err := enc.Encode(respST)
			if err != nil {
				fmt.Println(err)
			}
			if e := sock.Send(b.Bytes()); e != nil {
				logError("Cannot get request: %v", e)
			}
		}

		relyInfo := func(resp *UploadRespInfo) {

			var b bytes.Buffer
			enc := gob.NewEncoder(&b)
			err := enc.Encode(resp)
			if err != nil {
				fmt.Println(err)
			}
			if e := sock.Send(b.Bytes()); e != nil {
				logError("Cannot get request: %v", e)
			}
		}

		for {
			// don't care about the content of received message
			uploadcmd := UploadCMD{}

			if m, e := sock.Recv(); e != nil {
				logError("Cannot get request: %v", e)
				continue
			} else {
				dec := gob.NewDecoder(bytes.NewBuffer(m))
				e = dec.Decode(&uploadcmd)
				if e != nil {
					logError("Cannot Decode uploadcmd: %v", e)
					continue
				}
			}
			//正式开始计算
			item, ok := cfg.ResNodes[uploadcmd.Proof.Node]
			if !ok {
				relyError("node:%s is not exist", uploadcmd.Proof.Node)
				continue
			}
			if item.UserPwd != uploadcmd.Proof.PWD {
				relyError("node PWD is not match")
				continue
			}
			// logInfo("/upload")
			var relyError = func() {
				respST := UploadRespInfo{
					Msg:        "上传出错",
					Status:     1,
					FileOffset: 0,
				}
				relyInfo(&respST)
			}

			curfilePath := path.Join(item.RootDir, uploadcmd.RemoteFile)
			os.MkdirAll(path.Dir(curfilePath), os.ModePerm)

			curfilePathTmp := curfilePath + ".tmp"
			curfileInfoPathTmp := curfilePath + ".upload.tmp"
			//首先要读出信息文件,如果信息文件不存在,并且
			checkfileinfo := &CheckFileRespInfo{
				FileOffset: 0,
			}
			readTmpInfoFile(curfileInfoPathTmp, checkfileinfo)

			if checkfileinfo.FileOffset != uploadcmd.FileOffset {
				logInfo("checkfileinfo.FileOffset=%d != uploadcmd.FileOffset:%d", checkfileinfo.FileOffset, uploadcmd.FileOffset)
				os.Remove(curfilePathTmp)
				os.Remove(curfilePath)
				os.Remove(curfileInfoPathTmp)
				relyError()
				continue
			}
			fileInfo := pblib.FileInfo{}
			pblib.GetFileInfo(curfilePathTmp, &fileInfo)

			if fileInfo.Size != uploadcmd.FileOffset {
				logInfo("fileInfo.Size != uploadcmd.FileOffset:%s", curfilePath)
				relyError()
				continue
			}

			// logInfo("开始接手文件:%s", curfilePath)

			offset, err := func() (int64, error) {

				fileAddr := curfilePathTmp
				buffer := uploadcmd.FileBinary
				var f *os.File
				var err error
				var size int64 = 0
				if fileInfo.Size == 0 {
					f, err = os.Create(fileAddr) //新建文件
					defer f.Close()              //使用完毕，需要关闭文件
				} else {
					size = fileInfo.Size
					// fmt.Printf("fileInfo2222.Size=%d\n", fileInfo.Size)
					f, err = os.OpenFile(fileAddr, os.O_CREATE|os.O_APPEND|os.O_WRONLY, os.ModeAppend) //打开文件，
					defer f.Close()                                                                    //使用完毕，需要关闭文件
				}
				if err != nil {
					logError("io open/create err=%s", err.Error())
					return size, err
				}

				_, err = f.Write(buffer)
				if err != nil {
					logError("io Write err=%s", err.Error())
					return size, err
				}
				// logInfo("写入文件：%d", fileAddr, len(buffer))
				size = size + int64(len(buffer))
				return size, err
			}()
			if err != nil {
				logError("err=%s", err.Error())
				respST := UploadRespInfo{
					Msg:        err.Error(),
					Status:     2,
					FileOffset: offset,
				}
				relyInfo(&respST)
				return
			}
			checkfileinfo.FileOffset = offset
			//继续写入curfileInfoPathTmp
			writeTmpInfoFile(curfileInfoPathTmp, checkfileinfo)

			//如果写入文件已经完成所有,则形成最终文件
			if uploadcmd.FileInfo.Size == offset {
				os.Rename(curfilePathTmp, curfilePath)
				os.Chtimes(curfilePath, time.UnixMilli(uploadcmd.FileInfo.MTime), time.UnixMilli(uploadcmd.FileInfo.MTime))
				logInfo("移动->%s", curfilePath)
				os.Remove(curfileInfoPathTmp)
				logTrace("删除->%s", curfileInfoPathTmp)
			}
			// fmt.Println("curfilePath=", curfilePath)
			respST := UploadRespInfo{
				Msg:        "ok",
				Status:     0,
				FileOffset: offset,
			}
			relyInfo(&respST)
		}
	})

	//把所有命令流向同一个连接方便减少连接数
	addReqHandler(mux, port, "/cmd", func(sock mangos.Socket) {
		var cmdstr string
		relyInfo := func(content string) {
			respST := ClientCommonRespInfo{
				Status: 0,
				Msg:    "ok",
				Body:   content,
			}
			buffer, _ := json.Marshal(respST)

			if e := sock.Send(buffer); e != nil {
				logError("cmd:%s,Cannot send reply: %v", cmdstr, e)
			}
		}
		relyError := func(format string, a ...interface{}) {
			str := fmt.Sprintf(format, a...)
			respST := ClientCommonRespInfo{
				Status: 1,
				Msg:    str,
			}
			buffer, _ := json.Marshal(respST)
			if e := sock.Send(buffer); e != nil {
				logError("cmd:%s,Cannot send reply: %v", cmdstr, e)
			}
		}
		//https://blog.csdn.net/weixin_42117918/article/details/105864520
		for {
			// don't care about the content of received message
			clientCMD := ClientCommonCMD{}
			if m, e := sock.Recv(); e != nil {
				logError("Cannot get request: %v", e)
				continue
			} else {
				dec := gob.NewDecoder(bytes.NewBuffer(m))
				e = dec.Decode(&clientCMD)
				if e != nil {
					logError("Cannot Decode uploadcmd: %v", e)
					continue
				}
			}
			cmdstr = clientCMD.Cmd

			proof := clientCMD.Proof
			//
			item, ok := cfg.ResNodes[proof.Node]
			if !ok {
				relyError("node:%s is not exist", proof.Node)
				continue
			}
			if item.UserPwd != proof.PWD {
				relyError("node PWD is not match")
				continue
			}

			if item.DeleteFile == 0 && clientCMD.Cmd == "deletefile" {
				relyError("node DeleteFile is not support")
				continue
			}

			switch clientCMD.Cmd {
			//增加应答复用路由删除/deletefile
			case "deletefile":
				{
					//删除文件必须手动支持配置才能允许被删除

					deleteFileCMD := clientCMD.Body.(DeleteFileCMD)
					curfilePath := path.Join(item.RootDir, deleteFileCMD.Path)
					// fmt.Println("curfilePath=", curfilePath)
					err := os.Remove(curfilePath)
					if err != nil {
						logError("deletefile %s,errinfo=", curfilePath, err.Error())
					}
					relyInfo("")
				}
				break
			case "list":
				{
					listDirCmd := clientCMD.Body.(ListDirCMD)
					// fmt.Println("curfilePath=", curfilePath)
					searchDir := path.Join(item.RootDir, listDirCmd.Dir)
					listFiles, err := pblib.ListDirDetail(searchDir)
					if err != nil {
						logError("listFiles %s,errinfo=", searchDir, err.Error())
					}
					reply, _ := json.MarshalIndent(listFiles, "", " ")
					relyInfo(string(reply))
				}
				break
			case "checkfile":
				{
					checkfilecmd := clientCMD.Body.(CheckFileCMD)
					target := path.Join(item.RootDir, checkfilecmd.Path)
					targetTmp := target + ".tmp"
					targetInfoTmp := target + ".upload.tmp"
					checkfilerespinfo := CheckFileRespInfo{
						FileOffset: 0,
						Status:     0,
					}
					func() {
						//假如信息文件存在那么直接读取出来
						if pblib.FileExist2(targetInfoTmp) {
							buffers, _ := ioutil.ReadFile(targetInfoTmp)
							if len(buffers) == 0 {
								logError("文件读取[%s] 读取失败", targetInfoTmp)
								return
							}
							//解析文件信息

							r1 := json.Unmarshal(buffers, &checkfilerespinfo)
							if r1 != nil {
								logError("[%s] 解析json失败", targetInfoTmp)
								return
							}
							checkfilerespinfo.Status = 0
							if checkfilerespinfo.FileOffset == checkfilecmd.FileInfo.Size {
								os.Rename(targetTmp, target)
								checkfilerespinfo.FileOffset = 0
							}
							if checkfilerespinfo.FileOffset > checkfilecmd.FileInfo.Size {
								os.Remove(targetTmp)
								checkfilerespinfo.FileOffset = 0
							}
						}
					}()
					//不管上面怎么输出只要看到完成文件是0,那么先清理掉文件信息临时文件,和文件临时文件
					if checkfilerespinfo.FileOffset == 0 {
						os.Remove(targetInfoTmp)
						os.Remove(targetTmp)
					}
					reply, _ := json.MarshalIndent(checkfilerespinfo, "", " ")
					relyInfo(string(reply))
				}
				break
			default:
				break
			}
			//
		}
	})

	//输出一些日志
	pblib.LogDebug(logkind, "server start listen:%d", port)
	e := http.ListenAndServe(fmt.Sprintf(":%d", port), mux)
	die("Http server died: %v", e)
}
