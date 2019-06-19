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
type File struct {
	io.Reader // To implement Reader interface
	name      string
	totalSize int64 // Total content size in bytes
	counter   int64 // Total # of bytes transferred
}

// parseURL takes string value link that must be a link,
// parses it and returns the same link with nil error if all is ok
// and empty string with corresponding error if something is wrong
func parseURL(link string) (string, error) {
	_, err := url.ParseRequestURI(link)
	if err != nil {
		return "", err
	}
	resp, err := http.Get(link)
	if err != nil {
		return "", fmt.Errorf("wget: error connecting url :%v", err)
	}
	defer resp.Body.Close()

	bodySize, err := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 0)
	if err != nil {
		return "", fmt.Errorf("wget: content length = %v :%v", bodySize, err)
	}
	return link, nil
}

func main() {

	var linksList []string
	// parsing urls
	// if correct, adding them to slice
	// else print error
	for _, inputLink := range os.Args[1:] {
		parsedLink, err := parseURL(inputLink)
		if err == nil {
			linksList = append(linksList, parsedLink)
		} else {
			fmt.Fprintf(os.Stderr, "wget: error parsing url %v: %v\n", inputLink, err)
		}
	}

	var fileList []*File
	wg := &sync.WaitGroup{}

	for _, link := range linksList {
		file := &File{}
		fileList = append(fileList, file)
		wg.Add(1)
		go file.Download(link, wg)
	}

	// we use it to wait until the last print function call will be terminated correctly
	printFinishedCh := make(chan struct{})

	ctx, cancel := context.WithCancel(context.Background())
	go Print(ctx, fileList, printFinishedCh)

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

// Print uses context with select
// and fileListOrder slice to prevent random
// print order of map fields during iteration
func Print(ctx context.Context, fileList []*File, printFinishedCh chan struct{}) {
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

func (file *File) Download(link string, wg *sync.WaitGroup) {
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

// Read 'overrides' the underlying io.Reader's Read method.
// This is the one that will be called by io.Copy(). We simply
// use it to keep track of byte counts and then forward the call.
func (pt *File) Read(p []byte) (int, error) {
	n, err := pt.Reader.Read(p)
	if err == nil {
		pt.counter += int64(n)
	}

	return n, err
}
