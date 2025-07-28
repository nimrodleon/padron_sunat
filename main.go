package main

import (
	"fmt"
	"os"
	"padron_sunat/downloader"
	"padron_sunat/importer"
)

const (
	padronURL = "http://www2.sunat.gob.pe/padron_reducido_ruc.zip"
	numParts  = 10
)

func main() {
	zipName := "padron_reducido_ruc.zip"

	/*──────── 1. Descarga multipart ────────*/
	if err := downloader.MultiPartDownload(padronURL, zipName, numParts); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "❌ descarga:", err)
		os.Exit(1)
	}

	/*──────── 2. Importar datos a SQLite ────────*/
	if err := importer.ImportToSQLite(zipName, "padron_sunat.db"); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "❌ importación:", err)
		os.Exit(1)
	}

	fmt.Println("🏁  Listo: padron_sunat.db creado")
}
