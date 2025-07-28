package main

import (
	"fmt"
	"os"
	"padron_sunat/downloader"
	"padron_sunat/importer"
	"path/filepath"
)

const (
	padronURL = "http://www2.sunat.gob.pe/padron_reducido_ruc.zip"
	numParts  = 10
)

func main() {
	// configurar rutas.
	snapPath := os.Getenv("SNAP_USER_COMMON")
	zipFile := filepath.Join(snapPath, "padron_reducido_ruc.zip")
	dbFile := filepath.Join(snapPath, "padron_sunat.db")

	/*──────── 1. Descarga multipart ────────*/
	if err := downloader.MultiPartDownload(padronURL, zipFile, numParts); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "❌ descarga:", err)
		os.Exit(1)
	}

	/*──────── 2. Importar datos a SQLite ────────*/
	if err := importer.ImportToSQLite(zipFile, dbFile); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "❌ importación:", err)
		os.Exit(1)
	}

	fmt.Println("🏁  Listo: padron_sunat.db creado")
}
