package main

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"
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
	db        *sqlx.DB
	wg        sync.WaitGroup // 在全局定义一个 WaitGroup
)

// 账号存入数据库
func InsertSQL(user_id, username, password string, conn net.Conn) {
	// 使用 bcrypt 对密码进行哈希
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		fmt.Println("Failed to hash password:", err)
		conn.Write([]byte("\t密码处理失败\n"))
		return
	}

	sql := "insert into user(user_id,username,password)values (?,?,?)"
	_, err = db.Exec(sql, user_id, username, hashedPassword)
	if err != nil {
		fmt.Println("exec failed,", err)
		conn.Write([]byte("\t账号导入失败\n"))
		return
	}
	fmt.Println(user_id + "账号导入成功")
}

// 输入账号密码
func Input(conn net.Conn) (string, string) {
	conn.Write([]byte("请输入账号：\n"))
	user_id := ReadInput(conn)

	conn.Write([]byte("请输入密码：\n"))
	password := ReadInput(conn)

	return user_id, password
}

// 注册
func Register(conn net.Conn) (string, string) {
	var u User
	for {
		user_id, password := Input(conn)
		sql := "SELECT * FROM user WHERE user_id= ?"
		err := db.Get(&u, sql, user_id)
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
			username := string(buf[:n-1])
			InsertSQL(user_id, username, password, conn)
			conn.Write([]byte("注册成功！\n\n"))
			return user_id, username
		}
	}
}

// 登入
func Login(conn net.Conn) (string, string) {
	var u User
	for {
		user_id, password := Input(conn)
		sql := "SELECT * FROM user WHERE user_id=?"
		err := db.Get(&u, sql, user_id)
		if err != nil {
			fmt.Println("db.Get err:", err)
			conn.Write([]byte("\t账号或密码有误，请重新输入!\n"))
			continue
		}

		// 使用 bcrypt.CompareHashAndPassword 比较密码
		if err = bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
			fmt.Println("Password compare failed:", err)
			conn.Write([]byte("\t账号或密码有误，请重新输入!\n"))
			continue
		}
		break
	}
	conn.Write([]byte("登入成功！\n\n"))
	return u.User_id, u.Username
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
			cliAccount, cliName = Register(conn)
			break
		} else if x == "2" {
			cliAccount, cliName = Login(conn)
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
	onlineMap.Store(cliAccount, cli)

	// 提示已进入聊天室，这个只能自己收到
	conn.Write([]byte("----------您可以跟所有人聊天了!----------\n"))
	conn.Write([]byte("          who：查看在线用户\n"))
	conn.Write([]byte("          rename：修改用户名\n"))
	conn.Write([]byte("          offline：下线\n\n"))

	// 新开一个协程，专门给客户端发送信息
	wg.Add(1)
	go WriteMsgToClient(cli, conn)

	// 广播某个人在线，所有客户端都能收到消息
	message <- ("[" + string(cliName) + "] 来到聊天室\n\n")

	// 对方是否主动退出
	isQuit := make(chan bool)

	// 对方是否有数据发送
	hasData := make(chan bool)

	// 接收用户发送过来的数据
	wg.Add(1)
	go func() {
		defer wg.Done()
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
				message <- SendMsg(cli, msg)
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
			onlineMap.Delete(cliAccount)    // 将当前用户从map中移除
			message <- (cli.Name + "已退出\n") // 广播下线
			return

		case <-hasData: // 有消息
			outTimer.Reset(time.Second * time.Duration(120)) // 重置超时计时器
			rdTicker.Reset(time.Second * time.Duration(110)) // 重置提醒计数器

		case <-rdTicker.C: // 定时提醒
			conn.Write([]byte("提醒：您的连接将在10秒后超时。\n"))

		case <-outTimer.C: // 超时
			onlineMap.Delete(cliAccount)      // 将当前用户从map中移除
			message <- (cli.Name + "已超时下线\n") // 广播下线
			return
		}
	}
}

// 显示在线用户列表
func ShowOnlineUsers(conn net.Conn) {
	conn.Write([]byte("User List:{\n"))

	// 遍历map，给当前用户发送所有成员
	onlineMap.Range(func(_, value interface{}) bool {
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
	newName := ReadInput(conn)
	oldName := cli.Name
	sql := "UPDATE user SET username =? WHERE user_id=?"
	_, err := db.Exec(sql, newName, cli.Account)
	if err != nil {
		fmt.Println("UPDATE error =", err)
	}
	cli.Name = newName
	message <- (oldName + " has changed the name to " + newName + "\n")
}

// 读取和修整来自连接的输入
func ReadInput(conn net.Conn) string {
	buf := make([]byte, 2048)
	n, readErr := conn.Read(buf)
	if readErr != nil {
		fmt.Println("conn.Read error =", readErr)
	}
	return strings.TrimSpace(string(buf[:n-1]))
}

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
				cli := value.(*Client)
				cli.C <- msg
				return true
			})

		case <-ticker.C:
			// 没有消息时的处理，可以选择休眠一段时间
			time.Sleep(time.Millisecond * 100)
		}
	}
}

// 初始化数据库连接
func InitDB() (*sqlx.DB, error) {
	var err error
	db, err = sqlx.Open("mysql", "root:rommel@tcp(127.0.0.1:3306)/test")
	if err != nil {
		return nil, fmt.Errorf("Failed to open SQL: %v", err)
	}

	// 设置数据库最大连接数和最大空闲连接数
	db.SetMaxOpenConns(dbMaxOpenConns)
	db.SetMaxIdleConns(dbMaxIdleConns)

	// 检查数据库连接
	if err := db.Ping(); err != nil {
		db.Close() // 关闭数据库连接
		return nil, fmt.Errorf("Failed to ping database: %v", err)
	}

	return db, nil
}

// 关闭数据库连接
func CloseDB() {
	if db != nil {
		db.Close()
	}
}

func main() {
	// 初始化数据库连接
	_, err := InitDB()
	if err != nil {
		fmt.Println("Failed to initialize DB:", err)
		return
	}
	defer CloseDB()

	// 该协程用于转发消息，只要有消息来了，遍历map，给map每个成员都发送此消息
	go Manager()

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
		go HandleConn(conn)
	}
}