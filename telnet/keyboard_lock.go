package telnet

import (
	"sync"
	"time"
)

const DefaultKeyboardLock = 5 * time.Second

type keyboardLock struct {
	control        sync.Mutex
	locks          map[string]time.Time
	nextExpiryTime time.Time

	timer  *time.Timer
	locked bool
	C      chan struct{}
}

func newKeyboardLock() *keyboardLock {
	lock := &keyboardLock{
		locks: make(map[string]time.Time),
		C:     make(chan struct{}),
	}

	timer := time.AfterFunc(0, func() {
		lock.control.Lock()
		defer lock.control.Unlock()

		lock.timerExpire()
	})
	timer.Stop()
	lock.timer = timer

	return lock
}

func (l *keyboardLock) timerExpire() {
	// The keyboard is now unlocked
	l.locked = false
	l.C <- struct{}{}
}

func (l *keyboardLock) newNextExpiry(expiry time.Time) {
	wasWaitingOnTimer := l.timer.Stop()

	if expiry.IsZero() || time.Now().After(expiry) {
		// We are expiring the timer

		if wasWaitingOnTimer {
			// The timer was not already expired
			l.timerExpire()
		}

		return
	}

	// We're setting an expiry- the timer may still have been live
	l.locked = true
	l.nextExpiryTime = expiry
	l.timer.Reset(time.Until(expiry))
}

func (l *keyboardLock) SetLock(lockName string, duration time.Duration) {
	expiry := time.Now().Add(duration)

	l.control.Lock()
	defer l.control.Unlock()

	l.locks[lockName] = expiry

	if expiry.After(l.nextExpiryTime) {
		l.newNextExpiry(expiry)
	}
}

func (l *keyboardLock) ClearLock(lockName string) {
	l.control.Lock()
	defer l.control.Unlock()

	oldExpiry, hasLock := l.locks[lockName]
	if !hasLock {
		return
	}

	delete(l.locks, lockName)

	if l.nextExpiryTime == oldExpiry {
		var newExpiry time.Time
		for _, expiry := range l.locks {
			if expiry.After(newExpiry) {
				newExpiry = expiry
			}
		}

		l.newNextExpiry(newExpiry)
	}
}

func (l *keyboardLock) HasActiveLock(lockName string) bool {
	l.control.Lock()
	defer l.control.Unlock()

	expiry, hasLock := l.locks[lockName]
	if !hasLock {
		return false
	}

	return expiry.After(time.Now())
}

func (l *keyboardLock) IsLocked() bool {
	l.control.Lock()
	defer l.control.Unlock()

	return l.locked
}
