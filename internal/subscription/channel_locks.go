package subscription

import "sync"

type channelLocks struct {
	mutex sync.Mutex
	locks map[string]*channelLock
}

type channelLock struct {
	mutex      sync.Mutex
	references int
}

func (l *channelLocks) lock(channelID string) func() {
	entry := l.acquire(channelID)
	entry.mutex.Lock()
	return func() {
		entry.mutex.Unlock()
		l.release(channelID, entry)
	}
}

func (l *channelLocks) acquire(channelID string) *channelLock {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if l.locks == nil {
		l.locks = make(map[string]*channelLock)
	}
	entry := l.locks[channelID]
	if entry == nil {
		entry = &channelLock{}
		l.locks[channelID] = entry
	}
	entry.references++
	return entry
}

func (l *channelLocks) release(channelID string, entry *channelLock) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	entry.references--
	if entry.references == 0 {
		delete(l.locks, channelID)
	}
}

func (s *Service) lockChannel(channelID string) func() {
	return s.locks.lock(channelID)
}
