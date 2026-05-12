package pydantic

import (
	"bytes"
	"encoding"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/Drelf2018/req/method"
)

// Indent 默认缩进为四个空格
var Indent string = "    "

// Model 用来描述一个结构体
type Model struct {
	Name    string      // 结构体名称
	Inline  []*Model    // 在字段中定义的内联结构体
	Fields  [][4]string // 导出的字段的下划线命名、pydantic 模型中允许的类型名、字段别名以及字段注释
	Parents []string    // 嵌入在本结构体中的类型名
}

// marshalText 将模型输出为 BaseModel 对象，参数为这个对象整体缩进层数
func (m *Model) marshalText(buf *bytes.Buffer, depth int) {
	// 写入首行
	buf.WriteString(strings.Repeat(Indent, depth))
	buf.WriteString("class ")
	buf.WriteString(m.Name)
	buf.WriteString("(")
	if len(m.Parents) != 0 {
		buf.WriteString(strings.Join(m.Parents, ", "))
	} else {
		buf.WriteString("BaseModel")
	}
	buf.WriteString("):")
	// 写入对象内容
	prefix := strings.Repeat(Indent, depth+1)
	// 内容为空提前返回
	if len(m.Inline) == 0 && len(m.Fields) == 0 {
		buf.WriteByte('\n')
		buf.WriteString(prefix)
		buf.WriteString("pass")
		return
	}
	// 先写入内联对象
	if len(m.Inline) != 0 {
		for _, a := range m.Inline {
			buf.WriteString("\n\n")
			a.marshalText(buf, depth+1)
		}
		if len(m.Fields) != 0 {
			buf.WriteByte('\n')
		}
	}
	// 再写入字段
	for _, f := range m.Fields {
		buf.WriteByte('\n')
		buf.WriteString(prefix)
		buf.WriteString(f[0])
		buf.WriteString(": ")
		buf.WriteString(f[1])
		if f[2] != "" && f[2] != f[0] {
			buf.WriteString(" = Field(alias=\"")
			buf.WriteString(f[2])
			buf.WriteString("\")")
		}
		if f[3] != "" {
			buf.WriteString("  # ")
			buf.WriteString(f[3])
		}
	}
}

// MarshalText 将模型输出为 BaseModel 对象
func (m *Model) MarshalText() ([]byte, error) {
	buf := &bytes.Buffer{}
	m.marshalText(buf, 0)
	return buf.Bytes(), nil
}

var _ encoding.TextMarshaler = (*Model)(nil)

// File 用来保存同一个包内的结构体
type File struct {
	Name   string           `json:"name"`   // 包名
	Files  map[string]*File `json:"-"`      // 包名映射表
	Import []string         `json:"import"` // 导入的其他包
	Models []*Model         `json:"models"` // 从包内读取到的结构体模型
}

// find 找到此结构体所在包对应的文件
func (f *File) find(t reflect.Type) *File {
	if f.Files == nil {
		f.Files = make(map[string]*File)
	}
	pkg := strings.ReplaceAll(t.PkgPath(), "-", "_")
	file, ok := f.Files[pkg]
	if !ok {
		file = &File{Name: pkg, Files: f.Files}
		f.Files[pkg] = file
	}
	return file
}

// importFile 用来不重复导入包名
func (f *File) importFile(name string) {
	for _, i := range f.Import {
		if i == name {
			return
		}
	}
	f.Import = append(f.Import, name)
}

// parseSturct 获取结构体的 reflect.Type 转化成的模型
func (f *File) parseStruct(t reflect.Type, fieldName string) *Model {
	// 检查是否已经保存过这个结构体
	for _, saved := range f.Models {
		if saved.Name == fieldName {
			return saved
		}
	}
	// 将结构体转为模型，如果其不是内联则立即添加进模型组，
	// 在它使用的字段类型也使用了包含它的类型时可以退出递归
	model := &Model{Name: fieldName}
	if t.Name() != "" {
		f.Models = append(f.Models, model)
	}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		if field.Anonymous {
			model.Parents = append(model.Parents, f.parse(field.Type, field.Name, model))
			continue
		}
		alias := strings.TrimSuffix(field.Tag.Get("json"), ",omitempty")
		if alias == "-" {
			continue
		}
		comment := field.Tag.Get("comment")
		if comment == "" {
			comment = field.Tag.Get("description")
		}
		model.Fields = append(model.Fields, [4]string{method.CamelToSnake(field.Name), f.parse(field.Type, field.Name, model), alias, comment})
	}
	return model
}

