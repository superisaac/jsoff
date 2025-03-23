package jsoffschema

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"github.com/superisaac/jsoff"
	"strings"
)

// schema validator
func (pos ErrorPos) Path() string {
	return strings.Join(pos.paths, "")
}

func (pos ErrorPos) Error() string {
	return fmt.Sprintf("Validation Error: %s %s", pos.Path(), pos.hint)
}

func (pos ErrorPos) ToMessage(reqmsg *jsoff.RequestMessage) *jsoff.ErrorMessage {
	err := &jsoff.RPCError{
		Code:    jsoff.ErrInvalidSchema.Code,
		Message: pos.Error(),
		Data:    nil}
	return err.ToMessage(reqmsg)
}

func NewSchemaValidator() *SchemaValidator {
	return &SchemaValidator{}
}

func (validator *SchemaValidator) NewErrorPos(hint string) *ErrorPos {
	var newPaths []string
	newPaths = append(newPaths, validator.paths...)

	// for _, path := range validator.paths {
	// 	newPaths = append(newPaths, path)
	// }
	return &ErrorPos{paths: newPaths, hint: hint}
}

func (validator *SchemaValidator) ValidateBytes(schema Schema, data []byte) *ErrorPos {
	var v any
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	err := dec.Decode(&v)
	if err != nil {
		panic(err)
	}
	return validator.Scan(schema, "", v)
}

func (validator *SchemaValidator) Validate(schema Schema, data any) *ErrorPos {
	return validator.Scan(schema, "", data)
}

func (validator *SchemaValidator) pushPath(path string) {
	if path != "" {
		validator.paths = append(validator.paths, path)
	}
}

func (validator *SchemaValidator) popPath(path string) {
	if path != "" {
		if len(validator.paths) < 1 || validator.paths[len(validator.paths)-1] != path {
			panic(errors.New(fmt.Sprintf("pop path %s is different from stack top %s", path, validator.paths[len(validator.paths)-1])))
		}
		validator.paths = validator.paths[:len(validator.paths)-1]
	}
}

func (validator *SchemaValidator) Scan(schema Schema, path string, data any) *ErrorPos {
	validator.pushPath(path)
	errPos := schema.Scan(validator, data)
	validator.popPath(path)
	return errPos
}
