package fdd

import (
	"github.com/rocinan/fdd/poller"
	"golang.org/x/sys/unix"
)

type TCPRelay struct {
	localSocket int

	cfg           *Config
	eventLoop     *poller.EventLoop
	socketHandler map[int]*TCPRelayHandler
}

func NewTCPRelay(cfg *Config) (*TCPRelay, error) {
	if fd, err := CreateTcpListenSocket(cfg.ListenAddr, cfg.ListenPort); err != nil {
		return nil, err
	} else {
		return &TCPRelay{
			cfg:           cfg,
			localSocket:   fd,
			socketHandler: make(map[int]*TCPRelayHandler, cfg.HandlerCap),
		}, nil
	}
}

func (t *TCPRelay) AddToLoop(ep *poller.EventLoop) error {
	t.eventLoop = ep
	return t.eventLoop.Register(t.localSocket, kPollIn|kPollErr, t)
}

func (t *TCPRelay) HandleEvent(fd, ev int) {
	if fd == INVALID_SOCKET {
		log.Warn("[tcp_relay] invalid tcp listen socket")
	} else if ev == kPollErr {
		log.Warn("[tcp_relay] handle event poll err: ", fd, ev)
		defer t.Close()
		return
	}
	if cfd, err := AcceptTcpConn(fd); err != nil {
		log.Error("[tcp_relay] accept new tcp conn error: ", err)
		return
	} else {
		defer SetNoBlock(cfd)
		if rfd, err := CreateRemoteSocket(t.cfg.RemoteAddr, t.cfg.RemotePort); err != nil {
			log.Warn("[tcp_relay] create new tcp conn error: ", err)
		} else {
			tcpRelayHandler := NewTCPRelayHandler(cfd, rfd, t, t.eventLoop)
			if err := t.eventLoop.Register(cfd, kPollIn|kPollErr, tcpRelayHandler); err != nil {
				log.Warn("[tcp_relay] reg new local conn err: ", err)
				return
			}
			if err := t.eventLoop.Register(rfd, kPollIn|kPollErr, tcpRelayHandler); err != nil {
				log.Warn("[tcp_relay] reg new remote conn err: ", err)
				return
			}
			t.socketHandler[cfd] = tcpRelayHandler
		}
	}
}

func (t *TCPRelay) Close() {
	for k, v := range t.socketHandler {
		v.Destroy()
		delete(t.socketHandler, k)
	}
	t.eventLoop.UnRegister(t.localSocket)
	if t.localSocket != INVALID_SOCKET {
		CloseSocket(t.localSocket)
	}
	log.Info("[tcp_relay] tcp relay service exit.")
}

type TCPRelayHandler struct {
	localSocket  int
	remoteSocket int

	flow      *Flow
	eventLoop *poller.EventLoop
}

func NewTCPRelayHandler(ls, rs int, ser *TCPRelay, ep *poller.EventLoop) *TCPRelayHandler {
	return &TCPRelayHandler{
		localSocket:  ls,
		remoteSocket: rs,
		flow:         NewFlow(ls, rs, ep),
		eventLoop:    ep,
	}
}

func (th *TCPRelayHandler) HandleEvent(fd, ev int) {
	if fd == th.remoteSocket {
		if (ev & kPollErr) != 0 {
			log.Warn("[tcp_handler]: handle remote event poll err: ", fd, ev)
			th.Destroy()
		} else if (ev & (kPollIn | kPollHup)) != 0 {
			th.onRemoteRead()
		} else if (ev & kPollOut) != 0 {
			th.onRemoteWrite()
		}
	} else if fd == th.localSocket {
		if (ev & kPollErr) != 0 {
			log.Warn("[tcp_handler]: handle local event poll err: ", fd, ev)
			th.Destroy()
		} else if (ev & (kPollIn | kPollHup)) != 0 {
			th.onLocalRead()
		} else if (ev & kPollOut) != 0 {
			th.onLocalWrite()
		}
	} else {
		log.Warn("[tcp_handler]: unkonwn socket")
	}
}

func (th *TCPRelayHandler) writeToSock(fd int, data *[]byte) {
	if len(*data) == 0 || fd == INVALID_SOCKET {
		return
	}
	uncomplete := false
	if _, err := BufferSend(fd, data); err != nil {
		if err == unix.EAGAIN {
			uncomplete = true
		} else {
			log.Warn("[tcp_handler] send buffer err: ", err)
			th.Destroy()
			return
		}
	}
	if uncomplete {
		if fd == th.localSocket {
			th.flow.DataWriteToLocal = append(th.flow.DataWriteToLocal, *data...)
			th.flow.Update(kStreamDown, kWaitStatusWriting)
		} else if fd == th.remoteSocket {
			th.flow.DataWriteToRemote = append(th.flow.DataWriteToRemote, *data...)
			th.flow.Update(kStreamUp, kWaitStatusWriting)
		}
	} else {
		if fd == th.localSocket {
			th.flow.Update(kStreamDown, kWaitStatusReading)
		} else if fd == th.remoteSocket {
			th.flow.Update(kStreamUp, kWaitStatusReading)
		}
	}
}

func (th *TCPRelayHandler) onLocalRead() {
	buf := make([]byte, kUpStreamBufSize)
	if n, err := BufferRecv(th.localSocket, &buf); err != nil || n == 0 {
		if err == unix.EAGAIN {
			return
		} else if err != nil {
			log.Warn("[tcp_handler]: on local read err: ", err)
		}
		th.Destroy()
	} else {
		buf = buf[:n]
		th.writeToSock(th.remoteSocket, &buf)
	}
}

func (th *TCPRelayHandler) onRemoteRead() {
	buf := make([]byte, kDownStreamBufSize)
	if n, err := BufferRecv(th.remoteSocket, &buf); err != nil || n == 0 {
		if err == unix.EAGAIN {
			return
		} else if err != nil {
			log.Warn("[tcp_handler] on remote read err: ", err)
		}
		th.Destroy()
	} else {
		buf = buf[:n]
		th.writeToSock(th.localSocket, &buf)
	}
}

func (th *TCPRelayHandler) onLocalWrite() {
	if len(th.flow.DataWriteToLocal) != 0 {
		data := th.flow.DataWriteToLocal
		th.flow.DataWriteToLocal = make([]byte, 0, kDownStreamBufSize)
		th.writeToSock(th.localSocket, &data)
		return
	}
	th.flow.Update(kStreamDown, kWaitStatusReading)
}

func (th *TCPRelayHandler) onRemoteWrite() {
	if len(th.flow.DataWriteToRemote) != 0 {
		data := th.flow.DataWriteToRemote
		th.flow.DataWriteToRemote = make([]byte, 0)
		th.writeToSock(th.remoteSocket, &data)
		return
	}
	th.flow.Update(kStreamUp, kWaitStatusReading)
}

func (th *TCPRelayHandler) Destroy() {
	if th.remoteSocket != INVALID_SOCKET {
		th.eventLoop.UnRegister(th.remoteSocket)
		CloseSocket(th.remoteSocket)
		th.remoteSocket = INVALID_SOCKET
	}
	if th.localSocket != INVALID_SOCKET {
		th.eventLoop.UnRegister(th.localSocket)
		CloseSocket(th.localSocket)
		th.localSocket = INVALID_SOCKET
	}
}
