package jsonzschema

import (
	//"fmt"
	"encoding/json"
	//"reflect"
)

func stringInList(a string, candidates ...string) bool {
	for _, ca := range candidates {
		if ca == a {
			return true
		}
	}
	return false
}

// util functions
func convertTypeMap(maybeType interface{}) (map[string]interface{}, bool) {
	if typeStr, ok := maybeType.(string); ok && stringInList(typeStr, "string", "number", "integer", "bool", "null") {
		// type is single string, build a simle map
		typeMap := map[string](interface{}){"type": typeStr}
		return typeMap, true
	}
	if typeMap, ok := maybeType.(map[string]interface{}); ok {
		// type is map
		if _, ok := typeMap["type"]; !ok {
			// type field missing, guess it's type
			if _, ok := typeMap["params"]; ok {
				// has field `params`, so this is a method schema
				typeMap["type"] = "method"
			} else if _, ok := typeMap["properties"]; ok {
				// has field `properties`, so this is a method schema
				typeMap["type"] = "object"
			} else {
				compositTypes := []string{"anyOf", "allOf", "not"}
				for _, tp := range compositTypes {
					if _, ok := typeMap[tp]; ok {
						typeMap["type"] = tp
						break
					}
				}
			}
		}
		return typeMap, ok
	} else {
		return nil, false
	}
}

func convertAttrMap(node map[string]interface{}, attrName string, optional bool) (map[string]interface{}, bool) {
	if v, ok := node[attrName]; ok {
		return convertTypeMap(v)
	} else if optional {
		return map[string](interface{}){}, true
	}
	return nil, false
}

func convertAttrList(node map[string]interface{}, attrName string, optional bool) ([]interface{}, bool) {
	if v, ok := node[attrName]; ok {
		// has attribute
		if aList, ok := v.([]interface{}); ok {
			// attribute is an array

			return aList, ok
		}
	} else if optional {
		return [](interface{}){}, true
	}
	return nil, false
}

func convertAttrInt(node map[string]interface{}, attrName string, optional bool) (int, bool) {
	if v, ok := node[attrName]; ok {
		if intv, ok := v.(int); ok {
			return intv, ok
		} else if n, ok := v.(json.Number); ok {
			intv, err := n.Int64()
			if err != nil {
				return 0, false
			} else {
				return int(intv), ok
			}
		}
	} else if optional {
		return 0, true
	}
	return 0, false
}

func convertAttrFloat(node map[string]interface{}, attrName string, optional bool) (float64, bool) {
	if v, ok := node[attrName]; ok {
		if n, ok := v.(int); ok {
			return float64(n), true
		} else if n, ok := v.(float64); ok {
			return n, true
		} else if n, ok := v.(json.Number); ok {
			fv, err := n.Float64()
			if err != nil {
				return 0, false
			} else {
				return fv, true
			}
		}
	} else if optional {
		return 0, true
	}
	return 0, false
}

func convertAttrMapOfMap(node map[string](interface{}), attrName string, optional bool) (map[string](map[string]interface{}), bool) {
	if mm, ok := convertAttrMap(node, attrName, optional); ok {
		resMap := make(map[string](map[string]interface{}))
		for name, value := range mm {
			mv, ok := convertTypeMap(value)
			if !ok {
				return nil, false
			}
			resMap[name] = mv
		}
		return resMap, true
	}
	return nil, false
}

func convertAttrListOfMap(node map[string]interface{}, attrName string, optional bool) ([](map[string]interface{}), bool) {
	if v, ok := node[attrName]; ok {
		if aList, ok := v.([]interface{}); ok {
			arr := make([](map[string]interface{}), 0)
			for _, item := range aList {
				itemMap, ok := convertTypeMap(item)
				if !ok {
					return nil, false
				}
				arr = append(arr, itemMap)
			}
			return arr, true
		}
	} else if optional {
		return [](map[string]interface{}){}, true
	}
	return nil, false
}

func convertAttrListOfString(node map[string]interface{}, attrName string, optional bool) ([]string, bool) {
	if v, ok := node[attrName]; ok {
		if aList, ok := v.([]interface{}); ok {
			arr := make([]string, 0)
			for _, item := range aList {
				strItem, ok := item.(string)
				if !ok {
					return nil, false
				}
				arr = append(arr, strItem)
			}
			return arr, true
		}
	} else if optional {
		return []string{}, true
	}
	return nil, false
}
