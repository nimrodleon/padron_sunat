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
		fmt.Fprintln(os.Stderr, "❌ descarga:", err)
		os.Exit(1)
	}

	/*──────── 2. Importa a SQLite ────────*/
	if err := importer.ImportToSQLite(zipName, "padron_reducido_ruc.db"); err != nil {
		fmt.Fprintln(os.Stderr, "❌ importación:", err)
		os.Exit(1)
	}

	fmt.Println("🏁  Listo: padron_reducido_ruc.db creado")
}
