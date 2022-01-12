//go:build linux

package poller

import (
	"errors"
	"fmt"
	"time"

	"golang.org/x/sys/unix"
)

type EventLoop struct {
	fd       int
	isStop   bool
	handler  map[int32]ISockNotify
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
		handler:  make(map[int32]ISockNotify, kEpollSize),
		waitDone: make(chan struct{}),
	}, nil
}

//Register 注册事件
func (e *EventLoop) Register(fd int, mod int, obj ISockNotify) error {
	events := 0
	e.handler[int32(fd)] = obj
	if (mod & kPollIn) != 0 {
		events = unix.EPOLLIN
	}
	if (mod & kPollOut) != 0 {
		events = unix.EPOLLOUT
	}
	return unix.EpollCtl(e.fd, unix.EPOLL_CTL_ADD, int(fd), &unix.EpollEvent{
		Fd:     int32(fd),
		Events: uint32(events),
	})
}

//Modify 修改事件
func (e *EventLoop) Modify(fd int, mod int) error {
	event := 0
	if (mod & kPollIn) != 0 {
		event = unix.EPOLLIN
	}
	if (mod & kPollOut) != 0 {
		event = unix.EPOLLOUT
	}
	return unix.EpollCtl(e.fd, unix.EPOLL_CTL_MOD, int(fd), &unix.EpollEvent{
		Events: uint32(event),
		Fd:     int32(fd),
	})
}

//UnRegister 销毁事件
func (e *EventLoop) UnRegister(fd int32) error {
	delete(e.handler, fd)
	return unix.EpollCtl(e.fd, unix.EPOLL_CTL_DEL, int(fd), nil)
}

//Run 启动epoll wait 循环
func (e *EventLoop) Run() {
	defer close(e.waitDone)
	events, timeout := make([]unix.EpollEvent, kEpollSize), 0
	for !e.isStop {
		nfds, err := unix.EpollWait(e.fd, events, timeout)
		if err != nil && err != unix.EINTR {
			fmt.Println("EpollWait: ", err)
			continue
		}
		if nfds <= 0 {
			timeout = kTimeoutPrecision * 1000
			continue
		}
		timeout = 0
		for i := 0; i < nfds; i++ {
			if obj, ok := e.handler[events[i].Fd]; ok {
				obj.HandleEvent(int(events[i].Fd), int(events[i].Events))
			} else {
				fmt.Println("warn event", events[i].Fd)
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
