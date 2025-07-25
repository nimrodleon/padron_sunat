package downloader

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

func downloadPart(url string, num, start, end int, wg *sync.WaitGroup, dir string, progress chan<- int) {
	defer wg.Done()

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("error parte", num, ":", err)
		return
	}
	defer resp.Body.Close()

	partPath := filepath.Join(dir, fmt.Sprintf("part_%d", num))
	f, _ := os.Create(partPath)
	defer f.Close()

	w, _ := io.Copy(f, resp.Body)
	progress <- int(w)
}

func mergeParts(dir, out string, parts int) error {
	dst, _ := os.Create(out)
	defer dst.Close()
	for i := 0; i < parts; i++ {
		src, err := os.Open(filepath.Join(dir, fmt.Sprintf("part_%d", i)))
		if err != nil {
			return err
		}
		if _, err = io.Copy(dst, src); err != nil {
			src.Close()
			return err
		}
		src.Close()
	}
	return nil
}
