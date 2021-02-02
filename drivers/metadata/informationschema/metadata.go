// Package informationschema provides metadata readers that query tables from
// the information_schema schema. It tries to be database agnostic,
// but there is a set of options to configure what tables and columns to expect.
package informationschema

import (
	"fmt"
	"strings"

	"github.com/xo/usql/drivers"
	"github.com/xo/usql/drivers/metadata"
)

// InformationSchema metadata reader
type InformationSchema struct {
	db             drivers.DB
	pf             func(int) string
	hasTypeDetails bool
	hasFunctions   bool
	hasSequences   bool
	hasIndexes     bool
}

var _ metadata.BasicReader = &InformationSchema{}

// New InformationSchema reader
func New(opts ...Option) func(db drivers.DB) metadata.Reader {
	s := &InformationSchema{
		pf:             func(n int) string { return fmt.Sprintf("$%d", n) },
		hasTypeDetails: true,
		hasFunctions:   true,
		hasSequences:   true,
		hasIndexes:     true,
	}
	for _, o := range opts {
		o(s)
	}

	return func(db drivers.DB) metadata.Reader {
		s.db = db
		return s
	}
}

// Option to configure the InformationSchema reader
type Option func(*InformationSchema)

// WithPlaceholder generator function, that usually returns either `?` or `$n`,
// where `n` is the argument.
func WithPlaceholder(pf func(int) string) Option {
	return func(s *InformationSchema) {
		s.pf = pf
	}
}

// WithTypeDetails when the `columns` table contains size and scale columns
func WithTypeDetails(typ bool) Option {
	return func(s *InformationSchema) {
		s.hasTypeDetails = typ
	}
}

// WithFunctions when the `routines` and `parameters` tables exists
func WithFunctions(fun bool) Option {
	return func(s *InformationSchema) {
		s.hasFunctions = fun
	}
}

// WithIndexes when the `statistics` table exists
func WithIndexes(ind bool) Option {
	return func(s *InformationSchema) {
		s.hasIndexes = ind
	}
}

// WithSequences when the `sequences` table exists
func WithSequences(seq bool) Option {
	return func(s *InformationSchema) {
		s.hasSequences = seq
	}
}

