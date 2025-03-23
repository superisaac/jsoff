package jsoffschema

import (
	"fmt"
	//"reflect"
	json "encoding/json"
)

// SchemaMixin
func (m *SchemaMixin) SetName(name string) {
	m.name = name
}
func (m SchemaMixin) GetName() string {
	return m.name
}

func (m *SchemaMixin) SetDescription(desc string) {
	m.description = desc
}

func (m SchemaMixin) GetDescription() string {
	return m.description
}

func (m SchemaMixin) rebuildType(nType string) map[string]interface{} {
	tp := map[string]interface{}{
		"type": nType,
	}
	if m.name != "" {
		tp["name"] = m.name
	}
	if m.description != "" {
		tp["description"] = m.description
	}
	return tp
}

// type = "any"

func (s AnySchema) Type() string {
	return "any"
}

func (s AnySchema) Equal(other Schema) bool {
	if otherSchema, ok := other.(*AnySchema); ok && otherSchema != nil {
		return (s.name == otherSchema.name &&
			s.description == otherSchema.description)
	}
	return false
}

func (s AnySchema) Map() map[string]interface{} {
	return s.rebuildType(s.Type())
}

func (s *AnySchema) Scan(validator *SchemaValidator, data interface{}) *ErrorPos {
	return nil
}

// type = "null"
func (s NullSchema) Type() string {
	return "null"
}

func (s NullSchema) Equal(other Schema) bool {
	if otherSchema, ok := other.(*NullSchema); ok && otherSchema != nil {
		return (s.name == otherSchema.name &&
			s.description == otherSchema.description)
	}
	return false
}

func (s NullSchema) Map() map[string]interface{} {
	return s.rebuildType(s.Type())
}

func (s *NullSchema) Scan(validator *SchemaValidator, data interface{}) *ErrorPos {
	if data != nil {
		return validator.NewErrorPos("data is not null")
	}
	return nil
}

// type= "bool"
func (s BoolSchema) Type() string {
	return "bool"
}
func (s BoolSchema) Equal(other Schema) bool {
	if otherSchema, ok := other.(*BoolSchema); ok && otherSchema != nil {
		return (s.name == otherSchema.name &&
			s.description == otherSchema.description)
	}
	return false
}

func (s BoolSchema) Map() map[string]interface{} {
	return s.rebuildType(s.Type())
}

func (s *BoolSchema) Scan(validator *SchemaValidator, data interface{}) *ErrorPos {
	if _, ok := data.(bool); ok {
		return nil
	}
	return validator.NewErrorPos("data is not bool")
}

// type = "number"
func NewNumberSchema() *NumberSchema {
	return &NumberSchema{}
}

func (s NumberSchema) Type() string {
	return "number"
}

func (s NumberSchema) Equal(other Schema) bool {
	if otherSchema, ok := other.(*NumberSchema); ok {
		if otherSchema == nil {
			return false
		}
		return (s.name == otherSchema.name &&
			s.description == otherSchema.description &&
			PointerEqual(s.Maximum, otherSchema.Maximum) &&
			PointerEqual(s.Minimum, otherSchema.Minimum) &&
			PointerEqual(s.ExclusiveMaximum, otherSchema.ExclusiveMaximum) &&
			PointerEqual(s.ExclusiveMinimum, otherSchema.ExclusiveMinimum))
	}
	return false
}

func (s NumberSchema) Map() map[string]interface{} {
	tp := s.rebuildType(s.Type())
	if s.Maximum != nil {
		tp["maximum"] = *s.Maximum
		if s.ExclusiveMaximum != nil {
			tp["exclusiveMaximum"] = *s.ExclusiveMaximum
		}
	}

	if s.Minimum != nil {
		tp["minimum"] = *s.Minimum
		if s.ExclusiveMinimum != nil {
			tp["exclusiveMinimum"] = *s.ExclusiveMinimum
		}
	}
	return tp
}

func (s *NumberSchema) Scan(validator *SchemaValidator, data interface{}) *ErrorPos {
	if n, ok := data.(json.Number); ok {
		f, _ := n.Float64()
		return s.checkRange(validator, f)
	}
	if n, ok := data.(int); ok {
		return s.checkRange(validator, float64(n))
	}

	if n, ok := data.(float64); ok {
		return s.checkRange(validator, n)
	}
	return validator.NewErrorPos("data is not number")
}

