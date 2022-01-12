package main

import (
	"fmt"
	"os"
	"os/signal"

	"net/http"
	_ "net/http/pprof"

	"github.com/rocinan/fdd"
)

func main() {
	fdd := new(fdd.Fdd)
	fdd.Start("0.0.0.0", 8888, "10.0.0.159", 8808)

	fmt.Println("start successfully")
	//test
	http.ListenAndServe("0.0.0.0:6060", nil)
	//wait exit
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	<-signalChan
}