// Columns from selected catalog (or all, if empty), matching schemas and tables
func (s InformationSchema) Columns(catalog, schemaPattern, tablePattern string) (*metadata.ColumnSet, error) {
	// column_size does not include interval_precision which doesn't exist in MySQL
	// numeric_precision_radix doesn't exist in MySQL so assume 10
	columns := []string{
		"table_catalog",
		"table_schema",
		"table_name",
		"column_name",
		"ordinal_position",
		"data_type",
		"COALESCE(column_default, '')",
		"COALESCE(is_nullable, '') AS is_nullable",
	}
	if s.hasTypeDetails {
		extraColumns := []string{
			"COALESCE(character_maximum_length, numeric_precision, datetime_precision, 0) AS column_size",
			"COALESCE(numeric_scale, 0)",
			"10 AS numeric_precision_radix",
			"COALESCE(character_octet_length, 0)",
		}
		columns = append(columns, extraColumns...)
	}

	qstr := "SELECT\n  " + strings.Join(columns, ",\n  ") + " FROM information_schema.columns\n"
	conds := []string{}
	vals := []interface{}{}
	if catalog != "" {
		vals = append(vals, catalog)
		conds = append(conds, fmt.Sprintf("table_catalog = %s", s.pf(len(vals))))
	}
	if schemaPattern != "" {
		vals = append(vals, schemaPattern)
		conds = append(conds, fmt.Sprintf("table_schema LIKE %s", s.pf(len(vals))))
	}
	if tablePattern != "" {
		vals = append(vals, tablePattern)
		conds = append(conds, fmt.Sprintf("table_name LIKE %s", s.pf(len(vals))))
	}
	if len(conds) != 0 {
		qstr += " WHERE " + strings.Join(conds, " AND ")
	}
	qstr += `
ORDER BY table_catalog, table_schema, table_name, ordinal_position`
	rows, err := s.db.Query(qstr, vals...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := []metadata.Column{}
	for rows.Next() {
		rec := metadata.Column{}
		targets := []interface{}{
			&rec.Catalog,
			&rec.Schema,
			&rec.Table,
			&rec.Name,
			&rec.OrdinalPosition,
			&rec.DataType,
			&rec.Default,
			&rec.IsNullable,
		}
		if s.hasTypeDetails {
			extraTargets := []interface{}{
				&rec.ColumnSize,
				&rec.DecimalDigits,
				&rec.NumPrecRadix,
				&rec.CharOctetLength,
			}
			targets = append(targets, extraTargets...)
		}
		err = rows.Scan(targets...)
		if err != nil {
			return nil, err
		}
		results = append(results, rec)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return metadata.NewColumnSet(results), nil
}

// Tables from selected catalog (or all, if empty), matching schemas, names and types
func (s InformationSchema) Tables(catalog, schemaPattern, namePattern string, types []string) (*metadata.TableSet, error) {
	qstr := `SELECT
  table_catalog,
  table_schema,
  table_name,
  table_type
FROM information_schema.tables
`
	conds := []string{}
	vals := []interface{}{}
	if catalog != "" {
		vals = append(vals, catalog)
		conds = append(conds, fmt.Sprintf("table_catalog = %s", s.pf(len(vals))))
	}
	if schemaPattern != "" {
		vals = append(vals, schemaPattern)
		conds = append(conds, fmt.Sprintf("table_schema LIKE %s", s.pf(len(vals))))
	}
	if namePattern != "" {
		vals = append(vals, namePattern)
		conds = append(conds, fmt.Sprintf("table_name LIKE %s", s.pf(len(vals))))
	}
	addSequences := false
	if len(types) != 0 {
		pholders := []string{}
		for _, t := range types {
			if t == "SEQUENCE" && s.hasSequences {
				addSequences = true
				continue
			}
			vals = append(vals, t)
			pholders = append(pholders, s.pf(len(vals)))
		}
		if len(pholders) != 0 {
			conds = append(conds, "table_type IN ("+strings.Join(pholders, ", ")+")")
		}
	}
	if len(conds) != 0 {
		qstr += " WHERE " + strings.Join(conds, " AND ")
	}
	if addSequences {
		qstr += `
UNION ALL
SELECT
  sequence_catalog AS table_catalog,
  sequence_schema AS table_schema,
  sequence_name AS table_name,
  'SEQUENCE' AS table_type
FROM information_schema.sequences
`
		conds = []string{}
		if catalog != "" {
			vals = append(vals, catalog)
			conds = append(conds, fmt.Sprintf("sequence_catalog = %s", s.pf(len(vals))))
		}
		if schemaPattern != "" {
			vals = append(vals, schemaPattern)
			conds = append(conds, fmt.Sprintf("sequence_schema LIKE %s", s.pf(len(vals))))
		}
		if namePattern != "" {
			vals = append(vals, namePattern)
			conds = append(conds, fmt.Sprintf("sequence_name LIKE %s", s.pf(len(vals))))
		}
		if len(conds) != 0 {
			qstr += " WHERE " + strings.Join(conds, " AND ")
		}
	}
	qstr += `
ORDER BY table_catalog, table_schema, table_type, table_name`
	rows, err := s.db.Query(qstr, vals...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := []metadata.Table{}
	for rows.Next() {
		rec := metadata.Table{}
		err = rows.Scan(&rec.Catalog, &rec.Schema, &rec.Name, &rec.Type)
		if err != nil {
			return nil, err
		}
		results = append(results, rec)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return metadata.NewTableSet(results), nil
}

// Schemas from selected catalog (or all, if empty), matching schemas and tables
func (s InformationSchema) Schemas(catalog, namePattern string) (*metadata.SchemaSet, error) {
	qstr := `SELECT
  schema_name,
  catalog_name
FROM information_schema.schemata
`
	conds := []string{}
	vals := []interface{}{}
	if catalog != "" {
		vals = append(vals, catalog)
		conds = append(conds, fmt.Sprintf("catalog_name = %s", s.pf(len(vals))))
	}
	if namePattern != "" {
		vals = append(vals, namePattern)
		conds = append(conds, fmt.Sprintf("schema_name LIKE %s", s.pf(len(vals))))
	}
	if len(conds) != 0 {
		qstr += " WHERE " + strings.Join(conds, " AND ")
	}
	qstr += `
ORDER BY catalog_name, schema_name`
	rows, err := s.db.Query(qstr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := []metadata.Schema{}
	for rows.Next() {
		rec := metadata.Schema{}
		err = rows.Scan(&rec.Schema, &rec.Catalog)
		if err != nil {
			return nil, err
		}
		results = append(results, rec)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return metadata.NewSchemaSet(results), nil
}

// Functions from selected catalog (or all, if empty), matching schemas, names and types
func (s InformationSchema) Functions(catalog, schemaPattern, namePattern string, types []string) (*metadata.FunctionSet, error) {
	if !s.hasFunctions {
		// TODO return a non supported error and let callers figure out how to handle
		return metadata.NewFunctionSet([]metadata.Function{}), nil
	}

	qstr := `SELECT
  specific_name,
  routine_catalog,
  routine_schema,
  routine_name,
  routine_type,
  data_type,
  routine_definition,
  COALESCE(external_language, routine_body) AS language,
  is_deterministic,
  security_type
FROM information_schema.routines
`
	conds := []string{}
	vals := []interface{}{}
	if catalog != "" {
		vals = append(vals, catalog)
		conds = append(conds, fmt.Sprintf("routine_catalog = %s", s.pf(len(vals))))
	}
	if schemaPattern != "" {
		vals = append(vals, schemaPattern)
		conds = append(conds, fmt.Sprintf("routine_schema LIKE %s", s.pf(len(vals))))
	}
	if namePattern != "" {
		vals = append(vals, namePattern)
		conds = append(conds, fmt.Sprintf("routine_name LIKE %s", s.pf(len(vals))))
	}
	if len(types) != 0 {
		pholders := []string{}
		for _, t := range types {
			vals = append(vals, t)
			pholders = append(pholders, s.pf(len(vals)))
		}
		if len(pholders) != 0 {
			conds = append(conds, "routine_type IN ("+strings.Join(pholders, ", ")+")")
		}
	}
	if len(conds) != 0 {
		qstr += " WHERE " + strings.Join(conds, " AND ")
	}
	qstr += `
ORDER BY routine_catalog, routine_schema, routine_type, routine_name`
	rows, err := s.db.Query(qstr, vals...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := []metadata.Function{}
	for rows.Next() {
		rec := metadata.Function{}
		err = rows.Scan(
			&rec.SpecificName,
			&rec.Catalog,
			&rec.Schema,
			&rec.Name,
			&rec.Type,
			&rec.ResultType,
			&rec.Source,
			&rec.Language,
			&rec.Volatility,
			&rec.Security,
		)
		if err != nil {
			return nil, err
		}
		results = append(results, rec)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return metadata.NewFunctionSet(results), nil
}

// FunctionColumns (arguments) from selected catalog (or all, if empty), matching schemas and functions
func (s InformationSchema) FunctionColumns(catalog, schemaPattern, functionPattern string) (*metadata.FunctionColumnSet, error) {
	if !s.hasFunctions {
		// TODO return a non supported error and let callers figure out how to handle
		return metadata.NewFunctionColumnSet([]metadata.FunctionColumn{}), nil
	}

	// column_size does not include interval_precision which doesn't exist in MySQL
	// numeric_precision_radix doesn't exist in MySQL so assume 10
	// TODO concat column size and numeric scale to data_type?
	qstr := `SELECT
  specific_catalog,
  specific_schema,
  specific_name,
  COALESCE(parameter_name, ''),
  ordinal_position,
  parameter_mode,
  data_type,
  COALESCE(character_maximum_length, numeric_precision, datetime_precision, 0) AS column_size,
  COALESCE(numeric_scale, 0),
  10 AS numeric_precision_radix,
  COALESCE(character_octet_length, 0)
FROM information_schema.parameters
`
	conds := []string{}
	vals := []interface{}{}
	if catalog != "" {
		vals = append(vals, catalog)
		conds = append(conds, fmt.Sprintf("specific_catalog = %s", s.pf(len(vals))))
	}
	if schemaPattern != "" {
		vals = append(vals, schemaPattern)
		conds = append(conds, fmt.Sprintf("specific_schema LIKE %s", s.pf(len(vals))))
	}
	if functionPattern != "" {
		vals = append(vals, functionPattern)
		conds = append(conds, fmt.Sprintf("specific_name LIKE %s", s.pf(len(vals))))
	}
	if len(conds) != 0 {
		qstr += " WHERE " + strings.Join(conds, " AND ")
	}
	qstr += `
ORDER BY specific_catalog, specific_schema, specific_name, ordinal_position, parameter_name`
	rows, err := s.db.Query(qstr, vals...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := []metadata.FunctionColumn{}
	for rows.Next() {
		rec := metadata.FunctionColumn{}
		err = rows.Scan(
			&rec.Catalog,
			&rec.Schema,
			&rec.FunctionName,
			&rec.Name,
			&rec.OrdinalPosition,
			&rec.Type,
			&rec.DataType,
			&rec.ColumnSize,
			&rec.DecimalDigits,
			&rec.NumPrecRadix,
			&rec.CharOctetLength,
		)
		if err != nil {
			return nil, err
		}
		results = append(results, rec)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return metadata.NewFunctionColumnSet(results), nil
}

// Indexes from selected catalog (or all, if empty), matching schemas and names
func (s InformationSchema) Indexes(catalog, schemaPattern, namePattern string) (*metadata.IndexSet, error) {
	if !s.hasIndexes {
		// TODO return a non supported error and let callers figure out how to handle
		return metadata.NewIndexSet([]metadata.Index{}), nil
	}

	qstr := `SELECT
  table_catalog,
  index_schema,
  table_name,
  index_name,
  CASE WHEN non_unique = 0 THEN 'YES' ELSE 'NO' END AS is_unique,
  CASE WHEN index_name = 'PRIMARY' THEN 'YES' ELSE 'NO' END AS is_primary,
  index_type
FROM information_schema.statistics
`
	conds := []string{}
	vals := []interface{}{}
	if catalog != "" {
		vals = append(vals, catalog)
		conds = append(conds, fmt.Sprintf("table_catalog = %s", s.pf(len(vals))))
	}
	if schemaPattern != "" {
		vals = append(vals, schemaPattern)
		conds = append(conds, fmt.Sprintf("index_schema LIKE %s", s.pf(len(vals))))
	}
	if namePattern != "" {
		vals = append(vals, namePattern)
		conds = append(conds, fmt.Sprintf("index_name LIKE %s", s.pf(len(vals))))
	}
	if len(conds) != 0 {
		qstr += " WHERE " + strings.Join(conds, " AND ")
	}
	qstr += `
GROUP BY table_catalog, index_schema, table_name, index_name,
  CASE WHEN non_unique = 0 THEN 'YES' ELSE 'NO' END,
  CASE WHEN index_name = 'PRIMARY' THEN 'YES' ELSE 'NO' END,
  index_type
ORDER BY table_catalog, index_schema, table_name, index_name
`
	rows, err := s.db.Query(qstr, vals...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := []metadata.Index{}
	for rows.Next() {
		rec := metadata.Index{}
		err = rows.Scan(&rec.Catalog, &rec.Schema, &rec.Table, &rec.Name, &rec.IsUnique, &rec.IsPrimary, &rec.Type)
		if err != nil {
			return nil, err
		}
		results = append(results, rec)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return metadata.NewIndexSet(results), nil
}

// IndexColumns from selected catalog (or all, if empty), matching schemas and indexes
func (s InformationSchema) IndexColumns(catalog, schemaPattern, indexPattern string) (*metadata.IndexColumnSet, error) {
	if !s.hasIndexes {
		// TODO return a non supported error and let callers figure out how to handle
		return metadata.NewIndexColumnSet([]metadata.IndexColumn{}), nil
	}

	qstr := `SELECT
  i.table_catalog,
  i.table_schema,
  i.table_name,
  i.index_name,
  i.column_name,
  c.data_type,
  i.seq_in_index,

FROM information_schema.statistics i
JOIN information_schema.columns c ON
  i.table_catalog = c.table_catalog AND
  i.table_schema = c.table_schema AND
  i.table_name = c.table_name AND
  i.column_name = c.column_name
`
	conds := []string{}
	vals := []interface{}{}
	if catalog != "" {
		vals = append(vals, catalog)
		conds = append(conds, fmt.Sprintf("table_catalog = %s", s.pf(len(vals))))
	}
	if schemaPattern != "" {
		vals = append(vals, schemaPattern)
		conds = append(conds, fmt.Sprintf("index_schema LIKE %s", s.pf(len(vals))))
	}
	if indexPattern != "" {
		vals = append(vals, indexPattern)
		conds = append(conds, fmt.Sprintf("index_name LIKE %s", s.pf(len(vals))))
	}
	if len(conds) != 0 {
		qstr += " WHERE " + strings.Join(conds, " AND ")
	}
	qstr += `
ORDER BY table_catalog, index_schema, index_name, seq_in_index`
	rows, err := s.db.Query(qstr, vals...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := []metadata.IndexColumn{}
	for rows.Next() {
		rec := metadata.IndexColumn{}
		err = rows.Scan(&rec.Catalog, &rec.Schema, &rec.Table, &rec.IndexName, &rec.Name, &rec.DataType, &rec.OrdinalPosition)
		if err != nil {
			return nil, err
		}
		results = append(results, rec)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return metadata.NewIndexColumnSet(results), nil
}

// Sequences from selected catalog (or all, if empty), matching schemas and names
func (s InformationSchema) Sequences(catalog, schemaPattern, namePattern string) (*metadata.SequenceSet, error) {
	if !s.hasSequences {
		// TODO return a non supported error and let callers figure out how to handle
		return metadata.NewSequenceSet([]metadata.Sequence{}), nil
	}

	qstr := `SELECT
  sequence_catalog,
  sequence_schema,
  sequence_name,
  data_type,
  start_value,
  minimum_value,
  maximum_value,
  increment,
  cycle_option
FROM information_schema.sequences
`
	conds := []string{}
	vals := []interface{}{}
	if catalog != "" {
		vals = append(vals, catalog)
		conds = append(conds, fmt.Sprintf("sequence_catalog = %s", s.pf(len(vals))))
	}
	if schemaPattern != "" {
		vals = append(vals, schemaPattern)
		conds = append(conds, fmt.Sprintf("sequence_schema LIKE %s", s.pf(len(vals))))
	}
	if namePattern != "" {
		vals = append(vals, namePattern)
		conds = append(conds, fmt.Sprintf("sequence_name LIKE %s", s.pf(len(vals))))
	}
	if len(conds) != 0 {
		qstr += " WHERE " + strings.Join(conds, " AND ")
	}
	qstr += `
ORDER BY sequence_catalog, sequence_schema, sequence_name`
	rows, err := s.db.Query(qstr, vals...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := []metadata.Sequence{}
	for rows.Next() {
		rec := metadata.Sequence{}
		err = rows.Scan(&rec.Catalog, &rec.Schema, &rec.Name, &rec.DataType, &rec.Start, &rec.Min, &rec.Max, &rec.Increment, &rec.Cycles)
		if err != nil {
			return nil, err
		}
		results = append(results, rec)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return metadata.NewSequenceSet(results), nil
}
