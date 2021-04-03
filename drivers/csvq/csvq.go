// Package csvq defines and registers usql's CSVQ driver.
//
// See: https://github.com/mithrandie/csvq-driver
package csvq

import (
	"os"
	"strings"

	"github.com/mithrandie/csvq-driver" // DRIVER: csvq
	"github.com/mithrandie/csvq/lib/query"
	"github.com/xo/usql/drivers"
)

func init() {
	drivers.Register("csvq", drivers.Driver{
		Process: func(prefix string, sqlstr string) (string, string, bool, error) {
			typ, q := drivers.QueryExecType(prefix, sqlstr)
			csvq.SetStdout(query.NewDiscard())
			if strings.HasPrefix(prefix, "SHOW") {
				csvq.SetStdout(os.Stdout)
				q = false
			}
			return typ, sqlstr, q, nil
		},
	})
}
