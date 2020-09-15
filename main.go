package main

import (
	"context"
	"fmt"
	"github.com/99designs/gqlgen/codegen/config"
	"github.com/99designs/gqlgen/codegen/templates"
	"github.com/pkg/errors"
	"github.com/sunfmin/gogen"
	"github.com/vektah/gqlparser/v2/ast"
	"go/types"
	"os"
)

type Field struct {
	Description string
	Name        string
	Type        types.Type
	Tag         string
}

type Object struct {
	Description string
	Name        string
	Fields      []*Field
	Implements  []string
}

func main() {
	cfg, err := config.LoadConfigFromDefaultLocations()
	if os.IsNotExist(errors.Cause(err)) {
		cfg = config.DefaultConfig()
	} else if err != nil {
		panic(err)
	}

	err = cfg.LoadSchema()
	if err != nil {
		panic(err)
	}

	binder := cfg.NewBinder()
	schema := cfg.Schema

	it := &Object{
		Name: "Data",
	}

	for _, schemaType := range schema.Types {
		switch schemaType.Kind {
		case ast.Object, ast.InputObject:
			if schemaType != schema.Query && schemaType != schema.Mutation && schemaType != schema.Subscription {
				continue
			}

			for _, field := range schemaType.Fields {
				var typ types.Type
				fieldDef := schema.Types[field.Type.Name()]
				if field.Type.Name() == "__Schema" || field.Type.Name() == "__Type" {
					continue
				}

				switch fieldDef.Kind {
				case ast.Scalar:
					// no user defined model, referencing a default scalar
					typ = types.NewNamed(
						types.NewTypeName(0, nil, "string", nil),
						nil,
						nil,
					)

				case ast.Interface, ast.Union:
					// no user defined model, referencing a generated interface type
					typ = types.NewNamed(
						types.NewTypeName(0, nil, templates.ToGo(field.Type.Name()), nil),
						types.NewInterfaceType([]*types.Func{}, []types.Type{}),
						nil,
					)

				case ast.Enum:
					// no user defined model, must reference a generated enum
					typ = types.NewNamed(
						types.NewTypeName(0, nil, templates.ToGo(field.Type.Name()), nil),
						nil,
						nil,
					)

				case ast.Object, ast.InputObject:
					// no user defined model, must reference a generated struct
					typ = types.NewNamed(
						types.NewTypeName(0, nil, templates.ToGo(field.Type.Name()), nil),
						types.NewStruct(nil, nil),
						nil,
					)

				default:
					panic(fmt.Errorf("unknown ast type %s", fieldDef.Kind))
				}

				name := field.Name
				if nameOveride := cfg.Models[schemaType.Name].Fields[field.Name].FieldName; nameOveride != "" {
					name = nameOveride
				}

				typ = binder.CopyModifiersFromAst(field.Type, typ)

				if isStruct(typ) && (fieldDef.Kind == ast.Object || fieldDef.Kind == ast.InputObject) {
					typ = types.NewPointer(typ)
				}

				it.Fields = append(it.Fields, &Field{
					Name:        name,
					Type:        typ,
					Description: field.Description,
					Tag:         `json:"` + field.Name + `"`,
				})
			}
		}
	}

	data := gogen.Struct("Data")

	for _, f := range it.Fields {
		data.Fields(gogen.Field(templates.ToGo(f.Name), f.Type.String(), f.Tag))
	}

	dataFile := gogen.File("").Package(cfg.Model.Pkg().Name()).Body(
		data,
	)

	dataFile.Fprint(os.Stdout, context.Background())
}

func isStruct(t types.Type) bool {
	_, is := t.Underlying().(*types.Struct)
	return is
}
