package fdd

import "github.com/rocinan/fdd/poller"

type UDPRelay struct {
}

func (ur *UDPRelay) AddToLoop(eventLoop *poller.EventLoop) bool {
	return true
}

func (ur *UDPRelay) HandleEvent(fd, event int) {}

func (ur *UDPRelay) handleClient() {}

func (ur *UDPRelay) handleServer() {}

func (ur *UDPRelay) Destroy() {}
