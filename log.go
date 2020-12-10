package loguru

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	LevelEmergency = iota
	LevelAlert
	LevelCritical
	LevelError
	LevelWarning
	LevelNotice
	LevelInformational
	LevelDebug
)

const levelLoggerImpl = -1

const (
	AdapterConsole   = "console"
	AdapterFile      = "file"
	AdapterOnline    = "online"
	AdapterMultiFile = "multifile"
	AdapterMail      = "smtp"
	AdapterConn      = "conn"
)

const (
	LevelInfo  = LevelInformational
	LevelTrace = LevelDebug
	LevelWarn  = LevelWarning
)

const (
	Console    = 1
	OnlineLog  = 2
	Write2File = 3
)

type newLoggerFunc func() Logger

type Logger interface {
	Init(config string) error
	WriteMsg(msg *LogMsg) error
	Destroy()
	Flush()
	SetFormatter(f LogFormatter)
}

var adapters = make(map[string]newLoggerFunc)
var levelPrefix = [LevelDebug + 1]string{"Emergency", "ALERT", "CRITICAL", "ERROR", "WARNING", "NOTICE", "INFO", "DEBUG"}
var levelNames = [...]string{"emergency", "alert", "critical", "error", "warning", "notice", "info", "debug"}

func Register(name string, log newLoggerFunc) {
	if log == nil {
		panic("logs: Register provide is nil")
	}
	if _, dup := adapters[name]; dup {
		panic("logs: Register called twice for provider " + name)
	}
	adapters[name] = log
}

type MyLogger struct {
	lock                sync.Mutex
	level               int
	init                bool
	mode                int
	enableFuncCallDepth bool
	loggerFuncCallDepth int
	asynchronous        bool
	prefix              string
	msgChanLen          int64
	msgChan             chan *LogMsg
	signalChan          chan string
	wg                  sync.WaitGroup
	outputs             []*nameLogger
}

const defaultAsyncMsgLen = 1e3

type nameLogger struct {
	Logger
	name string
}

var logMsgPool *sync.Pool

func NewLogger(mode int, channelLens ...int64) *MyLogger {
	bl := new(MyLogger)
	bl.mode = mode
	bl.level = LevelDebug
	bl.enableFuncCallDepth = true
	bl.loggerFuncCallDepth = 3
	bl.msgChanLen = append(channelLens, 0)[0]
	if bl.msgChanLen <= 0 {
		bl.msgChanLen = defaultAsyncMsgLen
	}
	bl.signalChan = make(chan string, 1)
	return bl
}

func (bl *MyLogger) Async(msgLen ...int64) *MyLogger {
	bl.lock.Lock()
	defer bl.lock.Unlock()
	if bl.asynchronous {
		return bl
	}
	bl.asynchronous = true
	if len(msgLen) > 0 && msgLen[0] > 0 {
		bl.msgChanLen = msgLen[0]
	}
	bl.msgChan = make(chan *LogMsg, bl.msgChanLen)
	logMsgPool = &sync.Pool{
		New: func() interface{} {
			return &LogMsg{}
		},
	}
	bl.wg.Add(1)
	go bl.startLogger()
	return bl
}

func (bl *MyLogger) setLogger(adapterName string, configs ...string) error {
	config := append(configs, "{}")[0]
	for _, l := range bl.outputs {
		if l.name == adapterName {
			return fmt.Errorf("logs: duplicate adaptername %q (you have set this logger before)", adapterName)
		}
	}

	logAdapter, ok := adapters[adapterName]
	if !ok {
		return fmt.Errorf("logs: unknown adaptername %q (forgotten Register?)", adapterName)
	}

	lg := logAdapter()
	err := lg.Init(config)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "logs.MyLogger.SetLogger: "+err.Error())
		return err
	}
	bl.outputs = append(bl.outputs, &nameLogger{name: adapterName, Logger: lg})
	return nil
}

func (bl *MyLogger) SetLogger(adapterName string, configs ...string) error {
	bl.lock.Lock()
	defer bl.lock.Unlock()
	if !bl.init {
		bl.outputs = []*nameLogger{}
		bl.init = true
	}
	return bl.setLogger(adapterName, configs...)
}

func (bl *MyLogger) DelLogger(adapterName string) error {
	bl.lock.Lock()
	defer bl.lock.Unlock()
	var outputs []*nameLogger
	for _, lg := range bl.outputs {
		if lg.name == adapterName {
			lg.Destroy()
		} else {
			outputs = append(outputs, lg)
		}
	}
	if len(outputs) == len(bl.outputs) {
		return fmt.Errorf("logs: unknown adaptername %q (forgotten Register?)", adapterName)
	}
	bl.outputs = outputs
	return nil
}

func (bl *MyLogger) writeToLoggers(when time.Time, msg string, level int) {
	for _, l := range bl.outputs {
		err := l.WriteMsg(&LogMsg{
			When:  when,
			Msg:   msg,
			Level: level,
		})
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "unable to WriteMsg to adapter:%v,error:%v\n", l.name, err)
		}
	}
}

