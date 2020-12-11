package loguru

type brush func(string) string

func newBrush(color string) brush {
	pre := "\033["
	reset := "\033[0m"
	return func(text string) string {
		return pre + color + "m" + text + reset
	}
}

var colors = []brush{
	newBrush("1;41"), // Emergency          white
	newBrush("1;36"), // Alert              cyan
	newBrush("1;35"), // Critical           magenta
	newBrush("1;31"), // Error              red
	newBrush("1;33"), // Warning            yellow
	newBrush("1;32"), // Notice             green
	newBrush("1;38"), // Informational      blue
	newBrush("1;34"), // Debug              Background blue
}

func White(text string) string {
	return colors[0](text)
}

func Cyan(text string) string {
	return colors[1](text)
}

func Magenta(text string) string {
	return colors[2](text)
}

func Red(text string) string {
	return colors[3](text)
}

func Yellow(text string) string {
	return colors[4](text)
}

func Green(text string) string {
	return colors[5](text)
}

func Blue(text string) string {
	return colors[6](text)
}

func BackBlue(text string) string {
	return colors[7](text)
}
