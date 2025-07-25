package importer

import (
	"archive/zip"
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
	"io"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const (
	batchSize = 50000
)

func ImportToSQLite(zipFile, dbFile string) error {
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

	decoder := transform.NewReader(rawTxt, charmap.ISO8859_1.NewDecoder())

	db, err := sql.Open("sqlite3", dbFile)
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
CREATE TABLE IF NOT EXISTS padrones (
  ruc                     TEXT PRIMARY KEY,
  nombre_razon_social     TEXT,
  estado_contribuyente    TEXT,
  condicion_domicilio     TEXT,
  ubigeo                  TEXT,
  tipo_via                TEXT,
  codigo_zona             TEXT,
  tipo_zona               TEXT,
  numero                  TEXT,
  interior                TEXT,
  lote                    TEXT,
  departamento            TEXT,
  manzana                 TEXT,
  kilometro               TEXT
);`
	if _, err = db.Exec(schema); err != nil {
		return err
	}

	const cols = 14
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	insertSQL := `
INSERT OR REPLACE INTO padrones (
  ruc, nombre_razon_social, estado_contribuyente, condicion_domicilio, ubigeo,
  tipo_via, codigo_zona, tipo_zona, numero, interior,
  lote, departamento, manzana, kilometro
)
VALUES (?,?,?,?,?,
        ?,?,?,?,?,?,
        ?,?,?);`
	stmt, err := tx.PrepareContext(ctx, insertSQL)
	if err != nil {
		return err
	}
	defer stmt.Close()

	sc := bufio.NewScanner(decoder)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)

	var rows int
	start := time.Now()

	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "RUC|") {
			continue
		}
		parts := strings.Split(line, "|")
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
			parts[10], parts[11], parts[12], parts[13],
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
