package main

type Semaphore struct {
	lock chan bool
}

func NewSemaphore(size int) Semaphore {
	var S Semaphore
	S.lock = make(chan bool, size)
	return S
}

func (s *Semaphore) Lock() {
	debugOut.Println("Locking...")
	s.lock <- true
	debugOut.Println("Proceeding")
}

func (s *Semaphore) Unlock() {
	<-s.lock
}
