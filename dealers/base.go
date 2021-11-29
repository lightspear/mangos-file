package dealers

import (
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

//上传文件前文件的预校验
type CheckFileCMD struct {
	Path     string
	FileInfo pblib.FileInfo
	Proof    ProofInfo
}

type CheckFileRespInfo struct {
	Status     int
	Msg        string
	FileOffset int64
}

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
