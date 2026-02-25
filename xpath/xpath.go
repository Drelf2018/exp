package xpath

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"
)

// UnmarshalValue 将节点反序列化进 reflect.Value
func UnmarshalValue(nodes []*html.Node, value reflect.Value) error {
	if len(nodes) == 0 {
		if !value.IsZero() && value.CanSet() {
			value.Set(reflect.Zero(value.Type()))
		}
		return nil
	}
	switch kind := value.Kind(); kind {
	case reflect.String:
		node := nodes[0]
		if node.Type == html.TextNode {
			value.SetString(node.Data)
		} else if node.LastChild != nil {
			value.SetString(node.LastChild.Data)
		}
		return nil
	case reflect.Pointer:
		if value.IsNil() {
			value.Set(reflect.New(value.Type().Elem()))
		}
		return UnmarshalValue(nodes, value.Elem())
	case reflect.Array:
		length := value.Len()
		if len(nodes) < length {
			length = len(nodes)
		}
		for idx := 0; idx < length; idx++ {
			err := UnmarshalValue(nodes[idx:idx+1], value.Index(idx))
			if err != nil {
				return err
			}
		}
		return nil
	case reflect.Slice:
		value.Set(reflect.MakeSlice(value.Type(), len(nodes), len(nodes)))
		for idx := range nodes {
			err := UnmarshalValue(nodes[idx:idx+1], value.Index(idx))
			if err != nil {
				return err
			}
		}
		return nil
	case reflect.Struct:
		node := nodes[0]
		valueType := value.Type()
		for i := 0; i < valueType.NumField(); i++ {
			field := valueType.Field(i)
			if !field.IsExported() {
				continue
			}
			tag := field.Tag.Get("xpath")
			if tag == "" {
				continue
			}
			children, err := htmlquery.QueryAll(node, tag)
			if err != nil {
				return err
			}
			err = UnmarshalValue(children, value.Field(i))
			if err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("xpath: invalid value type: %v", value)
	}
}

// XPath 用来提供初始 XPath 路径
type XPath interface {
	XPath() string
}

// UnmarshalNode 将节点反序列化进对象
func UnmarshalNode(node *html.Node, v any) error {
	if v, ok := v.(XPath); ok {
		children, err := htmlquery.QueryAll(node, v.XPath())
		if err != nil {
			return err
		}
		return UnmarshalValue(children, reflect.ValueOf(v))
	}
	return UnmarshalValue([]*html.Node{node}, reflect.ValueOf(v))
}

// UnmarshalReader 将读取器反序列化进对象
func UnmarshalReader(r io.Reader, v any) error {
	node, err := html.Parse(r)
	if err != nil {
		return err
	}
	return UnmarshalNode(node, v)
}

// UnmarshalText 将文本反序列化进对象
func UnmarshalText(text string, v any) error {
	return UnmarshalReader(strings.NewReader(text), v)
}

// Unmarshal 将字节切片反序列化进对象
func Unmarshal(b []byte, v any) error {
	return UnmarshalReader(bytes.NewReader(b), v)
}

// LoadURL 加载链接并反序列化进对象
func LoadURL(url string, v any) error {
	node, err := htmlquery.LoadURL(url)
	if err != nil {
		return err
	}
	return UnmarshalNode(node, v)
}

// LoadDoc 加载文件并反序列化进对象
func LoadDoc(path string, v any) error {
	node, err := htmlquery.LoadDoc(path)
	if err != nil {
		return err
	}
	return UnmarshalNode(node, v)
}
