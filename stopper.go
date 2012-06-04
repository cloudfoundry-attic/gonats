package nats

import (
	"sync"
)

type Stopper struct {
	sync.Once

	// Stop channel channel, stop acknowledgement channel
	scc   chan chan bool
	sackc chan bool
}

func (s *Stopper) Init() {
	s.Do(func() { s.scc = make(chan chan bool, 1) })
}

func (s *Stopper) Stop() {
	var sc chan bool

	s.Init()

	select {
	case sc = <-s.scc:
		// Trigger stop
		close(sc)

		// Wait for acknowledgement
		<-s.sackc

	default:
		return
	}
}

func (s *Stopper) MarkStart() chan bool {
	var sc chan bool

	s.Init()

	// Create stop acknowledgement channel
	// This doesn't need a lock because it can only be used after Stop() has
	// acquired the stop channel, which is not yet available at this point.
	s.sackc = make(chan bool)

	// Create stop channel
	sc = make(chan bool)
	s.scc <- sc

	return sc
}

func (s *Stopper) MarkStop() {
	var sc chan bool

	// Close stop channel if it is still available
	select {
	case sc = <-s.scc:
		close(sc)

	default:
	}

	// Acknowledge stop by closing the acknowledgement channel
	close(s.sackc)
}
