package jlibschema

import (
	//json "encoding/json"
	//"fmt"
	"testing"
	//"reflect"
	"github.com/stretchr/testify/assert"
)

func TestBuildBasicSchema(t *testing.T) {
	assert := assert.New(t)

	s1 := []byte(`{"type": "number"}`)
	builder := NewSchemaBuilder()
	s, err := builder.BuildBytes(s1)
	assert.Nil(err)
	assert.Equal("number", s.Type())

	s1 = []byte(`{"type": "integer"}`)
	builder = NewSchemaBuilder()
	s, err = builder.BuildBytes(s1)
	assert.Nil(err)
	assert.Equal("integer", s.Type())

	s1 = []byte(`"string"`)
	builder = NewSchemaBuilder()
	s, err = builder.BuildBytes(s1)
	assert.Nil(err)
	assert.Equal("string", s.Type())

	s1 = []byte(`{"type": "bad"}`)
	s, err = builder.BuildBytes(s1)
	assert.NotNil(err)
	assert.Equal("SchemaBuildError unknown type, paths: ", err.Error())

	s1 = []byte(`"bad2"`)
	s, err = builder.BuildBytes(s1)
	assert.NotNil(err)
	assert.Equal("SchemaBuildError data is not an object, paths: ", err.Error())

	s1 = []byte(`{"aa": 4}`)
	s, err = builder.BuildBytes(s1)
	assert.NotNil(err)
	assert.Equal("SchemaBuildError no type presented, paths: ", err.Error())

	s1 = []byte(`{"type": "string"}`)
	s, err = builder.BuildBytes(s1)
	assert.Nil(err)
	assert.Equal("string", s.Type())

	s1 = []byte(`{"type": "null"}`)
	s, err = builder.BuildBytes(s1)
	assert.Nil(err)
	assert.Equal("null", s.Type())

	s1 = []byte(`{"type": "any"}`)
	s, err = builder.BuildBytes(s1)
	assert.Nil(err)
	assert.Equal("any", s.Type())

}

func TestBuildListSchema(t *testing.T) {
	assert := assert.New(t)

	s1 := []byte(`{"type": "list"}`)
	builder := NewSchemaBuilder()
	_, err := builder.BuildBytes(s1)
	assert.NotNil(err)
	assert.Equal("SchemaBuildError no items, paths: ", err.Error())

	s1 = []byte(`{
"type": "list",
"items": "number"
}`)
	builder = NewSchemaBuilder()
	s, err := builder.BuildBytes(s1)
	assert.Nil(err)
	assert.Equal("list", s.Type())

	s1 = []byte(`{
"type": "list",
"items": [
"number",
{"type": "string"},
"bool"
]}`)
	builder = NewSchemaBuilder()
	s, err = builder.BuildBytes(s1)
	assert.Nil(err)
	assert.Equal("list", s.Type())
	tupleSchema, ok := s.(*TupleSchema)
	assert.True(ok)
	assert.Equal(3, len(tupleSchema.Children))
	assert.Equal("number", tupleSchema.Children[0].Type())
	assert.Equal("string", tupleSchema.Children[1].Type())
	assert.Equal("bool", tupleSchema.Children[2].Type())
}

func TestBuildObjectSchema(t *testing.T) {
	assert := assert.New(t)

	s1 := []byte(`{"type": "object"}`)
	builder := NewSchemaBuilder()
	_, err := builder.BuildBytes(s1)
	assert.Nil(err)

	s1 = []byte(`{
"type": "object",
"properties": {
  "aaa": "string",
  "bbb": {"type": "number"}
}
}`)
	builder = NewSchemaBuilder()
	s, err := builder.BuildBytes(s1)
	assert.Nil(err)
	assert.Equal("object", s.Type())

	obj, ok := s.(*ObjectSchema)
	assert.True(ok)
	assert.Equal("string", obj.Properties["aaa"].Type())
	assert.Equal("number", obj.Properties["bbb"].Type())

	props, ok := s.Map()["properties"].(map[string]interface{})
	assert.True(ok)
	assert.Equal(2, len(props))

	s1 = []byte(`{
"type": "object",
"properties": {
  "aaa": {"type": "string"},
  "bbb": {"type": "number"}
},
"requires": ["nosuch", "aaa"]
}`)
	builder = NewSchemaBuilder()
	s, err = builder.BuildBytes(s1)
	assert.NotNil(err)
	assert.Equal("SchemaBuildError cannot find required prop, paths: .requires.nosuch", err.Error())

	s1 = []byte(`{
"type": "object",
"properties": {
  "aaa": {"type": "string"},
  "bbb": {"type": "number"},
  "ccc": {"type": "list", "items": {"type": "string"}}
},
"requires": ["aaa", "bbb"]
}`)
	builder = NewSchemaBuilder()
	s, err = builder.BuildBytes(s1)
	assert.Nil(err)

	obj, ok = s.(*ObjectSchema)
	assert.True(ok)

	assert.Equal(2, len(obj.Requires))
}

