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

func ProcessSpace(lm *LogMsg) (string, string, string) {
	c1 := " |  "
	switch lm.Level {
	case 0:
	case 2:
		c1 = " " + c1
	case 1, 3, 8, 9:
		c1 = "    " + c1
	case 4, 5:
		c1 = "  " + c1
	case 6:
		c1 = "   " + c1
	case 7:
		c1 = "     " + c1
	}
	msg1 := strings.Split(lm.Msg, " ")
	msg2 := strings.Replace(lm.Msg, msg1[0], "", 1)

	space := " "
	for i := 0; i < 18-(len(msg1[0])); i++ {
		space += " "
	}
	msg3 := fmt.Sprintf("%s%s ▶  ", msg1[0], space)
	return c1, msg2, msg3
}

func (lm *LogMsg) ColorStyleFormat() string {
	msg := lm.Msg

	if len(lm.Args) > 0 {
		lm.Msg = fmt.Sprintf(lm.Msg, lm.Args...)
	}

	c1, msg2, msg3 := ProcessSpace(lm)
	msg = lm.Prefix + colorsMap["red"](c1) + fileColor(msg3) + colors[lm.Level](msg2)

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

	c1, msg2, msg3 := ProcessSpace(lm)

	if lm.enableFuncCallDepth {
		filePath := lm.FilePath
		if !lm.enableFullFilePath {
			_, filePath = path.Split(filePath)
		}
		msg = fmt.Sprintf("[%s:%d] %s", filePath, lm.LineNumber, msg)
	}

	msg = "| " + levelPrefix[lm.Level] + c1 + msg3 + msg2
	return msg
}