func (bl *MyLogger) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}
	if p[len(p)-1] == '\n' {
		p = p[0 : len(p)-1]
	}
	err = bl.writeMsg(levelLoggerImpl, string(p))
	if err == nil {
		return len(p), err
	}
	return 0, err
}

func (bl *MyLogger) writeMsg(logLevel int, msg string, v ...interface{}) error {
	bl.lock.Lock()
	switch bl.mode {
	case 1:
		bl.loggerFuncCallDepth = 4
		_ = bl.setLogger(AdapterConsole)
	case 2:
		executePath, _ := os.Getwd()
		configBytes, _ := ioutil.ReadFile(executePath + "/logs/file.json")
		_ = bl.setLogger(AdapterFile, string(configBytes))
	case 3:
		executePath, _ := os.Getwd()
		configBytes, _ := ioutil.ReadFile(executePath + "/logs/online.json")
		_ = bl.setLogger(AdapterOnline, string(configBytes))
	}
	bl.lock.Unlock()

	if len(v) > 0 {
		msg = fmt.Sprintf(msg, v...)
	}

	when := time.Now()
	if bl.enableFuncCallDepth {
		_, file, line, ok := runtime.Caller(bl.loggerFuncCallDepth)
		if !ok {
			file = "???"
			line = 0
		}
		_, filename := path.Split(file)
		msg = "[" + filename + ":" + strconv.Itoa(line) + "] " + msg
	}

	if bl.asynchronous {
		lm := logMsgPool.Get().(*LogMsg)
		lm.Level = logLevel
		lm.Msg = msg
		lm.When = when
		if bl.outputs != nil {
			bl.msgChan <- lm
		} else {
			logMsgPool.Put(lm)
		}
	} else {
		bl.writeToLoggers(when, msg, logLevel)
	}
	return nil
}

func (bl *MyLogger) SetLevel(l int) {
	bl.level = l
}

func (bl *MyLogger) GetLevel() int {
	return bl.level
}

func (bl *MyLogger) SetLogFuncCallDepth(d int) {
	bl.loggerFuncCallDepth = d
}

func (bl *MyLogger) GetLogFuncCallDepth() int {
	return bl.loggerFuncCallDepth
}

func (bl *MyLogger) EnableFuncCallDepth(b bool) {
	bl.enableFuncCallDepth = b
}

func (bl *MyLogger) SetPrefix(s string) {
	bl.prefix = s
}

func (bl *MyLogger) startLogger() {
	gameOver := false
	for {
		select {
		case bm := <-bl.msgChan:
			bl.writeToLoggers(bm.When, bm.Msg, bm.Level)
			logMsgPool.Put(bm)
		case sg := <-bl.signalChan:
			bl.flush()
			if sg == "close" {
				for _, l := range bl.outputs {
					l.Destroy()
				}
				bl.outputs = nil
				gameOver = true
			}
			bl.wg.Done()
		}
		if gameOver {
			break
		}
	}
}

func (bl *MyLogger) Emergency(format string, v ...interface{}) {
	if LevelEmergency > bl.level {
		return
	}
	_ = bl.writeMsg(LevelEmergency, format, v...)
}

func (bl *MyLogger) Alert(format string, v ...interface{}) {
	if LevelAlert > bl.level {
		return
	}
	_ = bl.writeMsg(LevelAlert, format, v...)
}

func (bl *MyLogger) Critical(format string, v ...interface{}) {
	if LevelCritical > bl.level {
		return
	}
	_ = bl.writeMsg(LevelCritical, format, v...)
}

func (bl *MyLogger) Error(format string, v ...interface{}) {
	if LevelError > bl.level {
		return
	}
	_ = bl.writeMsg(LevelError, format, v...)
}

func (bl *MyLogger) Warning(format string, v ...interface{}) {
	if LevelWarn > bl.level {
		return
	}
	_ = bl.writeMsg(LevelWarn, format, v...)
}

func (bl *MyLogger) Notice(format string, v ...interface{}) {
	if LevelNotice > bl.level {
		return
	}
	_ = bl.writeMsg(LevelNotice, format, v...)
}

func (bl *MyLogger) Informational(format string, v ...interface{}) {
	if LevelInfo > bl.level {
		return
	}
	_ = bl.writeMsg(LevelInfo, format, v...)
}

func (bl *MyLogger) Debug(format string, v ...interface{}) {
	if LevelDebug > bl.level {
		return
	}
	_ = bl.writeMsg(LevelDebug, format, v...)
}

func (bl *MyLogger) Warn(format string, v ...interface{}) {
	if LevelWarn > bl.level {
		return
	}
	_ = bl.writeMsg(LevelWarn, format, v...)
}

