package codegen

import "strings"

// goMapping describes how a SQL column type maps onto the jooq field types: the
// Go field type used in the generated struct, the constructor that builds it,
// and any additional package imports the field type requires.
type goMapping struct {
	fieldType   string
	constructor string
	imports     []string
}

// typeMapping captures the refined non-nullable jooq field mapping for a SQL
// data type together with the information needed to derive its nullable mapping.
//
// A nullable column is mapped in one of three ways, in order of precedence:
//  1. If nullableOverride is set, it is used verbatim. This is required for JSON
//     types, whose non-nullable json.RawMessage destination cannot receive a SQL
//     NULL through database/sql.
//  2. Otherwise, if element is non-empty, it is wrapped in the generic sql.Null
//     type, for example sql.Null[string].
//  3. Otherwise the non-nullable mapping is reused, which is correct for types
//     that already scan NULL as a nil value (such as []byte).
type typeMapping struct {
	// nonNullable is the field descriptor used when the column is NOT NULL.
	nonNullable goMapping
	// element is the Go element type for the column, for example "string" or
	// "int64". It is empty for types that have no scalar element that can be
	// wrapped in sql.Null (such as []byte and json.RawMessage).
	element string
	// elementImports lists the standard library packages required to name the
	// element type, for example "time" for time.Time.
	elementImports []string
	// nullableOverride, when its fieldType is set, is the field descriptor used
	// for a nullable column instead of the sql.Null wrapping. It exists for JSON
	// types, where the nullable destination must be a plain []byte.
	nullableOverride goMapping
}

// normalizeType reduces a raw SQL data type to a comparable key. It lowercases
// the input, trims surrounding whitespace, removes any parenthesized size or
// precision suffix (for example "(10,2)" or "(255)"), and drops trailing
// "unsigned" and "zerofill" modifiers so the base type is recognized.
func normalizeType(dataType string) string {
	t := strings.ToLower(strings.TrimSpace(dataType))
	if i := strings.IndexByte(t, '('); i >= 0 {
		t = t[:i]
	}
	t = strings.TrimSpace(t)
	t = strings.TrimSuffix(t, " zerofill")
	t = strings.TrimSuffix(t, " unsigned")
	return strings.TrimSpace(t)
}

// mappingFor translates a normalized SQL data type into its mapping, capturing
// both the non-nullable jooq field descriptor and the underlying Go element
// type. Recognized integer types map to a NumericField[int64], floating and
// fixed-point types to a NumericField[float64], boolean types to a Field[bool],
// temporal types to a Field[time.Time], binary types to a Field[[]byte], JSON
// types to a Field[json.RawMessage], and textual types to a StringField. Any
// unrecognized type falls back to a StringField.
func mappingFor(dataType string) typeMapping {
	switch normalizeType(dataType) {
	case "integer", "int", "int2", "int4", "int8", "smallint", "bigint",
		"tinyint", "mediumint", "serial", "bigserial", "smallserial":
		return typeMapping{
			nonNullable: goMapping{fieldType: "gooq.NumericField[int64]", constructor: "gooq.NewNumericField[int64]"},
			element:     "int64",
		}

	case "numeric", "decimal", "real", "double", "double precision",
		"float", "float4", "float8", "money", "dec", "fixed":
		return typeMapping{
			nonNullable: goMapping{fieldType: "gooq.NumericField[float64]", constructor: "gooq.NewNumericField[float64]"},
			element:     "float64",
		}

	case "boolean", "bool":
		return typeMapping{
			nonNullable: goMapping{fieldType: "gooq.Field[bool]", constructor: "gooq.NewField[bool]"},
			element:     "bool",
		}

	case "text", "varchar", "char", "character", "character varying",
		"uuid", "citext", "name", "bpchar", "nvarchar", "nchar",
		"tinytext", "mediumtext", "longtext", "enum", "set":
		return typeMapping{
			nonNullable: goMapping{fieldType: "gooq.StringField", constructor: "gooq.NewStringField"},
			element:     "string",
		}

	case "timestamp", "timestamptz", "timestamp with time zone",
		"timestamp without time zone", "date", "time", "timetz",
		"time with time zone", "time without time zone", "datetime":
		return typeMapping{
			nonNullable: goMapping{
				fieldType:   "gooq.Field[time.Time]",
				constructor: "gooq.NewField[time.Time]",
				imports:     []string{"time"},
			},
			element:        "time.Time",
			elementImports: []string{"time"},
		}

	case "bytea", "blob", "binary", "varbinary",
		"tinyblob", "mediumblob", "longblob":
		return typeMapping{
			nonNullable: goMapping{fieldType: "gooq.Field[[]byte]", constructor: "gooq.NewField[[]byte]"},
			// A byte slice already scans NULL as nil, so there is no scalar
			// element to wrap in sql.Null.
			element: "",
		}

	case "json", "jsonb":
		return typeMapping{
			nonNullable: goMapping{
				fieldType:   "gooq.Field[json.RawMessage]",
				constructor: "gooq.NewField[json.RawMessage]",
				imports:     []string{"encoding/json"},
			},
			// A SQL NULL cannot be scanned into the named slice type
			// json.RawMessage: database/sql only assigns a nil driver value to
			// *[]byte, *sql.RawBytes, or *any. A nullable JSON column therefore
			// maps to a plain []byte, which still accepts json.Unmarshal.
			element: "",
			nullableOverride: goMapping{
				fieldType:   "gooq.Field[[]byte]",
				constructor: "gooq.NewField[[]byte]",
			},
		}

	default:
		return typeMapping{
			nonNullable: goMapping{fieldType: "gooq.StringField", constructor: "gooq.NewStringField"},
			element:     "string",
		}
	}
}

// mapSQLType maps a SQL data type and nullability flag onto a goMapping.
//
// Non-nullable columns use the refined field types (NumericField, StringField,
// or a typed Field). Nullable columns whose element type is a scalar wrap that
// element in the generic sql.Null type and fall back to a plain Field, because
// the refined field types provide no nullable variant. Byte slices and JSON raw
// messages already scan NULL as nil and are therefore left unwrapped.
func mapSQLType(dataType string, nullable bool) goMapping {
	mapping := mappingFor(dataType)
	if !nullable {
		return mapping.nonNullable
	}
	if mapping.nullableOverride.fieldType != "" {
		return mapping.nullableOverride
	}
	if mapping.element == "" {
		return mapping.nonNullable
	}

	imports := append([]string{"database/sql"}, mapping.elementImports...)
	wrapped := "sql.Null[" + mapping.element + "]"
	return goMapping{
		fieldType:   "gooq.Field[" + wrapped + "]",
		constructor: "gooq.NewField[" + wrapped + "]",
		imports:     imports,
	}
}
