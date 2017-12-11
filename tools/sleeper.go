package tools

import "time"

type RealSleeper struct{}

func (c RealSleeper) Sleep(t time.Duration) { time.Sleep(t) }
