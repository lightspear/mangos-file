// +build windows

package m

import (
	"fmt"
	"io"
	"syscall"
	"unsafe"

	"os/exec"
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

type Color struct {
	colorVal int
}

func (c *Color) Fprintf(w io.Writer, format string, a ...interface{}) (n int, err error) {
	handle, _, _ := proc.Call(uintptr(syscall.Stdout), uintptr(c.colorVal))
	defer CloseHandle.Call(handle)
	return fmt.Fprintf(w, format, a...)
}

func (c *Color) Printf(format string, a ...interface{}) (n int, err error) {
	handle, _, _ := proc.Call(uintptr(syscall.Stdout), uintptr(c.colorVal))
	defer CloseHandle.Call(handle)
	return fmt.Printf(format, a...)
}

func (c *Color) Println(a ...interface{}) (n int, err error) {
	handle, _, _ := proc.Call(uintptr(syscall.Stdout), uintptr(c.colorVal))
	defer CloseHandle.Call(handle)
	return fmt.Println(a...)
}

var FpErr Color
var FpInfo Color
var FpWarn Color
var FpDebug Color
var FpTrace Color

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

	colorEnum := ColorEnum{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	FpErr.colorVal = colorEnum.light_red
	FpInfo.colorVal = colorEnum.light_green
	FpDebug.colorVal = colorEnum.light_cyan
	FpWarn.colorVal = colorEnum.light_purple
	FpTrace.colorVal = colorEnum.light_yellow
}

func Logsys(logtype int, str string) {

	syscall.Syscall(Plogfunc,
		uintptr(2),
		uintptr(logtype),
		uintptr(unsafe.Pointer(syscall.StringBytePtr(str))), 0)

}

type ColorEnum struct {
	black        int // 黑色
	blue         int // 蓝色
	green        int // 绿色
	cyan         int // 青色
	red          int // 红色
	purple       int // 紫色
	yellow       int // 黄色
	light_gray   int // 淡灰色（系统默认值）
	gray         int // 灰色
	light_blue   int // 亮蓝色
	light_green  int // 亮绿色
	light_cyan   int // 亮青色
	light_red    int // 亮红色
	light_purple int // 亮紫色
	light_yellow int // 亮黄色
	white        int // 白色
}
