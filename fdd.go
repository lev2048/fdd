package fdd

import (
	"errors"
	"os"

	nested "github.com/antonfisher/nested-logrus-formatter"
	"github.com/rocinan/fdd/poller"
	"github.com/sirupsen/logrus"
)

var log = logrus.New()

func init() {
	log.SetFormatter(&nested.Formatter{
		HideKeys:    true,
		FieldsOrder: []string{"component", "category"},
	})
	log.SetOutput(os.Stdout)
}

type Fdd struct {
	tcpServer *TCPRelay
	udpServer *UDPRelay
	eventLoop *poller.EventLoop
}

func (f *Fdd) Start(cfg *Config) (err error) {
	f.eventLoop, err = poller.Create()
	if err != nil {
		return err
	}
	f.tcpServer, err = NewTCPRelay(cfg)
	if err != nil {
		return errors.New("start TcpServer err: " + err.Error())
	}
	f.udpServer, err = NewUDPRelay(cfg)
	if err != nil {
		return errors.New("start TcpServer err: " + err.Error())
	}
	f.tcpServer.AddToLoop(f.eventLoop)
	f.udpServer.AddToLoop(f.eventLoop)
	go f.eventLoop.Run()
	return nil
}

func (f *Fdd) Stop() {
	log.Info("stop server ...")
	f.tcpServer.Close()
	f.udpServer.Close()
	if err := f.eventLoop.Close(); err != nil {
		log.Warn(err)
	}
	log.Info("[eventLoop] poller exit.")
	log.Info("stop server done.")
}
