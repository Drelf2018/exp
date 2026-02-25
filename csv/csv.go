package csv

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"encoding/csv"
)

// key 包含字段的索引和标签值
type key struct {
	index int
	value string
}

// keyCache 缓存类型的索引顺序切片
var keyCache sync.Map // map[reflect.Type][]key

// keys 返回结构体类型中字段的 csv 标签与其索引的顺序切片
func keys(t reflect.Type) []key {
	if h, ok := keyCache.Load(t); ok {
		return h.([]key)
	}
	h := make([]key, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		if field := t.Field(i); field.IsExported() {
			if tag := field.Tag.Get("csv"); tag != "" {
				h = append(h, key{i, tag})
			}
		}
	}
	keyCache.Store(t, h)
	return h
}

// ReaderHandler 是 *csv.Reader 预处理函数
type ReaderHandler func(*csv.Reader)

// Comma 用来指定字段分隔符
func Comma(c rune) ReaderHandler {
	return func(reader *csv.Reader) {
		reader.Comma = c
	}
}

// Comment 用来指定注释字符
func Comment(c rune) ReaderHandler {
	return func(reader *csv.Reader) {
		reader.Comment = c
	}
}

// FieldsPerRecord 用来指定每条记录预期包含的字段数量
func FieldsPerRecord(i int) ReaderHandler {
	return func(reader *csv.Reader) {
		reader.FieldsPerRecord = i
	}
}

// LazyQuotes 引号可以出现在未加引号的字段中，且未被双写的引号也可以出现在加引号的字段中
func LazyQuotes(reader *csv.Reader) {
	reader.LazyQuotes = true
}

var _ ReaderHandler = LazyQuotes

// TrimLeadingSpace 字段中的前导空格将被忽略
func TrimLeadingSpace(reader *csv.Reader) {
	reader.TrimLeadingSpace = true
}

var _ ReaderHandler = TrimLeadingSpace

// Unmarshaler 反序列化 CSV 数据的接口
type Unmarshaler interface {
	UnmarshalCSV(string) error
}

// unmarshalerType 是 Unmarshaler 接口的反射类型
var unmarshalerType = reflect.TypeOf((*Unmarshaler)(nil)).Elem()

// unmarshal 将字符串反序列化至反射值
func unmarshal(s string, value reflect.Value) error {
	valueType := value.Type()
	if valueType.Implements(unmarshalerType) {
		switch value.Kind() {
		case reflect.Pointer:
			if value.IsNil() {
				value.Set(reflect.New(valueType.Elem()))
			}
		case reflect.Map:
			if value.IsNil() {
				value.Set(reflect.MakeMap(valueType))
			}
		}
		return value.Interface().(Unmarshaler).UnmarshalCSV(s)
	}
	switch kind := value.Kind(); kind {
	case reflect.String:
		value.SetString(s)
		return nil
	case reflect.Pointer:
		if value.IsNil() {
			value.Set(reflect.New(valueType.Elem()))
		}
		return unmarshal(s, value.Elem())
	case reflect.Bool:
		x, err := strconv.ParseBool(s)
		if err != nil {
			return err
		}
		value.SetBool(x)
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		var bitSize int
		if kind >= reflect.Int8 {
			bitSize = 1 << kind
		}
		x, err := strconv.ParseInt(s, 10, bitSize)
		if err != nil {
			return err
		}
		value.SetInt(x)
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		var bitSize int
		if kind >= reflect.Uint8 {
			bitSize = 1 << (kind - 5)
		}
		x, err := strconv.ParseUint(s, 10, bitSize)
		if err != nil {
			return err
		}
		value.SetUint(x)
		return nil
	case reflect.Float32, reflect.Float64:
		x, err := strconv.ParseFloat(s, 1<<(kind-8))
		if err != nil {
			return err
		}
		value.SetFloat(x)
		return nil
	default:
		if kind == reflect.Slice && valueType.Elem().Kind() == reflect.Uint8 {
			value.SetBytes([]byte(s))
			return nil
		}
		return fmt.Errorf("csv: failed to unmarshal %q to %v", s, valueType)
	}
}

// UnmarshalCSVReader 将 *csv.Reader 中的数据反序列化为切片
func UnmarshalCSVReader[T any](reader *csv.Reader, handlers ...ReaderHandler) ([]T, error) {
	// 设置 csv 读取器参数
	for _, handler := range handlers {
		handler(reader)
	}
	// 强制复用字符串切片
	reader.ReuseRecord = true
	// 获取读取器首行作为表头
	values, err := reader.Read()
	if err != nil {
		return nil, err
	}
	first := make(map[string]int, len(values))
	for i, value := range values {
		// 移除 Unicode BOM
		if i == 0 {
			value = strings.TrimPrefix(value, "\uFEFF")
		}
		first[value] = i
	}
	// 获取首行键顺序到结构体字段索引的映射
	indexes := make(map[int]int, len(values))
	for _, key := range keys(reflect.TypeOf((*T)(nil)).Elem()) {
		if index, ok := first[key.value]; ok {
			indexes[index] = key.index
		}
	}
	// 表头不存在直接返回零值
	if len(indexes) == 0 {
		return nil, nil
	}
	// 读取所有数据
	records := make([]T, 0)
	for {
		values, err = reader.Read()
		if err == io.EOF {
			return records, nil
		} else if err != nil {
			return nil, err
		}
		var record T
		elem := reflect.ValueOf(&record).Elem()
		for i, value := range values {
			if index, ok := indexes[i]; ok {
				err := unmarshal(value, elem.Field(index))
				if err != nil {
					return nil, err
				}
			}
		}
		records = append(records, record)
	}
}

