package wget

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

func Execute() {
	wg := &sync.WaitGroup{}
	var linksList []string
	dm := NewDownloadManager()
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	// parsing urls
	// if correct, adding them to slice
	// else print error
	for _, inputLink := range os.Args[1:] {
		err := ParseURL(inputLink)
		if err == nil {
			linksList = append(linksList, inputLink)
		} else {
			fmt.Fprintf(os.Stderr, "wget: error parsing url: %v\n", err)
		}
	}

	// locking map here
	dm.fileListLock.Lock()
	for _, link := range linksList {
		dm.fileList[link] = &File{}
		dm.fileListOrder = append(dm.fileListOrder, link)
		wg.Add(1)
		go dm.Download(link, wg)
	}
	dm.fileListLock.Unlock()

	go dm.Print(ctx)

	// waiting for all download goroutines
	// to finish their work
	wg.Wait()
	// cancel ctx context which causes print func
	// to return
	cancel()
	// ensure that last print will finish correctly
	dm.WaitPrintEnd()
	fmt.Println()
}

// Print uses context with select
// and fileListOrder slice to prevent random
// print order of map fields during iteration
func (dm *DownloadManager) Print(ctx context.Context) {
	defer close(dm.printFinishedCh)

	print := func() {
		fmt.Print("\r")
		dm.fileListLock.Lock()
		for _, key := range dm.fileListOrder {
			if dm.fileList[key].totalSize != 0 {
				fmt.Printf("%.2f %% ", float64(dm.fileList[key].counter)/float64(dm.fileList[key].totalSize)*100)
			} else {
				fmt.Print("0.00 %% ")
			}
		}
		dm.fileListLock.Unlock()
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

// WaitPrintEnd helps us to be sure that
// last call of Print function will be executed
// before main gourutine ends
func (dm *DownloadManager) WaitPrintEnd() {
	<-dm.printFinishedCh
	return
}

func (dm *DownloadManager) Download(link string, wg *sync.WaitGroup) {
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
	local := path.Base(resp.Request.URL.Path)

	f, err := os.Create(local)
	if err != nil {
		fmt.Fprintf(os.Stderr, "wget: error creating file :%v\n", err)
		return
	}

	// locking map
	dm.fileListLock.Lock()
	file := dm.fileList[link]
	file.Reader = resp.Body
	file.totalSize = bodySize
	dm.fileListLock.Unlock()

	// Copy need to be changed to Tee.Reader
	_, err1 := io.Copy(f, file)
	if err1 != nil {
		//return false, 0, fmt.Errorf("wget: error reading from %s: %v", link, err)
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
