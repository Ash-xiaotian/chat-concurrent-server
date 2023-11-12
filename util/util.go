package util

import (
    "net"
	"fmt"
	"strings"
)
// 读取和修整来自连接的输入
func ReadInput(conn net.Conn) string {
	buf := make([]byte, 2048)
	n, readErr := conn.Read(buf)
	if readErr != nil {
		fmt.Println("conn.Read error =", readErr)
	}
	return strings.TrimSpace(string(buf[:n-1]))
}