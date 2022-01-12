package poller

const (
	kEpollSize        = 1024
	kMaxEpollSize     = 102400
	kTimeoutPrecision = 10
)

const (
	kPollIn   = 0x01
	kPollOut  = 0x04
	kPollErr  = 0x08
	kPollHup  = 0x10
	kPollNval = 0x20
	kPollNull = 0x00
)

type Event uint32

type ISockNotify interface {
	HandleEvent(fd, event int)
}
