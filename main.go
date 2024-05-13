package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

var (
	workers   = 2
	min       = 5
	max       = 100
	chunkSize = 262144
	totals    = Totals{}
)

type Result struct {
	Size          int
	ReadBytes     int
	WriteBytes    int
	ReadError     bool
	WriteError    bool
	ReadDuration  time.Duration
	WriteDuration time.Duration
}

type Totals struct {
	Operations       int
	Bytes            int
	ReadBytes        int
	WriteBytes       int
	ReadErrors       int
	WriteErrors      int
	ReadMaxDuration  time.Duration
	WriteMaxDuration time.Duration
}

func (t *Totals) Update(r Result) {
	t.Operations++
	if r.ReadError {
		t.ReadErrors++
	}
	if r.WriteError {
		t.WriteErrors++
	}
	t.Bytes += r.Size
	t.ReadBytes += r.ReadBytes
	t.WriteBytes += r.WriteBytes
	if t.WriteMaxDuration < r.WriteDuration {
		t.WriteMaxDuration = r.WriteDuration
	}
	if t.ReadMaxDuration < r.ReadDuration {
		t.ReadMaxDuration = r.ReadDuration
	}
}

func (t *Totals) String() string {
	buffer := bytes.Buffer{}
	buffer.WriteRune('{')
	buffer.WriteString(fmt.Sprintf("operations: %d, ", t.Operations))
	buffer.WriteString(fmt.Sprintf("expectedBytes: %s, ", byteCountIEC(uint64(t.Bytes))))
	buffer.WriteString(fmt.Sprintf("readBytes: %s, ", byteCountIEC(uint64(t.ReadBytes))))
	buffer.WriteString(fmt.Sprintf("writeBytes: %s, ", byteCountIEC(uint64(t.WriteBytes))))
	buffer.WriteString(fmt.Sprintf("readErrors: %d, ", t.ReadErrors))
	buffer.WriteString(fmt.Sprintf("writeErrors: %d, ", t.WriteErrors))
	buffer.WriteString(fmt.Sprintf("maxReadDuration: %s, ", t.ReadMaxDuration))
	buffer.WriteString(fmt.Sprintf("maxWriteDuration: %s", t.WriteMaxDuration))
	buffer.WriteRune('}')
	return buffer.String()
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
	flag.IntVar(&chunkSize, "cs", chunkSize, "Chunk size (read/write)")

	flag.Parse()

	size := getAvailDiskSpace(wd)
	slog.Info("Available Disk Space", slog.String("path", wd), slog.String("free", byteCountIEC(size)))

	potentialMax := uint64(workers * max * 1024 * 1024)
	if potentialMax > size {
		slog.Error("Cannot execute test due to disk space", slog.String("expected", byteCountIEC(potentialMax)))
		os.Exit(1)
	}

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
	if err := cleanUp(wd); err != nil {
		slog.Error("Cannot cleanup", slog.String("error", err.Error()))
	}
	slog.Info("Good bye", slog.String("results", totals.String()))
}

func processResults(results chan Result) {
	for result := range results {
		totals.Update(result)
	}
}

func writeFile(fileName string, size int) (int, error) {
	total := 0
	chunk := bytes.Repeat([]byte{48}, chunkSize)
	if fo, err := os.Create(fileName); err == nil {
		for total <= size {
			if out, err := fo.Write(chunk); err == nil {
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
		buf := make([]byte, chunkSize)
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
	fileName := filepath.Join(path, fmt.Sprintf("test_file_%d", id))
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
			if werr != nil {
				slog.Error(werr.Error())
			}

			slog.Info("Start read", slog.Int("id", id), slog.String("size", byteCountIEC(uint64(size))))
			start = time.Now()
			rout, rerr := readFile(fileName)
			rdur := time.Since(start)
			slog.Info("End read", slog.Int("id", id), slog.String("total", byteCountIEC(uint64(rout))), slog.Duration("duration", rdur), slog.Bool("error", rerr != nil))
			if rerr != nil {
				slog.Error(rerr.Error())
			}

			results <- Result{
				Size:          size,
				ReadBytes:     rout,
				WriteBytes:    wout,
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

func cleanUp(path string) error {
	files, err := filepath.Glob(filepath.Join(path, "test_file_*"))
	if err != nil {
		return err
	}
	for _, f := range files {
		if err := os.Remove(f); err != nil {
			return err
		}
	}
	return nil
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
