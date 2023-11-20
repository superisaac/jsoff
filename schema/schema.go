package jsoffschema

import (
	"fmt"
	//"reflect"
	json "encoding/json"
)

// SchemaMixin
func (self *SchemaMixin) SetName(name string) {
	self.name = name
}
func (self SchemaMixin) GetName() string {
	return self.name
}

func (self *SchemaMixin) SetDescription(desc string) {
	self.description = desc
}

func (self SchemaMixin) GetDescription() string {
	return self.description
}

func (self SchemaMixin) rebuildType(nType string) map[string]interface{} {
	tp := map[string]interface{}{
		"type": nType,
	}
	if self.name != "" {
		tp["name"] = self.name
	}
	if self.description != "" {
		tp["description"] = self.description
	}
	return tp
}

// type = "any"

func (self AnySchema) Type() string {
	return "any"
}

func (self AnySchema) Equal(other Schema) bool {
	if otherSchema, ok := other.(*AnySchema); ok && otherSchema != nil {
		return (self.name == otherSchema.name &&
			self.description == otherSchema.description)
	}
	return false
}

func (self AnySchema) Map() map[string]interface{} {
	return self.rebuildType(self.Type())
}

func (self *AnySchema) Scan(validator *SchemaValidator, data interface{}) *ErrorPos {
	return nil
}

// type = "null"
func (self NullSchema) Type() string {
	return "null"
}

func (self NullSchema) Equal(other Schema) bool {
	if otherSchema, ok := other.(*NullSchema); ok && otherSchema != nil {
		return (self.name == otherSchema.name &&
			self.description == otherSchema.description)
	}
	return false
}

func (self NullSchema) Map() map[string]interface{} {
	return self.rebuildType(self.Type())
}

func (self *NullSchema) Scan(validator *SchemaValidator, data interface{}) *ErrorPos {
	if data != nil {
		return validator.NewErrorPos("data is not null")
	}
	return nil
}

// type= "bool"
func (self BoolSchema) Type() string {
	return "bool"
}
func (self BoolSchema) Equal(other Schema) bool {
	if otherSchema, ok := other.(*BoolSchema); ok && otherSchema != nil {
		return (self.name == otherSchema.name &&
			self.description == otherSchema.description)
	}
	return false
}

func (self BoolSchema) Map() map[string]interface{} {
	return self.rebuildType(self.Type())
}

func (self *BoolSchema) Scan(validator *SchemaValidator, data interface{}) *ErrorPos {
	if _, ok := data.(bool); ok {
		return nil
	}
	return validator.NewErrorPos("data is not bool")
}

// type = "number"
func NewNumberSchema() *NumberSchema {
	return &NumberSchema{}
}

func (self NumberSchema) Type() string {
	return "number"
}

func (self NumberSchema) Equal(other Schema) bool {
	if otherSchema, ok := other.(*NumberSchema); ok {
		if otherSchema == nil {
			return false
		}
		return (self.name == otherSchema.name &&
			self.description == otherSchema.description &&
			PointerEqual(self.Maximum, otherSchema.Maximum) &&
			PointerEqual(self.Minimum, otherSchema.Minimum) &&
			PointerEqual(self.ExclusiveMaximum, otherSchema.ExclusiveMaximum) &&
			PointerEqual(self.ExclusiveMinimum, otherSchema.ExclusiveMinimum))
	}
	return false
}

func (self NumberSchema) Map() map[string]interface{} {
	tp := self.rebuildType(self.Type())
	if self.Maximum != nil {
		tp["maximum"] = *self.Maximum
		if self.ExclusiveMaximum != nil {
			tp["exclusiveMaximum"] = *self.ExclusiveMaximum
		}
	}

	if self.Minimum != nil {
		tp["minimum"] = *self.Minimum
		if self.ExclusiveMinimum != nil {
			tp["exclusiveMinimum"] = *self.ExclusiveMinimum
		}
	}
	return tp
}

