package main

import (
	"chat-concurrent-server/chat"
	"chat-concurrent-server/database"
	"fmt"
	"net"
)

const listenAddress = ":8002"

func main() {
	// 初始化数据库连接
	_, err := database.InitDB()
	if err != nil {
		fmt.Println("database.InitDB error:", err)
		return
	}
	defer database.CloseDB()

	// 该协程用于转发消息，只要有消息来了，遍历map，给map每个成员都发送此消息
	go chat.Manager()

	// 开始监听导入连接
	listener, err := net.Listen("tcp", listenAddress)
	if err != nil {
		fmt.Println("net.Listen error:", err)
		return
	}
	defer listener.Close()

	// 主协程，循环阻塞等待用户连接
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("listener error", err)
			continue
		}

		// 处理用户连接
		go chat.HandleConn(conn)
	}
}
