package loguru

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type fileLogWriter struct {
	sync.RWMutex
	Filename   string `json:"filename"`
	fileWriter *os.File

	MaxLines         int `json:"maxlines"`
	maxLinesCurLines int

	MaxFiles         int `json:"maxfiles"`
	MaxFilesCurFiles int

	MaxSize        int `json:"maxsize"`
	maxSizeCurSize int

	Daily         bool  `json:"daily"`
	MaxDays       int64 `json:"maxdays"`
	dailyOpenDate int
	dailyOpenTime time.Time

	Hourly         bool  `json:"hourly"`
	MaxHours       int64 `json:"maxhours"`
	hourlyOpenDate int
	hourlyOpenTime time.Time

	Rotate bool `json:"rotate"`

	Level int `json:"level"`

	Perm string `json:"perm"`

	RotatePerm string `json:"rotateperm"`

	fileNameOnly, suffix string

	formatter LogFormatter
	Formatter string `json:"formatter"`
}

func newFileWriter() Logger {
	w := &fileLogWriter{
		Daily:      true,
		MaxDays:    7,
		Hourly:     false,
		MaxHours:   168,
		Rotate:     true,
		RotatePerm: "0440",
		Level:      LevelTrace,
		Perm:       "0660",
		MaxLines:   10000000,
		MaxFiles:   999,
		MaxSize:    1 << 28,
	}
	w.formatter = w
	return w
}

func (w *fileLogWriter) Format(lm *LogMsg) string {
	msg := lm.NormalFormat()
	hd, _, _ := formatTimeHeader(lm.When)
	msg = fmt.Sprintf("%s %s\n", string(hd), msg)
	return msg
}

func (w *fileLogWriter) SetFormatter(f LogFormatter) {
	w.formatter = f
}

func (w *fileLogWriter) Init(config string) error {
	err := json.Unmarshal([]byte(config), w)
	if err != nil {
		return err
	}
	if len(w.Filename) == 0 {
		return errors.New("json config must have filename")
	}
	w.suffix = filepath.Ext(w.Filename)
	w.fileNameOnly = strings.TrimSuffix(w.Filename, w.suffix)
	if w.suffix == "" {
		w.suffix = ".log"
	}

	if len(w.Formatter) > 0 {
		fmtr, ok := GetFormatter(w.Formatter)
		if !ok {
			return errors.New(fmt.Sprintf("the formatter with name: %s not found", w.Formatter))
		}
		w.formatter = fmtr
	}
	err = w.startLogger()
	return err
}

func (w *fileLogWriter) startLogger() error {
	file, err := w.createLogFile()
	if err != nil {
		return err
	}
	if w.fileWriter != nil {
		_ = w.fileWriter.Close()
	}
	w.fileWriter = file
	return w.initFd()
}

func (w *fileLogWriter) needRotateDaily(day int) bool {
	return (w.MaxLines > 0 && w.maxLinesCurLines >= w.MaxLines) ||
		(w.MaxSize > 0 && w.maxSizeCurSize >= w.MaxSize) ||
		(w.Daily && day != w.dailyOpenDate)
}

func (w *fileLogWriter) needRotateHourly(hour int) bool {
	return (w.MaxLines > 0 && w.maxLinesCurLines >= w.MaxLines) ||
		(w.MaxSize > 0 && w.maxSizeCurSize >= w.MaxSize) ||
		(w.Hourly && hour != w.hourlyOpenDate)

}

func (w *fileLogWriter) WriteMsg(lm *LogMsg) error {
	if lm.Level > w.Level {
		return nil
	}

	_, d, h := formatTimeHeader(lm.When)

	msg := w.formatter.Format(lm)
	if w.Rotate {
		w.RLock()
		if w.needRotateHourly(h) {
			w.RUnlock()
			w.Lock()
			if w.needRotateHourly(h) {
				if err := w.doRotate(lm.When); err != nil {
					_, _ = fmt.Fprintf(os.Stderr, "FileLogWriter(%q): %s\n", w.Filename, err)
				}
			}
			w.Unlock()
		} else if w.needRotateDaily(d) {
			w.RUnlock()
			w.Lock()
			if w.needRotateDaily(d) {
				if err := w.doRotate(lm.When); err != nil {
					_, _ = fmt.Fprintf(os.Stderr, "FileLogWriter(%q): %s\n", w.Filename, err)
				}
			}
			w.Unlock()
		} else {
			w.RUnlock()
		}
	}

	w.Lock()
	_, err := w.fileWriter.Write([]byte(msg))
	if err == nil {
		w.maxLinesCurLines++
		w.maxSizeCurSize += len(msg)
	}
	w.Unlock()
	return err
}

func (w *fileLogWriter) createLogFile() (*os.File, error) {
	perm, err := strconv.ParseInt(w.Perm, 8, 64)
	if err != nil {
		return nil, err
	}

	filePath := path.Dir(w.Filename)
	_ = os.MkdirAll(filePath, os.FileMode(perm))

	fd, err := os.OpenFile(w.Filename, os.O_WRONLY|os.O_APPEND|os.O_CREATE, os.FileMode(perm))
	if err == nil {
		_ = os.Chmod(w.Filename, os.FileMode(perm))
	}
	return fd, err
}

