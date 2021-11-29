package dealers

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
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
}

func StartServer(cfg setting.GobalServerConf) {

	port := cfg.App.Port

	const logkind string = "mainloop"
	// logInfo := func(format string, a ...interface{}) {
	// 	pblib.LogInfo(logkind, format, a...)
	// }
	// logTrace := func(format string, a ...interface{}) {
	// 	pblib.LogTrace(logkind, format, a...)
	// }
	logError := func(format string, a ...interface{}) {
		pblib.LogError(logkind, format, a...)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/static", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "STATIC")
	})

	//增加应答复用路由命令/list
	addReqHandler(mux, port, "/list", func(sock mangos.Socket) {

		for {
			// don't care about the content of received message
			buffer, e := sock.Recv()
			if e != nil {
				logError("Cannot get request: %v", e)
				continue
			}

			listCMD := listCMD{}
			json.Unmarshal(buffer, &listCMD)
			var listFiles []pblib.FileInfo
			var reply []byte
			//
			item, ok := cfg.ResNodes[listCMD.Proof.Node]
			if !ok {
				logError("node:%s is not exist", listCMD.Proof.Node)
				goto END1
			}
			if item.UserPwd != listCMD.Proof.PWD {
				logError("node PWD is not match")
				goto END1
			}

			//如果存在
			listFiles, _ = pblib.ListDirDetail(path.Join(item.RootDir, listCMD.Dir))
			reply, _ = json.MarshalIndent(listFiles, "", " ")
			if e := sock.Send(reply); e != nil {
				die("Cannot send reply: %v", e)
			}
			continue

		END1:
			reply = []byte("[]")
			if e := sock.Send(reply); e != nil {
				logError("Cannot get request: %v", e)
			}
		}

	})
	//增加应答复用路由命令/download
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
	//增加应答复用路由命令/checkfile
	addReqHandler(mux, port, "/checkfile", func(sock mangos.Socket) {
		relyError := func(format string, a ...interface{}) {
			str := fmt.Sprintf(format, a...)
			respST := CheckFileRespInfo{
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
		relyInfo := func(resp *CheckFileRespInfo) {
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

			checkfilecmd := CheckFileCMD{}
			json.Unmarshal(buffer, &checkfilecmd)

			//
			item, ok := cfg.ResNodes[checkfilecmd.Proof.Node]
			if !ok {
				relyError("node:%s is not exist", checkfilecmd.Proof.Node)
				continue
			}
			if item.UserPwd != checkfilecmd.Proof.PWD {
				relyError("node PWD is not match")
				continue
			}

			//如果存在
			target := path.Join(item.RootDir, checkfilecmd.Path)
			targetTmp := target + ".tmp"
			fileInfo := pblib.FileInfo{}
			retFlag, _ := pblib.GetFileInfo(targetTmp, &fileInfo)

			var _FileOffset int64 = 0
			if retFlag {
				_FileOffset = fileInfo.Size
				//如果.tmp和目标传入文件大小一样,则直接
				if fileInfo.Size == checkfilecmd.FileInfo.Size {
					os.Rename(targetTmp, target)
				}
				if fileInfo.Size > checkfilecmd.FileInfo.Size {
					os.Remove(targetTmp)
					_FileOffset = 0
				}
			}

			checkfilerespinfo := CheckFileRespInfo{
				Status:     0,
				Msg:        "ok",
				FileOffset: _FileOffset,
			}

			relyInfo(&checkfilerespinfo)
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

			curfilePath := path.Join(item.RootDir, uploadcmd.RemoteFile)
			os.MkdirAll(path.Dir(curfilePath), os.ModePerm)

			curfilePathTmp := curfilePath + ".tmp"
			offset, _ := func() (int64, error) {
				fileAddr := curfilePathTmp
				buffer := uploadcmd.FileBinary
				fileInfo := pblib.FileInfo{}
				retFlag, _ := pblib.GetFileInfo(fileAddr, &fileInfo)
				var f *os.File
				var size int64 = 0
				var err error
				if retFlag && fileInfo.Size < uploadcmd.FileInfo.Size {
					size = fileInfo.Size
					f, _ = os.OpenFile(fileAddr, os.O_APPEND, 0666) //打开文件，
				} else {
					f, err = os.Create(fileAddr) //新建文件
				}
				defer f.Close() //使用完毕，需要关闭文件
				if err != nil {
					return size, err
				}
				_, err = f.Write(buffer)
				if err != nil {
					return size, nil
				}
				size = size + int64(len(buffer))
				return size, err
			}()

			//如果写入文件已经完成所有,则形成最终文件
			if uploadcmd.FileInfo.Size == offset {
				os.Rename(curfilePathTmp, curfilePath)
				os.Chtimes(curfilePath, time.UnixMilli(uploadcmd.FileInfo.MTime), time.UnixMilli(uploadcmd.FileInfo.MTime))
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

	//输出一些日志
	pblib.LogDebug(logkind, "server start listen:%d", port)
	e := http.ListenAndServe(fmt.Sprintf(":%d", port), mux)
	die("Http server died: %v", e)
}
