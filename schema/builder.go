package jlibschema

import (
	"bytes"
	"encoding/json"
	"fmt"
	yaml "gopkg.in/yaml.v2"
	"strings"
)

func NewNonStringMap(paths ...string) *NonStringMap {
	return &NonStringMap{paths: paths}
}

func (self NonStringMap) Error() string {
	return fmt.Sprintf("not string key %s", strings.Join(self.paths, ""))
}

// Schema build error
func (self SchemaBuildError) Error() string {
	return fmt.Sprintf("SchemaBuildError %s, paths: %s", self.info, strings.Join(self.paths, ""))
}

func NewBuildError(info string, paths []string) *SchemaBuildError {
	return &SchemaBuildError{info: info, paths: paths}
}

// Builder
func NewSchemaBuilder() *SchemaBuilder {
	return &SchemaBuilder{}
}

func (self *SchemaBuilder) BuildBytes(data []byte) (Schema, error) {

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var v interface{}
	err := dec.Decode(&v)
	if err != nil {
		return nil, err
	}
	return self.Build(v)
}

func (self *SchemaBuilder) Build(data interface{}) (Schema, error) {
	return self.buildNode(data)
}

//
func (self SchemaBuilder) FixYamlMaps(src interface{}, paths ...string) (interface{}, error) {
	if anyMap, ok := src.(map[interface{}]interface{}); ok {
		strMap := make(map[string]interface{})
		for k, v := range anyMap {
			if sk, ok := k.(string); ok {
				newPaths := append(paths, fmt.Sprintf(".%s", k))
				newV, err := self.FixYamlMaps(v, newPaths...)
				if err != nil {
					return nil, err
				}
				strMap[sk] = newV
			} else {
				newPaths := append(paths, fmt.Sprintf(".%v", k))
				return nil, NewNonStringMap(newPaths...)
			}
		}
		return strMap, nil
	} else if anyList, ok := src.([]interface{}); ok {
		list1 := make([]interface{}, 0)
		for i, elem := range anyList {
			newPaths := append(paths, fmt.Sprintf("[%d]", i))
			newElem, err := self.FixYamlMaps(elem, newPaths...)
			if err != nil {
				return nil, err
			}
			list1 = append(list1, newElem)
		}
		return list1, nil
	} else {
		return src, nil
	}
}

func (self *SchemaBuilder) BuildYamlInterface(data interface{}) (Schema, error) {
	jsonData, err := self.FixYamlMaps(data)
	if err != nil {
		return nil, err
	}
	s, err := self.Build(jsonData)
	if err != nil {
		return nil, err
	}
	return s, err
}

func (self *SchemaBuilder) BuildYamlBytes(bytes []byte) (Schema, error) {
	data := make(map[interface{}]interface{})
	err := yaml.Unmarshal(bytes, &data)
	if err != nil {
		return nil, err
	}
	return self.BuildYamlInterface(data)
}

func (self *SchemaBuilder) buildNode(data interface{}, paths ...string) (Schema, error) {
	if typeMap, ok := convertTypeMap(data); ok {
		return self.buildNodeMap(typeMap, paths...)
	} else {
		return nil, NewBuildError("data is not an object", paths)
	}
}

func (self *SchemaBuilder) buildNodeMap(node map[string](interface{}), paths ...string) (Schema, error) {
	nodeType, ok := node["type"]
	if !ok {
		return nil, NewBuildError("no type presented", paths)
	}
	var schema Schema = nil
	var err error = nil

	switch nodeType {
	case "number":
		schema, err = self.buildNumberSchema(node, paths...)
	case "integer":
		schema, err = self.buildIntegerSchema(node, paths...)
	case "bool":
		schema = &BoolSchema{}
	case "any":
		schema = &AnySchema{}
	case "null":
		schema = &NullSchema{}
	case "string":
		schema, err = self.buildStringSchema(node, paths...)
	case "anyOf":
		schema, err = self.buildAnyOfSchema(node, paths...)
	case "allOf":
		schema, err = self.buildAllOfSchema(node, paths...)
	case "not":
		schema, err = self.buildNotSchema(node, paths...)
	case "list":
		schema, err = self.buildListSchema(node, paths...)
	case "object":
		schema, err = self.buildObjectSchema(node, paths...)
	case "method":
		schema, err = self.buildMethodSchema(node, paths...)
	default:
		err = NewBuildError("unknown type", paths)
	}

	if err != nil {
		return nil, err
	}

	err = self.buildMixin(schema, node, paths...)
	if err != nil {
		return nil, err
	}

	return schema, nil
}