// UnmarshalReader 将 io.Reader 中的数据反序列化为切片
func UnmarshalReader[T any](r io.Reader, handlers ...ReaderHandler) ([]T, error) {
	return UnmarshalCSVReader[T](csv.NewReader(r), handlers...)
}

// UnmarshalFile 将文件中的数据反序列化为切片
func UnmarshalFile[T any](name string, handlers ...ReaderHandler) (records []T, err error) {
	f, err := os.Open(name)
	if err != nil {
		return
	}
	defer f.Close()
	records, err = UnmarshalReader[T](f, handlers...)
	return
}

// Unmarshal 将字符切片反序列化为切片
func Unmarshal[T any](data []byte, handlers ...ReaderHandler) (records []T, err error) {
	return UnmarshalReader[T](bytes.NewReader(data), handlers...)
}

// UnmarshalText 将字符串反序列化为切片
func UnmarshalText[T any](s string, handlers ...ReaderHandler) (records []T, err error) {
	return UnmarshalReader[T](strings.NewReader(s), handlers...)
}

// WriterHandler 是 *csv.Writer 预处理函数
type WriterHandler func(*csv.Writer)

// WriterComma 用来指定字段分隔符
func WriterComma(c rune) WriterHandler {
	return func(writer *csv.Writer) {
		writer.Comma = c
	}
}

// UseCRLF 使用 \r\n 作为行终止符
func UseCRLF(writer *csv.Writer) {
	writer.UseCRLF = true
}

var _ WriterHandler = UseCRLF

// Marshaler 序列化 CSV 数据的接口
type Marshaler interface {
	MarshalCSV() (string, error)
}

// marshal 将反射值序列化为字符串
func marshal(value reflect.Value) (string, error) {
	valueType := value.Type()
	if v, ok := value.Interface().(Marshaler); ok {
		return v.MarshalCSV()
	}
	switch kind := value.Kind(); kind {
	case reflect.String:
		return value.String(), nil
	case reflect.Pointer:
		if value.IsNil() {
			return "", nil
		}
		return marshal(value.Elem())
	case reflect.Bool:
		return strconv.FormatBool(value.Bool()), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(value.Int(), 10), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(value.Uint(), 10), nil
	case reflect.Float32:
		return strconv.FormatFloat(value.Float(), 'f', -1, 32), nil
	case reflect.Float64:
		return strconv.FormatFloat(value.Float(), 'f', -1, 64), nil
	default:
		if kind == reflect.Slice && valueType.Elem().Kind() == reflect.Uint8 {
			return string(value.Bytes()), nil
		}
		return "", fmt.Errorf("csv: failed to marshal %v", valueType)
	}
}

// Header 用来指定表头字段顺序
type Header interface {
	CSVHeader() []string
}

// MarshalCSVWriter 将切片序列化并写入 *csv.Writer
func MarshalCSVWriter[T any](writer *csv.Writer, records []T, handlers ...WriterHandler) error {
	if len(records) == 0 {
		return nil
	}
	// 设置 csv 写入器参数
	for _, handler := range handlers {
		handler(writer)
	}
	// 获取表头和结构体字段索引
	var value []string
	var indexes []key
	if r, ok := any(records[0]).(Header); ok {
		keymap := make(map[string]key)
		for _, key := range keys(reflect.TypeOf((*T)(nil)).Elem()) {
			keymap[key.value] = key
		}
		value = r.CSVHeader()
		for _, value := range value {
			key, ok := keymap[value]
			if !ok {
				key.index = -1
			}
			indexes = append(indexes, key)
		}
	} else {
		indexes = keys(reflect.TypeOf((*T)(nil)).Elem())
		value = make([]string, 0, len(indexes))
		for _, h := range indexes {
			value = append(value, h.value)
		}
	}
	if len(value) == 0 {
		return nil
	}
	// 写入表头
	err := writer.Write(value)
	if err != nil {
		return err
	}
	if len(indexes) == 0 {
		return nil
	}
	elem := reflect.ValueOf(records)
	for index := range records {
		record := elem.Index(index)
		for i, key := range indexes {
			if key.index >= 0 {
				value[i], err = marshal(record.Field(key.index))
				if err != nil {
					return err
				}
			} else {
				value[i] = ""
			}
		}
		err = writer.Write(value)
		if err != nil {
			return err
		}
	}

	writer.Flush()
	return writer.Error()
}

// MarshalWriter 将切片序列化并写入 io.Writer
func MarshalWriter[T any](w io.Writer, records []T, handlers ...WriterHandler) error {
	return MarshalCSVWriter(csv.NewWriter(w), records, handlers...)
}

// MarshalFile 将切片序列化并写入文件
func MarshalFile[T any](name string, records []T, handlers ...WriterHandler) error {
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return err
	}
	defer f.Close()
	// 写入 Unicode BOM
	_, err = f.WriteString("\uFEFF")
	if err != nil {
		return err
	}
	return MarshalWriter(f, records, handlers...)
}

// Marshal 将切片序列化为字符切片
func Marshal[T any](records []T, handlers ...WriterHandler) ([]byte, error) {
	buf := &bytes.Buffer{}
	err := MarshalWriter(buf, records, handlers...)
	return buf.Bytes(), err
}

// MarshalText 将切片序列化为字符串
func MarshalText[T any](records []T, handlers ...WriterHandler) (string, error) {
	b := &strings.Builder{}
	err := MarshalWriter(b, records, handlers...)
	return b.String(), err
}
