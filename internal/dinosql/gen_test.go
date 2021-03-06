package dinosql

import (
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/kyleconroy/sqlc/internal/pg"
)

func TestColumnsToStruct(t *testing.T) {
	cols := []pg.Column{
		{
			Name:     "other",
			DataType: "text",
			NotNull:  true,
		},
		{
			Name:     "count",
			DataType: "bigint",
			NotNull:  true,
		},
		{
			Name:     "count",
			DataType: "bigint",
			NotNull:  true,
		},
		{
			Name:     "tags",
			DataType: "text",
			NotNull:  true,
			IsArray:  true,
		},
		{
			Name:     "byte_seq",
			DataType: "bytea",
			NotNull:  true,
		},
		{
			Name:     "retyped",
			DataType: "text",
			NotNull:  true,
		},
		{
			Name:     "languages",
			DataType: "text",
			IsArray:  true,
		},
	}

	// all of the columns are on the 'foo' table
	for i := range cols {
		cols[i].Table = pg.FQN{Schema: "public", Rel: "foo"}
	}

	// set up column-based override test
	o := Override{
		GoType: "example.com/pkg.CustomType",
		Column: "foo.retyped",
	}
	o.Parse()

	// set up column-based array override test
	oa := Override{
		GoType: "github.com/lib/pq.StringArray",
		Column: "foo.languages",
	}
	oa.Parse()

	pkgName := "test_override"

	r := Result{
		packageName: pkgName,
	}
	mockSettings.PackageMap[pkgName] = PackageSettings{
		Overrides: []Override{o, oa},
	}

	actual := r.columnsToStruct("Foo", cols, mockSettings)
	expected := &GoStruct{
		Name: "Foo",
		Fields: []GoField{
			{Name: "Other", Type: "string", Tags: map[string]string{"json:": "other"}},
			{Name: "Count", Type: "int64", Tags: map[string]string{"json:": "count"}},
			{Name: "Count_2", Type: "int64", Tags: map[string]string{"json:": "count_2"}},
			{Name: "Tags", Type: "[]string", Tags: map[string]string{"json:": "tags"}},
			{Name: "ByteSeq", Type: "[]byte", Tags: map[string]string{"json:": "byte_seq"}},
			{Name: "Retyped", Type: "pkg.CustomType", Tags: map[string]string{"json:": "retyped"}},
			{Name: "Languages", Type: "pq.StringArray", Tags: map[string]string{"json:": "languages"}},
		},
	}
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("struct mismatch: \n%s", diff)
	}
}

var mockSettings GenerateSettings

func init() {
	mockSettings = GenerateSettings{
		Version: "1",
		Packages: []PackageSettings{
			PackageSettings{
				Name: "db",
			},
			PackageSettings{
				Name:                "prepared",
				Queries:             filepath.Join("testdata", "ondeck", "query"),
				EmitPreparedQueries: true,
			},
			PackageSettings{
				Name:         "ondeck",
				Queries:      filepath.Join("testdata", "ondeck", "query"),
				EmitJSONTags: true,
			},
		},
		Overrides: []Override{},
	}
	mockSettings.PopulatePkgMap()
}

func TestInnerType(t *testing.T) {
	r := Result{packageName: "db"}
	types := map[string]string{
		// Numeric Types
		// https://www.postgresql.org/docs/current/datatype-numeric.html
		"integer":            "int32",
		"int":                "int32",
		"pg_catalog.int4":    "int32",
		"pg_catalog.numeric": "string",

		// Character Types
		// https://www.postgresql.org/docs/current/datatype-character.html
		"string": "string",

		// Date/Time Types
		// https://www.postgresql.org/docs/current/datatype-datetime.html
		"date":                   "time.Time",
		"pg_catalog.time":        "time.Time",
		"pg_catalog.timetz":      "time.Time",
		"pg_catalog.timestamp":   "time.Time",
		"pg_catalog.timestamptz": "time.Time",
		"timestamptz":            "time.Time",
	}
	for k, v := range types {
		dbType := k
		goType := v
		t.Run(k+"-"+v, func(t *testing.T) {
			col := pg.Column{DataType: dbType, NotNull: true}
			if goType != r.goType(col, mockSettings) {
				t.Errorf("expected Go type for %s to be %s, not %s", dbType, goType, r.goType(col, mockSettings))
			}
		})
	}
}

func TestNullInnerType(t *testing.T) {
	r := Result{packageName: "db"}
	types := map[string]string{
		// Numeric Types
		// https://www.postgresql.org/docs/current/datatype-numeric.html
		"integer":            "sql.NullInt32",
		"int":                "sql.NullInt32",
		"pg_catalog.int4":    "sql.NullInt32",
		"pg_catalog.numeric": "sql.NullString",

		// Character Types
		// https://www.postgresql.org/docs/current/datatype-character.html
		"string": "sql.NullString",

		// Date/Time Types
		// https://www.postgresql.org/docs/current/datatype-datetime.html
		"date":                   "sql.NullTime",
		"pg_catalog.time":        "sql.NullTime",
		"pg_catalog.timetz":      "sql.NullTime",
		"pg_catalog.timestamp":   "sql.NullTime",
		"pg_catalog.timestamptz": "sql.NullTime",
		"timestamptz":            "sql.NullTime",
	}
	for k, v := range types {
		dbType := k
		goType := v
		t.Run(k+"-"+v, func(t *testing.T) {
			col := pg.Column{DataType: dbType, NotNull: false}
			if goType != r.goType(col, mockSettings) {
				t.Errorf("expected Go type for %s to be %s, not %s", dbType, goType, r.goType(col, mockSettings))
			}
		})
	}
}

func TestEnumValueName(t *testing.T) {
	values := map[string]string{
		// Valid separators
		"foo-bar": "FooBar",
		"foo_bar": "FooBar",
		"foo:bar": "FooBar",
		"foo/bar": "FooBar",
		// Strip unknown characters
		"foo@bar": "Foobar",
		"foo+bar": "Foobar",
		"foo!bar": "Foobar",
	}
	for k, v := range values {
		input := k
		expected := v
		t.Run(k+"-"+v, func(t *testing.T) {
			actual := enumValueName(k)
			if actual != expected {
				t.Errorf("expected name for %s to be %s, not %s", input, expected, actual)
			}
		})
	}
}
