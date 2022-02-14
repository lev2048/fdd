package fdd

import (
	"github.com/rocinan/fdd/poller"
	"golang.org/x/sys/unix"
)

type TCPRelay struct {
	isClose      bool
	listenAddr   string
	remoteAddr   string
	listenPort   int
	remotePort   int
	serverSocket int

	eventLoop     *poller.EventLoop
	socketHandler map[int]*TCPRelayHandler
}

// NewTCPRelay 创建TCP中继 la => 监听地址 eg: 0.0.0.0 lp => 监听端口 eg: 1080
func NewTCPRelay(la string, lp int, ra string, rp int, cap int) (*TCPRelay, error) {
	if fd, err := CreateTcpListenSocket(la, lp); err != nil {
		return nil, err
	} else {
		return &TCPRelay{
			isClose:       false,
			listenAddr:    la,
			listenPort:    lp,
			remoteAddr:    ra,
			remotePort:    rp,
			serverSocket:  fd,
			socketHandler: make(map[int]*TCPRelayHandler, cap),
		}, nil
	}
}

func (tr *TCPRelay) AddToLoop(eventLoop *poller.EventLoop) bool {
	if tr.eventLoop != nil {
		log.Warn("already add to loop")
		return false
	}
	tr.eventLoop = eventLoop
	if err := tr.eventLoop.Register(tr.serverSocket, kPollIn|kPollErr, tr); err != nil {
		log.Warn("failed to register poller: ", err)
		return false
	}
	return true
}

func (tr *TCPRelay) HandleEvent(fd, event int) {
	if fd == INVALID_SOCKET {
		log.Warn("invalid tcp listen socket")
		return
	}
	if fd != tr.serverSocket {
		log.Warn("invalid socket ", fd)
		return
	}
	if event == kPollErr {
		defer tr.Close()
		tr.eventLoop.Close()
		log.Error("KPollErr")
		return
	}
	if cfd, err := AcceptTcpConn(fd); err != nil {
		log.Error("accept new tcp conn error: ", err)
		return
	} else {
		defer SetNoBlock(cfd)
		log.Info("create new tcphandler")
		if rfd, err := CreateRemoteSocket(tr.remoteAddr, tr.remotePort); err != nil {
			log.Warn("create new tcp remote conn error: ", err)
			return
		} else {
			tcpRelayHandler := NewTCPRelayHandler(cfd, rfd, tr, tr.eventLoop)
			if err := tr.eventLoop.Register(cfd, kPollIn|kPollErr, tcpRelayHandler); err != nil {
				log.Warn("new tcp local socket add to eventlopp error: ", err)
				return
			}
			if err := tr.eventLoop.Register(rfd, kPollOut|kPollErr, tcpRelayHandler); err != nil {
				log.Warn("new tcp remote socket add to eventlopp error: ", err)
				return
			}
			tr.socketHandler[cfd] = tcpRelayHandler
		}
	}
}

func (tr *TCPRelay) Close() {
	tr.isClose = true
	if tr.eventLoop != nil {
		tr.eventLoop.UnRegister(int32(tr.serverSocket))
	}
	if tr.serverSocket != INVALID_SOCKET {
		CloseSocket(tr.serverSocket)
	}
}

type TCPRelayHandler struct {
	localSocket       int
	remoteSocket      int
	upStreamStatus    int
	downStreamStatus  int
	dataWriteToLocal  []byte
	dataWriteToRemote []byte

	server    *TCPRelay
	eventLoop *poller.EventLoop
}

func NewTCPRelayHandler(lfd, rfd int, ser *TCPRelay, ep *poller.EventLoop) *TCPRelayHandler {
	return &TCPRelayHandler{
		localSocket:       lfd,
		remoteSocket:      rfd,
		upStreamStatus:    kWaitStatusReadWriting,
		downStreamStatus:  kWaitStatusReading,
		dataWriteToLocal:  make([]byte, 0),
		dataWriteToRemote: make([]byte, 0),
		server:            ser,
		eventLoop:         ep,
	}
}

func (th *TCPRelayHandler) HandleEvent(fd, event int) {
	if fd == th.remoteSocket {
		if (event & kPollErr) != 0 {
			th.onError("remote")
		}
		if (event & (kPollIn | kPollHup)) != 0 {
			th.onRemoteRead()
		}
		if (event & kPollOut) != 0 {
			th.onRemoteWrite()
		}
	} else if fd == th.localSocket {
		if (event & kPollErr) != 0 {
			th.onError("local")
		}
		if (event & (kPollIn | kPollHup)) != 0 {
			th.onLocalRead()
		}
		if (event & kPollOut) != 0 {
			th.onLocalWrite()
		}
	} else {
		log.Warn("unkonwn socket")
	}
}

