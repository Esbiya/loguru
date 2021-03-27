package loguru

import (
	"encoding/json"
	"errors"
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
	LevelSuccess
	LevelNotice
	LevelInformational
	LevelInput
	LevelDebug
)

const levelLoggerImpl = -1

const (
	AdapterConsole = "console"
	AdapterFile    = "file"
	AdapterOnline  = "online"
	AdapterMail    = "smtp"
	AdapterConn    = "conn"
)

const (
	LevelInfo  = LevelInformational
	LevelTrace = LevelDebug
	LevelWarn  = LevelWarning
)

const (
	Console   = 1
	FileLog   = 2
	OnlineLog = 3
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
var levelPrefix = [LevelDebug + 1]string{"EMERGENCY", "ALERT", "CRITICAL", "ERROR", "WARNING", "SUCCESS", "NOTICE", "INFO", "INPUT", "DEBUG"}
var levelNames = [...]string{"emergency", "alert", "critical", "error", "warning", "notice", "info", "debug", "input", "success"}

func Register(name string, log newLoggerFunc) {
	if log == nil {
		panic("logs: Register provide is nil")
	}
	if _, dup := adapters[name]; dup {
		panic("logs: Register called twice for provider " + name)
	}
	adapters[name] = log
}

type Loguru struct {
	space               int
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

func NewLogger(mode int, channelLens ...int64) *Loguru {
	bl := new(Loguru)
	bl.mode = mode
	bl.level = LevelDebug
	bl.enableFuncCallDepth = true
	bl.loggerFuncCallDepth = 3
	bl.msgChanLen = append(channelLens, 0)[0]
	if bl.msgChanLen <= 0 {
		bl.msgChanLen = defaultAsyncMsgLen
	}
	bl.signalChan = make(chan string, 1)
	bl.space = 18
	return bl
}

func (bl *Loguru) Async(msgLen ...int64) *Loguru {
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

func (bl *Loguru) setLogger(adapterName string, configs ...string) error {
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
		_, _ = fmt.Fprintln(os.Stderr, "loguru.SetLogger: "+err.Error())
		return err
	}
	bl.outputs = append(bl.outputs, &nameLogger{name: adapterName, Logger: lg})
	return nil
}

func (bl *Loguru) SetLogger(adapterName string, configs ...string) error {
	bl.lock.Lock()
	defer bl.lock.Unlock()
	if !bl.init {
		bl.outputs = []*nameLogger{}
		bl.init = true
	}
	return bl.setLogger(adapterName, configs...)
}

func (bl *Loguru) DelLogger(adapterName string) error {
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

func (bl *Loguru) writeToLoggers(when time.Time, msg string, level int) {
	for _, l := range bl.outputs {
		err := l.WriteMsg(&LogMsg{
			Space: bl.space,
			When:  when,
			Msg:   msg,
			Level: level,
		})
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "unable to WriteMsg to adapter:%v,error:%v\n", l.name, err)
		}
	}
}

func (bl *Loguru) Write(p []byte) (n int, err error) {
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

func (bl *Loguru) writeMsg(logLevel int, msg string, v ...interface{}) error {
	bl.lock.Lock()
	switch bl.mode {
	case Console:
		_ = bl.setLogger(AdapterConsole)
	case FileLog:
		executePath, _ := os.Getwd()
		configBytes, _ := ioutil.ReadFile(executePath + "/logs/file.json")
		_ = bl.setLogger(AdapterFile, string(configBytes))
	case OnlineLog:
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

func (bl *Loguru) SetLevel(l int) {
	bl.level = l
}

func (bl *Loguru) GetLevel() int {
	return bl.level
}

func (bl *Loguru) SetLogFuncCallDepth(d int) {
	bl.loggerFuncCallDepth = d
}

func (bl *Loguru) GetLogFuncCallDepth() int {
	return bl.loggerFuncCallDepth
}

func (bl *Loguru) EnableFuncCallDepth(b bool) {
	bl.enableFuncCallDepth = b
}

func (bl *Loguru) SetPrefix(s string) {
	bl.prefix = s
}

func (bl *Loguru) startLogger() {
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

func (bl *Loguru) Emergency(format string, v ...interface{}) {
	if LevelEmergency > bl.level {
		return
	}
	_ = bl.writeMsg(LevelEmergency, format, v...)
}

func (bl *Loguru) Alert(format string, v ...interface{}) {
	if LevelAlert > bl.level {
		return
	}
	_ = bl.writeMsg(LevelAlert, format, v...)
}

func (bl *Loguru) Critical(format string, v ...interface{}) {
	if LevelCritical > bl.level {
		return
	}
	_ = bl.writeMsg(LevelCritical, format, v...)
}

func (bl *Loguru) Error(format string, v ...interface{}) {
	if LevelError > bl.level {
		return
	}
	_ = bl.writeMsg(LevelError, format, v...)
}

func (bl *Loguru) Warning(format string, v ...interface{}) {
	if LevelWarn > bl.level {
		return
	}
	_ = bl.writeMsg(LevelWarn, format, v...)
}

func (bl *Loguru) Notice(format string, v ...interface{}) {
	if LevelNotice > bl.level {
		return
	}
	_ = bl.writeMsg(LevelNotice, format, v...)
}

func (bl *Loguru) Informational(format string, v ...interface{}) {
	if LevelInfo > bl.level {
		return
	}
	_ = bl.writeMsg(LevelInfo, format, v...)
}

func (bl *Loguru) Debug(format string, v ...interface{}) {
	if LevelDebug > bl.level {
		return
	}
	_ = bl.writeMsg(LevelDebug, format, v...)
}

func (bl *Loguru) Warn(format string, v ...interface{}) {
	if LevelWarn > bl.level {
		return
	}
	_ = bl.writeMsg(LevelWarn, format, v...)
}

func (bl *Loguru) Info(format string, v ...interface{}) {
	if LevelInfo > bl.level {
		return
	}
	_ = bl.writeMsg(LevelInfo, format, v...)
}

func (bl *Loguru) Success(format string, v ...interface{}) {
	if LevelSuccess > bl.level {
		return
	}
	_ = bl.writeMsg(LevelSuccess, format, v...)
}

func (bl *Loguru) Input(format string, v ...interface{}) {
	if LevelDebug > bl.level {
		return
	}
	_ = bl.writeMsg(LevelInput, format, v...)
}

func (bl *Loguru) Trace(format string, v ...interface{}) {
	if LevelDebug > bl.level {
		return
	}
	_ = bl.writeMsg(LevelDebug, format, v...)
}

func (bl *Loguru) Flush() {
	if bl.asynchronous {
		bl.signalChan <- "flush"
		bl.wg.Wait()
		bl.wg.Add(1)
		return
	}
	bl.flush()
}

func (bl *Loguru) Close() {
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

func (bl *Loguru) Reset() {
	bl.Flush()
	for _, l := range bl.outputs {
		l.Destroy()
	}
	bl.outputs = nil
}

func (bl *Loguru) flush() {
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

var logger = NewLogger(Console)

func GetLgLogger() *Loguru {
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

func Async(msgLen ...int64) *Loguru {
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

func Success(f interface{}, v ...interface{}) {
	logger.Success(formatLog(f, v...))
}

func Trace(f interface{}, v ...interface{}) {
	logger.Trace(formatLog(f, v...))
}

func Input(f interface{}, v ...interface{}) string {
	var r string
	logger.Input(formatLog(f, v...))
	fmt.Scanf("%s", &r)
	return r
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

func SetColor(level int, color string) {
	colors = append(colors[:level], colors[level+1:]...)
	after := append([]brush{}, colors[level:]...)
	colors = append(colors[0:level], newBrush(color))
	colors = append(colors, after...)
}

func ResetEmergencyColor(color string) {
	SetColor(LevelEmergency, color)
}

func ResetAlertColor(color string) {
	SetColor(LevelAlert, color)
}

func ResetCriticalColor(color string) {
	SetColor(LevelCritical, color)
}

func ResetErrorColor(color string) {
	SetColor(LevelError, color)
}

func ResetWarningColor(color string) {
	SetColor(LevelWarning, color)
}

func ResetSuccessColor(color string) {
	SetColor(LevelSuccess, color)
}

func ResetNoticeColor(color string) {
	SetColor(LevelNotice, color)
}

func ResetInfoColor(color string) {
	SetColor(LevelInfo, color)
}

func ResetDebugColor(color string) {
	SetColor(LevelDebug, color)
}

func ResetTimeColor(color string) {
	timeColor = newBrush(color)
}

func ResetFileColor(color string) {
	fileColor = newBrush(color)
}

func ResetSpace(space int) {
	logger.space = space
}

func Enable(mode int) error {
	switch mode {
	case FileLog:
		if !IsDir("./logs/") {
			_ = os.Mkdir("./logs/", 0777)
		}
		if !Exists("./logs/file.json") {
			f, _ := os.Create("./logs/file.json")
			data, _ := json.MarshalIndent(map[string]interface{}{
				"filename": "logs/out.log",
				"maxLines": 10000,
				"maxsize":  5242880,
				"daily":    true,
				"maxDays":  7,
				"rotate":   true,
				"perm":     "0600",
			}, "", "    ")
			_, _ = f.Write(data)
		}
		executePath, _ := os.Getwd()
		configBytes, err := ioutil.ReadFile(executePath + "/logs/file.json")
		if err != nil {
			return err
		}
		return logger.setLogger(AdapterFile, string(configBytes))
	case OnlineLog:
		executePath, _ := os.Getwd()
		configBytes, err := ioutil.ReadFile(executePath + "/logs/online.json")
		if err != nil {
			return err
		}
		return logger.setLogger(AdapterOnline, string(configBytes))
	default:
		return errors.New("unknown log type")
	}
}

func Disable(mode int) error {
	var err error
	switch mode {
	case FileLog:
		err = logger.DelLogger(AdapterFile)
	case OnlineLog:
		err = logger.DelLogger(AdapterOnline)
	default:
		err = errors.New("unknown log type")
	}
	return err
}
