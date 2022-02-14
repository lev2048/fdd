package fdd

import (
	"fmt"
	"os"

	"github.com/rocinan/fdd/poller"
	"golang.org/x/sys/unix"
)

type UDPRelay struct {
	localSocket int

	cfg           Config
	eventLoop     *poller.EventLoop
	socketHandler map[string]*UDPRelayHandler
}

func NewUDPRelay(cfg Config) (*UDPRelay, error) {
	if fd, err := CreateUdpListenSocket(cfg.ListenAddr, cfg.ListenPort); err != nil {
		return nil, err
	} else {
		return &UDPRelay{
			cfg:           cfg,
			localSocket:   fd,
			socketHandler: make(map[string]*UDPRelayHandler, cfg.HandlerCap),
		}, nil
	}
}

func (ur *UDPRelay) AddToLoop(eventLoop *poller.EventLoop) bool {
	if ur.eventLoop != nil {
		log.Warn("[udp]already add to loop")
		return false
	}
	ur.eventLoop = eventLoop
	if err := ur.eventLoop.Register(ur.localSocket, kPollIn|kPollErr, ur); err != nil {
		log.Warn("[udp]failed to register poller: ", err)
		return false
	}
	return true
}

func (ur *UDPRelay) HandleEvent(fd, event int) {
	if fd == ur.localSocket {
		if event == kPollErr {
			log.Error("[udp]KPollErr")
			return
		}
		if err := ur.handleClient(fd); err != nil {
			log.Error("[udp] handle client error:", err)
		}
	} else {
		log.Warn("[udp] warn socket fd recived: ", fd, event)
	}
}

func (ur *UDPRelay) handleClient(fd int) error {
	buf := make([]byte, os.Getpagesize())
	n, sa, err := PacketRecv(fd, &buf)
	if err != nil {
		return err
	}
	buf = buf[:n]
	var handler *UDPRelayHandler
	if uh, ok := ur.socketHandler[MD5Addr(sa.Addr[:], sa.Port)]; !ok {
		if serverSocket, err := CreateUdpRemoteSocket(ur.cfg.RemoteAddr, ur.cfg.RemotePort); err != nil {
			return err
		} else {
			handler = NewUDPRealyHandler(ur.localSocket, serverSocket, unix.SockaddrInet4{
				Addr: sa.Addr,
				Port: sa.Port,
			}, SockAddrParse(ur.cfg.RemoteAddr, ur.cfg.RemotePort))
			ur.socketHandler[MD5Addr(sa.Addr[:], sa.Port)] = handler
			err = ur.eventLoop.Register(handler.remoteSocket, kPollIn|kPollErr, handler)
			fmt.Println(handler.remoteSocket, err)
		}
	} else {
		handler = uh
	}
	return handler.SendToRemote(buf)
}

func (ur *UDPRelay) Destroy() {}

type UDPRelayHandler struct {
	src  unix.SockaddrInet4
	dest unix.SockaddrInet4

	localSocket  int
	remoteSocket int
}

func NewUDPRealyHandler(ls, rs int, src, dest unix.SockaddrInet4) *UDPRelayHandler {
	return &UDPRelayHandler{
		src:          src,
		dest:         dest,
		localSocket:  ls,
		remoteSocket: rs,
	}
}

func (uh *UDPRelayHandler) HandleEvent(fd, event int) {
	if fd == uh.remoteSocket {
		if event == kPollErr {
			log.Error("[udp]KPollErr")
			return
		}
		if err := uh.handleServer(fd); err != nil {
			log.Error("[udp] handle server error:", err)
		}
	} else {
		log.Warn("[udp] warn socket fd recived: ", fd, event)
	}
}

func (uh *UDPRelayHandler) handleServer(fd int) error {
	buf := make([]byte, os.Getpagesize())
	n, _, err := PacketRecv(fd, &buf)
	if err != nil {
		return err
	}
	buf = buf[:n]
	return PacketSend(uh.localSocket, &buf, &uh.src)
}

func (uh *UDPRelayHandler) SendToRemote(pkt []byte) error {
	return PacketSend(uh.remoteSocket, &pkt, &uh.dest)
}

func (uh *UDPRelayHandler) Destroy() {}
