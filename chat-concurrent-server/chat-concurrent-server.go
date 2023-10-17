package main

import (
	"fmt"
	"net"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
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

// 保存在线用户
var onlineMap map[string]*Client

// 消息通道
var message = make(chan string)

var db *sqlx.DB

// 初始化数据库连接，init()方法系统会在动在main方法之前执行。
func init() {
	path, err := sqlx.Open("mysql", "root:rommel@tcp(127.0.0.1:3306)/test")
	if err != nil {
		fmt.Println("open SQL failed", err)
	}
	db = path
}

// 账号存入数据库
func InsertSQL(user_id, username, password string, conn net.Conn) {
	sql := "insert into user(user_id,username,password)values (?,?,?)"
	_, err := db.Exec(sql, user_id, username, password)
	if err != nil {
		fmt.Println("exec failed,", err)
	}
}

// 输入账号密码
func Input(conn net.Conn) (string, string) {
	conn.Write([]byte("请输入账号：\n"))
	buf := make([]byte, 2048)
	n, readErr := conn.Read(buf)
	if readErr != nil {
		fmt.Println("conn.Read id error =", readErr)
	}
	user_id := string(buf[:n-1])

	conn.Write([]byte("请输入密码：\n"))
	buf = make([]byte, 2048)
	n, readErr = conn.Read(buf)
	if readErr != nil {
		fmt.Println("conn.Read password error =", readErr)
	}
	password := string(buf[:n-1])
	return user_id, password
}

// 注册
func Register(conn net.Conn) (string, string) {
	var u User
	for {
		Ud, Pd := Input(conn)
		sql := "SELECT * FROM user WHERE user_id= ?"
		err := db.Get(&u, sql, Ud)
		if err == nil {
			conn.Write([]byte("\t该账号已被注册,请重新输入!\n"))
			continue
		} else {
			fmt.Println(err)
			conn.Write([]byte("请输入您的名称：\n"))
			buf := make([]byte, 2048)
			n, readErr := conn.Read(buf)
			if readErr != nil {
				fmt.Println("conn.Read Name error =", readErr)
			}
			Un := string(buf[:n-1])
			InsertSQL(Ud, Un, Pd, conn)
			conn.Write([]byte("注册成功！\n\n"))
			return Ud, Un
		}
	}
}

// 登入
func Login(conn net.Conn) (string, string) {
	var u User
	for {
		Ud, Pd := Input(conn)
		sql := "SELECT * FROM user WHERE user_id=?"
		err := db.Get(&u, sql, Ud)
		if err != nil {
			fmt.Println(err)
			conn.Write([]byte("\t账号或密码有误,请重新输入!\n"))
			continue
		}
		if u.Password != Pd {
			conn.Write([]byte("\t账号或密码有误,请重新输入!\n"))
		} else {
			break
		}
	}
	conn.Write([]byte("登入成功！\n\n"))
	return u.User_id, u.Username
}

// 只要有消息来了，遍历map，给map每个成员都发送此消息
func Manager() {
	// 给map分配空间
	onlineMap = make(map[string]*Client)
	for {
		msg := <-message // 没有消息时，这里会阻塞

		// 遍历map，给map每个成员都发送此消息
		for _, cli := range onlineMap {
			cli.C <- msg
		}
	}
}

// 向客户端发送消息
func WriteMsgToClient(cli *Client, conn net.Conn) {
	for msg := range cli.C { // 给当前客户端发送信息
		_, _ = conn.Write([]byte(msg))
	}
}

// 用户发送消息
func SendMsg(cli *Client, text string) (buf string) {
	return "[" + cli.Account +"-"+  cli.Name +"]"+ "对大家说:" + text + "\n"
}

// 处理用户连接（用户上线了的处理）
func HandleConn(conn net.Conn) {
	// 获取客户端的网络地址
	conn.Write([]byte("----------欢迎来到聊天室----------\n"))
	conn.Write([]byte("          请选择:\n"))
	conn.Write([]byte("           1、注册\n"))
	conn.Write([]byte("           2、登入\n"))
	buf := make([]byte, 2048)
	n, readErr := conn.Read(buf)
	if readErr != nil {
		fmt.Println("conn.Read Account error =", readErr)
	}
	x := string(buf[:n-1])
	var cliAccount, cliName string
	if x == "1" {
		cliAccount, cliName = Register(conn)
	} else {
		cliAccount, cliName = Login(conn)
	}
	conn.Write([]byte("账号： "+cliAccount+" , 姓名： "+cliName))
	// 创建一个结构体
	cli := &Client{make(chan string), cliAccount, cliName}

	// 把结构体添加到map
	onlineMap[cliAccount] = cli

	// 提示已进入聊天室，这个只能自己收到
	conn.Write([]byte("\t您可以跟所有人聊天了!\n"))

	// 新开一个协程，专门给客户端发送信息
	go WriteMsgToClient(cli, conn)

	// 广播某个人在线，所有客户端都能收到消息
	message <- ("["+string(cliName) + "] 来到聊天室\n\n")

	// 对方是否主动退出
	isQuit := make(chan bool)

	// 对方是否有数据发送
	hasData := make(chan bool)

	// 接收用户发送过来的数据
	go func() {
		buf = make([]byte, 2048)
		for {
			n, readErr := conn.Read(buf)
			if readErr != nil {
				fmt.Println("conn.Read error =", readErr)
			}

			if n == 0 { // 对方断开or出问题
				isQuit <- true
				fmt.Println("conn.Read error =", readErr)
				return
			}

			msg := string(buf[:n-1]) // nc 多一个换行
			if msg == "who" { // 当收到“who”指令时，改为发送用户列表
				_, _ = conn.Write([]byte("User List:{\n"))
				// 遍历map，给当前用户发送所有成员
				for _, tmp := range onlineMap {
					msg = tmp.Account + ":" + tmp.Name + "\n"
					_, _ = conn.Write([]byte(msg))
				}
				_, _ = conn.Write([]byte("}\n"))

			} else if msg == "rename" {
				// 更改用户名
				conn.Write([]byte("请输入您的新用户名："))
				buf = make([]byte, 2048)
				n, readErr = conn.Read(buf)
				if readErr != nil {
					fmt.Println("conn.Read Name error =", readErr)
				}
				newName := buf[:n-1]
				oldName := cli.Name
				sql := "UPDATE user SET username =? WHERE user_id=?"
				_, err := db.Exec(sql, newName, cli.Account)
				if err != nil {
					fmt.Println("UPDATA error =", err)
				}
				cli.Name = string(newName)
				message <- (string(oldName) + "已改名成" + string(newName) + "\n")

			} else {
				// 转发此内容
				conn.Write([]byte("---Send message success!---\n\n"))
				message <- SendMsg(cli, msg)
			}
			hasData <- true // 代表有数据
		}
	}()

	// 做个死循环，不要让方法结束
	for {
		// 通过select来检测channel的流动
		select {

		case <-isQuit:
			delete(onlineMap, cliAccount) // 将当前用户从map中移除
			message <- (cli.Name + "已退出") // 广播下线
			return

		case <-hasData://有消息

		case <-time.After(time.Second * 120): // 120s后
			delete(onlineMap, cliAccount)   // 将当前用户从map中移除
			message <- (cli.Name + "已超时下线") // 广播下线
			return 
		}
	}
}

func main() {
	listener, listenErr := net.Listen("tcp", ":8002")
	if listenErr != nil {
		fmt.Println("net.Listen error =", listenErr)
		return
	}
	defer listener.Close()

	// 该协程用于转发消息，只要有消息来了，遍历map，给map每个成员都发送此消息
	go Manager()

	// 主协程，循环阻塞等待用户连接
	for {
		conn, connErr := listener.Accept()
		if connErr != nil {
			fmt.Println("listener.Accept =", connErr)
			continue
		}

		go HandleConn(conn) // 处理用户连接
	}
}
