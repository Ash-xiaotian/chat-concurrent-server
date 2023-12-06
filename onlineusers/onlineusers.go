package onlineusers

// 使用 map 来存储在线用户的 ID
var OnlineUsers = make(map[string]bool)

// 添加用户到在线列表
func AddUser(userID string) {
	OnlineUsers[userID] = true
}

// 从在线列表中删除用户
func RemoveUser(userID string) {
	delete(OnlineUsers, userID)
}
// 检查用户是否在线
func CheckUserOnline(userID string) bool {
	_, exists := OnlineUsers[userID]
	return exists
}
