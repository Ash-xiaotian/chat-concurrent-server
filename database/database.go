package database

import (
	"chat-concurrent-server/onlineusers"
	"fmt"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"

	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"
)

const (
	dbMaxOpenConns = 10 // 数据库最大连接数
	dbMaxIdleConns = 5  // 数据库最大空闲连接数
	bcryptCost     = 10
)

// 客户端结构体
type Client struct {
	C        chan string
	Userid   string
	Username string
}

var (
	db  *sqlx.DB
	cli *Client
)

// 账号存入数据库
func InsertSQL(userid, username, password string) {
	// 使用 bcrypt 对密码进行哈希
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		fmt.Println("GenerateFromPassword error:", err)
		return
	}

	sql := "insert into user(userid,username,password)values (?,?,?)"
	_, err = db.Exec(sql, userid, username, hashedPassword)
	if err != nil {
		fmt.Println("exec error:", err)
		return
	}
}

// 初始化数据库连接
func InitDB() (*sqlx.DB, error) {
	var err error
	db, err = sqlx.Open("mysql", "root:123456@tcp(127.0.0.1:3306)/sys")
	if err != nil {
		fmt.Println("sqlx.Open error:", err)
		return db, err
	}

	// 设置数据库最大连接数和最大空闲连接数
	db.SetMaxOpenConns(dbMaxOpenConns)
	db.SetMaxIdleConns(dbMaxIdleConns)

	// 检查数据库连接
	if err = db.Ping(); err != nil {
		db.Close() // 关闭数据库连接
		fmt.Println("db.Ping error:", err)
		return db, err
	}

	return db, err
}

// 关闭数据库连接
func CloseDB() {
	if db != nil {
		db.Close()
	}
}

// 注册
func Register(conn *gin.Context) {

	type User struct {
		Userid   string `db:"userid"`
		Password string `db:"password"`
		Username string `db:"username"`
	}
	var u User
	userid := conn.PostForm("userid")
	username := conn.PostForm("username")
	password := conn.PostForm("password")
	cfpassword := conn.PostForm("cfpassword")
	if password != cfpassword {
		conn.String(200, "notmatch")
		return
	}
	sql := "SELECT * FROM user WHERE userid=?"
	err := db.Get(&u, sql, userid)
	if err == nil {
		conn.String(200, "duplicate")
		return
	}
	InsertSQL(userid, username, password)
	conn.String(200, "success")
}

// 登入
func Login(conn *gin.Context) {
	type User struct {
		Userid   string `db:"userid"`
		Password string `db:"password"`
		Username string `db:"username"`
	}
	var u User
	// 读取消息
	userid := conn.PostForm("userid")
	password := conn.PostForm("password")
	sql := "SELECT * FROM user WHERE userid=?"
	err := db.Get(&u, sql, userid)
	if err != nil {
		fmt.Println("db.Get err:", err)
		// 发送消息
		conn.String(200, "failure")
		return
	}

	// 使用 bcrypt.CompareHashAndPassword 比较密码
	if err = bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
		fmt.Println("CompareHashAndPassword error:", err)
		// 发送消息
		conn.String(200, "failure")
		return
	}

	// 检查用户是否已经在在线列表中，如果是则拒绝登录
	if found := onlineusers.CheckUserOnline(userid); found {
		conn.String(200, "duplicate")
		return
	}
	conn.String(200, "success")
	// 创建一个结构体
	cli = &Client{make(chan string), u.Userid, u.Username}
	onlineusers.AddUser(u.Userid) // 从在线列表中添加用户
}
func GetUser() *Client {
	return cli
}

// 改名
func Changename(newName, Account string) {
	sql := "UPDATE user SET username =? WHERE user_id=?"
	_, err := db.Exec(sql, newName, Account)
	if err != nil {
		fmt.Println("UPDATE error :", err)
	}
}

