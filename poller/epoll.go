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

func (e *EventLoop) Register(fd int32, mod int, obj ISockNotify) error {
	events := 0
	e.handler[fd] = obj
	if mod == kPollIn {
		events = unix.EPOLLIN
	}
	if mod == kPollOut {
		events = unix.EPOLLOUT
	}
	return unix.EpollCtl(e.fd, unix.EPOLL_CTL_ADD, int(fd), &unix.EpollEvent{
		Fd:     fd,
		Events: uint32(events),
	})
}

func (e *EventLoop) Modify(fd int32, mod int) error {
	events := 0
	if mod == kPollIn {
		events = unix.EPOLLIN
	}
	if mod == kPollOut {
		events = unix.EPOLLOUT
	}
	return unix.EpollCtl(e.fd, unix.EPOLL_CTL_MOD, int(fd), &unix.EpollEvent{
		Events: uint32(events),
		Fd:     int32(fd),
	})
}

func (e *EventLoop) UnRegister(fd int32) error {
	delete(e.handler, fd)
	return unix.EpollCtl(e.fd, unix.EPOLL_CTL_DEL, int(fd), nil)
}

func (e *EventLoop) Run() {
	defer close(e.waitDone)
	events, timeout := make([]unix.EpollEvent, kEpollSize), 0
	for !e.isStop {
		nfds, err := unix.EpollWait(e.fd, events, timeout)
		fmt.Println("xx:xx", nfds)
		if err != nil && err != unix.EINTR {
			fmt.Println("EpollWait: ", err)
			continue
		}
		if nfds <= 0 {
			timeout = kTimeoutPrecision * 1000
			continue
		}
		timeout = 0
		for _, v := range events {
			if obj, ok := e.handler[v.Fd]; ok {
				obj.HandleEvent(uint32(v.Fd), v.Events)
			}
		}
	}
}

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
