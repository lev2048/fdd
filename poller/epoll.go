//go:build linux

package poller

import (
	"errors"
	"log"
	"time"

	"golang.org/x/sys/unix"
)

type EventLoop struct {
	fd       int
	isStop   bool
	handler  map[int]ISockNotify
	sockMode map[int]int
	waitDone chan struct{}
}

//Create 创建Poller
func Create() (*EventLoop, error) {
	fd, err := unix.EpollCreate(kMaxEpollSize)
	if err != nil {
		return nil, err
	}
	return &EventLoop{
		fd:       fd,
		isStop:   false,
		handler:  make(map[int]ISockNotify, kEpollSize),
		sockMode: make(map[int]int, kEpollSize),
		waitDone: make(chan struct{}),
	}, nil
}

//Register 注册事件
func (e *EventLoop) Register(s int, mode int, obj ISockNotify) error {
	e.sockMode[s], e.handler[s] = mode, obj
	ev := &unix.EpollEvent{
		Events: 0,
		Fd:     int32(s),
	}
	if Judge(mode & kPollIn) {
		ev.Events |= unix.EPOLLIN
	}
	if Judge(mode & kPollOut) {
		ev.Events |= unix.EPOLLOUT
	}
	return unix.EpollCtl(e.fd, unix.EPOLL_CTL_ADD, s, ev)
}

//UnRegister 销毁事件
func (e *EventLoop) UnRegister(s int) error {
	delete(e.handler, s)
	mode := e.sockMode[s]
	defer delete(e.sockMode, s)
	ev := &unix.EpollEvent{Events: 0, Fd: int32(s)}
	if Judge(mode & kPollIn) {
		ev.Events |= unix.EPOLLIN
	}
	if Judge(mode & kPollOut) {
		ev.Events |= unix.EPOLLOUT
	}
	return unix.EpollCtl(e.fd, unix.EPOLL_CTL_DEL, s, ev)
}

//Modify 修改事件
func (e *EventLoop) Modify(s int, mode int) error {
	e.sockMode[s] = mode
	ev := &unix.EpollEvent{Events: 0, Fd: int32(s)}
	if Judge(mode & kPollIn) {
		ev.Events |= unix.EPOLLIN
	}
	if Judge(mode & kPollOut) {
		ev.Events |= unix.EPOLLOUT
	}
	return unix.EpollCtl(e.fd, unix.EPOLL_CTL_MOD, s, ev)
}

//Run 启动epoll循环
func (e *EventLoop) Run() {
	defer close(e.waitDone)
	events, timeout := make([]unix.EpollEvent, kEpollSize), 0
	for !e.isStop {
		nfds, err := unix.EpollWait(e.fd, events, timeout)
		if err != nil && err == unix.EINTR {
			continue
		}
		if err != nil || nfds == -1 {
			log.Default().Println("[EventLoop] error: ", err)
			return
		}
		if nfds == 0 {
			timeout = 1000
			continue
		}
		timeout = 0
		for i := 0; i < nfds; i++ {
			mode := 0
			if Judge(int(events[i].Events) & unix.EPOLLIN) {
				mode |= kPollIn
			}
			if Judge(int(events[i].Events) & unix.EPOLLOUT) {
				mode |= kPollOut
			}
			if obj, ok := e.handler[int(events[i].Fd)]; ok {
				obj.HandleEvent(int(events[i].Fd), mode)
			} else {
				log.Default().Println("[EventLoop] unknow fileDescriptor: ", events[i].Fd)
			}
		}
	}
}

//Close 关闭epoll
func (e *EventLoop) Close() error {
	e.isStop = true
	select {
	case <-e.waitDone:
		_ = unix.Close(e.fd)
		return nil
	case <-time.After(time.Second * 15):
		e.isStop = false
		return errors.New("close eventloop error: timeout")
	}
}
