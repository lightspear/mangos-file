// +build windows

package m

import (
	"fmt"
	"io"
	"syscall"
	"unsafe"

	"os/exec"

	color "github.com/fatih/color"
)

var (
	kernel32             *syscall.LazyDLL  = syscall.NewLazyDLL(`kernel32.dll`)
	getconsolewindowProc *syscall.LazyProc = kernel32.NewProc(`GetConsoleWindow`)
	proc                 *syscall.LazyProc = kernel32.NewProc(`SetConsoleTextAttribute`)
	CloseHandle          *syscall.LazyProc = kernel32.NewProc(`CloseHandle`)
	user32               *syscall.LazyDLL  = syscall.NewLazyDLL(`user32.dll`)

	getsystemmetricsProc *syscall.LazyProc = user32.NewProc(`GetSystemMetrics`)
	getwindowrectProc    *syscall.LazyProc = user32.NewProc(`GetWindowRect`)
	moveWindowProc       *syscall.LazyProc = user32.NewProc(`MoveWindow`)

	// 给字体颜色对象赋值

)

type PrintColor struct {
	fgcolor color.Attribute
}

func (c *PrintColor) Fprintf(w io.Writer, format string, a ...interface{}) (n int, err error) {

	color.Set(c.fgcolor, color.Bold)
	defer color.Unset()
	return fmt.Fprintf(w, format, a...)
}

func (c *PrintColor) Printf(format string, a ...interface{}) (n int, err error) {
	color.Set(c.fgcolor, color.Bold)
	defer color.Unset()
	return fmt.Printf(format, a...)
}

func (c *PrintColor) Println(a ...interface{}) (n int, err error) {
	color.Set(c.fgcolor, color.Bold)
	defer color.Unset()
	return fmt.Println(a...)
}

var FpErr PrintColor
var FpInfo PrintColor
var FpWarn PrintColor
var FpDebug PrintColor
var FpTrace PrintColor

func CenterWindowInDesktop() {
	// a := rect{}
	hwnd, _, _ := getconsolewindowProc.Call()
	// getwindowrectProc.Call(hwnd, uintptr(unsafe.Pointer(&a)))
	// fmt.Println("a=", a)
	w, _, _ := getsystemmetricsProc.Call(uintptr(0))
	h, _, _ := getsystemmetricsProc.Call(uintptr(1))

	win_w := int32(float32(w) * 0.8)
	win_h := int32(float32(h) * 0.8)
	x := (int32(w) - win_w) / 2
	y := (int32(h) - win_h) / 2
	moveWindowProc.Call(hwnd, uintptr(x), uintptr(y), uintptr(win_w), uintptr(win_h), uintptr(1))
}

var Plogfunc uintptr

func init() {
	Plogfunc = 0
	// exec.Command("cls")
	exec.Command("mode con cols=120 lines=1000")
	// exec.Command("title", "fasdfsd")
	CenterWindowInDesktop()

	FpErr.fgcolor = color.FgHiRed
	FpInfo.fgcolor = color.FgHiGreen
	FpDebug.fgcolor = color.FgHiCyan
	FpWarn.fgcolor = color.FgHiMagenta
	FpTrace.fgcolor = color.FgHiYellow
}

func Logsys(logtype int, str string) {

	syscall.Syscall(Plogfunc,
		uintptr(2),
		uintptr(logtype),
		uintptr(unsafe.Pointer(syscall.StringBytePtr(str))), 0)

}