func (w *fileLogWriter) initFd() error {
	fd := w.fileWriter
	fInfo, err := fd.Stat()
	if err != nil {
		return fmt.Errorf("get stat err: %s", err)
	}
	w.maxSizeCurSize = int(fInfo.Size())
	w.dailyOpenTime = time.Now()
	w.dailyOpenDate = w.dailyOpenTime.Day()
	w.hourlyOpenTime = time.Now()
	w.hourlyOpenDate = w.hourlyOpenTime.Hour()
	w.maxLinesCurLines = 0
	if w.Hourly {
		go w.hourlyRotate(w.hourlyOpenTime)
	} else if w.Daily {
		go w.dailyRotate(w.dailyOpenTime)
	}
	if fInfo.Size() > 0 && w.MaxLines > 0 {
		count, err := w.lines()
		if err != nil {
			return err
		}
		w.maxLinesCurLines = count
	}
	return nil
}

func (w *fileLogWriter) dailyRotate(openTime time.Time) {
	y, m, d := openTime.Add(24 * time.Hour).Date()
	nextDay := time.Date(y, m, d, 0, 0, 0, 0, openTime.Location())
	tm := time.NewTimer(time.Duration(nextDay.UnixNano() - openTime.UnixNano() + 100))
	<-tm.C
	w.Lock()
	if w.needRotateDaily(time.Now().Day()) {
		if err := w.doRotate(time.Now()); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "FileLogWriter(%q): %s\n", w.Filename, err)
		}
	}
	w.Unlock()
}

func (w *fileLogWriter) hourlyRotate(openTime time.Time) {
	y, m, d := openTime.Add(1 * time.Hour).Date()
	h, _, _ := openTime.Add(1 * time.Hour).Clock()
	nextHour := time.Date(y, m, d, h, 0, 0, 0, openTime.Location())
	tm := time.NewTimer(time.Duration(nextHour.UnixNano() - openTime.UnixNano() + 100))
	<-tm.C
	w.Lock()
	if w.needRotateHourly(time.Now().Hour()) {
		if err := w.doRotate(time.Now()); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "FileLogWriter(%q): %s\n", w.Filename, err)
		}
	}
	w.Unlock()
}

func (w *fileLogWriter) lines() (int, error) {
	fd, err := os.Open(w.Filename)
	if err != nil {
		return 0, err
	}
	defer fd.Close()

	buf := make([]byte, 32768) // 32k
	count := 0
	lineSep := []byte{'\n'}

	for {
		c, err := fd.Read(buf)
		if err != nil && err != io.EOF {
			return count, err
		}

		count += bytes.Count(buf[:c], lineSep)

		if err == io.EOF {
			break
		}
	}

	return count, nil
}

func (w *fileLogWriter) doRotate(logTime time.Time) error {
	num := w.MaxFilesCurFiles + 1
	fName := ""
	format := ""
	var openTime time.Time
	rotatePerm, err := strconv.ParseInt(w.RotatePerm, 8, 64)
	if err != nil {
		return err
	}

	_, err = os.Lstat(w.Filename)
	if err != nil {
		goto RestartLogger
	}

	if w.Hourly {
		format = "2006010215"
		openTime = w.hourlyOpenTime
	} else if w.Daily {
		format = "2006-01-02"
		openTime = w.dailyOpenTime
	}

	if w.MaxLines > 0 || w.MaxSize > 0 {
		for ; err == nil && num <= w.MaxFiles; num++ {
			fName = w.fileNameOnly + fmt.Sprintf(".%s.%03d%s", logTime.Format(format), num, w.suffix)
			_, err = os.Lstat(fName)
		}
	} else {
		fName = w.fileNameOnly + fmt.Sprintf(".%s.%03d%s", openTime.Format(format), num, w.suffix)
		_, err = os.Lstat(fName)
		w.MaxFilesCurFiles = num
	}

	if err == nil {
		return fmt.Errorf("Rotate: Cannot find free log number to rename %s", w.Filename)
	}

	w.fileWriter.Close()

	err = os.Rename(w.Filename, fName)
	if err != nil {
		goto RestartLogger
	}

	err = os.Chmod(fName, os.FileMode(rotatePerm))

RestartLogger:

	startLoggerErr := w.startLogger()
	go w.deleteOldLog()

	if startLoggerErr != nil {
		return errors.New(fmt.Sprintf("Rotate StartLogger: %s", startLoggerErr.Error()))
	}
	if err != nil {
		return errors.New(fmt.Sprintf("Rotate: %s", err.Error()))
	}
	return nil
}

func (w *fileLogWriter) deleteOldLog() {
	dir := filepath.Dir(w.Filename)
	absolutePath, err := filepath.EvalSymlinks(w.Filename)
	if err == nil {
		dir = filepath.Dir(absolutePath)
	}
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) (returnErr error) {
		defer func() {
			if r := recover(); r != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Unable to delete old log '%s', error: %v\n", path, r)
			}
		}()

		if info == nil {
			return
		}
		if w.Hourly {
			if !info.IsDir() && info.ModTime().Add(1*time.Hour*time.Duration(w.MaxHours)).Before(time.Now()) {
				if strings.HasPrefix(filepath.Base(path), filepath.Base(w.fileNameOnly)) &&
					strings.HasSuffix(filepath.Base(path), w.suffix) {
					_ = os.Remove(path)
				}
			}
		} else if w.Daily {
			if !info.IsDir() && info.ModTime().Add(24*time.Hour*time.Duration(w.MaxDays)).Before(time.Now()) {
				if strings.HasPrefix(filepath.Base(path), filepath.Base(w.fileNameOnly)) &&
					strings.HasSuffix(filepath.Base(path), w.suffix) {
					_ = os.Remove(path)
				}
			}
		}
		return
	})
}

func (w *fileLogWriter) Destroy() {
	_ = w.fileWriter.Close()
}

func (w *fileLogWriter) Flush() {
	_ = w.fileWriter.Sync()
}

func init() {
	Register(AdapterFile, newFileWriter)
}
