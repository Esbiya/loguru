package loguru

import (
	"fmt"
	"net"
)

type OnlineLogger struct {
	conn          net.Conn
	app           string
	onlineLogHost string
}

func NewOnlineLogger(host, app string) (*OnlineLogger, error) {
	c, err := net.Dial("tcp", host)
	if err != nil {
		return nil, err
	}
	return &OnlineLogger{
		conn:          c,
		app:           app,
		onlineLogHost: host,
	}, nil
}

func (o *OnlineLogger) DisplayLogOnline(level, msg string) {
	message := append([]byte(fmt.Sprintf("+msg|%s|%s|%s", o.app, level, msg)), 0)
	_, _ = o.conn.Write(message)
}
