package schema

import (
	"fmt"
	simplejson "github.com/bitly/go-simplejson"
	"github.com/pkg/errors"
	"github.com/superisaac/jsonz"
	"strings"
)

// schema validator
func (self ErrorPos) Path() string {
	return strings.Join(self.paths, "")
}

func (self ErrorPos) Error() string {
	return fmt.Sprintf("Validation Error: %s %s", self.Path(), self.hint)
}

func (self ErrorPos) ToMessage(reqmsg *jsonz.RequestMessage) *jsonz.ErrorMessage {
	err := &jsonz.RPCError{
		Code:    jsonz.ErrInvalidSchema.Code,
		Message: self.Error(),
		Data:    nil}
	return err.ToMessage(reqmsg)
}

func NewSchemaValidator() *SchemaValidator {
	return &SchemaValidator{}
}

func (self *SchemaValidator) NewErrorPos(hint string) *ErrorPos {
	var newPaths []string
	for _, path := range self.paths {
		newPaths = append(newPaths, path)
	}
	return &ErrorPos{paths: newPaths, hint: hint}
}

func (self *SchemaValidator) ValidateBytes(schema Schema, bytes []byte) *ErrorPos {
	data, err := simplejson.NewJson(bytes)
	if err != nil {
		panic(err)
	}
	return self.Scan(schema, "", data.Interface())
}

func (self *SchemaValidator) Validate(schema Schema, data interface{}) *ErrorPos {
	return self.Scan(schema, "", data)
}

func (self *SchemaValidator) pushPath(path string) {
	if path != "" {
		self.paths = append(self.paths, path)
	}
}

func (self *SchemaValidator) popPath(path string) {
	if path != "" {
		if len(self.paths) < 1 || self.paths[len(self.paths)-1] != path {
			panic(errors.New(fmt.Sprintf("pop path %s is different from stack top %s", path, self.paths[len(self.paths)-1])))
		}
		self.paths = self.paths[:len(self.paths)-1]
	}
}

func (self *SchemaValidator) Scan(schema Schema, path string, data interface{}) *ErrorPos {
	self.pushPath(path)
	errPos := schema.Scan(self, data)
	self.popPath(path)
	return errPos
}