func (s NumberSchema) checkRange(validator *SchemaValidator, v float64) *ErrorPos {
	if s.Maximum != nil {
		exmax := false
		if s.ExclusiveMaximum != nil {
			exmax = *s.ExclusiveMaximum
		}
		if exmax && *s.Maximum <= v {
			return validator.NewErrorPos("value >= maximum")
		}

		if !exmax && *s.Maximum < v {
			return validator.NewErrorPos("value > maximum")
		}
	}

	if s.Minimum != nil {
		exmin := false
		if s.ExclusiveMinimum != nil {
			exmin = *s.ExclusiveMinimum
		}
		if !exmin && *s.Minimum > v {
			return validator.NewErrorPos("value < minimum")
		}
		if exmin && *s.Minimum >= v {
			return validator.NewErrorPos("value <= minimum")
		}

	}
	return nil
}

// type = "integer"
func NewIntegerSchema() *IntegerSchema {
	return &IntegerSchema{}
}
func (s IntegerSchema) Type() string {
	return "integer"
}
func (s IntegerSchema) Map() map[string]interface{} {
	tp := s.rebuildType(s.Type())
	if s.Maximum != nil {
		tp["maximum"] = *s.Maximum
		if s.ExclusiveMaximum != nil {
			tp["exclusiveMaximum"] = *s.ExclusiveMaximum
		}
	}

	if s.Minimum != nil {
		tp["minimum"] = *s.Minimum
		if s.ExclusiveMinimum != nil {
			tp["exclusiveMinimum"] = *s.ExclusiveMinimum
		}
	}
	return tp
}

func (s IntegerSchema) Equal(other Schema) bool {
	if otherSchema, ok := other.(*IntegerSchema); ok && otherSchema != nil {
		return (s.name == otherSchema.name &&
			s.description == otherSchema.description &&
			PointerEqual(s.Maximum, otherSchema.Maximum) &&
			PointerEqual(s.Minimum, otherSchema.Minimum) &&
			PointerEqual(s.ExclusiveMaximum, otherSchema.ExclusiveMaximum) &&
			PointerEqual(s.ExclusiveMinimum, otherSchema.ExclusiveMinimum))
	}
	return false
}

func (s *IntegerSchema) Scan(validator *SchemaValidator, data interface{}) *ErrorPos {
	if n, ok := data.(json.Number); ok {
		if in, err := n.Int64(); err == nil {
			return s.checkRange(validator, in)
		}
	}
	if n, ok := data.(int); ok {
		return s.checkRange(validator, int64(n))
	}

	return validator.NewErrorPos("data is not integer")
}

func (s IntegerSchema) checkRange(validator *SchemaValidator, v int64) *ErrorPos {
	if s.Maximum != nil {
		exmax := false
		if s.ExclusiveMaximum != nil {
			exmax = *s.ExclusiveMaximum
		}
		if exmax && *s.Maximum <= v {
			return validator.NewErrorPos("value >= maximum")
		}

		if !exmax && *s.Maximum < v {
			return validator.NewErrorPos("value > maximum")
		}
	}

	if s.Minimum != nil {
		exmin := false
		if s.ExclusiveMinimum != nil {
			exmin = *s.ExclusiveMinimum
		}
		if !exmin && *s.Minimum > v {
			return validator.NewErrorPos("value < minimum")
		}

		if exmin && *s.Minimum >= v {
			return validator.NewErrorPos("value <= minimum")
		}

	}
	return nil
}

// type = "string"
func NewStringSchema() *StringSchema {
	return &StringSchema{}
}

func (s StringSchema) Type() string {
	return "string"
}
func (s StringSchema) Map() map[string]interface{} {
	return s.rebuildType(s.Type())
}

func (s StringSchema) Equal(other Schema) bool {
	if otherSchema, ok := other.(*StringSchema); ok && otherSchema != nil {
		return (s.name == otherSchema.name &&
			s.description == otherSchema.description &&
			PointerEqual(s.MaxLength, otherSchema.MaxLength) &&
			PointerEqual(s.MinLength, otherSchema.MinLength))
	}
	return false
}

func (s *StringSchema) Scan(validator *SchemaValidator, data interface{}) *ErrorPos {
	if str, ok := data.(string); ok {
		if s.MaxLength != nil && len(str) > *s.MaxLength {
			return validator.NewErrorPos("len(str) > maxLength")
		}

		if s.MinLength != nil && len(str) < *s.MinLength {
			return validator.NewErrorPos("len(str) < minLength")
		}
		return nil
	}
	return validator.NewErrorPos("data is not string")
}

