// +build linux

package m

import (
	"syscall"
	"unsafe"

	color "github.com/fatih/color"
)

var FpErr *color.Color
var FpInfo *color.Color
var FpDebug *color.Color
var FpTrace *color.Color
var FpWarn *color.Color

var Plogfunc uintptr

func init() {
	Plogfunc = 0
	FpErr = color.New(color.FgHiRed, color.Bold)
	FpInfo = color.New(color.FgHiGreen, color.Bold)
	FpDebug = color.New(color.FgHiCyan, color.Bold)
	FpTrace = color.New(color.FgHiYellow, color.Bold)
	FpWarn = color.New(color.FgHiYellow, color.Bold)

	// loglevelMap[0] = "LogTrace"
}

func Logsys(logtype int, str string) {

	syscall.Syscall(Plogfunc,
		uintptr(2),
		uintptr(logtype),
		uintptr(unsafe.Pointer(syscall.StringBytePtr(str))))

}
