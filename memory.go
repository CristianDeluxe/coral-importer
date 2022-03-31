package main

import (
	"context"
	"runtime"
	"time"

	"github.com/sirupsen/logrus"
)

func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}

func LogMemoryStats(run int64) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	logrus.WithFields(logrus.Fields{
		"run":        run,
		"alloc":      bToMb(m.Alloc),
		"totalAlloc": bToMb(m.TotalAlloc),
		"sys":        bToMb(m.Sys),
		"numGC":      m.NumGC,
	}).Debug("memory stats")
}

func StartLoggingMemoryStats(ctx context.Context, frequency time.Duration) {
	run := time.Now().Unix()

	LogMemoryStats(run)

	for {
		select {
		case <-time.After(frequency):
			LogMemoryStats(run)
		case <-ctx.Done():
			LogMemoryStats(run)
			return
		}
	}
}
