package fdd

import (
	"os"

	nested "github.com/antonfisher/nested-logrus-formatter"
	"github.com/rocinan/fdd/poller"
	"github.com/sirupsen/logrus"
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
	kUpStreamBufSize   = 16384
	kDownStreamBufSize = 32768
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

var log = logrus.New()

func init() {
	log.SetFormatter(&nested.Formatter{
		HideKeys:    true,
		FieldsOrder: []string{"component", "category"},
	})
	log.SetOutput(os.Stdout)
}

type Fdd struct{}

func (f *Fdd) Start(la string, lp int, ra string, rp int) {
	eventLoop, err := poller.Create()
	if err != nil {
		log.Error("start EventLoop err: ", err)
		return
	}
	tcpServer, err := NewTCPRelay(la, lp, ra, rp, 1024)
	if err != nil {
		log.Error("start TcpServer err: ", err)
		return
	}
	tcpServer.AddToLoop(eventLoop)
	go eventLoop.Run()
}

func (f *Fdd) Stop() {}
