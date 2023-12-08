package chat

import (
	"chat-concurrent-server/database"
	"chat-concurrent-server/onlineusers"
	"context"
	"fmt"
	"github.com/gorilla/websocket"
	"net/http"
	"sync"
	"time"
)

var (
	message   = make(chan string, 100) // 消息通道
	onlineMap sync.Map                 // 保存在线用户
	upgrader  = websocket.Upgrader{
		HandshakeTimeout: 2 * time.Second, //握手超时时间
		ReadBufferSize:   1024,            //读缓冲大小
		WriteBufferSize:  1024,            //写缓冲大小
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
)

// 只要有消息来了，遍历map，给map每个成员都发送此消息
func Manager() {
	// 设置一个定时器，每隔一段时间检查一次message
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {

		case msg := <-message:
			// 遍历 map，给 map 每个成员都发送此消息
			onlineMap.Range(func(_, value interface{}) bool {
				cli := value.(*database.Client)
				cli.C <- msg
				return true
			})

		case <-ticker.C:
			// 没有消息时的处理，可以选择休眠一段时间
			time.Sleep(time.Millisecond * 100)
		}
	}
}

// 向客户端发送消息

func WriteMsgToClient(cli *database.Client, conn *websocket.Conn, ctx context.Context) {
	for {
		select {
		case <-ctx.Done(): // 结束 WriteMsgToClient 协程
			return
		case msg := <-cli.C: // 给当前客户端发送信息
			err := conn.WriteMessage(websocket.TextMessage, []byte(msg))
			if err != nil {
				fmt.Println("Error sending message:", err)
				return
			}
		}
	}
}

// 显示在线用户列表
func ShowOnlineUsers(conn *websocket.Conn) {
	conn.WriteMessage(websocket.TextMessage, []byte("------"+"在线用户:"))

	// 遍历map，给当前用户发送所有成员
	onlineMap.Range(func(_, value interface{}) bool {
		tmp := value.(*database.Client)
		msg := "------账号:" + tmp.Userid + " 姓名:" + tmp.Username + "------"
		conn.WriteMessage(websocket.TextMessage, []byte(msg))
		return true
	})

}

// 更新数据库中的用户名并广播更改
func ChangeUsername(cli *database.Client, conn *websocket.Conn) {
	// 更改用户名
	conn.WriteMessage(websocket.TextMessage, []byte("------请输入您的新用户名："))
	_, newName, err := conn.ReadMessage()
	if err != nil {
		fmt.Println("rename err:", err)
	}
	oldName := cli.Username
	database.Changename(string(newName), cli.Userid)
	cli.Username = string(newName)
	message <- (oldName + " 已经改名为： " + string(newName) + "\n")
}

// 处理用户连接（用户上线了的处理）
func HandleConn(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("升级失败:", err)
		return
	}
	defer conn.Close()
	ctx, cancel := context.WithCancel(context.Background()) // 获取一个终止函数
	defer cancel()
	//获取用户信息
	cli := database.GetUser()
	onlineMap.Store(cli.Userid, cli) // 将当前用户添加到map中
	// 广播某个人在线，所有客户端都能收到消息
	message <- ("[ " + string(cli.Username) + " ] 来到聊天室")
	conn.WriteMessage(websocket.TextMessage, []byte("------who显示用户列表------offline下线------rename改名------"))
	// 对方是否主动退出
	isQuit := make(chan bool)
	// 对方是否有数据发送
	hasData := make(chan bool)
	go WriteMsgToClient(cli, conn, ctx)
	// 接收用户发送过来的数据
	go func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done(): // 结束 Goroutine
				return
			default:
				_, msg, err := conn.ReadMessage()
				if err != nil {
					// 远程主机强制关闭连接，结束 goroutine
					fmt.Println(cli.Username + "远程主机强制关闭连接")
					message <- (cli.Username + "已退出\n") // 广播下线
					onlineusers.RemoveUser(cli.Userid)  // 从在线列表中移除用户
					onlineMap.Delete(cli.Userid)        // 将当前用户从map中移除
					cancel()                            // 发送关闭信号
					return
				}
				m := string(msg)
				switch m {
				case "offline": // 下线
					isQuit <- true
				case "who": // 发送用户列表
					ShowOnlineUsers(conn)
				case "rename": // 改名
					ChangeUsername(cli, conn)
				default:
					m = "[ " + cli.Username + " ]" + " : " + m
					message <- m // 将消息发送到消息通道
				}
				hasData <- true // 代表有数据
			}
		}
	}(ctx)
	//超时计时器和提醒计数器
	outTimer := time.NewTimer(time.Second * time.Duration(120))  // 超时计时器
	rdTicker := time.NewTicker(time.Second * time.Duration(110)) // 提醒计数器
	// 做个死循环，不要让方法结束
	for {
		// 通过select来检测channel的流动
		select {
		case <-isQuit: // 下线
			message <- (cli.Username + "已退出\n") // 广播下线
			onlineusers.RemoveUser(cli.Userid)  // 从在线列表中移除用户
			onlineMap.Delete(cli.Userid)        // 将当前用户从map中移除
			return

		case <-hasData: // 有消息
			outTimer.Reset(time.Second * time.Duration(120)) // 重置超时计时器
			rdTicker.Reset(time.Second * time.Duration(110)) // 重置提醒计数器

		case <-rdTicker.C: // 定时提醒
			conn.WriteMessage(websocket.TextMessage, []byte("提醒：您的连接将在10秒后超时!"))

		case <-outTimer.C: // 超时
			message <- (cli.Username + "已超时下线\n") // 广播下线
			onlineusers.RemoveUser(cli.Userid)    // 从在线列表中移除用户
			onlineMap.Delete(cli.Userid)          // 将当前用户从map中移除
			return
		}
	}
}


