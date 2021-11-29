package m

import (
	"fmt"

	"os"
	"time"
)

var loglevelMap map[int]string

// 	0: "LogTrace",
// 	1: "LogDebug",
// 	2: "LogInfo",
// 	3: "LogWarn",
// 	4: "LogError"
// }

type LogEntity struct {
	RoomId      string
	Level       int32
	Repeat      int32
	Text        string
	Millisecond int64
}

var LogCh chan LogEntity // 声明一个传递int切片的通道
var IsSendGRPCLog bool

func init() {

	IsSendGRPCLog = false
	// loglevelMap[0] = "LogTrace"
}

func LogTrace(logkind, format string, a ...interface{}) {
	str := fmt.Sprintf(format, a...)
	t := time.Now().Local()
	timestr := t.Format("2006/01/02 15:04:05.000")
	FpTrace.Fprintf(os.Stdout, "%s [% 13s] %s\n", timestr, logkind, str)
	//压入通道
	if IsSendGRPCLog {
		ent := LogEntity{
			RoomId:      logkind,
			Level:       0,
			Repeat:      0,
			Millisecond: t.UnixNano() / 1e6,
			Text:        fmt.Sprintf(format, a...),
		}
		LogCh <- ent
	}
}

func LogInfo(logkind, format string, a ...interface{}) {
	str := fmt.Sprintf(format, a...)
	t := time.Now().Local()
	timestr := t.Format("2006/01/02 15:04:05.000")
	FpInfo.Fprintf(os.Stdout, "%s [% 13s] %s\n", timestr, logkind, str)

	//压入通道
	if IsSendGRPCLog {
		ent := LogEntity{
			RoomId:      logkind,
			Level:       2,
			Repeat:      0,
			Millisecond: t.UnixNano() / 1e6,
			Text:        fmt.Sprintf(format, a...),
		}
		LogCh <- ent
	}
}

func LogDebug(logkind, format string, a ...interface{}) {
	str := fmt.Sprintf(format, a...)
	t := time.Now().Local()
	timestr := t.Format("2006/01/02 15:04:05.000")
	FpDebug.Fprintf(os.Stdout, "%s [% 13s] %s\n", timestr, logkind, str)

	//压入通道
	if IsSendGRPCLog {
		ent := LogEntity{
			RoomId:      logkind,
			Level:       1,
			Repeat:      0,
			Millisecond: t.UnixNano() / 1e6,
			Text:        fmt.Sprintf(format, a...),
		}
		LogCh <- ent
	}
}

func LogError(logkind, format string, a ...interface{}) {
	str := fmt.Sprintf(format, a...)
	t := time.Now().Local()
	timestr := t.Format("2006/01/02 15:04:05.000")
	FpErr.Fprintf(os.Stdout, "%s [% 13s] %s\n", timestr, logkind, str)

	//压入通道
	if IsSendGRPCLog {
		ent := LogEntity{
			RoomId:      logkind,
			Level:       4,
			Repeat:      0,
			Millisecond: t.UnixNano() / 1e6,
			Text:        fmt.Sprintf(format, a...),
		}
		LogCh <- ent
	}
}

func NetLogRepeat(logkind, format string, a ...interface{}) {
	str := fmt.Sprintf(format, a...)
	t := time.Now().Local()
	timestr := t.Format("2006/01/02 15:04:05.000")
	FpErr.Fprintf(os.Stdout, "\r%s [% 13s] %s", timestr, logkind, str)

	if IsSendGRPCLog {
		ent := LogEntity{
			RoomId:      logkind,
			Level:       2,
			Repeat:      1,
			Millisecond: t.UnixNano() / 1e6,
			Text:        fmt.Sprintf(format, a...),
		}
		LogCh <- ent
	}
}

//按room模糊清理
func NetLogClear(logkind string) {

}
