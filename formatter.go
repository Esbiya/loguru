package loguru

import (
	"path"
	"strconv"
)

var formatterMap = make(map[string]LogFormatter, 4)

type LogFormatter interface {
	Format(lm *LogMsg) string
}

type PatternLogFormatter struct {
	Pattern    string
	WhenFormat string
}

func (p *PatternLogFormatter) getWhenFormatter() string {
	s := p.WhenFormat
	if s == "" {
		s = "2006/01/02 15:04:05.123" // default style
	}
	return s
}

func (p *PatternLogFormatter) Format(lm *LogMsg) string {
	return p.ToString(lm)
}

func RegisterFormatter(name string, fmtr LogFormatter) {
	formatterMap[name] = fmtr
}

func GetFormatter(name string) (LogFormatter, bool) {
	res, ok := formatterMap[name]
	return res, ok
}

func (p *PatternLogFormatter) ToString(lm *LogMsg) string {
	s := []rune(p.Pattern)
	m := map[rune]string{
		'w': lm.When.Format(p.getWhenFormatter()),
		'm': lm.Msg,
		'n': strconv.Itoa(lm.LineNumber),
		'l': strconv.Itoa(lm.Level),
		't': levelPrefix[lm.Level-1],
		'T': levelNames[lm.Level-1],
		'F': lm.FilePath,
	}
	_, m['f'] = path.Split(lm.FilePath)
	res := ""
	for i := 0; i < len(s)-1; i++ {
		if s[i] == '%' {
			if k, ok := m[s[i+1]]; ok {
				res += k
				i++
				continue
			}
		}
		res += string(s[i])
	}
	return res
}
