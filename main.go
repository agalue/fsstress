package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

var workers, min, max = 2, 1, 100
var total_operations, total_errors = 0, 0

type Result struct {
	Size          int
	BytesRead     int
	BytesWrote    int
	ReadError     bool
	WriteError    bool
	ReadDuration  time.Duration
	WriteDuration time.Duration
}

func main() {
	wd, err := os.Getwd()
	if err != nil {
		fmt.Printf("Cannot get current directory: %s", err.Error())
		os.Exit(1)
	}
	flag.StringVar(&wd, "path", wd, "Destination Directory")
	flag.IntVar(&min, "min", min, "Minimum file size in Megabytes")
	flag.IntVar(&max, "max", max, "Maximum file size in Megabytes")
	flag.IntVar(&workers, "workers", workers, "Number of workers")

	flag.Parse()

	size := getAvailDiskSpace(wd)
	slog.Info("Available Disk Space", slog.String("path", wd), slog.String("free", byteCountIEC(size)))
	slog.Info("Starting data generation")

	ctx, cancel := context.WithCancel(context.Background())

	results := make(chan Result, 100)
	go processResults(results)

	wg := &sync.WaitGroup{}
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go startWorker(ctx, wg, i, wd, results)
	}

	var gracefulStop = make(chan os.Signal, 1)
	signal.Notify(gracefulStop, syscall.SIGTERM, syscall.SIGINT)
	<-gracefulStop
	slog.Info("Shutting down")
	cancel()
	wg.Wait()
	close(results)
	slog.Info("Good bye", slog.Int("total", total_operations), slog.Int("errors", total_errors))
}

func processResults(results chan Result) {
	for results := range results {
		total_operations++
		if results.ReadError {
			total_errors++
		}
		if results.WriteError {
			total_errors++
		}
	}
}

func writeFile(fileName string, size int) (int, error) {
	total := 0
	if fo, err := os.Create(fileName); err == nil {
		for total <= size {
			if out, err := fo.Write([]byte(strings.Repeat("0", 1024))); err == nil {
				total += out
			} else {
				return total, err
			}
		}
		return total, fo.Close()
	} else {
		return total, err
	}
}

func readFile(fileName string) (int, error) {
	total := 0
	if fi, err := os.Open(fileName); err == nil {
		buf := make([]byte, 1024)
		for {
			in, err := fi.Read(buf)
			if err != nil {
				if err != io.EOF {
					return total, err
				}
				break
			}
			total += in
		}
		return total, fi.Close()
	} else {
		return total, err
	}
}

func startWorker(ctx context.Context, wg *sync.WaitGroup, id int, path string, results chan Result) {
	defer wg.Done()
	slog.Info("Starting worker", slog.Int("id", id))
	fileName := fmt.Sprintf("%s/test_file_%d", path, id)
	for {
		select {
		case <-ctx.Done():
			slog.Info("Stopping worker", slog.Int("id", id))
			return
		default:
			size := min + rand.Intn(max-min)*1024*1024

			slog.Info("Start write", slog.Int("id", id), slog.String("size", byteCountIEC(uint64(size))))
			start := time.Now()
			wout, werr := writeFile(fileName, size)
			wdur := time.Since(start)
			slog.Info("End write", slog.Int("id", id), slog.String("total", byteCountIEC(uint64(wout))), slog.Duration("duration", wdur), slog.Bool("error", werr != nil))

			slog.Info("Start read", slog.Int("id", id), slog.String("size", byteCountIEC(uint64(size))))
			start = time.Now()
			rout, rerr := readFile(fileName)
			rdur := time.Since(start)
			slog.Info("End read", slog.Int("id", id), slog.String("total", byteCountIEC(uint64(rout))), slog.Duration("duration", rdur), slog.Bool("error", rerr != nil))

			results <- Result{
				Size:          size,
				BytesRead:     rout,
				BytesWrote:    wout,
				ReadDuration:  rdur,
				WriteDuration: wdur,
				ReadError:     rerr != nil,
				WriteError:    werr != nil,
			}
			slog.Info("Finish operation", slog.Int("id", id), slog.Bool("success", wout == rout))

			time.Sleep(time.Duration(200+rand.Intn(1000)) * time.Millisecond)
		}
	}
}

func getAvailDiskSpace(wd string) uint64 {
	var stat unix.Statfs_t
	unix.Statfs(wd, &stat)
	return stat.Bavail * uint64(stat.Bsize)
}

func byteCountIEC(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%dB", b)
	}
	div, exp := unit, 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f%ci", float64(b)/float64(div), "KMGTPE"[exp])
}
