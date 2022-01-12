package fdd

import (
	"net"

	"golang.org/x/sys/unix"
)

//socket常用操作方法封装 //todo ipv6 support
//CreateTcpListenSocket 根据地址端口创建tcp监听socket
func CreateTcpListenSocket(addr string, port int) (int, error) {
	if fd, err := unix.Socket(unix.AF_INET, unix.SOCK_STREAM, unix.IPPROTO_TCP); err != nil {
		return 0, err
	} else {
		SetNoBlock(fd)
		SetReUseAddr(fd)
		socketAddr := unix.SockaddrInet4{Port: port}
		copy(socketAddr.Addr[:], net.ParseIP(addr).To4())
		if err := unix.Bind(fd, &socketAddr); err != nil {
			return 0, err
		}
		if err := unix.Listen(fd, 128); err != nil {
			return 0, err
		}
		return fd, nil
	}
}

//AcceptTcpConn 接受tcp链接返回链接socketFD
func AcceptTcpConn(fd int) (int, error) {
	if fd, _, err := unix.Accept(fd); err != nil {
		return 0, err
	} else {
		return fd, nil
	}
}

//CreateRemoteSocket 创建tcp连接 socketFD
func CreateRemoteSocket(remoteAddr string, remotePort int) (int, error) {
	if fd, err := unix.Socket(unix.AF_INET, unix.SOCK_STREAM, unix.IPPROTO_TCP); err != nil {
		return 0, err
	} else {
		defer SetNoBlock(fd)
		socketAddr := unix.SockaddrInet4{Port: remotePort}
		copy(socketAddr.Addr[:], net.ParseIP(remoteAddr).To4())
		if err = unix.Connect(fd, &socketAddr); err != nil {
			return 0, err
		}
		return fd, nil
	}
}

func BufferSend(fd int, buffer *[]byte) (int, error) {
	return unix.Write(fd, *buffer)
}

func BufferRecv(fd int, buffer *[]byte) (int, error) {
	return unix.Read(fd, *buffer)
}

func SetNoBlock(fd int) error {
	return unix.SetNonblock(fd, true)
}

func SetReUseAddr(fd int) error {
	return unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
}

func CloseSocket(fd int) error {
	return unix.Close(fd)
}
