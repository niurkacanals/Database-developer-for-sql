// Package trino defines and registers usql's Trino driver.
//
// See: https://github.com/trinodb/trino-go-client
package trino

import (
	"io"
	"regexp"

	_ "github.com/trinodb/trino-go-client/trino" // DRIVER: trino
	"github.com/xo/tblfmt"
	"github.com/xo/usql/drivers"
	"github.com/xo/usql/drivers/metadata"
	"github.com/xo/usql/drivers/metadata/informationschema"
	"github.com/xo/usql/env"
)

func init() {
	endRE := regexp.MustCompile(`;?\s*$`)
	newReader := informationschema.New(
		informationschema.WithPlaceholder(func(int) string { return "?" }),
		informationschema.WithTypeDetails(false),
		informationschema.WithFunctions(false),
		informationschema.WithSequences(false),
		informationschema.WithIndexes(false),
	)
	drivers.Register("trino", drivers.Driver{
		AllowMultilineComments: true,
		Process: func(prefix string, sqlstr string) (string, string, bool, error) {
			sqlstr = endRE.ReplaceAllString(sqlstr, "")
			typ, q := drivers.QueryExecType(prefix, sqlstr)
			return typ, sqlstr, q, nil
		},
		Version: func(db drivers.DB) (string, error) {
			var ver string
			err := db.QueryRow(
				`SELECT node_version FROM system.runtime.nodes LIMIT 1`,
			).Scan(&ver)
			if err != nil {
				return "", err
			}
			return "Trino " + ver, nil
		},
		NewMetadataReader: newReader,
		NewMetadataWriter: func(db drivers.DB, w io.Writer) metadata.Writer {
			reader := newReader(db)
			opts := []metadata.Option{
				metadata.WithListAllDbs(func(pattern string, verbose bool) error {
					return listAllDbs(db, w, pattern, verbose)
				}),
			}
			return metadata.NewDefaultWriter(reader, opts...)(db, w)
		},
	})
}

func listAllDbs(db drivers.DB, w io.Writer, pattern string, verbose bool) error {
	rows, err := db.Query("SHOW catalogs")
	if err != nil {
		return err
	}
	defer rows.Close()

	return tblfmt.EncodeAll(w, rows, env.Pall())
}
