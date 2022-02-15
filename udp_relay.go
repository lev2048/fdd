package fdd

import (
	"github.com/rocinan/fdd/poller"
	"golang.org/x/sys/unix"
)

type LocalData struct {
	data []byte
	dest unix.SockaddrInet4
}

type UDPRelay struct {
	localSocket int

	cfg              *Config
	eventLoop        *poller.EventLoop
	socketHandler    map[string]*UDPRelayHandler
	dataWriteToLocal []LocalData
}

func NewUDPRelay(cfg *Config) (*UDPRelay, error) {
	if fd, err := CreateUdpListenSocket(cfg.ListenAddr, cfg.ListenPort); err != nil {
		return nil, err
	} else {
		return &UDPRelay{
			cfg:              cfg,
			localSocket:      fd,
			socketHandler:    make(map[string]*UDPRelayHandler, cfg.HandlerCap),
			dataWriteToLocal: make([]LocalData, 0, kDownStreamBufSize),
		}, nil
	}
}

func (u *UDPRelay) AddToLoop(ep *poller.EventLoop) error {
	u.eventLoop = ep
	return u.eventLoop.Register(u.localSocket, kPollIn|kPollErr, u)
}

func (u *UDPRelay) HandleEvent(fd, ev int) {
	if (ev & kPollErr) != 0 {
		log.Warn("[udp_relay]: handle event poll err: ", fd, ev)
		defer u.Close()
	} else if (ev & (kPollIn | kPollHup)) != 0 {
		u.onLocalRead()
	} else if (ev & kPollOut) != 0 {
		u.onLocalWrite()
	} else {
		log.Warn("[udp_realy]: handle event err: ", fd, ev)
	}
}

func (u *UDPRelay) onLocalRead() {
	var handler *UDPRelayHandler
	buf := make([]byte, kBuffSize)
	n, sa, err := PacketRecv(u.localSocket, &buf)
	if ok := CheckError("[udp_realay] on local read err: ", err); !ok {
		return
	}
	if uh, ok := u.socketHandler[MD5Addr(sa.Addr[:], sa.Port)]; !ok {
		serverSocket, err := CreateUdpRemoteSocket(u.cfg.RemoteAddr, u.cfg.RemotePort)
		if ok := CheckError("[udp_realay] create handler err: ", err); !ok {
			return
		}
		handler = NewUDPRealyHandler(u.localSocket, serverSocket, unix.SockaddrInet4{
			Addr: sa.Addr,
			Port: sa.Port,
		}, SockAddrParse(u.cfg.RemoteAddr, u.cfg.RemotePort), u.eventLoop)
		u.socketHandler[MD5Addr(sa.Addr[:], sa.Port)] = handler
		u.eventLoop.Register(handler.remoteSocket, kPollIn|kPollErr, handler)
	} else {
		handler = uh
	}
	buf = buf[:n]
	handler.SendToRemote(&buf)
}

func (u *UDPRelay) onLocalWrite() {
	if len(u.dataWriteToLocal) != 0 {
		pkt := u.dataWriteToLocal
		u.dataWriteToLocal = make([]LocalData, 0, kDownStreamBufSize)
		for _, v := range pkt {
			u.SendToLocal(u.localSocket, &v.data, &v.dest)
		}
		return
	}
	CheckError("[udp_relay] on local write err: ", u.eventLoop.Modify(u.localSocket, kPollIn|kPollErr))
}

func (u *UDPRelay) SendToLocal(fd int, p *[]byte, sa *unix.SockaddrInet4) {
	uncomplete := false
	if len(*p) == 0 || fd == INVALID_SOCKET {
		log.Warn("[udp_relay] send pkg to local err: ", len(*p), fd)
		return
	}
	if err := PacketSend(fd, p, sa); err != nil {
		if err == unix.EAGAIN {
			uncomplete = true
		} else {
			log.Error("[udp_relay] send pkg to local err: ", err)
			u.Close()
			return
		}
	}
	if uncomplete {
		u.dataWriteToLocal = append(u.dataWriteToLocal, LocalData{
			dest: *sa,
			data: *p,
		})
		u.eventLoop.Modify(u.localSocket, kWaitStatusReadWriting)
	} else {
		u.eventLoop.Modify(u.localSocket, kWaitStatusReading)
	}
}

