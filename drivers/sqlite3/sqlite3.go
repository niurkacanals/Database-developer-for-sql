// Package sqlite3 defines and registers usql's SQLite3 driver. Requires CGO.
//
// See: https://github.com/mattn/go-sqlite3
package sqlite3

import (
	"context"
	"strconv"
	"strings"

	"github.com/mattn/go-sqlite3" // DRIVER: sqlite3
	"github.com/xo/usql/drivers"
	"github.com/xo/usql/drivers/metadata"
	"github.com/xo/xoutil"
)

func init() {
	newReader := func(db drivers.DB, opts ...metadata.ReaderOption) metadata.Reader {
		return &metaReader{
			LoggingReader: metadata.NewLoggingReader(db, opts...),
		}
	}
	drivers.Register("sqlite3", drivers.Driver{
		AllowMultilineComments: true,
		ForceParams: drivers.ForceQueryParameters([]string{
			"loc", "auto",
		}),
		Version: func(ctx context.Context, db drivers.DB) (string, error) {
			var ver string
			err := db.QueryRowContext(ctx, `SELECT sqlite_version()`).Scan(&ver)
			if err != nil {
				return "", err
			}
			return "SQLite3 " + ver, nil
		},
		Err: func(err error) (string, string) {
			if e, ok := err.(sqlite3.Error); ok {
				return strconv.Itoa(int(e.Code)), e.Error()
			}
			code, msg := "", err.Error()
			if e, ok := err.(sqlite3.ErrNo); ok {
				code = strconv.Itoa(int(e))
			}
			return code, msg
		},
		ConvertBytes: func(buf []byte, tfmt string) (string, error) {
			// attempt to convert buf if it matches a time format, and if it
			// does, then return a formatted time string.
			s := string(buf)
			if s != "" && strings.TrimSpace(s) != "" {
				t := new(xoutil.SqTime)
				if err := t.Scan(buf); err == nil {
					return t.Format(tfmt), nil
				}
			}
			return s, nil
		},
		NewMetadataReader: newReader,
	})
}
