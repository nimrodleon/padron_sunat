package downloader

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"
)

func MultiPartDownload(url, output string, parts int) error {
	size, err := getFileSize(url)
	if err != nil {
		return err
	}

	tmpBase := os.Getenv("SNAP_USER_COMMON")
	tmpDir, err := os.MkdirTemp(tmpBase, "padron_parts")
	fmt.Println("tmpDir:", tmpDir)
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	partSize := size / parts
	var wg sync.WaitGroup
	progress := make(chan int, parts)
	total := 0

	go func() {
		for p := range progress {
			total += p
			fmt.Printf("\rDescargando: %.2f%%", float64(total)/float64(size)*100)
		}
	}()

	for i := 0; i < parts; i++ {
		start := i * partSize
		end := start + partSize - 1
		if i == parts-1 {
			end = size - 1
		}
		wg.Add(1)
		go downloadPart(url, i, start, end, &wg, tmpDir, progress)
	}
	wg.Wait()
	close(progress)
	fmt.Println()

	return mergeParts(tmpDir, output, parts)
}

func getFileSize(url string) (int, error) {
	resp, err := http.Head(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	return strconv.Atoi(resp.Header.Get("Content-Length"))
}