func (self *SchemaBuilder) buildMixin(schema Schema, node map[string]interface{}, paths ...string) error {
	if name, ok := node["name"]; ok {
		if strName, ok := name.(string); ok {
			schema.SetName(strName)
		} else {
			newPaths := append(paths, ".name")
			return NewBuildError("name must be string", newPaths)
		}
	}

	if desc, ok := node["description"]; ok {
		if strDesc, ok := desc.(string); ok {
			schema.SetDescription(strDesc)
		} else {
			newPaths := append(paths, ".description")
			return NewBuildError("decsription must be string", newPaths)
		}
	}
	return nil
}

func (self *SchemaBuilder) buildListSchema(node map[string](interface{}), paths ...string) (Schema, error) {
	items, ok := node["items"]
	if !ok {
		return nil, NewBuildError("no items", paths)
	}

	// build tuple
	if itemsTuple, ok := items.([]interface{}); ok {
		schema := NewTupleSchema()
		for i, item := range itemsTuple {
			newPaths := append(paths, fmt.Sprintf("[%d]", i))
			childSchema, err := self.buildNode(item, newPaths...)
			if err != nil {
				return nil, err
			}
			schema.Children = append(schema.Children, childSchema)
		}
		// additional items
		if additional, ok := node["additionalItems"]; ok {
			newPaths := append(paths, ".additionalItems")
			addSchema, err := self.buildNode(additional, newPaths...)
			if err != nil {
				return nil, err
			}
			schema.AdditionalSchema = addSchema
		}

		return schema, nil
	}

	// build list

	itemSchema, err := self.buildNode(items, paths...)
	if err != nil {
		return nil, err
	}
	schema := NewListSchema()
	schema.Item = itemSchema

	if maxItems, ok := convertAttrInt(node, "maxItems", false); ok && maxItems >= 0 {
		schema.MaxItems = &maxItems
	}

	if minItems, ok := convertAttrInt(node, "minItems", false); ok && minItems >= 0 {
		schema.MinItems = &minItems
	}
	return schema, nil
}

func (self *SchemaBuilder) buildAnyOfSchema(node map[string](interface{}), paths ...string) (*AnyOfSchema, error) {
	schema := NewAnyOfSchema()
	if choices, ok := convertAttrListOfMap(node, "anyOf", false); ok {
		for i, choiceNode := range choices {
			newPaths := append(paths, ".anyOf", fmt.Sprintf("[%d]", i))
			c, err := self.buildNodeMap(choiceNode, newPaths...)
			if err != nil {
				return nil, err
			}
			schema.Choices = append(schema.Choices, c)
		}
	} else {
		return nil, NewBuildError("no valid anyOf attribute", paths)
	}
	return schema, nil
}

func (self *SchemaBuilder) buildAllOfSchema(node map[string](interface{}), paths ...string) (*AllOfSchema, error) {
	schema := NewAllOfSchema()
	if choices, ok := convertAttrListOfMap(node, "allOf", false); ok {
		for i, choiceNode := range choices {
			newPaths := append(paths, ".allOf", fmt.Sprintf("[%d]", i))
			c, err := self.buildNodeMap(choiceNode, newPaths...)
			if err != nil {
				return nil, err
			}
			schema.Choices = append(schema.Choices, c)
		}
	} else {
		return nil, NewBuildError("no valid allOf attribute", paths)
	}
	return schema, nil
}

