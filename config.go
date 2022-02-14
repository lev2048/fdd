package fdd

type Config struct {
	ListenPort int
	RemotePort int
	UdpTimeOut int
	HandlerCap int
	ListenAddr string
	RemoteAddr string
}

func NewConfig(la, ra string, lp, rp, hcp, timeout int) Config {
	return Config{
		ListenAddr: la,
		RemoteAddr: ra,
		ListenPort: lp,
		RemotePort: rp,
		UdpTimeOut: timeout,
		HandlerCap: 1024,
	}
}
