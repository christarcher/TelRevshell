package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"golang.org/x/term"
)

type ReverseShellServer struct {
	port       string
	listener   net.Listener
	conn       net.Conn
	oldState   *term.State
	mu         sync.Mutex
	active     bool
	escapeMode bool
}

func NewReverseShellServer(port string) *ReverseShellServer {
	return &ReverseShellServer{
		port: port,
	}
}

// 获取终端大小 (跨平台兼容)
func (s *ReverseShellServer) getTerminalSize() (int, int) {
	if term.IsTerminal(int(os.Stdout.Fd())) {
		width, height, err := term.GetSize(int(os.Stdout.Fd()))
		if err == nil {
			fmt.Printf("终端大小: %dx%d\n", width, height)
			return width, height
		}
	}
	// 默认大小
	fmt.Println("使用默认终端大小: 80x24")
	return 80, 24
}

// 设置终端为raw模式
func (s *ReverseShellServer) setRawMode() error {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		log.Println("警告: stdin不是终端，某些功能可能不可用")
		return nil
	}

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("设置raw模式失败: %v", err)
	}

	s.oldState = oldState

	// 获取终端大小
	s.getTerminalSize()

	return nil
}

// 恢复终端模式
func (s *ReverseShellServer) restoreTerminal() {
	if s.oldState != nil {
		term.Restore(int(os.Stdin.Fd()), s.oldState)
		s.oldState = nil
	}
}

// 启动服务器
func (s *ReverseShellServer) Start() error {
	listener, err := net.Listen("tcp", ":"+s.port)
	if err != nil {
		return fmt.Errorf("监听端口失败: %v", err)
	}
	s.listener = listener

	fmt.Printf("反向Shell服务端启动，监听端口: %s\n", s.port)
	fmt.Printf("运行平台: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Println("等待客户端连接...")

	return nil
}

// 等待客户端连接
func (s *ReverseShellServer) WaitForConnection() error {
	conn, err := s.listener.Accept()
	if err != nil {
		return fmt.Errorf("接受连接失败: %v", err)
	}

	s.mu.Lock()
	s.conn = conn
	s.active = true
	s.escapeMode = false
	s.mu.Unlock()

	fmt.Printf("客户端已连接: %s\n", conn.RemoteAddr())
	fmt.Println("进入交互模式...")
	fmt.Println("快捷键:")
	fmt.Println("  Ctrl+] - 进入命令模式")
	fmt.Println("  Ctrl+\\ - 强制退出")
	fmt.Println("----------------------------------------")

	return nil
}

// 处理命令模式
func (s *ReverseShellServer) handleCommandMode() {
	s.restoreTerminal()
	fmt.Print("\r\n命令模式 (输入h查看帮助): ")

	buffer := make([]byte, 1)
	for {
		n, err := os.Stdin.Read(buffer)
		if err != nil || n == 0 {
			break
		}

		cmd := buffer[0]
		switch cmd {
		case 'q', 'e': // quit/exit
			fmt.Print("\r\n退出会话\r\n")
			s.closeConnection()
			return
		case 'c': // continue
			fmt.Print("\r\n继续会话...\r\n")
			s.setRawMode()
			s.mu.Lock()
			s.escapeMode = false
			s.mu.Unlock()
			return
		case 's': // status
			s.mu.Lock()
			if s.conn != nil {
				fmt.Printf("\r\n连接状态: 已连接到 %s\r\n", s.conn.RemoteAddr())
			} else {
				fmt.Print("\r\n连接状态: 未连接\r\n")
			}
			s.mu.Unlock()
			// fmt.Print("命令模式: ")
		case 'r': // reconnect
			fmt.Print("\r\n等待重新连接...\r\n")
			s.closeConnection()
			return
		case 'h': // help
			fmt.Print("\r\n命令帮助:\r\n")
			fmt.Print("  c - 继续会话\r\n")
			fmt.Print("  s - 显示状态\r\n")
			fmt.Print("  r - 断开并等待重连\r\n")
			fmt.Print("  q/e - 退出程序\r\n")
			fmt.Print("  h - 显示帮助\r\n")
			// fmt.Print("命令模式: ")
		case '\r', '\n':
			fmt.Print("命令模式: ")
		default:
			fmt.Printf("\r\n未知命令: %c (输入h查看帮助)\r\n", cmd)
			// fmt.Print("命令模式: ")
		}
	}
}

// 从stdin读取并发送到远程
func (s *ReverseShellServer) handleStdinInput() {
	buffer := make([]byte, 4096)

	for {
		s.mu.Lock()
		if !s.active || s.conn == nil {
			s.mu.Unlock()
			return
		}
		if s.escapeMode {
			s.mu.Unlock()
			time.Sleep(100 * time.Millisecond)
			continue
		}
		conn := s.conn
		s.mu.Unlock()

		n, err := os.Stdin.Read(buffer)
		if err != nil {
			if err != io.EOF {
				log.Printf("读取stdin失败: %v", err)
			}
			return
		}

		if n > 0 {
			// 处理特殊键序列
			for i := 0; i < n; i++ {
				switch buffer[i] {
				case 29: // Ctrl+] (ASCII 29)
					s.mu.Lock()
					s.escapeMode = true
					s.mu.Unlock()
					go s.handleCommandMode()
					continue
				case 28: // Ctrl+\ (ASCII 28)
					fmt.Print("\r\n强制退出\r\n")
					s.closeConnection()
					return
				}
			}

			_, err = conn.Write(buffer[:n])
			if err != nil {
				log.Printf("写入Socket失败: %v", err)
				s.closeConnection()
				return
			}
		}
	}
}

// 从远程读取并输出到stdout
func (s *ReverseShellServer) handleRemoteOutput() {
	buffer := make([]byte, 4096)

	for {
		s.mu.Lock()
		if !s.active || s.conn == nil {
			s.mu.Unlock()
			return
		}
		conn := s.conn
		s.mu.Unlock()

		// 设置读取超时
		conn.SetReadDeadline(time.Now().Add(time.Second))
		n, err := conn.Read(buffer)

		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue // 超时继续
			}
			if err != io.EOF {
				log.Printf("读取远程数据失败: %v", err)
			}
			s.closeConnection()
			return
		}

		if n > 0 {
			_, err = os.Stdout.Write(buffer[:n])
			if err != nil {
				log.Printf("输出数据失败: %v", err)
				return
			}
		}
	}
}