// type = "anyOf"
func NewAnyOfSchema() *AnyOfSchema {
	return &AnyOfSchema{Choices: make([]Schema, 0)}
}

func (s AnyOfSchema) Map() map[string]interface{} {
	tp := s.rebuildType(s.Type())
	arr := make([](map[string]interface{}), 0)
	for _, choice := range s.Choices {
		arr = append(arr, choice.Map())
	}
	tp["anyOf"] = arr
	return tp
}

func (s AnyOfSchema) Type() string {
	return "anyOf"
}

func (s AnyOfSchema) Equal(other Schema) bool {
	if otherSchema, ok := other.(*AnyOfSchema); ok && otherSchema != nil {
		return (s.name == otherSchema.name &&
			s.description == otherSchema.description &&
			SchemaListEqual(s.Choices, otherSchema.Choices))
	}
	return false
}

func (s *AnyOfSchema) Scan(validator *SchemaValidator, data interface{}) *ErrorPos {
	for _, schema := range s.Choices {
		if errPos := validator.Scan(schema, "", data); errPos == nil {
			return nil
		}
	}
	return validator.NewErrorPos("data is not any of the types")
}

// type = "allOf"
func NewAllOfSchema() *AllOfSchema {
	return &AllOfSchema{Choices: make([]Schema, 0)}
}

func (s AllOfSchema) Map() map[string]interface{} {
	tp := s.rebuildType(s.Type())
	arr := make([](map[string]interface{}), 0)
	for _, choice := range s.Choices {
		arr = append(arr, choice.Map())
	}
	tp["allOf"] = arr
	return tp
}

func (s AllOfSchema) Type() string {
	return "allOf"
}
func (s AllOfSchema) Equal(other Schema) bool {
	if otherSchema, ok := other.(*AllOfSchema); ok && otherSchema != nil {
		return (s.name == otherSchema.name &&
			s.description == otherSchema.description &&
			SchemaListEqual(s.Choices, otherSchema.Choices))
	}
	return false
}

func (s *AllOfSchema) Scan(validator *SchemaValidator, data interface{}) *ErrorPos {
	for _, schema := range s.Choices {
		if errPos := validator.Scan(schema, "", data); errPos != nil {
			return errPos
		}
	}
	return nil
}

// type = "not"
func NewNotSchema() *NotSchema {
	return &NotSchema{}
}

func (s NotSchema) Map() map[string]interface{} {
	tp := s.rebuildType(s.Type())
	tp["not"] = s.Child.Map()
	return tp
}

func (s NotSchema) Type() string {
	return "not"
}

func (s NotSchema) Equal(other Schema) bool {
	if otherSchema, ok := other.(*NotSchema); ok && otherSchema != nil {
		return (s.name == otherSchema.name &&
			s.description == otherSchema.description &&
			SubSchemaEqual(s.Child, otherSchema.Child))
	}
	return false
}

func (s *NotSchema) Scan(validator *SchemaValidator, data interface{}) *ErrorPos {

	errPos := s.Child.Scan(validator, data)
	if errPos == nil {
		return validator.NewErrorPos("not validator failed")
	}
	return nil
}

// type = "array", items is object

func NewListSchema() *ListSchema {
	return &ListSchema{}
}

func (s ListSchema) Type() string {
	return "list"
}

func (s ListSchema) Equal(other Schema) bool {
	if otherSchema, ok := other.(*ListSchema); ok && otherSchema != nil {
		return (s.name == otherSchema.name &&
			s.description == otherSchema.description &&
			PointerEqual(s.MaxItems, otherSchema.MaxItems) &&
			PointerEqual(s.MinItems, otherSchema.MinItems) &&
			SubSchemaEqual(s.Item, otherSchema.Item))
	}
	return false
}

func (s ListSchema) Map() map[string]interface{} {
	tp := s.rebuildType(s.Type())
	tp["items"] = s.Item.Map()
	return tp
}

func (s *ListSchema) Scan(validator *SchemaValidator, data interface{}) *ErrorPos {
	items, ok := data.([]interface{})
	if !ok {
		return validator.NewErrorPos("data is not a list")
	}

	if s.MaxItems != nil && len(items) > *s.MaxItems {
		return validator.NewErrorPos("len(items) > maxItems")
	}

	if s.MinItems != nil && len(items) < *s.MinItems {
		return validator.NewErrorPos("len(items) < minItems")
	}

	for i, item := range items {
		if errPos := validator.Scan(s.Item, fmt.Sprintf("[%d]", i), item); errPos != nil {
			return errPos
		}
	}
	return nil
}

