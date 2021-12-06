package poller

import (
	"testing"
	"time"
)

func TestPoller_Close(t *testing.T) {
	s, err := Create()
	if err != nil {
		t.Fatal(err)
	}
	go s.Run()
	time.Sleep(time.Second * 5)
	if err = s.Close(); err != nil {
		t.Fatal("poller should be closed")
	}
}
