package server

import (
	"sync"
	"time"
)

type RateLimiter struct {
	loginAttempts   map[string][]time.Time
	messageAttempts map[string][]time.Time
	mutex           sync.RWMutex
	maxAttempts     int
	windowSize      time.Duration
	messageLimit    int
	messageWindow   time.Duration
}

func NewRateLimiter(maxAttempts int, windowSize time.Duration, messageLimit int, messageWindow time.Duration) *RateLimiter {
	return &RateLimiter{
		loginAttempts:   make(map[string][]time.Time),
		messageAttempts: make(map[string][]time.Time),
		maxAttempts:     maxAttempts,
		windowSize:      windowSize,
		messageLimit:    messageLimit,
		messageWindow:   messageWindow,
	}
}

func (rl *RateLimiter) AllowLogin(username string) bool {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	now := time.Now()
	windowStart := now.Add(-rl.windowSize)

	// Очистка старых попыток
	var recent []time.Time
	for _, attempt := range rl.loginAttempts[username] {
		if attempt.After(windowStart) {
			recent = append(recent, attempt)
		}
	}

	// Проверка лимита
	if len(recent) >= rl.maxAttempts {
		return false
	}

	recent = append(recent, now)
	rl.loginAttempts[username] = recent
	return true
}

func (rl *RateLimiter) AllowMessage(username string) bool {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	now := time.Now()
	windowStart := now.Add(-rl.messageWindow)

	// Очистка старых сообщений
	var recent []time.Time
	for _, attempt := range rl.messageAttempts[username] {
		if attempt.After(windowStart) {
			recent = append(recent, attempt)
		}
	}

	// Проверка лимита сообщений
	if len(recent) >= rl.messageLimit {
		return false
	}

	recent = append(recent, now)
	rl.messageAttempts[username] = recent
	return true
}

func (rl *RateLimiter) Cleanup() {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	now := time.Now()

	// Очистка устаревших записей
	for username, attempts := range rl.loginAttempts {
		var recent []time.Time
		for _, attempt := range attempts {
			if attempt.After(now.Add(-rl.windowSize)) {
				recent = append(recent, attempt)
			}
		}
		if len(recent) == 0 {
			delete(rl.loginAttempts, username)
		} else {
			rl.loginAttempts[username] = recent
		}
	}

	for username, attempts := range rl.messageAttempts {
		var recent []time.Time
		for _, attempt := range attempts {
			if attempt.After(now.Add(-rl.messageWindow)) {
				recent = append(recent, attempt)
			}
		}
		if len(recent) == 0 {
			delete(rl.messageAttempts, username)
		} else {
			rl.messageAttempts[username] = recent
		}
	}
}
