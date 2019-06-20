package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"sync"
	"time"
)

// File represents infromation about file
// we are going to download
type file struct {
	io.Reader // To implement Reader interface
	name      string
	totalSize int64 // Total content size in bytes
	counter   int64 // Total # of bytes transferred
}

func main() {

	var fileList []*file
	// creating WaitGroup for download goroutines
	wg := &sync.WaitGroup{}
	// parsing urls
	// if correct, adding them to slice
	// else print error
	for _, inputLink := range os.Args[1:] {
		parsedLink, bodySize, err := parseURL(inputLink)
		if err == nil {
			file := &file{totalSize: bodySize}
			fileList = append(fileList, file)
			wg.Add(1)
			go file.download(parsedLink, wg)
		} else {
			fmt.Fprintf(os.Stderr, "wget: error parsing url %v: %v\n", inputLink, err)
		}
	}

	// we use channel to wait until the last
	// Print function call will be terminated correctly
	printFinishedCh := make(chan struct{})
	// creating context variable in order to stop Print func
	// exactly in time we need it to be stopped
	ctx, cancel := context.WithCancel(context.Background())
	go print(ctx, fileList, printFinishedCh)

	// waiting for all download goroutines
	// to finish their work
	wg.Wait()
	// cancel ctx context which causes print func
	// to return
	cancel()
	// ensure that last print will finish correctly
	<-printFinishedCh
	fmt.Println()
}

// Download tries to download file from remote address using
// given url. Takes WaitGroup pointer to provide parallelism.
// Prints error to os.Stderr and return if something goes wrong.
func (file *file) download(link string, wg *sync.WaitGroup) {
	defer wg.Done()
	resp, err := http.Get(link)
	if err != nil {
		fmt.Fprintf(os.Stderr, "wget: error connecting url :%v\n", err)
		return
	}
	defer resp.Body.Close()

	bodySize, err := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "wget: error getting content length :%v\n", err)
		return
	}

	//get filename from URL
	fileNameToSave := path.Base(resp.Request.URL.Path)

	f, err := os.Create(fileNameToSave)
	if err != nil {
		fmt.Fprintf(os.Stderr, "wget: error creating file :%v\n", err)
		return
	}

	file.name = fileNameToSave
	file.Reader = resp.Body
	file.totalSize = bodySize

	// Copy need to be changed to Tee.Reader
	_, err1 := io.Copy(f, file)
	if err1 != nil {
		fmt.Fprintf(os.Stderr, "wget: error reading from %s: %v\n", link, err1)
		return
	}

	if err := f.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "wget: error closing file %s: %v\n", f.Name(), err)
		return
	}
}

// Print iterates over given slice of Files, computes
// percentages of download of all Files and shows them in stdout
// in pseudo-table format until ctx.Done() call
func print(ctx context.Context, fileList []*file, printFinishedCh chan struct{}) {
	defer close(printFinishedCh)

	print := func() {
		fmt.Print("\r")
		for _, file := range fileList {
			if file.totalSize != 0 {
				percentage := float64(file.counter) / float64(file.totalSize) * 100
				fmt.Printf("%.2f %% ", percentage)
			} else {
				fmt.Print("0.00 %% ")
			}
		}
	}

	for {
		select {
		case <-ctx.Done():
			print()
			return
		default:
			print()
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// parseURL takes string value link that must be a correct URL,
// parses it and tries to get content-length
// it returns the same link with bodySize and nil error if all is ok
// and empty string with 0 bodySize and corresponding error if something is wrong
func parseURL(link string) (parsedLink string, bodySize int64, err error) {
	_, err = url.ParseRequestURI(link)
	if err != nil {
		return
	}
	resp, err := http.Get(link)
	if err != nil {
		err = fmt.Errorf("wget: error connecting url :%v", err)
		return
	}
	defer resp.Body.Close()

	bodySize, err = strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 0)
	if err != nil {
		err = fmt.Errorf("wget: content length = %v :%v", bodySize, err)
		return
	}

	parsedLink = link
	return
}

// Read 'overrides' the underlying io.Reader's Read method.
// This is the one that will be called by io.Copy(). We simply
// use it to keep track of byte counts and then forward the call.
func (file *file) Read(p []byte) (int, error) {
	n, err := file.Reader.Read(p)
	if err == nil {
		file.counter += int64(n)
	}

	return n, err
}
