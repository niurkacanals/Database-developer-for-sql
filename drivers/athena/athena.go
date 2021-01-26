// Package athena defines and registers usql's AWS Athena driver.
//
// See: https://github.com/uber/athenadriver
package athena

import (
	"regexp"

	_ "github.com/uber/athenadriver/go" // DRIVER: athena
	"github.com/xo/usql/drivers"
)

func init() {
	endRE := regexp.MustCompile(`;?\s*$`)
	drivers.Register("athena", drivers.Driver{
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
			return "Athena " + ver, nil
		},
	})
}