func TestBasicValidator(t *testing.T) {
	assert := assert.New(t)

	// number schema
	s1 := []byte(`{
"type": "number",
"maximum": 6000,
"exclusiveMaximum": true,
"minimum": -1980}`)

	builder := NewSchemaBuilder()
	schema, err := builder.BuildBytes(s1)
	assert.Nil(err)
	numberSchema, ok := schema.(*NumberSchema)
	assert.True(ok)

	validator := NewSchemaValidator()
	errPos := validator.ValidateBytes(numberSchema, []byte(`6.3`))
	assert.Nil(errPos)

	errPos = validator.ValidateBytes(numberSchema, []byte(`13001.27`))
	assert.NotNil(errPos)
	assert.Equal("value >= maximum", errPos.hint)

	errPos = validator.ValidateBytes(numberSchema, []byte(`6000`))
	assert.NotNil(errPos)
	assert.Equal("value >= maximum", errPos.hint)

	errPos = validator.ValidateBytes(numberSchema, []byte(`-8888.99`))
	assert.NotNil(errPos)
	assert.Equal("value < minimum", errPos.hint)

	errPos = validator.ValidateBytes(numberSchema, []byte(`-1980`))
	assert.Nil(errPos)

	//validator = NewSchemaValidator()
	errPos = validator.ValidateBytes(numberSchema, []byte(`"a string"`))
	assert.NotNil(errPos)
	assert.Equal("data is not number", errPos.hint)
	assert.Equal("", errPos.Path())

	// integer schema
	s1 = []byte(`{"type": "integer"}`)
	builder = NewSchemaBuilder()
	schema, err = builder.BuildBytes(s1)
	assert.Nil(err)
	intSchema, ok := schema.(*IntegerSchema)
	assert.True(ok)

	errPos = validator.ValidateBytes(intSchema, []byte(`899`))
	assert.Nil(errPos)

	errPos = validator.ValidateBytes(intSchema, []byte(`6.3`))
	assert.NotNil(errPos)
	assert.Equal("data is not integer", errPos.hint)

	validator = NewSchemaValidator()
	errPos = validator.ValidateBytes(intSchema, []byte(`"a string"`))
	assert.NotNil(errPos)
	assert.Equal("data is not integer", errPos.hint)
	assert.Equal("", errPos.Path())

}

func TestStringValidator(t *testing.T) {
	assert := assert.New(t)

	// number schema
	s1 := []byte(`{"type": "string", "maxLength": 10, "minLength": 1}`)
	builder := NewSchemaBuilder()
	schema, err := builder.BuildBytes(s1)
	assert.Nil(err)
	stringSchema, ok := schema.(*StringSchema)
	assert.True(ok)
	assert.Equal(10, *stringSchema.MaxLength)

	validator := NewSchemaValidator()
	errPos := validator.ValidateBytes(stringSchema, []byte(`"a string"`))
	assert.Nil(errPos)

	// test maxLength
	errPos = validator.ValidateBytes(stringSchema, []byte(`"a very loooooooooooooooooooooong string"`))
	assert.NotNil(errPos)
	assert.Equal("len(str) > maxLength", errPos.hint)
	assert.Equal("", errPos.Path())

	// test minLength
	errPos = validator.ValidateBytes(stringSchema, []byte(`""`))
	assert.NotNil(errPos)
	assert.Equal("len(str) < minLength", errPos.hint)
	assert.Equal("", errPos.Path())
}

func TestAnyOfValidator(t *testing.T) {
	assert := assert.New(t)
	s1 := []byte(`{"type": "anyOf"}`)
	builder := NewSchemaBuilder()
	_, err := builder.BuildBytes(s1)
	assert.NotNil(err)
	assert.Equal("SchemaBuildError no valid anyOf attribute, paths: ", err.Error())

	s1 = []byte(`{
"anyOf": [
  {"type": "number"},
  {"type": "string"}
]
}`)
	builder = NewSchemaBuilder()
	s, err := builder.BuildBytes(s1)
	assert.Nil(err)

	uschema, ok := s.(*AnyOfSchema)
	assert.True(ok)

	validator := NewSchemaValidator()
	data := []byte(`true`)
	errPos := validator.ValidateBytes(uschema, data)
	assert.NotNil(errPos)
	assert.Equal("", errPos.Path())
	assert.Equal("data is not any of the types", errPos.hint)

	validator = NewSchemaValidator()
	data = []byte(`{}`)
	errPos = validator.ValidateBytes(uschema, data)
	assert.NotNil(errPos)
	assert.Equal("", errPos.Path())
	assert.Equal("data is not any of the types", errPos.hint)

	validator = NewSchemaValidator()
	data = []byte(`-3.88`)
	errPos = validator.ValidateBytes(uschema, data)
	assert.Nil(errPos)

	validator = NewSchemaValidator()
	data = []byte(`"a string"`)
	errPos = validator.ValidateBytes(uschema, data)
	assert.Nil(errPos)

}