// 关闭连接
func (s *ReverseShellServer) closeConnection() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conn != nil {
		s.conn.Close()
		s.conn = nil
	}
	s.active = false
	s.escapeMode = false
}

// 交互式会话
func (s *ReverseShellServer) InteractiveSession() {
	// 设置raw模式
	if err := s.setRawMode(); err != nil {
		log.Printf("设置raw模式失败: %v", err)
		return
	}
	defer s.restoreTerminal()

	// 启动数据转发goroutine
	go s.handleStdinInput()
	go s.handleRemoteOutput()

	// 等待连接关闭
	for {
		s.mu.Lock()
		active := s.active
		s.mu.Unlock()

		if !active {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	fmt.Println("连接已断开")
}

// 停止服务器
func (s *ReverseShellServer) Stop() {
	s.closeConnection()
	if s.listener != nil {
		s.listener.Close()
	}
	s.restoreTerminal()
}

// 设置信号处理 (跨平台兼容)
func (s *ReverseShellServer) setupSignalHandler() {
	c := make(chan os.Signal, 1)

	// 根据平台设置不同的信号
	if runtime.GOOS == "windows" {
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	} else {
		signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
		// SIGWINCH在Linux上才有效，暂时移除以保证跨平台兼容
	}

	go func() {
		sig := <-c
		fmt.Printf("\r\n收到信号: %v\r\n", sig)
		s.Stop()
		os.Exit(0)
	}()
}

func main() {
	asciilogo := `  _______   _ _____                _          _ _ 
 |__   __| | |  __ \              | |        | | |
    | | ___| | |__) |_____   _____| |__   ___| | |
    | |/ _ \ |  _  // _ \ \ / / __| '_ \ / _ \ | |
    | |  __/ | | \ \  __/\ V /\__ \ | | |  __/ | |
    |_|\___|_|_|  \_\___| \_/ |___/_| |_|\___|_|_|
                                                  
                                                  `


    fmt.Println(asciilogo)

	if len(os.Args) != 2 {
		fmt.Printf("用法: %s <监听端口>\n", os.Args[0])
		fmt.Printf("示例: %s 1337\n", os.Args[0])
		os.Exit(1)
	}

	port := os.Args[1]
	server := NewReverseShellServer(port)

	// 设置信号处理
	server.setupSignalHandler()

	// 启动服务器
	if err := server.Start(); err != nil {
		log.Fatalf("启动服务器失败: %v", err)
	}
	defer server.Stop()

	// 主循环：等待连接并处理
	for {
		if err := server.WaitForConnection(); err != nil {
			log.Printf("等待连接失败: %v", err)
			continue
		}

		// 进入交互模式
		server.InteractiveSession()

		fmt.Println("等待下一个连接...")
	}
}