func (self *SchemaBuilder) buildNotSchema(node map[string](interface{}), paths ...string) (*NotSchema, error) {
	schema := NewNotSchema()
	if childMap, ok := convertAttrMap(node, "not", false); ok {
		newPaths := append(paths, ".not")
		c, err := self.buildNodeMap(childMap, newPaths...)
		if err != nil {
			return nil, err
		}
		schema.Child = c
	} else {
		return nil, NewBuildError("no valid not attribute", paths)
	}
	return schema, nil
}

func (self *SchemaBuilder) buildMethodSchema(node map[string](interface{}), paths ...string) (*MethodSchema, error) {
	schema := NewMethodSchema()
	if paramsNodes, ok := convertAttrListOfMap(node, "params", false); ok {
		for i, paramNode := range paramsNodes {
			newPaths := append(paths, ".params", fmt.Sprintf("[%d]", i))
			c, err := self.buildNodeMap(paramNode, newPaths...)
			if err != nil {
				return nil, err
			}
			schema.Params = append(schema.Params, c)
		}
	} else {
		return nil, NewBuildError("params is not a list of objects", paths)
	}

	// additional items
	if additional, ok := node["additionalParams"]; ok {
		newPaths := append(paths, ".additionalParams")
		addSchema, err := self.buildNode(additional, newPaths...)
		if err != nil {
			return nil, err
		}
		schema.AdditionalSchema = addSchema
	}

	if resultNode, ok := convertAttrMap(node, "returns", true); ok {
		if _, ok := resultNode["type"]; !ok {
			resultNode["type"] = "any"
		}
		newPaths := append(paths, ".returns")
		c, err := self.buildNodeMap(resultNode, newPaths...)
		if err != nil {
			return nil, err
		}
		schema.Returns = c
	}
	return schema, nil
}

func (self *SchemaBuilder) buildNumberSchema(node map[string](interface{}), paths ...string) (*NumberSchema, error) {
	schema := NewNumberSchema()
	if maximum, ok := convertAttrFloat(node, "maximum", false); ok {
		schema.Maximum = &maximum
	}
	if minimum, ok := convertAttrFloat(node, "minimum", false); ok {
		schema.Minimum = &minimum
	}
	return schema, nil
}

func (self *SchemaBuilder) buildIntegerSchema(node map[string](interface{}), paths ...string) (*IntegerSchema, error) {
	schema := NewIntegerSchema()
	if maximum, ok := convertAttrInt(node, "maximum", false); ok {
		n := int64(maximum)
		schema.Maximum = &n
	}
	if minimum, ok := convertAttrInt(node, "minimum", false); ok {
		n := int64(minimum)
		schema.Minimum = &n
	}
	return schema, nil
}

func (self *SchemaBuilder) buildStringSchema(node map[string](interface{}), paths ...string) (*StringSchema, error) {
	schema := NewStringSchema()
	if maxLength, ok := convertAttrInt(node, "maxLength", false); ok && maxLength >= 0 {
		schema.MaxLength = &maxLength
	}

	if minLength, ok := convertAttrInt(node, "minLength", false); ok && minLength >= 0 {
		schema.MinLength = &minLength
	}

	return schema, nil
}

func (self *SchemaBuilder) buildObjectSchema(node map[string](interface{}), paths ...string) (*ObjectSchema, error) {
	schema := NewObjectSchema()
	if propNodes, ok := convertAttrMapOfMap(node, "properties", false); ok {
		for propName, propNode := range propNodes {
			newPaths := append(paths, ".properties", fmt.Sprintf(".%s", propName))
			child, err := self.buildNodeMap(propNode, newPaths...)
			if err != nil {
				return nil, err
			}
			schema.Properties[propName] = child
		}
	} else {
		return nil, NewBuildError("properties is not a map of objects", paths)
	}

	if requireList, ok := convertAttrListOfString(node, "requires", true); ok {
		for _, reqProp := range requireList {
			if _, found := schema.Properties[reqProp]; !found {
				newPath := append(paths, ".requires", fmt.Sprintf(".%s", reqProp))
				return nil, NewBuildError("cannot find required prop", newPath)
			}
			schema.Requires[reqProp] = true
		}
	} else {
		newPaths := append(paths, ".requires")
		return nil, NewBuildError("requires is not a list of strings", newPaths)
	}
	return schema, nil
}
