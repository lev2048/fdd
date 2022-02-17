package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"time"

	nested "github.com/antonfisher/nested-logrus-formatter"
	"github.com/rocinan/fdd"
	"github.com/sirupsen/logrus"
)

var (
	log *logrus.Logger
	la  *string
	lp  *int
	ra  *string
	rp  *int
)

func init() {
	log = logrus.New()
	log.SetFormatter(&nested.Formatter{
		HideKeys:    true,
		FieldsOrder: []string{"component", "category"},
	})
	log.SetOutput(os.Stdout)
	la = flag.String("la", "0.0.0.0", "listen addr")
	ra = flag.String("ra", "", "target address ip or domain")
	lp = flag.Int("lp", 9001, "listen port")
	rp = flag.Int("rp", 0, "target port")
}

func main() {
	flag.Parse()
	if *ra == "" || *rp == 0 {
		log.Error("target info is required")
		os.Exit(-1)
	}
	cfg := &fdd.Config{
		ListenPort: *lp,
		RemotePort: *rp,
		ListenAddr: *la,
		RemoteAddr: *ra,
		UdpTimeOut: 50,
		HandlerCap: 2048,
	}
	rp := new(fdd.Fdd)
	CheckDomain(cfg)
	if err := rp.Start(cfg); err != nil {
		log.Error(err)
		os.Exit(-1)
	}
	log.Info("Start Service Successfully")
	log.Info("PID: ", os.Getpid())
	log.Info("DIR: " + fmt.Sprintf("%s:%d => %s:%d", cfg.ListenAddr, cfg.ListenPort, cfg.RemoteAddr, cfg.RemotePort))
	//wait exit
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	<-signalChan
	fmt.Println()
	rp.Stop()
}

func CheckDomain(cfg *fdd.Config) {
	if net.ParseIP(cfg.RemoteAddr) == nil {
		log.Info("domain detected")
		cfg.TargetDomain = cfg.RemoteAddr
		log.Info("start resolver domain...")
		if ip, err := fdd.GetDomainIp(cfg.TargetDomain); err != nil || ip == "" {
			log.Error("resolver domain err:", err, cfg.TargetDomain, ip)
			os.Exit(-1)
		} else {
			cfg.RemoteAddr = ip
			log.Info("resolver successful: " + cfg.TargetDomain + " => " + ip)
			ticker := time.NewTicker(time.Minute * 5)
			log.Info("start resolver loop(5 min)")
			defer ticker.Stop()
			go func() {
				for range ticker.C {
					if ip, err := fdd.GetDomainIp(cfg.TargetDomain); err != nil || ip == "" {
						log.Error("resolver domain err:", err, cfg.TargetDomain, ip)
						os.Exit(-1)
					} else {
						cfg.RemoteAddr = ip
						log.Info("resolver successful: " + cfg.TargetDomain + " => " + ip)
					}
				}
			}()
		}
	}
}
