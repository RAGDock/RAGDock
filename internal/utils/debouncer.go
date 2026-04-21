package utils

import (
	"sync"
	"time"
)

type TaskDebouncer struct {
	timers sync.Map
	delay  time.Duration
}

func NewTaskDebouncer(delay time.Duration) *TaskDebouncer {
	return &TaskDebouncer{delay: delay}
}

func (d *TaskDebouncer) Schedule(key string, task func()) {
	if v, ok := d.timers.Load(key); ok {
		v.(*time.Timer).Stop()
	}
	timer := time.AfterFunc(d.delay, func() {
		d.timers.Delete(key)
		task()
	})
	d.timers.Store(key, timer)
}
