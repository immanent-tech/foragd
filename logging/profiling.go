// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package logging

// #nosec G108
//revive:disable:blank-imports
import (
	"fmt"
	"log/slog"
	"math"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"
	"runtime/pprof"
	"runtime/trace"
	"strconv"
)

const (
	bytesFactor float64 = 1024.0
)

type ProfileFlags map[string]string

func StartProfiling(logger *slog.Logger, flags ProfileFlags) error {
	for flagKey, flagVal := range flags {
		switch flagKey {
		case "webui":
			if err := startWebProfiler(logger, flagVal); err != nil {
				return fmt.Errorf("could not start web profiler: %w", err)
			}
		case "heapprofile":
			logger.Debug("Heap profiling enabled.",
				slog.String("file", flagVal))
		case "cpuprofile":
			if err := startCPUProfiler(logger, flagVal); err != nil {
				return fmt.Errorf("could not start CPU profiling: %w", err)
			}
		case "traceprofile":
			if err := startTraceProfiling(logger, flagVal); err != nil {
				return fmt.Errorf("could not start trace profiling: %w", err)
			}
		default:
			return fmt.Errorf("unknown argument for profiling: %s=%s", flagKey, flagVal)
		}
	}

	logger.Debug("Profiling started.")

	return nil
}

func StopProfiling(logger *slog.Logger, flags ProfileFlags) error {
	for flagKey, flagVal := range flags {
		switch flagKey {
		case "heapprofile":
			heapFile, err := os.Create(flagVal)
			if err != nil {
				return fmt.Errorf("cannot create heap profile file: %w", err)
			}

			var ms runtime.MemStats

			runtime.ReadMemStats(&ms)
			printMemStats(logger, &ms)

			if err = pprof.WriteHeapProfile(heapFile); err != nil {
				return fmt.Errorf("cannot write to heap profile file: %w", err)
			}

			if err = heapFile.Close(); err != nil {
				return fmt.Errorf("cannot close heap profile: %w", err)
			}

			logger.Debug("Wrote heap profile.", slog.String("file", flagVal))
		case "cpuprofile":
			pprof.StopCPUProfile()
		case "traceprofile":
			trace.Stop()
		}
	}

	logger.Debug("Profiling stopped.")

	return nil
}

// printMemStats and formatMemory functions are taken from golang-ci source.
func printMemStats(logger *slog.Logger, stats *runtime.MemStats) {
	logger.Debug(
		"Memory stats",
		"alloc",
		prettyByteSize(stats.Alloc),
		"total_alloc",
		prettyByteSize(stats.TotalAlloc),
		"sys",
		prettyByteSize(stats.Sys),
		"heap_alloc",
		prettyByteSize(stats.HeapAlloc),
		"heap_sys",
		prettyByteSize(stats.HeapSys),
		"heap_idle",
		prettyByteSize(stats.HeapIdle),
		"heap_released",
		prettyByteSize(stats.HeapReleased),
		"heap_in_use",
		prettyByteSize(stats.HeapInuse),
		"stack_in_use",
		prettyByteSize(stats.StackInuse),
		"stack_sys",
		prettyByteSize(stats.StackSys),
		"mspan_sys",
		prettyByteSize(stats.MSpanSys),
		"mcache_sys",
		prettyByteSize(stats.MCacheSys),
		"buck_hash_sys",
		prettyByteSize(stats.BuckHashSys),
		"gc_sys",
		prettyByteSize(stats.GCSys),
		"other_sys",
		prettyByteSize(stats.OtherSys),
		"mallocs_n",
		stats.Mallocs,
		"frees_n",
		stats.Frees,
		"heap_objects",
		stats.HeapObjects,
		"gc_cpu_fraction",
		stats.GCCPUFraction,
	)
}

func prettyByteSize(b uint64) string {
	value := float64(b)
	for _, unit := range []string{"", "Ki", "Mi", "Gi", "Ti", "Pi", "Ei", "Zi"} {
		if math.Abs(value) < bytesFactor {
			return fmt.Sprintf("%3.1f%sB", value, unit)
		}
		value /= bytesFactor
	}
	return fmt.Sprintf("%.1fYiB", value)
}

func startWebProfiler(logger *slog.Logger, enable string) error {
	webui, err := strconv.ParseBool(enable)
	if err != nil {
		return fmt.Errorf("could not interpret webui value: %w", err)
	}

	if webui {
		go func() {
			for i := 6060; i < 6070; i++ {
				logger.Debug("Starting profiler web interface.",
					slog.String("address", "http://localhost:"+strconv.Itoa(i)))

				if err := http.ListenAndServe("localhost:"+strconv.Itoa(i), nil); err != nil { // #nosec G114
					logger.Warn("Could not start profiler web interface. Trying different port.")
				}
			}
		}()
	}

	return nil
}

func startCPUProfiler(logger *slog.Logger, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("cannot create CPU profile file: %w", err)
	}

	if err := pprof.StartCPUProfile(f); err != nil {
		return fmt.Errorf("could not start CPU profiling: %w", err)
	}

	logger.Debug("CPU profiling enabled.",
		slog.String("file", path))

	return nil
}

func startTraceProfiling(logger *slog.Logger, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("cannot create trace profile file: %w", err)
	}

	if err = trace.Start(f); err != nil {
		return fmt.Errorf("could not start trace profiling: %w", err)
	}

	logger.Debug("Trace profiling enabled.",
		slog.String("file", path))

	return nil
}
