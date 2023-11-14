package database

import (
	"chat-concurrent-server/util"
	"fmt"
	"net"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"
)

const (
	dbMaxOpenConns = 10 // 数据库最大连接数
	dbMaxIdleConns = 5  // 数据库最大空闲连接数
	bcryptCost     = 10
)

// 用户结构体
type User struct {
	User_id  string
	Username string
	Password string
}

var db *sqlx.DB

// 账号存入数据库
func InsertSQL(user_id, username, password string, conn net.Conn) {
	// 使用 bcrypt 对密码进行哈希
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		fmt.Println("GenerateFromPassword error:", err)
		conn.Write([]byte("\t密码处理失败\n"))
		return
	}

	sql := "insert into user(user_id,username,password)values (?,?,?)"
	_, err = db.Exec(sql, user_id, username, hashedPassword)
	if err != nil {
		fmt.Println("exec error:", err)
		conn.Write([]byte("\t账号导入失败\n"))
		return
	}
	fmt.Println(user_id + "账号导入成功")
}

// 初始化数据库连接
func InitDB() (*sqlx.DB, error) {
	var err error
	db, err = sqlx.Open("mysql", "root:rommel@tcp(127.0.0.1:3306)/test")
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

// 输入账号密码
func Input(conn net.Conn) (string, string) {
	conn.Write([]byte("请输入账号：\n"))
	user_id := util.ReadInput(conn)

	conn.Write([]byte("请输入密码：\n"))
	password := util.ReadInput(conn)

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
			fmt.Println("CompareHashAndPassword error:", err)
			conn.Write([]byte("\t账号或密码有误，请重新输入!\n"))
			continue
		}
		break
	}
	conn.Write([]byte("登入成功！\n\n"))
	return u.User_id, u.Username
}

// 改名
func Changename(newName, Account string) {
	sql := "UPDATE user SET username =? WHERE user_id=?"
	_, err := db.Exec(sql, newName, Account)
	if err != nil {
		fmt.Println("UPDATE error :", err)
	}
}
