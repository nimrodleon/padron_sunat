package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
)

const url = "http://www2.sunat.gob.pe/padron_reducido_ruc.zip"
const numParts = 10 // Número de partes en las que dividir la descarga

// Descarga una parte del archivo usando HTTP Range Request
func downloadPart(url string, partNum, start, end int, wg *sync.WaitGroup, tempDir string, progressChan chan<- int) {
	defer wg.Done()

	// Crear la solicitud HTTP con rango
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Println("Error al crear solicitud:", err)
		return
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))

	// Realizar la solicitud
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error al descargar parte:", err)
		return
	}
	defer resp.Body.Close()

	// Crear el archivo temporal para guardar la parte descargada
	partPath := filepath.Join(tempDir, fmt.Sprintf("part_%d", partNum))
	partFile, err := os.Create(partPath)
	if err != nil {
		fmt.Println("Error al crear archivo temporal:", err)
		return
	}
	defer partFile.Close()

	// Escribir la respuesta al archivo
	written, err := io.Copy(partFile, resp.Body)
	if err != nil {
		fmt.Println("Error al escribir la parte:", err)
		return
	}

	// Enviar el progreso al canal
	progressChan <- int(written)
}

// Obtiene el tamaño total del archivo a descargar
func getFileSize(url string) (int, error) {
	resp, err := http.Head(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	size, err := strconv.Atoi(resp.Header.Get("Content-Length"))
	if err != nil {
		return 0, fmt.Errorf("no se pudo obtener el tamaño del archivo")
	}
	return size, nil
}

// Une las partes descargadas en un archivo final
func mergeParts(tempDir, outputFileName string, numParts int) error {
	outputFile, err := os.Create(outputFileName)
	if err != nil {
		return err
	}
	defer outputFile.Close()

	// Leer y escribir cada parte en el archivo final
	for i := 0; i < numParts; i++ {
		partPath := filepath.Join(tempDir, fmt.Sprintf("part_%d", i))
		partFile, err := os.Open(partPath)
		if err != nil {
			return err
		}
		defer partFile.Close()

		_, err = io.Copy(outputFile, partFile)
		if err != nil {
			return err
		}
	}
	return nil
}

func main() {
	// Obtener el tamaño total del archivo
	fileSize, err := getFileSize(url)
	if err != nil {
		fmt.Println("Error al obtener tamaño del archivo:", err)
		return
	}

	// Crear un directorio temporal para guardar las partes
	tempDir, err := os.MkdirTemp("", "download_parts")
	if err != nil {
		fmt.Println("Error al crear directorio temporal:", err)
		return
	}
	defer os.RemoveAll(tempDir) // Elimina el directorio temporal al final

	// Calcular el tamaño de cada parte
	partSize := fileSize / numParts

	var wg sync.WaitGroup
	progressChan := make(chan int, numParts)
	totalDownloaded := 0

	// Iniciar la descarga de cada parte
	for i := 0; i < numParts; i++ {
		start := i * partSize
		end := start + partSize - 1
		if i == numParts-1 {
			end = fileSize - 1 // La última parte incluye cualquier byte restante
		}

		wg.Add(1)
		go downloadPart(url, i, start, end, &wg, tempDir, progressChan)
	}

	// Goroutine para escuchar el progreso
	go func() {
		for progress := range progressChan {
			totalDownloaded += progress
			fmt.Printf("\rProgreso: %.2f%%", float64(totalDownloaded)/float64(fileSize)*100)
		}
	}()

	// Esperar a que todas las partes se descarguen
	wg.Wait()
	close(progressChan)

	// Unir todas las partes en un archivo final
	outputFileName := "padron_reducido_ruc.zip"
	err = mergeParts(tempDir, outputFileName, numParts)
	if err != nil {
		fmt.Println("Error al unir partes:", err)
		return
	}

	fmt.Println("\nDescarga completada y archivo guardado como", outputFileName)
}
