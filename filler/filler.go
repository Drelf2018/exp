package filler

import (
	"fmt"
	"reflect"
	"strings"
	"text/template"
)

// parse 遍历导出的字段，当其类型是字符串且值不为空时创建模板，当其类型是结构体时递归遍历
func parse(tmpl *template.Template, prefix string, elem reflect.Value) error {
	structType := elem.Type()
	for i := 0; i < structType.NumField(); i++ {
		fieldType := structType.Field(i)
		if !fieldType.IsExported() {
			continue
		}
		switch fieldType.Type.Kind() {
		case reflect.String:
			field := elem.Field(i)
			if field.IsZero() {
				continue
			}
			_, err := tmpl.New(prefix + "." + fieldType.Name).Parse(field.String())
			if err != nil {
				return err
			}
		case reflect.Struct:
			err := parse(tmpl, prefix+"."+fieldType.Name, elem.Field(i))
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// Parse 将结构体对象解析在已有模板中
func Parse(tmpl *template.Template, v any) error {
	elem := reflect.ValueOf(v)
	if elem.Kind() == reflect.Pointer {
		elem = elem.Elem()
	}
	if elem.Kind() != reflect.Struct {
		return fmt.Errorf("filler: invalid value: expected (a pointer to) a struct, got: %T(%v)", v, elem.Kind())
	}
	return parse(tmpl, elem.Type().Name(), elem)
}

// New 将结构体对象解析在新模板中
func New(v any) (*template.Template, error) {
	tmpl := template.New("")
	return tmpl, Parse(tmpl, v)
}

// fill 填充结构体对象字段值
func fill(tmpl *template.Template, prefix string, data any, elem reflect.Value) error {
	var b strings.Builder
	structType := elem.Type()
	for i := 0; i < structType.NumField(); i++ {
		fieldType := structType.Field(i)
		if !fieldType.IsExported() {
			continue
		}
		switch fieldType.Type.Kind() {
		case reflect.String:
			field := elem.Field(i)
			if !field.IsZero() || !field.CanSet() {
				continue
			}
			templateName := prefix + "." + fieldType.Name
			t := tmpl.Lookup(templateName)
			if t == nil {
				continue
			}
			if err := t.Execute(&b, data); err != nil {
				return fmt.Errorf("filler: failed to execute template %q: %w", templateName, err)
			}
			field.SetString(b.String())
			b.Reset()
		case reflect.Struct:
			err := fill(tmpl, prefix+"."+fieldType.Name, data, elem.Field(i))
			if err != nil {
				return err
			}
		}

	}
	return nil
}

// Fill 填充结构体对象字段值，必须传入其指针
func Fill(tmpl *template.Template, data any, v any) error {
	val := reflect.ValueOf(v)
	if val.Kind() != reflect.Pointer || val.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("filler: invalid value: expected a pointer to a struct, got: %T(%v)", v, val.Kind())
	}
	return fill(tmpl, val.Elem().Type().Name(), data, val.Elem())
}
