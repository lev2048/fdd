package fdd

import (
	"github.com/rocinan/fdd/poller"
	"golang.org/x/sys/unix"
)

type UDPRelay struct {
	localSocket int

	cfg           *Config
	eventLoop     *poller.EventLoop
	remoteSocket  map[int]unix.SockaddrInet4
	remoteSrcAddr map[string]int
}

func NewUDPRelay(cfg *Config) (*UDPRelay, error) {
	if fd, err := CreateUdpListenSocket(cfg.ListenAddr, cfg.ListenPort); err != nil {
		return nil, err
	} else {
		return &UDPRelay{
			cfg:           cfg,
			localSocket:   fd,
			remoteSocket:  make(map[int]unix.SockaddrInet4, cfg.HandlerCap),
			remoteSrcAddr: make(map[string]int, cfg.HandlerCap),
		}, nil
	}
}

func (ur *UDPRelay) AddToLoop(ep *poller.EventLoop) error {
	ur.eventLoop = ep
	return ur.eventLoop.Register(ur.localSocket, kPollIn|kPollErr, ur)
}

func (ur *UDPRelay) HandleEvent(s, ev int) {
	if s == ur.localSocket {
		if Judge(ev & kPollErr) {
			log.Warn("[UDPRelay] client socket event err: ", s, ev)
			return
		}
		ur.handleClient()
	} else if s != INVALID_SOCKET {
		if _, ok := ur.remoteSocket[s]; ok {
			if Judge(ev & kPollErr) {
				log.Warn("[UDPRealy] socket event err: ", s, ev)
				ur.eventLoop.UnRegister(s)
				return
			}
			ur.handleRemote(s)
		}
	}
}

func (ur *UDPRelay) handleClient() {
	buf := make([]byte, kBuffSize)
	n, sa, err := PacketRecv(ur.localSocket, &buf)
	if ok := CheckError("[UDPRelay] on local read err: ", err); !ok {
		return
	}
	buf = buf[:n]
	remoteSocket := 0
	if s, ok := ur.remoteSrcAddr[MD5Addr(sa.Addr[:], sa.Port)]; !ok {
		log.Info("[UDPRelay] new client : ", Addr2Str(sa))
		if ns, err := CreateUdpRemoteSocket(); err != nil {
			log.Error("[UDPRelay] create remote socket err: ", err)
			return
		} else {
			remoteSocket = ns
			ur.remoteSocket[ns] = *sa
			ur.remoteSrcAddr[MD5Addr(sa.Addr[:], sa.Port)] = ns
			ur.eventLoop.Register(ns, kPollIn, ur)
		}
	} else {
		remoteSocket = s
	}
	if err := PacketSend(remoteSocket, &buf, SockAddrParse(ur.cfg.RemoteAddr, ur.cfg.RemotePort)); err != nil {
		if err == unix.EAGAIN {
			log.Warn("[UDPRelay] send pkg to remote err: EAGAIN")
		} else {
			log.Error("[udp_relay] send pkg to remote err: ", err)
		}
	}

}

func (ur *UDPRelay) handleRemote(s int) {
	buf, src := make([]byte, kBuffSize), ur.remoteSocket[s]
	n, _, err := PacketRecv(s, &buf)
	if ok := CheckError("[UDPRelay] on remote read err: ", err); !ok {
		return
	}
	buf = buf[:n]
	CheckError("[UDPRelay] on send pkg to local err: ", PacketSend(ur.localSocket, &buf, &src))
}

func (ur *UDPRelay) Close() {
	for s := range ur.remoteSocket {
		ur.eventLoop.UnRegister(s)
		if s != INVALID_SOCKET {
			CloseSocket(s)
		}
		delete(ur.remoteSocket, s)
	}
	ur.eventLoop.UnRegister(ur.localSocket)
	if ur.localSocket != INVALID_SOCKET {
		CloseSocket(ur.localSocket)
	}
	log.Info("[UDPRelay] udp relay service exit.")
}
