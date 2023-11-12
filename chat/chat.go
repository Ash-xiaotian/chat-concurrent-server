package chat

import (
    "net"
    "strings"
	"fmt"
	"time"
    "chat-concurrent-server/database"
    "chat-concurrent-server/util"
	"chat-concurrent-server/manager"
)

// 客户端结构体
type Client struct {
    C       chan string
    Account string
    Name    string
}

// 向客户端发送消息
func WriteMsgToClient(cli *Client, conn net.Conn) {
	for msg := range cli.C { // 给当前客户端发送信息
		conn.Write([]byte(msg))
	}
}

// 用户发送消息
func SendMsg(cli *Client, text string) (buf string) {
	return "[" + cli.Account + "-" + cli.Name + "]" + "对大家说:" + text + "\n"
}

// 处理用户连接（用户上线了的处理）
func HandleConn(conn net.Conn) {
	defer conn.Close()

	conn.Write([]byte("----------欢迎来到聊天室----------\n"))
	conn.Write([]byte("          请选择:\n"))
	conn.Write([]byte("          1、注册\n"))
	conn.Write([]byte("          2、登入\n"))

	var cliAccount, cliName string

	for {
		buf := make([]byte, 2048)
		n, readErr := conn.Read(buf)
		if readErr != nil {
			fmt.Println("conn.Read Account error =", readErr)
		}

		x := strings.TrimSpace(string(buf[:n-1]))

		if x == "1" {
			cliAccount, cliName = database.Register(conn)
			break
		} else if x == "2" {
			cliAccount, cliName = database.Login(conn)
			break
		} else {
			conn.Write([]byte("无效的选择，请重新输入:\n"))
			conn.Write([]byte("          1、注册\n"))
			conn.Write([]byte("          2、登入\n"))
		}
	}

	conn.Write([]byte("账号： " + cliAccount + " , 姓名： " + cliName + "\n"))

	// 创建一个结构体
	cli := &Client{make(chan string), cliAccount, cliName}

	// 把结构体添加到map
	database.onlineMap.Store(cliAccount, cli)

	// 提示已进入聊天室，这个只能自己收到
	conn.Write([]byte("----------您可以跟所有人聊天了!----------\n"))
	conn.Write([]byte("          who：查看在线用户\n"))
	conn.Write([]byte("          rename：修改用户名\n"))
	conn.Write([]byte("          offline：下线\n\n"))

	// 新开一个协程，专门给客户端发送信息
	manager.wg.Add(1)
	go WriteMsgToClient(cli, conn)

	// 广播某个人在线，所有客户端都能收到消息
	manager.message <- ("[" + string(cliName) + "] 来到聊天室\n\n")

	// 对方是否主动退出
	isQuit := make(chan bool)

	// 对方是否有数据发送
	hasData := make(chan bool)

	// 接收用户发送过来的数据
	manager.wg.Add(1)
	go func() {
		defer manager.wg.Done()
		buf := make([]byte, 2048)
		for {
			n, readErr := conn.Read(buf)
			if readErr != nil {
				fmt.Println("conn.Read error =", readErr)
			}

			msg := strings.TrimSpace(string(buf[:n-1]))

			switch msg {
			case "offline": // 下线
				isQuit <- true
			case "who": // 发送用户列表
				ShowOnlineUsers(conn)
			case "rename": // 改名
				ChangeUsername(cli, conn)
			default:
				conn.Write([]byte("---Message sent successfully!---\n\n"))
				manager.message <- SendMsg(cli, msg)
			}
			hasData <- true // 代表有数据
		}
	}()
	// 超时计时器和提醒计数器
	outTimer := time.NewTimer(time.Second * time.Duration(120))  // 超时计时器
	rdTicker := time.NewTicker(time.Second * time.Duration(110)) // 提醒计数器
	// 做个死循环，不要让方法结束
	for {
		// 通过select来检测channel的流动
		select {

		case <-isQuit: // 下线
			database.onlineMap.Delete(cliAccount)    // 将当前用户从map中移除
			manager.message <- (cli.Name + "已退出\n") // 广播下线
			return

		case <-hasData: // 有消息
			outTimer.Reset(time.Second * time.Duration(120)) // 重置超时计时器
			rdTicker.Reset(time.Second * time.Duration(110)) // 重置提醒计数器

		case <-rdTicker.C: // 定时提醒
			conn.Write([]byte("提醒：您的连接将在10秒后超时。\n"))

		case <-outTimer.C: // 超时
			database.onlineMap.Delete(cliAccount)      // 将当前用户从map中移除
			manager.message <- (cli.Name + "已超时下线\n") // 广播下线
			return
		}
	}
}

// 显示在线用户列表
func ShowOnlineUsers(conn net.Conn) {
	conn.Write([]byte("User List:{\n"))

	// 遍历map，给当前用户发送所有成员
	database.onlineMap.Range(func(_, value interface{}) bool {
		tmp := value.(*Client)
		msg := tmp.Account + "-" + tmp.Name + "\n"
		conn.Write([]byte(msg))
		return true
	})
}

// 更新数据库中的用户名并广播更改
func ChangeUsername(cli *Client, conn net.Conn) {
	// 更改用户名
	conn.Write([]byte("请输入您的新用户名："))
	newName := util.ReadInput(conn)
	oldName := cli.Name
	sql := "UPDATE user SET username =? WHERE user_id=?"
	_, err := database.db.Exec(sql, newName, cli.Account)
	if err != nil {
		fmt.Println("UPDATE error =", err)
	}
	cli.Name = newName
	manager.message <- (oldName + " has changed the name to " + newName + "\n")
}