func (th *TCPRelayHandler) updateStream(stream, status int) {
	if stream == kStreamDown {
		if th.downStreamStatus != status {
			th.downStreamStatus = status
		} else {
			return
		}
	} else if stream == kStreamUp {
		if th.upStreamStatus != status {
			th.upStreamStatus = status
		} else {
			return
		}
	}
	if th.localSocket != INVALID_SOCKET {
		event := kPollErr
		if (th.downStreamStatus & kWaitStatusWriting) != 0 {
			event |= kPollOut
		}
		if th.upStreamStatus == kWaitStatusReading {
			event |= kPollIn
		}
		th.eventLoop.Modify(th.localSocket, event)
	}
	if th.remoteSocket != INVALID_SOCKET {
		event := kPollErr
		if (th.downStreamStatus & kWaitStatusReading) != 0 {
			event |= kPollIn
		}
		if (th.upStreamStatus & kWaitStatusWriting) != 0 {
			event |= kPollOut
		}
		th.eventLoop.Modify(th.remoteSocket, event)
	}
}

func (th *TCPRelayHandler) writeToSock(fd int, data []byte) {
	if len(data) == 0 || fd == INVALID_SOCKET {
		return
	}
	uncomplete := false
	if _, err := BufferSend(fd, &data); err != nil {
		if err == unix.EAGAIN {
			uncomplete = true
		} else {
			th.Destroy()
			return
		}
	}
	if uncomplete {
		if fd == th.localSocket {
			th.dataWriteToLocal = append(th.dataWriteToLocal, data...)
			th.updateStream(kStreamDown, kWaitStatusWriting)
		} else if fd == th.remoteSocket {
			th.dataWriteToRemote = append(th.dataWriteToRemote, data...)
			th.updateStream(kStreamUp, kWaitStatusWriting)
		}
	} else {
		if fd == th.localSocket {
			th.updateStream(kStreamDown, kWaitStatusReading)
		} else if fd == th.remoteSocket {
			th.updateStream(kStreamUp, kWaitStatusReading)
		}
	}
}

func (th *TCPRelayHandler) onLocalRead() {
	buf := make([]byte, kUpStreamBufSize)
	if n, err := BufferRecv(th.localSocket, &buf); err != nil || n == 0 {
		if err == unix.EAGAIN {
			return
		} else if err != nil {
			log.Warn("onLocalReadError: ", err)
		}
		if n == 0 {
			log.Info("local conn close. ")
		}
		th.Destroy()
	} else {
		buf = buf[:n]
		th.writeToSock(th.remoteSocket, buf)
	}
}

func (th *TCPRelayHandler) onRemoteRead() {
	buf := make([]byte, kUpStreamBufSize)
	if n, err := BufferRecv(th.remoteSocket, &buf); err != nil || n == 0 {
		if err == unix.EAGAIN {
			return
		} else if err != nil {
			log.Warn("onLocalReadError: ", err)
		}
		if n == 0 {
			log.Info("local conn close. ")
		}
		th.Destroy()
	} else {
		buf = buf[:n]
		th.writeToSock(th.localSocket, buf)
	}
}

func (th *TCPRelayHandler) onLocalWrite() {
	if len(th.dataWriteToLocal) != 0 {
		data := th.dataWriteToLocal
		th.dataWriteToLocal = make([]byte, 0)
		th.writeToSock(th.localSocket, data)
		return
	}
	th.updateStream(kStreamDown, kWaitStatusReading)
}

func (th *TCPRelayHandler) onRemoteWrite() {
	if len(th.dataWriteToRemote) != 0 {
		data := th.dataWriteToRemote
		th.dataWriteToRemote = make([]byte, 0)
		th.writeToSock(th.remoteSocket, data)
		return
	}
	th.updateStream(kStreamUp, kWaitStatusReading)
}

func (th *TCPRelayHandler) onError(stream string) {
	log.Error("got " + " error")
	th.Destroy()
}

func (th *TCPRelayHandler) Destroy() {
	if th.remoteSocket != INVALID_SOCKET {
		th.eventLoop.UnRegister(int32(th.remoteSocket))
		CloseSocket(th.remoteSocket)
		th.remoteSocket = INVALID_SOCKET
	}
	if th.localSocket != INVALID_SOCKET {
		th.eventLoop.UnRegister(int32(th.localSocket))
		CloseSocket(th.localSocket)
		th.localSocket = INVALID_SOCKET
	}
	delete(th.server.socketHandler, th.localSocket)
}
