package wget

import (
	"fmt"
	"os"
	"path"
	"sync"
	"testing"
)

func TestDownload(t *testing.T) {
	wg := &sync.WaitGroup{}
	dm := NewDownloadManager()

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

	for _, test := range tests {
		wg.Add(1)
		dm.fileList[test.inputLink] = &File{}
		go dm.Download(test.inputLink, wg)
	}
	wg.Wait()

	for _, test := range tests {
		link := test.inputLink
		got, err := WasFileDownloaded(link, dm.fileList[link].totalSize)
		if got != test.want {
			t.Errorf("Download(%q) = %v : %v", test.inputLink, got, err.Error())
		}
	}
}

func WasFileDownloaded(link string, totalSizeURL int64) (bool, error) {
	//get filename from URL
	fileName := path.Base(link)
	file, err := os.Stat(fileName)
	if err != nil {
		return false, fmt.Errorf("%s: %v", fileName, err)
	}

	if file.Size() != totalSizeURL {
		return false, fmt.Errorf("%s: sizes are not the same", fileName)
	}

	return true, nil
}
