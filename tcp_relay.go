package fdd

type TCPRelay struct {
}

func NewTCPRelay() *TCPRelay {
	return &TCPRelay{}
}

func (tr *TCPRelay) Init() bool {
	return true
}

func (tr *TCPRelay) AddToLoop() bool {
	return true
}

func (tr *TCPRelay) AddHandler() {}

func (tr *TCPRelay) RemoveHandler() {}

func (tr *TCPRelay) HandleEvent() {}

func (tr *TCPRelay) Close() {}

type TCPRelayHandler struct {
}

func (th *TCPRelayHandler) HandleEvent(fd, event int) {}

func (th *TCPRelayHandler) IsDestroyed() {}

func (th *TCPRelayHandler) updateStream(stream, status int) {}

func (th *TCPRelayHandler) writeToSock(data []byte, sfd int) {}

func (th *TCPRelayHandler) createRemoteSocket(ip string, port int) {}

func (th *TCPRelayHandler) onLocalRead() {}

func (th *TCPRelayHandler) onRemoteRead() {}

func (th *TCPRelayHandler) onLocalWrite() {}

func (th *TCPRelayHandler) onRemoteWrite() {}

func (th *TCPRelayHandler) onLocalError() {}

func (th *TCPRelayHandler) onRemoteError() {}

func (th *TCPRelayHandler) destroy() {}
