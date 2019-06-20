package main

import (
	"fmt"
	"os"
	"path"
	"sync"
	"testing"
)

func TestDownload(t *testing.T) {

	var tests = []struct {
		inputLink string
		want      bool
	}{
		{"https://dev.1c-bitrix.ru/examples/3_type.zip", true},
		{"sfafasfafas", false},
		{"https://wordpress.org/latest.zip", true},
		{"ds4a2awdaw", false},
		{"https://www.google.by", false},
		{"https://wordpress.org/wordpress-4.4.2.zip", true},
	}

	wg := &sync.WaitGroup{}
	var fileList []*file

	for _, test := range tests {
		parsedLink, bodySize, err := parseURL(test.inputLink)
		if err == nil {
			file := &file{totalSize: bodySize}
			fileList = append(fileList, file)
			wg.Add(1)
			go file.download(parsedLink, wg)
		} else {
			t.Logf("wget: error parsing url %v: %v\n", test.inputLink, err)
		}
	}
	wg.Wait()

	for _, test := range tests {
		link := test.inputLink
		//get filename from URL
		fileNameFromURL := path.Base(link)

		var wantTotalSize int64
		for i := range fileList {
			if fileList[i].name == fileNameFromURL {
				wantTotalSize = fileList[i].totalSize
				break
			}
		}

		got, err := fileDownloaded(fileNameFromURL, wantTotalSize)
		if got != test.want {
			t.Errorf("Download(%q) = %v : %v", link, got, err.Error())
		}
	}
}

func fileDownloaded(fileName string, wantTotalSize int64) (bool, error) {
	file, err := os.Stat(fileName)
	if err != nil {
		return false, fmt.Errorf("%s: %v", fileName, err)
	}

	if file.Size() != wantTotalSize {
		return false, fmt.Errorf("%s: sizes are not the same", fileName)
	}

	return true, nil
}