// type = "array", items is list

func NewTupleSchema() *TupleSchema {
	return &TupleSchema{Children: make([]Schema, 0)}
}
func (s TupleSchema) Type() string {
	return "list"
}

func (s TupleSchema) Equal(other Schema) bool {
	if otherSchema, ok := other.(*TupleSchema); ok && otherSchema != nil {
		return (s.name == otherSchema.name &&
			s.description == otherSchema.description &&
			SubSchemaEqual(s.AdditionalSchema, otherSchema.AdditionalSchema) &&
			SchemaListEqual(s.Children, otherSchema.Children))
	}
	return false
}

func (s TupleSchema) Map() map[string]interface{} {
	tp := s.rebuildType(s.Type())
	arr := make([](map[string]interface{}), 0)
	for _, child := range s.Children {
		arr = append(arr, child.Map())
	}
	tp["items"] = arr
	return tp
}

func (s *TupleSchema) Scan(validator *SchemaValidator, data interface{}) *ErrorPos {
	items, ok := data.([]interface{})
	if !ok {
		return validator.NewErrorPos("data is not a list")
	}
	if s.AdditionalSchema == nil {
		if len(items) != len(s.Children) {
			return validator.NewErrorPos("tuple items length mismatch")
		}
	} else {
		if len(items) < len(s.Children) {
			return validator.NewErrorPos("data items length smaller than expected")
		}
	}

	for i, schema := range s.Children {
		item := items[i]
		if errPos := validator.Scan(schema, fmt.Sprintf("[%d]", i), item); errPos != nil {
			return errPos
		}
	}
	if s.AdditionalSchema != nil {
		for i, item := range items[len(s.Children):] {
			pos := fmt.Sprintf("[%d]", i+len(s.Children))
			if errPos := validator.Scan(s.AdditionalSchema, pos, item); errPos != nil {
				return errPos
			}
		}
	}
	return nil
}

// type = "method"
func NewMethodSchema() *MethodSchema {
	return &MethodSchema{Params: make([]Schema, 0), Returns: nil}
}
func (s MethodSchema) Type() string {
	return "method"
}

func (s MethodSchema) Equal(other Schema) bool {
	if otherSchema, ok := other.(*MethodSchema); ok && otherSchema != nil {
		return (s.name == otherSchema.name &&
			s.description == otherSchema.description &&
			SubSchemaEqual(s.AdditionalSchema, otherSchema.AdditionalSchema) &&
			SchemaListEqual(s.Params, otherSchema.Params) &&
			SubSchemaEqual(s.Returns, otherSchema.Returns))

	}
	return false
}

func (s MethodSchema) Map() map[string]interface{} {
	tp := s.rebuildType(s.Type())
	arr := make([](map[string]interface{}), 0)
	for _, p := range s.Params {
		arr = append(arr, p.Map())
	}
	tp["params"] = arr
	if s.Returns != nil {
		tp["returns"] = s.Returns.Map()
	}
	if s.AdditionalSchema != nil {
		tp["additionalParams"] = s.AdditionalSchema.Map()
	}
	return tp
}

func (s *MethodSchema) Scan(validator *SchemaValidator, data interface{}) *ErrorPos {
	dataMap, ok := data.(map[string]interface{})
	if !ok {
		return validator.NewErrorPos("data is not object")
	}

	if params, ok := convertAttrList(dataMap, "params", false); ok {
		errPos := s.ScanParams(validator, params)
		if errPos != nil {
			return errPos
		}
		return nil
	}

	if result, ok := dataMap["result"]; ok {
		errPos := s.ScanResult(validator, result)
		return errPos
	}

	return validator.NewErrorPos("data is not a JSONRPC message")
}

func (s *MethodSchema) ScanParams(validator *SchemaValidator, params []interface{}) *ErrorPos {
	validator.pushPath(".params")
	defer validator.popPath(".params")

	if len(params) < len(s.Params) {
		return validator.NewErrorPos("length of params mismatch")
	}

	for i, paramSchema := range s.Params {
		errPos := validator.Scan(paramSchema, fmt.Sprintf("[%d]", i), params[i])
		if errPos != nil {
			return errPos
		}
	}
	if len(params) > len(s.Params) {
		if s.AdditionalSchema == nil {
			return validator.NewErrorPos("length of params mismatch")
		}
		for i := len(s.Params); i < len(params); i++ {
			errPos := validator.Scan(s.AdditionalSchema, fmt.Sprintf("[%d]", i), params[i])
			if errPos != nil {
				return errPos
			}
		}
	}
	return nil
}

