package main

import (
    "net"
    "sync"
	"fmt"
    "chat-concurrent-server/chat"
    "chat-concurrent-server/database"
)

const (
	listenAddress  = ":8002"
	dbMaxOpenConns = 10 // 数据库最大连接数
	dbMaxIdleConns = 5  // 数据库最大空闲连接数
	bcryptCost     = 10
)

type Client struct {
	C       chan string // 用于发送数据的管道
	Account string      // 用户名
	Name    string      // 网络地址
}
type User struct {
	User_id  string // 账号
	Username string // 名字
	Password string // 密码
}

var (
	onlineMap sync.Map            // 保存在线用户
	message   = make(chan string) // 消息通道
	wg        sync.WaitGroup // 在全局定义一个 WaitGroup
)

func main() {
	// 初始化数据库连接
	_, err := database.InitDB()
	if err != nil {
		fmt.Println("Failed to initialize DB:", err)
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
			fmt.Println("listener.Accept =", err)
			continue
		}
		
		// 处理用户连接
		go chat.HandleConn(conn)
	}
}