func (u *UDPRelay) Close() {
	for k, v := range u.socketHandler {
		v.Destroy()
		delete(u.socketHandler, k)
	}
	u.eventLoop.UnRegister(u.localSocket)
	if u.localSocket != INVALID_SOCKET {
		CloseSocket(u.localSocket)
	}
	log.Info("[udp_relay] udp relay service exit.")
}

type UDPRelayHandler struct {
	localSocket  int
	remoteSocket int

	flow      *Flow
	src       unix.SockaddrInet4
	dest      unix.SockaddrInet4
	eventLoop *poller.EventLoop
}

func NewUDPRealyHandler(ls, rs int, src, dest unix.SockaddrInet4, ev *poller.EventLoop) *UDPRelayHandler {
	return &UDPRelayHandler{
		src:          src,
		dest:         dest,
		flow:         NewFlow(ls, rs, ev),
		eventLoop:    ev,
		localSocket:  ls,
		remoteSocket: rs,
	}
}

func (uh *UDPRelayHandler) HandleEvent(fd, ev int) {
	if (ev & kPollErr) != 0 {
		log.Warn("[udp_handler]: handle event poll err: ", fd, ev)
		uh.Destroy()
	} else if (ev & (kPollIn | kPollHup)) != 0 {
		uh.onRemoteRead()
	} else if (ev & kPollOut) != 0 {
		uh.onRemoteWrite()
	} else {
		log.Warn("[udp_handler]: handle event err: ", fd, ev)
	}
}

func (uh *UDPRelayHandler) onRemoteRead() {
	buf := make([]byte, kBuffSize)
	n, _, err := PacketRecv(uh.remoteSocket, &buf)
	if ok := CheckError("[udp_handler] recv packet error: ", err); !ok {
		uh.Destroy()
		return
	}
	buf = buf[:n]
	uh.sendToSock(uh.localSocket, &buf, &uh.src)
}

func (uh *UDPRelayHandler) onRemoteWrite() {
	if len(uh.flow.DataWriteToRemote) != 0 {
		pkt := uh.flow.DataWriteToRemote
		uh.flow.DataWriteToRemote = make([]byte, 0)
		uh.sendToSock(uh.remoteSocket, &pkt, &uh.dest)
		return
	}
	uh.flow.Update(kStreamUp, kWaitStatusReading)
}

func (uh *UDPRelayHandler) SendToRemote(pkt *[]byte) {
	uh.sendToSock(uh.remoteSocket, pkt, &uh.dest)
}

func (uh *UDPRelayHandler) sendToSock(fd int, p *[]byte, sa *unix.SockaddrInet4) {
	if len(*p) == 0 || fd == INVALID_SOCKET {
		return
	}
	uncomplete := false
	if err := PacketSend(fd, p, sa); err != nil {
		if err == unix.EAGAIN {
			uncomplete = true
		} else {
			uh.Destroy()
			return
		}
	}
	if uncomplete {
		if fd == uh.localSocket {
			uh.flow.DataWriteToLocal = append(uh.flow.DataWriteToLocal, *p...)
			uh.flow.Update(kStreamDown, kWaitStatusWriting)
		} else if fd == uh.remoteSocket {
			uh.flow.DataWriteToRemote = append(uh.flow.DataWriteToRemote, *p...)
			uh.flow.Update(kStreamUp, kWaitStatusWriting)
		}
	} else {
		if fd == uh.localSocket {
			uh.flow.Update(kStreamDown, kWaitStatusReading)
		} else if fd == uh.remoteSocket {
			uh.flow.Update(kStreamUp, kWaitStatusReading)
		}
	}
}

func (uh *UDPRelayHandler) Destroy() {
	if uh.remoteSocket != INVALID_SOCKET {
		uh.eventLoop.UnRegister(uh.remoteSocket)
		CloseSocket(uh.remoteSocket)
		uh.remoteSocket = INVALID_SOCKET
	}
}
