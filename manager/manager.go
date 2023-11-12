package manager

import (
	"fmt"
	"sync"
	"time"
)

var (
	message = make(chan string) // 消息通道
	wg      sync.WaitGroup      // 在全局定义一个 WaitGroup
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

