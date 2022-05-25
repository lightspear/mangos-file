package dealers

import (
	"encoding/gob"
	"fmt"
	pblib "m/pblib"
	"os"
)

func die(format string, v ...interface{}) {
	fmt.Fprintln(os.Stderr, fmt.Sprintf(format, v...))
	os.Exit(1)
}

const mainLOOP string = "mainloop"

type RespInfo struct {
	Code int
	Msg  string
}

type CallbackSkipDirFunc func(relativedir string, level int) string

type PriorInfo struct {
	DownloadCount int64
	SkipFunc      CallbackSkipDirFunc
}

type ProofInfo struct {
	Name string
	PWD  string
	Node string
}

type listCMD struct {
	Dir   string
	Proof ProofInfo
}

type ListDirCMD struct {
	Dir string
}

//
type DeleteFileCMD struct {
	Path string
}

type DeleteFileRespInfo struct {
	Status int
	Msg    string
}

//上传文件前文件的预校验
type CheckFileCMD struct {
	Path     string
	FileInfo pblib.FileInfo
}

type CheckFileRespInfo struct {
	FileOffset int64
	Status     int
	Msg        string
}

//下载和上传暂不改造

type DownloadCMD struct {
	RemoteFile     string
	FileOffset     int64
	FileOffsetSize int
	FileInfo       pblib.FileInfo
	Proof          ProofInfo
}

type DownloadRespInfo struct {
	Status         int
	Msg            string
	FileOffset     int64
	FileOffsetSize int
	FileBinary     []byte
}

type UploadCMD struct {
	RemoteFile string
	FileOffset int64
	FileInfo   pblib.FileInfo
	Proof      ProofInfo
	FileBinary []byte
}

type UploadRespInfo struct {
	Status     int
	Msg        string
	FileOffset int64
}

//统一命令发送
type ClientCommonCMD struct {
	Cmd   string
	Proof ProofInfo
	Body  interface{}
}

//统一命令发送
type ClientCommonRespInfo struct {
	Status int
	Msg    string
	Body   string
}

func init() {

	gob.Register(DeleteFileCMD{})
	gob.Register(ListDirCMD{})
	gob.Register(CheckFileCMD{})

}
