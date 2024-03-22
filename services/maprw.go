package services

import "sync"

type rw struct {
	is_w bool
	r    uint32
}

type MapRW struct {
	lock sync.Mutex
	rw_  map[string]*rw
}

func NewMapRW() *MapRW {
	return &MapRW{rw_: map[string]*rw{}}
}

func (s *MapRW) TryRLock(path string) bool {
	s.lock.Lock()
	defer s.lock.Unlock()

	lock, ok := s.rw_[path]
	if ok && lock.is_w {
		return false
	}

	if ok {
		lock.r += 1
	} else {
		s.rw_[path] = &rw{r: 1}
	}

	return true
}

func (s *MapRW) RUnlock(path string) {
	s.lock.Lock()
	defer s.lock.Unlock()

	lock := s.rw_[path]
	lock.r -= 1

	if lock.r == 0 {
		delete(s.rw_, path)
	}
}

func (s *MapRW) TryLock(path string) bool {
	s.lock.Lock()
	defer s.lock.Unlock()

	_, ok := s.rw_[path]
	if ok {
		return false
	}

	s.rw_[path] = &rw{is_w: true}
	return true
}

func (s *MapRW) Unlock(path string) {
	s.lock.Lock()
	defer s.lock.Unlock()

	delete(s.rw_, path)
}
