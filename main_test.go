package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestByteCountIEC(t *testing.T) {
	testCases := []struct {
		number   uint64
		expected string
	}{
		{2097152, "2.00Mi"},
		{3221225472, "3.00Gi"},
	}

	for _, tc := range testCases {
		out := byteCountIEC(tc.number)
		fmt.Printf("%d => %s\n", tc.number, tc.expected)
		if out != tc.expected {
			t.Fail()
		}
	}
}

func TestWriteAndReadFile(t *testing.T) {
	path, err := os.MkdirTemp("", "sample")
	if err != nil {
		t.FailNow()
	}
	defer os.RemoveAll(path)

	fileName := filepath.Join(path, "test_file_1")
	size := 1048576

	write_bytes, err := writeFile(fileName, size)
	if err != nil {
		t.FailNow()
	}
	if write_bytes < size {
		t.FailNow()
	}

	read_bytes, err := readFile(fileName)
	if err != nil {
		t.FailNow()
	}
	if write_bytes != read_bytes {
		t.FailNow()
	}
}

func TestWorkerFlow(t *testing.T) {
	path, err := os.MkdirTemp("", "sample")
	if err != nil {
		t.FailNow()
	}
	defer os.RemoveAll(path)

	wg := &sync.WaitGroup{}
	ctx, cancel := context.WithCancel(context.Background())
	results := make(chan Result)

	go startWorker(ctx, wg, 1, path, results)

	output := <-results
	cancel()
	wg.Wait()
	close(results)

	if output.ReadError || output.WriteError || output.ReadBytes != output.WriteBytes {
		t.Fail()
	}

	totals := Totals{}
	totals.Update(output)
	fmt.Printf("Totals: %s\n", totals.String())
	if totals.WriteErrors > 0 || totals.ReadErrors > 0 || totals.ReadBytes != totals.WriteBytes {
		t.Fail()
	}

	files, _ := os.ReadDir(path)
	if len(files) != 1 {
		t.Fail()
	}

	cleanUp(path)

	files, _ = os.ReadDir(path)
	if len(files) != 0 {
		t.Fail()
	}
}

func TestChunkSize(t *testing.T) {
	t.SkipNow()

	path, err := os.MkdirTemp("", "sample")
	if err != nil {
		t.FailNow()
	}
	defer os.RemoveAll(path)

	fileName := filepath.Join(path, "test_chunk")

	chunks := []int{1024, 4096, 32768, 65536, 131072, 262144, 524288}
	size := 1073741824 // ~ 1GB

	for _, chunk := range chunks {
		chunkSize = chunk

		start := time.Now()
		writeFile(fileName, size)
		fmt.Printf("Write for %d took %s seconds\n", chunk, time.Since(start))

		start = time.Now()
		readFile(fileName)
		fmt.Printf("Read for %d took %s seconds\n", chunk, time.Since(start))

		os.Remove(fileName)
	}
}

func TestGenerateRandomString(t *testing.T) {
	out := generateRandomString(16)
	if len(out) == 16 {
		fmt.Println(out)
	} else {
		t.Fail()
	}
}