func (self *NumberSchema) Scan(validator *SchemaValidator, data interface{}) *ErrorPos {
	if n, ok := data.(json.Number); ok {
		f, _ := n.Float64()
		return self.checkRange(validator, f)
	}
	if n, ok := data.(int); ok {
		return self.checkRange(validator, float64(n))
	}

	if n, ok := data.(float64); ok {
		return self.checkRange(validator, n)
	}
	return validator.NewErrorPos("data is not number")
}

func (self NumberSchema) checkRange(validator *SchemaValidator, v float64) *ErrorPos {
	if self.Maximum != nil {
		exmax := false
		if self.ExclusiveMaximum != nil {
			exmax = *self.ExclusiveMaximum
		}
		if exmax && *self.Maximum <= v {
			return validator.NewErrorPos("value >= maximum")
		}

		if !exmax && *self.Maximum < v {
			return validator.NewErrorPos("value > maximum")
		}
	}

	if self.Minimum != nil {
		exmin := false
		if self.ExclusiveMinimum != nil {
			exmin = *self.ExclusiveMinimum
		}
		if !exmin && *self.Minimum > v {
			return validator.NewErrorPos("value < minimum")
		}
		if exmin && *self.Minimum >= v {
			return validator.NewErrorPos("value <= minimum")
		}

	}
	return nil
}

// type = "integer"
func NewIntegerSchema() *IntegerSchema {
	return &IntegerSchema{}
}
func (self IntegerSchema) Type() string {
	return "integer"
}
func (self IntegerSchema) Map() map[string]interface{} {
	tp := self.rebuildType(self.Type())
	if self.Maximum != nil {
		tp["maximum"] = *self.Maximum
		if self.ExclusiveMaximum != nil {
			tp["exclusiveMaximum"] = *self.ExclusiveMaximum
		}
	}

	if self.Minimum != nil {
		tp["minimum"] = *self.Minimum
		if self.ExclusiveMinimum != nil {
			tp["exclusiveMinimum"] = *self.ExclusiveMinimum
		}
	}
	return tp
}

func (self IntegerSchema) Equal(other Schema) bool {
	if otherSchema, ok := other.(*IntegerSchema); ok && otherSchema != nil {
		return (self.name == otherSchema.name &&
			self.description == otherSchema.description &&
			PointerEqual(self.Maximum, otherSchema.Maximum) &&
			PointerEqual(self.Minimum, otherSchema.Minimum) &&
			PointerEqual(self.ExclusiveMaximum, otherSchema.ExclusiveMaximum) &&
			PointerEqual(self.ExclusiveMinimum, otherSchema.ExclusiveMinimum))
	}
	return false
}

func (self *IntegerSchema) Scan(validator *SchemaValidator, data interface{}) *ErrorPos {
	if n, ok := data.(json.Number); ok {
		if in, err := n.Int64(); err == nil {
			return self.checkRange(validator, in)
		}
	}
	if n, ok := data.(int); ok {
		return self.checkRange(validator, int64(n))
	}

	return validator.NewErrorPos("data is not integer")
}

func (self IntegerSchema) checkRange(validator *SchemaValidator, v int64) *ErrorPos {
	if self.Maximum != nil {
		exmax := false
		if self.ExclusiveMaximum != nil {
			exmax = *self.ExclusiveMaximum
		}
		if exmax && *self.Maximum <= v {
			return validator.NewErrorPos("value >= maximum")
		}

		if !exmax && *self.Maximum < v {
			return validator.NewErrorPos("value > maximum")
		}
	}

	if self.Minimum != nil {
		exmin := false
		if self.ExclusiveMinimum != nil {
			exmin = *self.ExclusiveMinimum
		}
		if !exmin && *self.Minimum > v {
			return validator.NewErrorPos("value < minimum")
		}

		if exmin && *self.Minimum >= v {
			return validator.NewErrorPos("value <= minimum")
		}

	}
	return nil
}