func TestAllOfValidator(t *testing.T) {
	assert := assert.New(t)
	s1 := []byte(`{"type": "allOf"}`)
	builder := NewSchemaBuilder()
	_, err := builder.BuildBytes(s1)
	assert.NotNil(err)
	assert.Equal("SchemaBuildError no valid allOf attribute, paths: ", err.Error())

	s1 = []byte(`{
"allOf": [
  {"type": "number", "maximum": 6000},
  {"type": "number", "minimum": -200}
]
}`)
	builder = NewSchemaBuilder()
	s, err := builder.BuildBytes(s1)
	assert.Nil(err)

	uschema, ok := s.(*AllOfSchema)
	assert.True(ok)

	validator := NewSchemaValidator()
	data := []byte(`9000`)
	errPos := validator.ValidateBytes(uschema, data)
	assert.NotNil(errPos)
	assert.Equal("", errPos.Path())
	assert.Equal("value > maximum", errPos.hint)

	validator = NewSchemaValidator()
	data = []byte(`-7890`)
	errPos = validator.ValidateBytes(uschema, data)
	assert.NotNil(errPos)
	assert.Equal("", errPos.Path())
	assert.Equal("value < minimum", errPos.hint)

	validator = NewSchemaValidator()
	data = []byte(`3799`)
	errPos = validator.ValidateBytes(uschema, data)
	assert.Nil(errPos)
}

func TestNotValidator(t *testing.T) {
	assert := assert.New(t)

	s1 := []byte(`{"type": "not"}`)
	builder := NewSchemaBuilder()
	_, err := builder.BuildBytes(s1)
	assert.NotNil(err)
	assert.Equal("SchemaBuildError no valid not attribute, paths: ", err.Error())

	s1 = []byte(`{
"not": "number"
}`)
	builder = NewSchemaBuilder()
	s, err := builder.BuildBytes(s1)
	assert.Nil(err)

	uschema, ok := s.(*NotSchema)
	assert.True(ok)

	validator := NewSchemaValidator()
	data := []byte(`true`)
	errPos := validator.ValidateBytes(uschema, data)
	assert.Nil(errPos)

	validator = NewSchemaValidator()
	data = []byte(`{}`)
	errPos = validator.ValidateBytes(uschema, data)
	assert.Nil(errPos)

	validator = NewSchemaValidator()
	data = []byte(`-3.88`)
	errPos = validator.ValidateBytes(uschema, data)
	assert.NotNil(errPos)
	assert.Equal("not validator failed", errPos.hint)
}

func TestComplexValidator(t *testing.T) {
	assert := assert.New(t)

	s1 := []byte(`{
"type": "list",
"items": [
   {"type": "string"},
   {"type": "object",
    "properties": {
       "abc": {"type": "string"},
       "def": {"type": "number"}
    },
    "requires": ["abc"]
   }
],
"additionalItems": {"type": "string"}
}`)
	builder := NewSchemaBuilder()
	schema, err := builder.BuildBytes(s1)
	assert.Nil(err)
	s, ok := schema.(*TupleSchema)
	assert.True(ok)

	validator := NewSchemaValidator()
	data := []byte(`["hello", {"abc": "world"}]`)
	errPos := validator.ValidateBytes(s, data)
	assert.Nil(errPos)

	validator = NewSchemaValidator()
	data = []byte(`["hello", {"abc": "world"}, "hello1", "hello2"]`)
	errPos = validator.ValidateBytes(s, data)
	assert.Nil(errPos)

	validator = NewSchemaValidator()
	data = []byte(`["hello", {"abc": 8}]`)
	errPos = validator.ValidateBytes(s, data)
	assert.NotNil(errPos)
	assert.Equal("data is not string", errPos.hint)
	assert.Equal("[1].abc", errPos.Path())

	validator = NewSchemaValidator()
	data = []byte(`["hello", {"def": 7}]`)
	errPos = validator.ValidateBytes(s, data)
	assert.NotNil(errPos)
	assert.Equal("required prop is not present", errPos.hint)
	assert.Equal("[1].abc", errPos.Path())

	validator = NewSchemaValidator()
	data = []byte(`["hello", {"abc": "world"}, "hello1", 123]`)
	errPos = validator.ValidateBytes(s, data)
	assert.NotNil(errPos)
	assert.Equal("data is not string", errPos.hint)
	assert.Equal("[3]", errPos.Path())

}

