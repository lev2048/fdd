package main

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/rocinan/fdd"
)

func main() {
	fdd := new(fdd.Fdd)
	fdd.Start("127.0.0.1", 9000, "127.0.0.1", 9002)
	fmt.Println("start successfully")
	fmt.Println("PID: ", os.Getpid())
	//wait exit
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	<-signalChan
}
