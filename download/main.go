package main

import (
	"archive/zip"
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const (
	padronURL   = "http://www2.sunat.gob.pe/padron_reducido_ruc.zip"
	numParts    = 10     // descargas paralelas
	expectedCol = 5      // ruc, nombre, estado, condición, ubigeo
	batchSize   = 50_000 // commit por lote
)

func main() {
	zipName := "padron_reducido_ruc.zip"

	/*──────── 1. Descarga multipart ────────*/
	if err := multiPartDownload(padronURL, zipName, numParts); err != nil {
		fmt.Fprintln(os.Stderr, "❌ descarga:", err)
		os.Exit(1)
	}

	/*──────── 2. Importa a SQLite ────────*/
	if err := importToSQLite(zipName, "padron_reducido_ruc.db"); err != nil {
		fmt.Fprintln(os.Stderr, "❌ importación:", err)
		os.Exit(1)
	}

	fmt.Println("🏁  Listo: padron_reducido_ruc.db creado")
}

/*──────────────── DESCARGA ─────────────────*/

func multiPartDownload(url, output string, parts int) error {
	size, err := getFileSize(url)
	if err != nil {
		return err
	}

	tmpDir, err := os.MkdirTemp("", "padron_parts")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	partSize := size / parts
	var wg sync.WaitGroup
	progress := make(chan int, parts)
	total := 0

	// progreso en consola
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

/*──────────────── IMPORTACIÓN ─────────────────*/

func importToSQLite(zipFile, dbFile string) error {
	/*── 1. Abre ZIP y localiza el .txt ──*/
	zr, err := zip.OpenReader(zipFile)
	if err != nil {
		return err
	}
	defer zr.Close()

	var rawTxt io.ReadCloser
	for _, f := range zr.File {
		if strings.HasSuffix(strings.ToLower(f.Name), ".txt") {
			rawTxt, err = f.Open()
			if err != nil {
				return err
			}
			defer rawTxt.Close()
			break
		}
	}
	if rawTxt == nil {
		return fmt.Errorf("txt no encontrado en zip")
	}

	/*── 2. Decodifica ISO-8859-1 a UTF-8 ──*/
	decoder := transform.NewReader(rawTxt, charmap.ISO8859_1.NewDecoder())

	/*── 3. Prepara SQLite ──*/
	db, err := sql.Open("sqlite3", dbFile) // ej. "padron_reducido_ruc.db"
	if err != nil {
		return err
	}
	defer db.Close()

	for _, p := range []string{
		"PRAGMA journal_mode=WAL;",
		"PRAGMA synchronous=OFF;",
		"PRAGMA temp_store=MEMORY;",
	} {
		if _, err = db.Exec(p); err != nil {
			return err
		}
	}

	const schema = `
CREATE TABLE IF NOT EXISTS padron_reducido_ruc (
  ruc                TEXT PRIMARY KEY,
  business_name      TEXT,
  taxpayer_status    TEXT,
  domicile_condition TEXT,
  ubigeo             TEXT,
  street_type        TEXT,
  street_name        TEXT,
  zone_code          TEXT,
  zone_type          TEXT,
  number             TEXT,
  interior           TEXT,
  lot                TEXT,
  department         TEXT,
  manzana            TEXT,
  kilometro          TEXT
);`
	if _, err = db.Exec(schema); err != nil {
		return err
	}

	/*── 4. Inserción en lotes ──*/
	const cols = 15 // número real de columnas en el TXT
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	insertSQL := `
INSERT OR REPLACE INTO padron_reducido_ruc
(ruc,business_name,taxpayer_status,domicile_condition,ubigeo,
 street_type,street_name,zone_code,zone_type,number,interior,
 lot,department,manzana,kilometro)
VALUES (?,?,?,?,?,
        ?,?,?,?,?,?,
        ?,?,?,?);`
	stmt, err := tx.PrepareContext(ctx, insertSQL)
	if err != nil {
		return err
	}
	defer stmt.Close()

	sc := bufio.NewScanner(decoder)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20) // líneas largas

	var rows int
	start := time.Now()

	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "RUC|") { // cabecera
			continue
		}

		parts := strings.Split(line, "|")

		// Rellena o recorta para asegurar exactamente 15 valores
		if len(parts) < cols {
			for len(parts) < cols {
				parts = append(parts, "")
			}
		}
		if len(parts) > cols {
			parts = parts[:cols]
		}

		if _, err = stmt.Exec(
			parts[0], parts[1], parts[2], parts[3], parts[4],
			parts[5], parts[6], parts[7], parts[8], parts[9],
			parts[10], parts[11], parts[12], parts[13], parts[14],
		); err != nil {
			return err
		}

		rows++
		if rows%batchSize == 0 {
			if err = tx.Commit(); err != nil {
				return err
			}
			tx, err = db.BeginTx(ctx, nil)
			if err != nil {
				return err
			}
			stmt, err = tx.PrepareContext(ctx, insertSQL)
			if err != nil {
				return err
			}
			fmt.Printf("\rImportando: %,d filas", rows)
		}
	}
	if err = sc.Err(); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}

	fmt.Printf("\r✔ %,d filas en %s\n", rows, time.Since(start).Truncate(time.Second))
	return nil
}