func TestMethodValidator(t *testing.T) {
	assert := assert.New(t)

	s1 := []byte(`{
"type": "method"
}`)
	builder := NewSchemaBuilder()
	_, err := builder.BuildBytes(s1)
	assert.NotNil(err)
	assert.Equal("SchemaBuildError params is not a list of objects, paths: ", err.Error())

	s1 = []byte(`{
"type": "method",
"params": [
  {"type": "number", "name": "a"},
  {"type": "string", "name": "b"},
  {
    "type": "object",
    "name": "options",
    "description": "calc options",
    "properties": {"aaa": {"type": "string"}, "bbb": {"type": "number"}},
    "requires": ["aaa"]
  }
],
"additionalParams": "string",
"returns": {"type": "string"}
}`)
	builder = NewSchemaBuilder()
	s, err := builder.BuildBytes(s1)
	assert.Nil(err)

	assert.Equal("method", s.Type())
	methodSchema, ok := s.(*MethodSchema)
	assert.True(ok)
	assert.Equal("calc options", methodSchema.Params[2].GetDescription())

	validator := NewSchemaValidator()
	data := []byte(`["hello", 5, {"abc": 8}]`)
	errPos := validator.ValidateBytes(s, data)
	assert.NotNil(errPos)
	assert.Equal("data is not object", errPos.hint)
	assert.Equal("", errPos.Path())

	validator = NewSchemaValidator()
	data = []byte(`{"params": ["hello", 5, {"abc": 8}]}`)
	errPos = validator.ValidateBytes(s, data)
	assert.NotNil(errPos)
	assert.Equal("data is not number", errPos.hint)
	assert.Equal(".params[0]", errPos.Path())

	validator = NewSchemaValidator()
	data = []byte(`{"params": [5, "hello", {"abc": 8}]}`)
	errPos = validator.ValidateBytes(s, data)
	assert.NotNil(errPos)
	assert.Equal("required prop is not present", errPos.hint)
	assert.Equal(".params[2].aaa", errPos.Path())

	validator = NewSchemaValidator()
	data = []byte(`{"params": [5, "hello", {"aaa": 8}]}`)
	errPos = validator.ValidateBytes(s, data)
	assert.NotNil(errPos)
	assert.Equal("data is not string", errPos.hint)
	assert.Equal(".params[2].aaa", errPos.Path())

	validator = NewSchemaValidator()
	data = []byte(`{"params": [5, "hello", {"aaa": "a string"}]}`)
	errPos = validator.ValidateBytes(s, data)
	assert.Nil(errPos)

	validator = NewSchemaValidator()
	data = []byte(`{"params": [5, "hello", {"aaa": "a string"}, "add1", "add2"]}`)
	errPos = validator.ValidateBytes(s, data)
	assert.Nil(errPos)

	validator = NewSchemaValidator()
	data = []byte(`{"params": [5, "hello", {"aaa": "a string"}, "add1", 3]}`)
	errPos = validator.ValidateBytes(s, data)
	assert.NotNil(errPos)
	assert.Equal("data is not string", errPos.hint)
	assert.Equal(".params[4]", errPos.Path())

	validator = NewSchemaValidator()
	data = []byte(`{"result": 8}`)
	errPos = validator.ValidateBytes(s, data)
	assert.NotNil(errPos)
	assert.Equal("data is not string", errPos.hint)
	assert.Equal(".result", errPos.Path())

	validator = NewSchemaValidator()
	data = []byte(`{"result": "a string"}`)
	errPos = validator.ValidateBytes(s, data)
	assert.Nil(errPos)
}

func TestBuildMethodSchema(t *testing.T) {
	assert := assert.New(t)
	s1 := []byte(`{"params": ["string", "number"], "returns": "string"}`)
	builder := NewSchemaBuilder()
	s, err := builder.BuildBytes(s1)

	assert.Nil(err)
	assert.Equal("method", s.Type())
	methodSchema, ok := s.(*MethodSchema)
	assert.True(ok)
	assert.Equal("string", methodSchema.Returns.Type())
}

func TestBuildYamlSchema(t *testing.T) {
	assert := assert.New(t)
	// schema without type field
	s0 := []byte(`---
properties:
  aaa: "string"
  bbb:
    type: string
`)
	builder := NewSchemaBuilder()
	s, err := builder.BuildYamlBytes(s0)
	assert.Nil(err)
	assert.Equal("object", s.Type())

	s1 := []byte(`---
type: object
properties:
  aaa: "string"
  bbb:
    type: string
`)
	builder = NewSchemaBuilder()
	s, err = builder.BuildYamlBytes(s1)
	assert.Nil(err)
	assert.Equal("object", s.Type())

	s2 := []byte(`---
type: object
properties:
  abc: "string"
  5:
    type: string
`)

	builder = NewSchemaBuilder()
	s, err = builder.BuildYamlBytes(s2)
	assert.NotNil(err)
	assert.Contains(err.Error(), ".properties.5")
}
