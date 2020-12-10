package loguru

import "testing"

func TestLog(t *testing.T) {
	// 终端输出
	//Debug("111")

	// 写入文件
	//logger := NewLogger(2)
	//logger.Debug("1111")

	// 在线输出
	logger := NewLogger(3)
	logger.Debug("222")
}
