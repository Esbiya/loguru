package loguru

type brush func(string) string

const (
	BLACK   = "1;30"
	RED     = "1;31"
	GREEN   = "1;32"
	YELLOW  = "1;33"
	BLUE    = "1;34"
	FUCHSIA = "1;35"
	CYAN    = "1;36"
	WHITE   = "1;38"

	BackBLACK   = "1;40"
	BackRED     = "1;41"
	BackGREEN   = "1;42"
	BackYELLOW  = "1;43"
	BackBLUE    = "1;44"
	BackFUCHSIA = "1;45"
	BackCYAN    = "1;46"
	BackWHITE   = "1;47"
)

func newBrush(color string) brush {
	pre := "\033["
	reset := "\033[0m"
	return func(text string) string {
		return pre + color + "m" + text + reset
	}
}

var colorsMap = map[string]brush{
	"black":       newBrush(BLACK),
	"red":         newBrush(RED),
	"green":       newBrush(GREEN),
	"yellow":      newBrush(YELLOW),
	"fuchsia":     newBrush(FUCHSIA),
	"cyan":        newBrush(CYAN),
	"blue":        newBrush(BLUE),
	"white":       newBrush(WHITE),
	"backBlack":   newBrush(BackBLACK),
	"backRed":     newBrush(BackRED),
	"backGreen":   newBrush(BackGREEN),
	"backYellow":  newBrush(BackYELLOW),
	"backFuchsia": newBrush(BackFUCHSIA),
	"backCyan":    newBrush(BackCYAN),
	"backBlue":    newBrush(BackBLUE),
	"backWhite":   newBrush(BackWHITE),
}

var colors = []brush{
	colorsMap["backRed"],   // Emergency          backRed
	colorsMap["backCyan"],  // Alert              cyan
	colorsMap["backBlue"],  // Critical           backBlue
	colorsMap["red"],       // Error              red
	colorsMap["yellow"],    // Warning            yellow
	colorsMap["green"],     // Success            green
	colorsMap["backGreen"], // Notice             green
	colorsMap["blue"],      // Informational      blue
	colorsMap["fuchsia"],   // Debug              fuchsia
}

var (
	timeColor = colorsMap["white"]
	fileColor = colorsMap["white"]
)

func Black(text string) string {
	return colorsMap["black"](text)
}

func BackBlack(text string) string {
	return colorsMap["backBlack"](text)
}

func Red(text string) string {
	return colorsMap["red"](text)
}

func BackRed(text string) string {
	return colorsMap["backRed"](text)
}

func Green(text string) string {
	return colorsMap["green"](text)
}

func BackGreen(text string) string {
	return colorsMap["backGreen"](text)
}

func Yellow(text string) string {
	return colorsMap["yellow"](text)
}

func BackYellow(text string) string {
	return colorsMap["backYellow"](text)
}

func Fuchsia(text string) string {
	return colorsMap["fuchsia"](text)
}

func BackFuchsia(text string) string {
	return colorsMap["backFuchsia"](text)
}

func White(text string) string {
	return colorsMap["white"](text)
}

func Cyan(text string) string {
	return colorsMap["cyan"](text)
}

func Blue(text string) string {
	return colorsMap["blue"](text)
}

func BackBlue(text string) string {
	return colorsMap["backBlue"](text)
}

func BackCyan(text string) string {
	return colorsMap["backCyan"](text)
}

func BackWhite(text string) string {
	return colorsMap["backWhite"](text)
}
