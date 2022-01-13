package pool

type Job func()

type Pool struct {
	WorkerPool chan chan Job
	JobChannel chan Job
	quit       chan bool
}

func NewPool(pool chan chan Job) *Pool {
	return &Pool{
		WorkerPool: pool,
		JobChannel: make(chan Job),
		quit:       make(chan bool),
	}
}

func (w *Pool) Start() {
	go func() {
		for {
			w.WorkerPool <- w.JobChannel
			select {
			case job := <-w.JobChannel:
				job()
			case <-w.quit:
				return
			}
		}
	}()
}

func (w *Pool) Stop() {
	w.quit <- true
}

type Dispatcher struct {
	WorkerCap  int
	JobQueue   chan Job
	WorkerPool chan chan Job
}

func NewDispatcher(maxWorkers int, maxQueue int) *Dispatcher {
	return &Dispatcher{maxWorkers, make(chan Job, maxQueue), make(chan chan Job, maxWorkers)}
}

func (d *Dispatcher) Run() {
	for i := 0; i <= d.WorkerCap; i++ {
		worker := NewPool(d.WorkerPool)
		worker.Start()
	}
	go d.dispatch()
}

func (d *Dispatcher) dispatch() {
	for job := range d.JobQueue {
		jobChannel := <-d.WorkerPool
		jobChannel <- job
	}
}
