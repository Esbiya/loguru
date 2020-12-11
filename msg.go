package loguru

import (
	"fmt"
	"path"
	"strings"
	"time"
)

type LogMsg struct {
	Level               int
	Msg                 string
	When                time.Time
	FilePath            string
	LineNumber          int
	Args                []interface{}
	Prefix              string
	enableFullFilePath  bool
	enableFuncCallDepth bool
}

func (lm *LogMsg) ColorStyleFormat() string {
	msg := lm.Msg

	if len(lm.Args) > 0 {
		lm.Msg = fmt.Sprintf(lm.Msg, lm.Args...)
	}

	c1 := " |  "
	switch lm.Level {
	case 0:
	case 2:
		c1 = " " + c1
	case 1, 3, 8:
		c1 = "    " + c1
	case 4, 5:
		c1 = "  " + c1
	case 6:
		c1 = "   " + c1
	case 7:
		c1 = "     " + c1
	}
	msg1 := strings.Split(msg, " ")
	msg2 := strings.Replace(msg, msg1[0], "", 1)

	space := " "
	for i := 0; i < 18-(len(msg1[0])); i++ {
		space += " "
	}
	msg3 := fmt.Sprintf("%s%s ▶  ", msg1[0], space)
	msg = lm.Prefix + colorsMap["red"](c1) + colorsMap["white"](msg3) + colors[lm.Level](msg2)

	if lm.enableFuncCallDepth {
		filePath := lm.FilePath
		if !lm.enableFullFilePath {
			_, filePath = path.Split(filePath)
		}
		msg = fmt.Sprintf("[%s:%d] %s", filePath, lm.LineNumber, msg)
	}

	msg = levelPrefix[lm.Level] + " " + msg
	return msg
}

func (lm *LogMsg) NormalFormat() string {
	msg := lm.Msg

	if len(lm.Args) > 0 {
		lm.Msg = fmt.Sprintf(lm.Msg, lm.Args...)
	}

	if lm.enableFuncCallDepth {
		filePath := lm.FilePath
		if !lm.enableFullFilePath {
			_, filePath = path.Split(filePath)
		}
		msg = fmt.Sprintf("[%s:%d] %s", filePath, lm.LineNumber, msg)
	}

	msg = " " + levelPrefix[lm.Level] + " " + msg
	return msg
}