func (bl *MyLogger) Info(format string, v ...interface{}) {
	if LevelInfo > bl.level {
		return
	}
	_ = bl.writeMsg(LevelInfo, format, v...)
}

func (bl *MyLogger) Trace(format string, v ...interface{}) {
	if LevelDebug > bl.level {
		return
	}
	_ = bl.writeMsg(LevelDebug, format, v...)
}

func (bl *MyLogger) Flush() {
	if bl.asynchronous {
		bl.signalChan <- "flush"
		bl.wg.Wait()
		bl.wg.Add(1)
		return
	}
	bl.flush()
}

func (bl *MyLogger) Close() {
	if bl.asynchronous {
		bl.signalChan <- "close"
		bl.wg.Wait()
		close(bl.msgChan)
	} else {
		bl.flush()
		for _, l := range bl.outputs {
			l.Destroy()
		}
		bl.outputs = nil
	}
	close(bl.signalChan)
}

func (bl *MyLogger) Reset() {
	bl.Flush()
	for _, l := range bl.outputs {
		l.Destroy()
	}
	bl.outputs = nil
}

func (bl *MyLogger) flush() {
	if bl.asynchronous {
		for {
			if len(bl.msgChan) > 0 {
				bm := <-bl.msgChan
				bl.writeToLoggers(bm.When, bm.Msg, bm.Level)
				logMsgPool.Put(bm)
				continue
			}
			break
		}
	}
	for _, l := range bl.outputs {
		l.Flush()
	}
}

var logger = NewLogger(1)

func GetLgLogger() *MyLogger {
	return logger
}

var loggerMap = struct {
	sync.RWMutex
	logs map[string]*log.Logger
}{
	logs: map[string]*log.Logger{},
}

func GetLogger(prefixes ...string) *log.Logger {
	prefix := append(prefixes, "")[0]
	if prefix != "" {
		prefix = fmt.Sprintf(`[%s] `, strings.ToUpper(prefix))
	}
	loggerMap.RLock()
	l, ok := loggerMap.logs[prefix]
	if ok {
		loggerMap.RUnlock()
		return l
	}
	loggerMap.RUnlock()
	loggerMap.Lock()
	defer loggerMap.Unlock()
	l, ok = loggerMap.logs[prefix]
	if !ok {
		l = log.New(logger, prefix, 0)
		loggerMap.logs[prefix] = l
	}
	return l
}

func Reset() {
	logger.Reset()
}

func Async(msgLen ...int64) *MyLogger {
	return logger.Async(msgLen...)
}

func SetLevel(l int) {
	logger.SetLevel(l)
}

func SetPrefix(s string) {
	logger.SetPrefix(s)
}

func EnableFuncCallDepth(b bool) {
	logger.enableFuncCallDepth = b
}

func SetLogFuncCall(b bool) {
	logger.EnableFuncCallDepth(b)
	logger.SetLogFuncCallDepth(4)
}

func SetLogFuncCallDepth(d int) {
	logger.loggerFuncCallDepth = d
}

func SetLogger(adapter string, config ...string) error {
	return logger.SetLogger(adapter, config...)
}

func Emergency(f interface{}, v ...interface{}) {
	logger.Emergency(formatLog(f, v...))
}

func Alert(f interface{}, v ...interface{}) {
	logger.Alert(formatLog(f, v...))
}

func Critical(f interface{}, v ...interface{}) {
	logger.Critical(formatLog(f, v...))
}

func Error(f interface{}, v ...interface{}) {
	logger.Error(formatLog(f, v...))
}

func Warning(f interface{}, v ...interface{}) {
	logger.Warn(formatLog(f, v...))
}

func Warn(f interface{}, v ...interface{}) {
	logger.Warn(formatLog(f, v...))
}

func Notice(f interface{}, v ...interface{}) {
	logger.Notice(formatLog(f, v...))
}

func Informational(f interface{}, v ...interface{}) {
	logger.Info(formatLog(f, v...))
}

func Info(f interface{}, v ...interface{}) {
	logger.Info(formatLog(f, v...))
}

func Debug(f interface{}, v ...interface{}) {
	logger.Debug(formatLog(f, v...))
}

func Trace(f interface{}, v ...interface{}) {
	logger.Trace(formatLog(f, v...))
}

func formatLog(f interface{}, v ...interface{}) string {
	var msg string
	switch f.(type) {
	case string:
		msg = f.(string)
		if len(v) == 0 {
			return msg
		}
		if strings.Contains(msg, "%") && !strings.Contains(msg, "%%") {
		} else {
			msg += strings.Repeat(" %v", len(v))
		}
	default:
		msg = fmt.Sprint(f)
		if len(v) == 0 {
			return msg
		}
		msg += strings.Repeat(" %v", len(v))
	}
	return fmt.Sprintf(msg, v...)
}

func DelLogger(name string) error {
	err := logger.DelLogger(name)
	if err != nil {
		return err
	}
	return nil
}
