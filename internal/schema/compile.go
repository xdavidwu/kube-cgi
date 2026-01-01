package schema

import (
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const (
	SchemaFileUrl = "file:///api.schema.json"
)

func CompileString(s string) (*jsonschema.Schema, error) {
	sc, err := jsonschema.UnmarshalJSON(strings.NewReader(s))
	if err != nil {
		return nil, err
	}

	c := jsonschema.NewCompiler()
	c.AddResource(SchemaFileUrl, sc)
	return c.Compile(SchemaFileUrl)
}
