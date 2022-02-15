package fdd

import (
	"github.com/rocinan/fdd/poller"
)

const (
	kStreamUp int = iota
	kStreamDown
)

const (
	kWaitStatusInit = iota
	kWaitStatusReading
	kWaitStatusWriting
	kWaitStatusReadWriting = kWaitStatusReading | kWaitStatusWriting
)

const (
	kBuffSize          = 16 * 1024
	kUpStreamBufSize   = 16 * 1024
	kDownStreamBufSize = 32 * 1024
)

const (
	kPollNull = 0x00
	kPollIn   = 0x01
	kPollOut  = 0x04
	kPollErr  = 0x08
	kPollHup  = 0x10
	kPollNval = 0x20
)

const (
	INVALID_SOCKET = -1
)

type Flow struct {
	UpStatus     int
	DownStatus   int
	localSocket  int
	remoteSocket int
	eventloop    *poller.EventLoop

	DataWriteToLocal  []byte
	DataWriteToRemote []byte
}

func NewFlow(ls, rs int, poller *poller.EventLoop) *Flow {
	return &Flow{
		localSocket:       ls,
		remoteSocket:      rs,
		UpStatus:          kWaitStatusReading,
		eventloop:         poller,
		DownStatus:        kWaitStatusReading,
		DataWriteToLocal:  make([]byte, 0, kDownStreamBufSize),
		DataWriteToRemote: make([]byte, 0, kUpStreamBufSize),
	}
}

//Update 更新数据流向和epoll监听 flow 方向 status 新状态
func (f *Flow) Update(flow, status int) error {
	if flow == kStreamDown {
		if f.DownStatus != status {
			f.DownStatus = status
		} else {
			return nil
		}
	}
	if flow == kStreamUp {
		if f.UpStatus != status {
			f.UpStatus = status
		} else {
			return nil
		}
	}
	if f.localSocket != INVALID_SOCKET {
		event := kPollErr
		if (f.DownStatus & kWaitStatusWriting) != 0 {
			event |= kPollOut
		}
		if f.UpStatus == kWaitStatusReading {
			event |= kPollIn
		}
		f.eventloop.Modify(f.localSocket, event)
	}
	if f.remoteSocket != INVALID_SOCKET {
		event := kPollErr
		if (f.DownStatus & kWaitStatusReading) != 0 {
			event |= kPollIn
		}
		if (f.UpStatus & kWaitStatusWriting) != 0 {
			event |= kPollOut
		}
		f.eventloop.Modify(f.remoteSocket, event)
	}
	return nil
}