// parse 获取 reflect.Type 在 pydantic 模型中允许的类型名
func (f *File) parse(t reflect.Type, fieldName string, model *Model) string {
	if t == nil {
		return "Any"
	}
	switch t.Kind() {
	case reflect.Bool:
		return "bool"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return "int"
	case reflect.Float32, reflect.Float64:
		return "float"
	case reflect.String:
		return "str"
	case reflect.Array, reflect.Slice:
		if t.Elem().Kind() == reflect.Uint8 {
			return "bytes"
		}
		return "List[" + f.parse(t.Elem(), strings.TrimSuffix(fieldName, "s"), model) + "]"
	case reflect.Map:
		return "Dict[" + f.parse(t.Key(), fieldName+"Key", model) + ", " + f.parse(t.Elem(), fieldName+"Value", model) + "]"
	case reflect.Struct:
		pkg := strings.ReplaceAll(t.PkgPath(), "-", "_")
		name := t.Name()
		// 是否为特殊类型
		switch pkg + "." + name {
		case "time.Time":
			f.importFile("datetime")
			return "datetime.datetime"
		}
		// 不处于相同包内的需要导入，在同一个包的需要判断是否为内联或者嵌入
		if pkg != f.Name && pkg != "" {
			f.importFile(pkg)
			f.find(t).parseStruct(t, name)
			return filepath.Base(pkg) + "." + name
		} else {
			if name == "" {
				model.Inline = append(model.Inline, f.parseStruct(t, fieldName))
				return fieldName
			} else {
				f.parseStruct(t, name)
				for _, m := range f.Models {
					switch m.Name {
					case name:
						return "\"" + name + "\""
					case model.Name:
						return name
					}
				}
				return name
			}
		}
	case reflect.Pointer:
		for t.Kind() == reflect.Pointer {
			t = t.Elem()
		}
		return "Optional[" + f.parse(t, fieldName, model) + "]"
	default:
		return "Any"
	}
}

// MarshalText 输出导入模型和已解析模型
func (f *File) MarshalText() ([]byte, error) {
	buf := &bytes.Buffer{}
	for _, i := range f.Import {
		_, err := fmt.Fprintf(buf, "import %s\n", filepath.Base(i))
		if err != nil {
			return nil, err
		}
	}
	_, err := buf.WriteString("from typing import Any, Dict, List, Optional\n\nfrom pydantic import BaseModel, Field")
	if err != nil {
		return nil, err
	}
	for i := range f.Models {
		_, err = buf.WriteString("\n\n\n")
		if err != nil {
			return nil, err
		}
		b, err := f.Models[len(f.Models)-i-1].MarshalText()
		if err != nil {
			return nil, err
		}
		_, err = buf.Write(b)
		if err != nil {
			return nil, err
		}
	}
	buf.WriteByte('\n')
	return buf.Bytes(), nil
}

var _ encoding.TextMarshaler = (*File)(nil)

// Save 将导入模型和已解析模型写入文件
func (f *File) Save(path string) error {
	b, err := f.MarshalText()
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(path, filepath.Base(f.Name+".py")), b, os.ModePerm)
}

// Parse 解析一个结构体
func (f *File) Parse(v any) {
	t := reflect.TypeOf(v)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		panic(fmt.Errorf("invalid type: expected (a pointer to) a struct, got: %v", t))
	}
	f.find(t).parseStruct(t, t.Name())
}

// Parse 解析一个结构体，返回其所需的所有文件
func Parse(v ...any) map[string]*File {
	f := &File{Files: map[string]*File{}}
	for _, i := range v {
		f.Parse(i)
	}
	return f.Files
}

// Save 将导入模型和已解析模型写入文件
func Save(path string, v ...any) error {
	err := os.MkdirAll(path, os.ModePerm)
	if err != nil {
		return err
	}
	for _, file := range Parse(v...) {
		err = file.Save(path)
		if err != nil {
			return err
		}
	}
	return nil
}
