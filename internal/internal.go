// +build !no_base

package internal

// Code generated by gen.sh. DO NOT EDIT.

//go:generate ./gen.sh

// KnownBuildTags returns a map of known driver names to its respective build
// tags.
func KnownBuildTags() map[string]string {
	return map[string]string{
		"adodb":      "adodb",       // github.com/mattn/go-adodb
		"avatica":    "avatica",     // github.com/apache/calcite-avatica-go/v3
		"cassandra":  "cql",         // github.com/MichaelS11/go-cql-driver
		"clickhouse": "clickhouse",  // github.com/kshvakov/clickhouse
		"couchbase":  "n1ql",        // github.com/couchbase/go_n1ql
		"firebird":   "firebirdsql", // github.com/nakagami/firebirdsql
		"ignite":     "ignite",      // github.com/amsokol/ignite-go-client/sql
		"mssql":      "mssql",       // github.com/denisenkom/go-mssqldb
		"mymysql":    "mymysql",     // github.com/ziutek/mymysql/godrv
		"mysql":      "mysql",       // github.com/go-sql-driver/mysql
		"odbc":       "odbc",        // github.com/alexbrainman/odbc
		"oracle":     "goracle",     // gopkg.in/goracle.v2
		"pgx":        "pgx",         // github.com/jackc/pgx/stdlib
		"postgres":   "postgres",    // github.com/lib/pq
		"presto":     "presto",      // github.com/prestodb/presto-go-client/presto
		"ql":         "ql",          // github.com/cznic/ql
		"sapase":     "tds",         // github.com/thda/tds
		"saphana":    "hdb",         // github.com/SAP/go-hdb/driver
		"snowflake":  "snowflake",   // github.com/snowflakedb/gosnowflake
		"sqlite3":    "sqlite3",     // github.com/mattn/go-sqlite3
		"voltdb":     "voltdb",      // github.com/VoltDB/voltdb-client-go/voltdbclient
	}
}
