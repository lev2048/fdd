package main

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/rocinan/fdd"
)

var (
	la string
	lp int
	ra string
	rp int
)

func init() {
	la, lp = "127.0.0.1", 9003
	ra, rp = "127.0.0.1", 9000
}

func main() {
	fdd := new(fdd.Fdd)
	fdd.Start(la, lp, ra, rp)
	fmt.Println("Start Service Successfully")
	fmt.Println("PID: ", os.Getpid())
	fmt.Println("DIR: " + fmt.Sprintf("%s:%d => %s:%d", la, lp, ra, rp))
	//wait exit
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	<-signalChan
	fmt.Println("")
	fdd.Stop()
}