// type = "string"
func NewStringSchema() *StringSchema {
	return &StringSchema{}
}

func (self StringSchema) Type() string {
	return "string"
}
func (self StringSchema) Map() map[string]interface{} {
	return self.rebuildType(self.Type())
}

func (self StringSchema) Equal(other Schema) bool {
	if otherSchema, ok := other.(*StringSchema); ok && otherSchema != nil {
		return (self.name == otherSchema.name &&
			self.description == otherSchema.description &&
			PointerEqual(self.MaxLength, otherSchema.MaxLength) &&
			PointerEqual(self.MinLength, otherSchema.MinLength))
	}
	return false
}

func (self *StringSchema) Scan(validator *SchemaValidator, data interface{}) *ErrorPos {
	if str, ok := data.(string); ok {
		if self.MaxLength != nil && len(str) > *self.MaxLength {
			return validator.NewErrorPos("len(str) > maxLength")
		}

		if self.MinLength != nil && len(str) < *self.MinLength {
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

func (self AnyOfSchema) Map() map[string]interface{} {
	tp := self.rebuildType(self.Type())
	arr := make([](map[string]interface{}), 0)
	for _, choice := range self.Choices {
		arr = append(arr, choice.Map())
	}
	tp["anyOf"] = arr
	return tp
}

func (self AnyOfSchema) Type() string {
	return "anyOf"
}

func (self AnyOfSchema) Equal(other Schema) bool {
	if otherSchema, ok := other.(*AnyOfSchema); ok && otherSchema != nil {
		return (self.name == otherSchema.name &&
			self.description == otherSchema.description &&
			SchemaListEqual(self.Choices, otherSchema.Choices))
	}
	return false
}

func (self *AnyOfSchema) Scan(validator *SchemaValidator, data interface{}) *ErrorPos {
	for _, schema := range self.Choices {
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

func (self AllOfSchema) Map() map[string]interface{} {
	tp := self.rebuildType(self.Type())
	arr := make([](map[string]interface{}), 0)
	for _, choice := range self.Choices {
		arr = append(arr, choice.Map())
	}
	tp["allOf"] = arr
	return tp
}

func (self AllOfSchema) Type() string {
	return "allOf"
}
func (self AllOfSchema) Equal(other Schema) bool {
	if otherSchema, ok := other.(*AllOfSchema); ok && otherSchema != nil {
		return (self.name == otherSchema.name &&
			self.description == otherSchema.description &&
			SchemaListEqual(self.Choices, otherSchema.Choices))
	}
	return false
}

func (self *AllOfSchema) Scan(validator *SchemaValidator, data interface{}) *ErrorPos {
	for _, schema := range self.Choices {
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

func (self NotSchema) Map() map[string]interface{} {
	tp := self.rebuildType(self.Type())
	tp["not"] = self.Child.Map()
	return tp
}

func (self NotSchema) Type() string {
	return "not"
}

func (self NotSchema) Equal(other Schema) bool {
	if otherSchema, ok := other.(*NotSchema); ok && otherSchema != nil {
		return (self.name == otherSchema.name &&
			self.description == otherSchema.description &&
			SubSchemaEqual(self.Child, otherSchema.Child))
	}
	return false
}

func (self *NotSchema) Scan(validator *SchemaValidator, data interface{}) *ErrorPos {

	errPos := self.Child.Scan(validator, data)
	if errPos == nil {
		return validator.NewErrorPos("not validator failed")
	}
	return nil
}

// type = "array", items is object

func NewListSchema() *ListSchema {
	return &ListSchema{}
}

func (self ListSchema) Type() string {
	return "list"
}

func (self ListSchema) Equal(other Schema) bool {
	if otherSchema, ok := other.(*ListSchema); ok && otherSchema != nil {
		return (self.name == otherSchema.name &&
			self.description == otherSchema.description &&
			PointerEqual(self.MaxItems, otherSchema.MaxItems) &&
			PointerEqual(self.MinItems, otherSchema.MinItems) &&
			SubSchemaEqual(self.Item, otherSchema.Item))
	}
	return false
}

func (self ListSchema) Map() map[string]interface{} {
	tp := self.rebuildType(self.Type())
	tp["items"] = self.Item.Map()
	return tp
}

func (self *ListSchema) Scan(validator *SchemaValidator, data interface{}) *ErrorPos {
	items, ok := data.([]interface{})
	if !ok {
		return validator.NewErrorPos("data is not a list")
	}

	if self.MaxItems != nil && len(items) > *self.MaxItems {
		return validator.NewErrorPos("len(items) > maxItems")
	}

	if self.MinItems != nil && len(items) < *self.MinItems {
		return validator.NewErrorPos("len(items) < minItems")
	}

	for i, item := range items {
		if errPos := validator.Scan(self.Item, fmt.Sprintf("[%d]", i), item); errPos != nil {
			return errPos
		}
	}
	return nil
}

// type = "array", items is list

func NewTupleSchema() *TupleSchema {
	return &TupleSchema{Children: make([]Schema, 0)}
}
func (self TupleSchema) Type() string {
	return "list"
}

func (self TupleSchema) Equal(other Schema) bool {
	if otherSchema, ok := other.(*TupleSchema); ok && otherSchema != nil {
		return (self.name == otherSchema.name &&
			self.description == otherSchema.description &&
			SubSchemaEqual(self.AdditionalSchema, otherSchema.AdditionalSchema) &&
			SchemaListEqual(self.Children, otherSchema.Children))
	}
	return false
}

func (self TupleSchema) Map() map[string]interface{} {
	tp := self.rebuildType(self.Type())
	arr := make([](map[string]interface{}), 0)
	for _, child := range self.Children {
		arr = append(arr, child.Map())
	}
	tp["items"] = arr
	return tp
}

func (self *TupleSchema) Scan(validator *SchemaValidator, data interface{}) *ErrorPos {
	items, ok := data.([]interface{})
	if !ok {
		return validator.NewErrorPos("data is not a list")
	}
	if self.AdditionalSchema == nil {
		if len(items) != len(self.Children) {
			return validator.NewErrorPos("tuple items length mismatch")
		}
	} else {
		if len(items) < len(self.Children) {
			return validator.NewErrorPos("data items length smaller than expected")
		}
	}

	for i, schema := range self.Children {
		item := items[i]
		if errPos := validator.Scan(schema, fmt.Sprintf("[%d]", i), item); errPos != nil {
			return errPos
		}
	}
	if self.AdditionalSchema != nil {
		for i, item := range items[len(self.Children):] {
			pos := fmt.Sprintf("[%d]", i+len(self.Children))
			if errPos := validator.Scan(self.AdditionalSchema, pos, item); errPos != nil {
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
func (self MethodSchema) Type() string {
	return "method"
}

func (self MethodSchema) Equal(other Schema) bool {
	if otherSchema, ok := other.(*MethodSchema); ok && otherSchema != nil {
		return (self.name == otherSchema.name &&
			self.description == otherSchema.description &&
			SubSchemaEqual(self.AdditionalSchema, otherSchema.AdditionalSchema) &&
			SchemaListEqual(self.Params, otherSchema.Params) &&
			SubSchemaEqual(self.Returns, otherSchema.Returns))

	}
	return false
}

func (self MethodSchema) Map() map[string]interface{} {
	tp := self.rebuildType(self.Type())
	arr := make([](map[string]interface{}), 0)
	for _, p := range self.Params {
		arr = append(arr, p.Map())
	}
	tp["params"] = arr
	if self.Returns != nil {
		tp["returns"] = self.Returns.Map()
	}
	if self.AdditionalSchema != nil {
		tp["additionalParams"] = self.AdditionalSchema.Map()
	}
	return tp
}

func (self *MethodSchema) Scan(validator *SchemaValidator, data interface{}) *ErrorPos {
	dataMap, ok := data.(map[string]interface{})
	if !ok {
		return validator.NewErrorPos("data is not object")
	}

	if params, ok := convertAttrList(dataMap, "params", false); ok {
		errPos := self.ScanParams(validator, params)
		if errPos != nil {
			return errPos
		}
		return nil
	}

	if result, ok := dataMap["result"]; ok {
		errPos := self.ScanResult(validator, result)
		return errPos
	}

	return validator.NewErrorPos("data is not a JSONRPC message")
}

func (self *MethodSchema) ScanParams(validator *SchemaValidator, params []interface{}) *ErrorPos {
	validator.pushPath(".params")
	defer validator.popPath(".params")

	if len(params) < len(self.Params) {
		return validator.NewErrorPos("length of params mismatch")
	}

	for i, paramSchema := range self.Params {
		errPos := validator.Scan(paramSchema, fmt.Sprintf("[%d]", i), params[i])
		if errPos != nil {
			return errPos
		}
	}
	if len(params) > len(self.Params) {
		if self.AdditionalSchema == nil {
			return validator.NewErrorPos("length of params mismatch")
		}
		for i := len(self.Params); i < len(params); i++ {
			errPos := validator.Scan(self.AdditionalSchema, fmt.Sprintf("[%d]", i), params[i])
			if errPos != nil {
				return errPos
			}
		}
	}
	return nil
}

func (self *MethodSchema) ScanResult(validator *SchemaValidator, result interface{}) *ErrorPos {
	if self.Returns != nil {
		return validator.Scan(self.Returns, ".result", result)
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

func (self ObjectSchema) Type() string {
	return "object"
}

func (self ObjectSchema) Equal(other Schema) bool {
	if otherSchema, ok := other.(*ObjectSchema); ok && otherSchema != nil {
		return (self.name == otherSchema.name &&
			self.description == otherSchema.description &&
			SchemaMapEqual(self.Properties, otherSchema.Properties) &&
			SchemaMapValueEqual(self.Requires, otherSchema.Requires) &&
			SubSchemaEqual(self.AdditionalProperties, otherSchema.AdditionalProperties))
	}
	return false
}

func (self ObjectSchema) Map() map[string]interface{} {
	tp := self.rebuildType(self.Type())
	props := make(map[string]interface{})
	for name, p := range self.Properties {
		props[name] = p.Map()
	}
	tp["properties"] = props

	arr := make([]string, 0)
	for name, _ := range self.Requires {
		arr = append(arr, name)
	}
	tp["requires"] = arr

	if self.AdditionalProperties != nil {
		tp["additionalProperties"] = self.AdditionalProperties.Map()
	}
	return tp
}

func (self *ObjectSchema) Scan(validator *SchemaValidator, data interface{}) *ErrorPos {
	obj, ok := data.(map[string]interface{})
	if !ok {
		return validator.NewErrorPos("data is not an object")
	}
	checked := map[string]bool{}
	for prop, schema := range self.Properties {
		checked[prop] = true
		if v, found := obj[prop]; found {
			if errPos := validator.Scan(schema, "."+prop, v); errPos != nil {
				return errPos
			}

		} else {
			if _, required := self.Requires[prop]; required {
				// prop is required but not present
				validator.pushPath("." + prop)
				errPos := validator.NewErrorPos("required prop is not present")
				validator.popPath("." + prop)
				return errPos
			}
		}
	}

	if self.AdditionalProperties != nil {
		for prop, v := range obj {
			if _, ok := checked[prop]; ok {
				continue
			}
			if errPos := validator.Scan(self.AdditionalProperties, "."+prop, v); errPos != nil {
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
