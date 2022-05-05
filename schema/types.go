package jlibschema

// Schema builder
type SchemaBuildError struct {
	info  string
	paths []string
}

type SchemaBuilder struct {
}

// Fix string map issue from yaml format
type NonStringMap struct {
	paths []string
}

// Schema validator
type SchemaValidator struct {
	paths     []string
	hint      string
	errorPath string
}

type ErrorPos struct {
	paths []string
	hint  string
}

type Schema interface {
	// returns the generated
	Type() string
	Map() map[string]interface{}
	Scan(validator *SchemaValidator, data interface{}) *ErrorPos
	SetName(name string)
	GetName() string
	SetDescription(desc string)
	GetDescription() string
}

type SchemaMixin struct {
	name        string
	description string
}

// schema subclasses
type AnySchema struct {
	SchemaMixin
}

type NullSchema struct {
	SchemaMixin
}
type BoolSchema struct {
	SchemaMixin
}

type NumberSchema struct {
	SchemaMixin
	Minimum          *float64
	Maximum          *float64
	ExclusiveMinimum *bool
	ExclusiveMaximum *bool
}

type IntegerSchema struct {
	SchemaMixin
	Minimum          *int64
	Maximum          *int64
	ExclusiveMinimum *bool
	ExclusiveMaximum *bool
}

type StringSchema struct {
	SchemaMixin
	MaxLength *int
	MinLength *int
}

// composits
type AnyOfSchema struct {
	SchemaMixin
	Choices []Schema
}

type AllOfSchema struct {
	SchemaMixin
	Choices []Schema
}

type NotSchema struct {
	SchemaMixin
	Child Schema
}

type ListSchema struct {
	SchemaMixin
	Item     Schema
	MinItems *int
	MaxItems *int
}

type TupleSchema struct {
	SchemaMixin
	Children         []Schema
	AdditionalSchema Schema
}

type ObjectSchema struct {
	SchemaMixin
	Properties           map[string]Schema
	Requires             map[string]bool
	AdditionalProperties Schema
}

type MethodSchema struct {
	SchemaMixin
	Params           []Schema
	Returns          Schema
	AdditionalSchema Schema
}
