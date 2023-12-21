package main

import (
	"chat-concurrent-server/database"
	"fmt"
	"chat-concurrent-server.go/chat"
	"github.com/gin-gonic/gin"
)

func main() {
	//运行后，打开http://127.0.0.1:8002/start
	
	//初始化数据库连接
	_, err := database.InitDB()
	if err != nil {
		fmt.Println("初始化数据库连接错误:", err)
		return
	}
	defer database.CloseDB()

	//该协程用于转发消息，只要有消息来了，遍历map，给map每个成员都发送此消息
	go chat.Manager()

	conn := gin.Default()
	conn.LoadHTMLGlob("view/*")
	conn.GET("/start", func(c *gin.Context) {
		c.HTML(200, "start.HTML", nil)
	})
	conn.GET("/login", func(c *gin.Context) {
		c.HTML(200, "login.HTML", nil)
	})
	conn.GET("/register", func(c *gin.Context) {
		c.HTML(200, "register.HTML", nil)
	})
	conn.GET("/chat", func(c *gin.Context) {
		c.HTML(200, "chat.HTML", nil)
	})
	conn.GET("/ws", func(c *gin.Context) {
		go chat.HandleConn(c.Writer, c.Request)
	})
	conn.POST("/login", database.Login)
	conn.POST("/register", database.Register)
	fmt.Println("服务器已启动，正在监听 http://127.0.0.1:8002/start")
	// 运行 Gin 服务器,当它执行时，它会一直阻塞，监听来自客户端的请求
	conn.Run(":8002")

}