func (s *MethodSchema) ScanResult(validator *SchemaValidator, result interface{}) *ErrorPos {
	if s.Returns != nil {
		return validator.Scan(s.Returns, ".result", result)
	}
	return nil
}

// type = "object"
func NewObjectSchema() *ObjectSchema {
	return &ObjectSchema{
		Properties: make(map[string]Schema),
		Requires:   make(map[string]bool),
	}
}

func (s ObjectSchema) Type() string {
	return "object"
}

func (s ObjectSchema) Equal(other Schema) bool {
	if otherSchema, ok := other.(*ObjectSchema); ok && otherSchema != nil {
		return (s.name == otherSchema.name &&
			s.description == otherSchema.description &&
			SchemaMapEqual(s.Properties, otherSchema.Properties) &&
			SchemaMapValueEqual(s.Requires, otherSchema.Requires) &&
			SubSchemaEqual(s.AdditionalProperties, otherSchema.AdditionalProperties))
	}
	return false
}

func (s ObjectSchema) Map() map[string]interface{} {
	tp := s.rebuildType(s.Type())
	props := make(map[string]interface{})
	for name, p := range s.Properties {
		props[name] = p.Map()
	}
	tp["properties"] = props

	arr := make([]string, 0)
	for name, _ := range s.Requires {
		arr = append(arr, name)
	}
	tp["requires"] = arr

	if s.AdditionalProperties != nil {
		tp["additionalProperties"] = s.AdditionalProperties.Map()
	}
	return tp
}

func (s *ObjectSchema) Scan(validator *SchemaValidator, data interface{}) *ErrorPos {
	obj, ok := data.(map[string]interface{})
	if !ok {
		return validator.NewErrorPos("data is not an object")
	}
	checked := map[string]bool{}
	for prop, schema := range s.Properties {
		checked[prop] = true
		if v, found := obj[prop]; found {
			if errPos := validator.Scan(schema, "."+prop, v); errPos != nil {
				return errPos
			}

		} else {
			if _, required := s.Requires[prop]; required {
				// prop is required but not present
				validator.pushPath("." + prop)
				errPos := validator.NewErrorPos("required prop is not present")
				validator.popPath("." + prop)
				return errPos
			}
		}
	}

	if s.AdditionalProperties != nil {
		for prop, v := range obj {
			if _, ok := checked[prop]; ok {
				continue
			}
			if errPos := validator.Scan(s.AdditionalProperties, "."+prop, v); errPos != nil {
				return errPos
			}
		}
	}

	return nil
}

func SchemaToString(schema Schema) string {
	structData := schema.Map()
	data, err := json.Marshal(structData)
	if err != nil {
		panic(err)
	}
	return string(data)
}

// equal checking
func PointerEqual[T comparable](p1 *T, p2 *T) bool {
	if p1 != nil && p2 != nil {
		return *p1 == *p2
	} else if p1 == nil && p2 == nil {
		return true
	} else {
		return false
	}
}

func SubSchemaEqual(s1 Schema, s2 Schema) bool {
	if s1 != nil && s2 != nil {
		return s1.Equal(s2)
	} else if s1 == nil && s2 == nil {
		return true
	} else {
		return false
	}
}

func SchemaListEqual(l1 []Schema, l2 []Schema) bool {
	if l1 != nil && l2 != nil {
		if len(l1) != len(l2) {
			return false
		}
		for i, elem := range l1 {
			if !elem.Equal(l2[i]) {
				return false
			}
		}
		return true
	} else if l1 == nil && l2 == nil {
		return true
	}
	return false
}

func SchemaMapValueEqual[T comparable](m1 map[string]T, m2 map[string]T) bool {
	if m1 != nil && m2 != nil {
		if len(m1) != len(m2) {
			return false
		}
		for key, elem1 := range m1 {
			elem2, ok := m2[key]
			if !ok {
				return false
			}
			if elem1 != elem2 {
				return false
			}
		}
		return true
	} else if m1 == nil && m2 == nil {
		return true
	}
	return false
}
func SchemaMapEqual(m1 map[string]Schema, m2 map[string]Schema) bool {
	if m1 != nil && m2 != nil {
		if len(m1) != len(m2) {
			return false
		}
		for key, elem1 := range m1 {
			elem2, ok := m2[key]
			if !ok {
				return false
			}
			if !elem1.Equal(elem2) {
				return false
			}
		}
		return true
	} else if m1 == nil && m2 == nil {
		return true
	}
	return false
}
