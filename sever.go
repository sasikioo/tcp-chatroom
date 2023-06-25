package TCP

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"net/smtp"
	"strconv"
	"time"
)

func main() {
	listener, err := net.Listen("tcp", ":2020")
	if err != nil {
		panic(err)
	}

	go broadcaster()

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println(err)
			continue
		}

		go handleConn(conn)
	}
}

// broadcaster 用于记录聊天室用户，并进行消息广播：
// 1. 新用户进来；2. 用户普通消息；3. 用户离开
func broadcaster() {
}

func handleConn(conn net.Conn) {
	defer conn.Close()

	// 1. 新用户进来，构建该用户的实例
	user := &User{
		ID:             GenUserID(),
		Addr:           conn.RemoteAddr().String(),
		EnterAt:        time.Now(),
		MessageChannel: make(chan string, 8),
	}

	// 2. 当前在一个新的 goroutine 中，用来进行读操作，因此需要开一个 goroutine 用于写操作
	// 读写 goroutine 之间可以通过 channel 进行通信
	go sendMessage(conn, user.Meassagechannel)

	// 3. 给当前用户发送欢迎信息；给所有用户告知新用户到来
	user.MessageChannel <- "Welcome," + user.String()
	messageChannel <- "user:'" + strconv.Itoa(user.ID) + "'has enter"

	// 4. 将该记录到全局的用户列表中，避免用锁
	enteringChannel <- user

	var userActive = make(chan struct{})
	go func() {
		d := 5 * time.Minute
		timer := time.NewTimer(d)
		for {
			select {
			case <-timer.C:
				conn.Close()
			case <-userActive:
				timer.Reset(d)
			}
		}
	}()

	// 5. 循环读取用户的输入
	input := bufio.NewScanner(conn)
	for input.Scan() {
		messageChannel <- strconv.Itoa(user.ID) + ":" + input.Text()
	}

	if err := input.Err(); err != nil {
		log.Println("读取错误", err)
	}
	// 6. 用户离开
	leavingChannel <- user
	messageChannel <- "user:'" + strconv.Itoa(user.ID) + "'has left"

	type User struct {
		ID             int
		Addr           string
		EnterAt        time.Time
		MessageChannel chan string
	}
	func sendMessage(conn net.Conn, ch <-chan string) {
		for msg := range ch {
			fmt.Fprintln(conn, msg)
		}
	}
	// broadcaster 用于记录聊天室用户，并进行消息广播：
	// 1. 新用户进来；2. 用户普通消息；3. 用户离开
	func broadcaster() {
		users := make(map[*User]struct{})

		for {
			select {
			case user := <-enteringChannel:
				// 新用户进入
				users[user] = struct{}{}
			case user := <-leavingChannel:
				// 用户离开
				delete(users, user)
				// 避免 goroutine 泄露
				close(user.MessageChannel)
			case msg := <-messageChannel:
				// 给所有在线用户发送消息
				for user := range users {
					user.MessageChannel <- msg
				}
			}
		}
	}
	var (
		// 新用户到来，通过该 channel 进行登记
		enteringChannel = make(chan *User)
		// 用户离开，通过该 channel 进行登记
		leavingChannel = make(chan *User)
		// 广播专用的用户普通消息 channel，缓冲是尽可能避免出现异常情况堵塞，这里简单给了 8，具体值根据情况调整
		messageChannel = make(chan string, 8)
	)
}
