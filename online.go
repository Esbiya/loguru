/*
 * @Author: your name
 * @Date: 2020-12-10 10:26:15
 * @LastEditTime: 2021-03-27 09:56:38
 * @LastEditors: your name
 * @Description: In User Settings Edit
 * @FilePath: /loguru/online.go
 */
package loguru

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"
)

type OnlineLogger struct {
	sync.Mutex
	conn      net.Conn
	Host      string `json:"host"`
	App       string `json:"app"`
	Formatter string `json:"formatter"`
	formatter LogFormatter
}

func (o *OnlineLogger) Format(lm *LogMsg) string {
	msg := lm.NormalFormat()
	hd, _, _ := formatTimeHeader(lm.When)
	msg = fmt.Sprintf("%s %s\n", string(hd), msg)
	return msg
}

func (o *OnlineLogger) Init(config string) error {
	err := json.Unmarshal([]byte(config), o)
	if err != nil {
		return err
	}
	c, err := net.Dial("tcp", o.Host)
	if err != nil {
		return err
	}
	o.conn = c
	return err
}

func (o *OnlineLogger) WriteMsg(lm *LogMsg) error {
	msg := o.formatter.Format(lm)
	message := append([]byte(fmt.Sprintf("+msg|%s|%s|%s", o.App, levelNames[lm.Level], msg)), 0)
	_, err := o.conn.Write(message)
	return err
}

func (o *OnlineLogger) SetFormatter(f LogFormatter) {
	o.formatter = f
}

func (o *OnlineLogger) Destroy() {
	_ = o.conn.Close()
}

func (o *OnlineLogger) Flush() {
	for _, level := range levelNames {
		message := append([]byte(fmt.Sprintf("-input|%s|%s", o.App, level)))
		_, _ = o.conn.Write(message)
	}
}

func NewOnlineLogger() Logger {
	cw := &OnlineLogger{}
	cw.formatter = cw
	return cw
}

func init() {
	Register(AdapterOnline, NewOnlineLogger)
}
