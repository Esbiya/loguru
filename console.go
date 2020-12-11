package loguru

import (
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"github.com/shiena/ansicolor"
	"os"
	"strings"
)

type consoleWriter struct {
	lg        *logWriter
	formatter LogFormatter
	Formatter string `json:"formatter"`
	Level     int    `json:"level"`
	Colorful  bool   `json:"color"`
}

func (c *consoleWriter) Format(lm *LogMsg) string {
	msg := lm.ColorStyleFormat()
	if c.Colorful {
		msg = strings.Replace(msg, levelPrefix[lm.Level], colors[lm.Level](levelPrefix[lm.Level]), 1)
	}
	h, _, _ := formatTimeHeader(lm.When)
	bytes := append(append([]byte(colorsMap["white"](string(h))), colorsMap["red"](" |  ")...), msg...)
	return string(bytes)
}

func (c *consoleWriter) SetFormatter(f LogFormatter) {
	c.formatter = f
}

func NewConsole() Logger {
	return newConsole()
}

func newConsole() *consoleWriter {
	cw := &consoleWriter{
		lg:       newLogWriter(ansicolor.NewAnsiColorWriter(os.Stdout)),
		Level:    LevelDebug,
		Colorful: true,
	}
	cw.formatter = cw
	return cw
}

func (c *consoleWriter) Init(config string) error {

	if len(config) == 0 {
		return nil
	}

	res := json.Unmarshal([]byte(config), c)
	if res == nil && len(c.Formatter) > 0 {
		fmtr, ok := GetFormatter(c.Formatter)
		if !ok {
			return errors.New(fmt.Sprintf("the formatter with name: %s not found", c.Formatter))
		}
		c.formatter = fmtr
	}
	return res
}

func (c *consoleWriter) WriteMsg(lm *LogMsg) error {
	if lm.Level > c.Level {
		return nil
	}
	msg := c.formatter.Format(lm)
	_, _ = c.lg.writeln(msg)
	return nil
}

func (c *consoleWriter) Destroy() {

}

func (c *consoleWriter) Flush() {

}

func init() {
	Register(AdapterConsole, NewConsole)
}
