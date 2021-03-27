package loguru

import (
	"log"
	"testing"
)

func TestLog(t *testing.T) {
	// 终端输出
	//ResetTimeColor(FUCHSIA)
	//ResetDebugColor(GREEN)
	//Debug("111")

	// 写入文件
	//logger := NewLogger(FileLog)
	//logger.Debug("1111")
	//logger.Info("2222")
	//logger.Warning("3333")
	//logger.Success("4444")
	//logger.Emergency("5555")

	// 在线输出
	//logger := NewLogger(3)
	//logger.Debug("嘿嘿嘿")
	x := Input("请输入: ")
	log.Println(x)
}
