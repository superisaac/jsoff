package jsoffschema

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
func convertTypeMap(maybeType any) (map[string]any, bool) {
	if typeStr, ok := maybeType.(string); ok && stringInList(typeStr, "string", "number", "integer", "bool", "null") {
		// type is single string, build a simle map
		typeMap := map[string]any{"type": typeStr}
		return typeMap, true
	}
	if typeMap, ok := maybeType.(map[string]any); ok {
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

func convertAttrMap(node map[string]any, attrName string, optional bool) (map[string]any, bool) {
	if v, ok := node[attrName]; ok {
		return convertTypeMap(v)
	} else if optional {
		return map[string]any{}, true
	}
	return nil, false
}

func convertAttrList(node map[string]any, attrName string, optional bool) ([]any, bool) {
	if v, ok := node[attrName]; ok {
		// has attribute
		if aList, ok := v.([]any); ok {
			// attribute is an array

			return aList, ok
		}
	} else if optional {
		return []any{}, true
	}
	return nil, false
}

func convertAttrBool(node map[string]any, attrName string, optional bool) (bool, bool) {
	if v, ok := node[attrName]; ok {
		if bf, ok := v.(bool); ok {
			return bf, true
		}
	} else if optional {
		return false, true
	}
	return false, false
}

func convertAttrInt(node map[string]any, attrName string, optional bool) (int, bool) {
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

func convertAttrFloat(node map[string]any, attrName string, optional bool) (float64, bool) {
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

func convertAttrMapOfMap(node map[string]any, attrName string, optional bool) (map[string](map[string]any), bool) {
	if mm, ok := convertAttrMap(node, attrName, false); ok {
		resMap := make(map[string](map[string]any))
		for name, value := range mm {
			mv, ok := convertTypeMap(value)
			if !ok {
				return nil, false
			}
			resMap[name] = mv
		}
		return resMap, true
	} else if optional {
		return map[string](map[string]any){}, true
	} else {
		return nil, false
	}
}

func convertAttrListOfMap(node map[string]any, attrName string, optional bool) ([](map[string]any), bool) {
	if v, ok := node[attrName]; ok {
		if aList, ok := v.([]any); ok {
			arr := make([](map[string]any), 0)
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
		return [](map[string]any){}, true
	}
	return nil, false
}

func convertAttrListOfString(node map[string]any, attrName string, optional bool) ([]string, bool) {
	if v, ok := node[attrName]; ok {
		if aList, ok := v.([]any); ok {
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
