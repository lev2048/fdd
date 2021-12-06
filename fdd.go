package fdd

const (
	kStreamUp int = iota
	kStreamDown
)

const (
	kWaitStatusInit = iota
	kWaitStatusReading
	kWaitStatusWriting
	kWaitStatusReadWriting
)

type Fdd struct{}

func (f *Fdd) Start() {

}

func (f *Fdd) Stop() {}
