// Package skill — schema.go
//
// Reflection-based JSON Schema generator.
// Given any Go struct, GenerateSchema walks its exported fields and produces
// a map[string]any that conforms to JSON Schema Draft-07.  This lets skill
// authors define their input as a plain Go struct and get the schema for free.
package skill

import (
	"reflect"
	"strings"
)

// GenerateSchema takes a Go struct (or a pointer to one) and returns a
// JSON-Schema-compatible map describing its fields.
//
// Supported field tags:
//
//	json:"name"        → property key (omit with "-")
//	schema:"required"  → marks the field as required
//	description:"..."  → becomes the "description" keyword
//
// Example:
//
//	type ShellInput struct {
//	    Command string `json:"command" schema:"required" description:"Shell command to execute"`
//	    Timeout int    `json:"timeout" description:"Max seconds to wait"`
//	}
//	schema := skill.GenerateSchema(ShellInput{})
func GenerateSchema(v any) map[string]any {
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return map[string]any{"type": "object"}
	}

	properties := make(map[string]any)
	var required []string

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}

		// Resolve property name from json tag, fall back to field name.
		name := field.Name
		if tag := field.Tag.Get("json"); tag != "" {
			parts := strings.Split(tag, ",")
			if parts[0] == "-" {
				continue
			}
			if parts[0] != "" {
				name = parts[0]
			}
		}

		prop := map[string]any{
			"type": goTypeToJSONType(field.Type),
		}

		if desc := field.Tag.Get("description"); desc != "" {
			prop["description"] = desc
		}

		if field.Tag.Get("schema") == "required" {
			required = append(required, name)
		}

		properties[name] = prop
	}

	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

// goTypeToJSONType maps basic Go reflect.Kind values to JSON Schema type strings.
func goTypeToJSONType(t reflect.Type) string {
	switch t.Kind() {
	case reflect.String:
		return "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return "integer"
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "integer"
	case reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Bool:
		return "boolean"
	case reflect.Slice, reflect.Array:
		return "array"
	case reflect.Map, reflect.Struct:
		return "object"
	default:
		return "string"
	}
}
