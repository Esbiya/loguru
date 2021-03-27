/*
 * @Author: your name
 * @Date: 2021-03-27 09:56:20
 * @LastEditTime: 2021-03-27 09:57:05
 * @LastEditors: your name
 * @Description: In User Settings Edit
 * @FilePath: /loguru/utils.go
 */
package loguru

import "os"

func IsDir(path string) bool {
	s, err := os.Stat(path)
	if err != nil {
		return false
	}
	return s.IsDir()
}

func Exists(path string) bool {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsExist(err) {
			return true
		}
		return false
	}
	return true
}
