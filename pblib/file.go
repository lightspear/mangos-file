package m

import (
	"errors"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"
)

func PathExists(path string) (bool, error) {

	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func PathExists2(path string) bool {

	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return false
}

func FileExist2(filePath string) bool {
	s, err := os.Stat(filePath)
	if err != nil {
		return false
	}
	return !s.IsDir()
}

func DirExist2(fileAddr string) bool {
	s, err := os.Stat(fileAddr)
	if err != nil {
		return false
	}
	return s.IsDir()
}

func WriteFile(fileAddr string, buffer []byte) (int64, error) {
	b := FileExist2(fileAddr)
	var f *os.File
	var size int64 = 0
	var err error
	if b {
		//打开文件，
		f, _ = os.OpenFile(fileAddr, os.O_APPEND, 0666)
	} else {
		//新建文件
		f, err = os.Create(fileAddr)
	}

	//使用完毕，需要关闭文件
	defer f.Close()

	if err != nil {
		return size, err
	}

	fi, _ := f.Stat()
	if err != nil {
		return size, err
	}

	size = fi.Size()

	_, err = f.Write(buffer)

	if err != nil {
		return size, err
	}
	return size, nil
}

func CopyFile(dstFileName string, srcFileName string) error {

	srcFile, err := os.Open(srcFileName)

	if err != nil {
		return err
	}

	defer srcFile.Close()

	os.MkdirAll(path.Dir(dstFileName), 0644)

	//打开dstFileName

	dstFile, err := os.OpenFile(dstFileName, os.O_WRONLY|os.O_CREATE, 0755)
	if err != nil {

		return err
	}

	defer dstFile.Close()
	io.Copy(dstFile, srcFile)
	return nil

}

func MoveFile(source, destination string) (err error) {
	src, err := os.Open(source)
	if err != nil {
		return err
	}
	defer src.Close()
	fi, err := src.Stat()
	if err != nil {
		return err
	}
	flag := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	perm := fi.Mode() & os.ModePerm
	dst, err := os.OpenFile(destination, flag, perm)
	if err != nil {
		return err
	}
	defer dst.Close()
	_, err = io.Copy(dst, src)
	if err != nil {
		dst.Close()
		os.Remove(destination)
		return err
	}
	err = dst.Close()
	if err != nil {
		return err
	}
	err = src.Close()
	if err != nil {
		return err
	}
	err = os.Remove(source)
	if err != nil {
		return err
	}
	return nil
}

func GetFileSize(path string) int64 {
	fs, err := os.Stat(path)
	if err == nil {
		return fs.Size()
	} else {
		return -1
	}
}

type FileInfo struct {
	IsDir bool
	Name  string
	Size  int64
	MTime int64
}

func GetFileInfo(filePath string, pfileInfo *FileInfo) (bool, error) {
	if FileExist2(filePath) == false {
		return false, errors.New("file is not exist")
	}

	fi, err1 := os.Stat(filePath)
	if err1 != nil {
		return false, err1
	}
	pfileInfo.Name = fi.Name()
	pfileInfo.IsDir = false
	pfileInfo.Size = fi.Size()
	pfileInfo.MTime = fi.ModTime().UnixMilli()
	return true, nil
}

//取出文件详细信息
func ListDirDetail(dirPth string) (files []FileInfo, err error) {
	_files := []FileInfo{}

	//如果连目录都不存在就直接返回了
	if DirExist2(dirPth) == false {
		return _files, nil
	}

	dir, err := ioutil.ReadDir(dirPth)
	if err != nil {
		return nil, err
	}
	for _, fi := range dir {
		info := FileInfo{}
		info.Name = fi.Name()
		if fi.IsDir() { // 忽略目录
			info.IsDir = true
		} else {
			info.IsDir = false
			info.Size = fi.Size()
			info.MTime = fi.ModTime().UnixMilli()
		}
		_files = append(_files, info)
	}
	return _files, nil
}

//搜索文件专用

type CallbackFileFunc func(fullpath string, relativepath string) bool
type CallbackSkipDirFunc func(relativedir string, level int) string
type CallbackIgnoreDirFunc func(relativedir string, level int) bool

func SearchFiles(rootdir, searchdir string, level int, skipfunc CallbackSkipDirFunc, ignoreFunc CallbackIgnoreDirFunc, callback CallbackFileFunc) (err error) {
	if searchdir == "" {
		searchdir = rootdir
	}

	searchdir += "/"
	relativedir := strings.Replace(searchdir, rootdir, "", 1)
	searchdir += skipfunc(relativedir, level)

	dir, err := ioutil.ReadDir(searchdir)
	if err != nil {
		return err
	}
	for _, fi := range dir {
		fullpath := path.Join(searchdir, fi.Name())
		if fi.IsDir() { // 目录, 递归遍历
			//假如确实处于忽略目录,那就跳过
			if ignoreFunc(fi.Name(), level+1) == true {
				continue
			}
			SearchFiles(rootdir, fullpath, level+1, skipfunc, ignoreFunc, callback)
		} else {
			//filepath兼容跨平台
			fullpath := path.Join(searchdir, fi.Name())
			relativepath := strings.Replace(fullpath, rootdir, "", 1)
			//一旦执行完回调结果返回false,则退出循环
			if callback(fullpath, relativepath) == false {
				return nil
			}
		}
	}
	return nil
}
