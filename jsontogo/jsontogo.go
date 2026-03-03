package jsontogo

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/format"
	"io"
	"os"
	"reflect"
	"strings"

	"github.com/Drelf2018/req"
)

var ErrInvalidDelim = errors.New("jsontogo: invalid delim")
var ErrInvalidToken = errors.New("jsontogo: invalid token")

func assert(dec *json.Decoder, token json.Delim) error {
	t, err := dec.Token()
	if err != nil {
		return err
	}
	if t, ok := t.(json.Delim); !ok || t != token {
		return ErrInvalidDelim
	}
	return nil
}

// 单个字段
//
// 这个结构体是为了实现有序的字典(map[string]any)创建的
//
// 众所周知 golang 的字典是无序的，因此为了保证导出的字段顺序与输入时一致
//
// 用了这个结构体的列表([]*object)来描述字典
type object struct {
	// 键名
	key string

	// 类型名
	rtype string

	// 是否在 json 标签中添加 ",omitempty"
	omit bool

	// 用来统计 array 中的 object 的这个键出现了几次
	// 如果出现次数等于 array 长度则代表不需要 omit
	count int

	// 值 可能的类型有 json.Token / []*object / []any
	value any

	// 注释 就是原本的值字符串
	comment string
}

// Converter 转换器，用于将 JSON 文本转换为能接收其值的结构体
type Converter struct {
	// 是否使用 interface{}
	UseInterface bool

	// 是否在字段后加注释
	AddComment bool

	// 已解读为 go 代码的字符
	buf *bytes.Buffer

	// JSON 解码器
	dec *json.Decoder

	// 当前缩进层数
	tab int
}

// buffer 返回非空 *bytes.Buffer
func (c *Converter) buffer() *bytes.Buffer {
	if c.buf == nil {
		c.buf = &bytes.Buffer{}
	}
	return c.buf
}

// add 添加字符串
func (c *Converter) add(s string) {
	c.buffer().WriteString(s)
}

// addTab 添加缩进
func (c *Converter) addTab() {
	c.add(strings.Repeat("\t", c.tab))
}

// parseValue 解析值
func (c *Converter) parseValue(objects []*object) (any, error) {
	t, err := c.dec.Token()
	if err != nil {
		return nil, err
	}
	if t, ok := t.(json.Delim); ok {
		switch t {
		case '{':
			// 以大括号开启代表接下来的值是一个对象
			// 调用对应方法
			return c.parseObject(objects)
		case '[':
			// 同理调用解析数组的方法
			return c.parseArray()
		}
	}
	return t, nil
}

// parseObject 解析对象
func (c *Converter) parseObject(objects []*object) ([]*object, error) {
outer:
	for c.dec.More() {
		t, err := c.dec.Token()
		if err != nil {
			return nil, err
		}

		key, ok := t.(string)
		if !ok {
			return nil, ErrInvalidToken
		}

		// 遍历这个列表
		// 如果已经保存过该字段 计数加 1
		// 直接读取并舍弃它的 Value
		for _, obj := range objects {
			if obj.key == key {
				obj.count++
				_, err := c.parseValue(nil)
				if err != nil {
					return nil, err
				}
				continue outer
			}
		}

		// 否则获取值并解析类型
		// 再添加进 objects 列表
		value, err := c.parseValue([]*object{})
		if err != nil {
			return nil, err
		}

		rtype, comment := ParseType(value)
		if rtype == "any" && c.UseInterface {
			rtype = "interface{}"
		}

		if objects != nil {
			objects = append(objects, &object{
				key:     key,
				rtype:   rtype,
				count:   1,
				value:   value,
				comment: comment,
			})
		}
	}

	// 劲爆尾杀
	return objects, assert(c.dec, '}')
}

// parseArray 解析数组
func (c *Converter) parseArray() (val []any, err error) {
	// 计数组长度
	length := 0
	// 数组中可能存在的对象
	objects := make([]*object, 0)
	for c.dec.More() {
		length++
		v, err := c.parseValue(objects)
		if err != nil {
			return nil, err
		}
		if objs, ok := v.([]*object); ok {
			// 如果包含对象则赋值
			// 类似 append 方法
			objects = objs
		} else {
			// 否则在返回值中添加解析出来的值
			val = append(val, v)
		}
	}
	// 劲爆尾杀
	err = assert(c.dec, ']')
	if err != nil {
		return
	}
	// 列表中没东西有两种情况
	// 一种是真没内容 => []any
	// 还有一种是里面有对象保存在 objects 中而非 val 中
	if len(val) == 0 {
		for _, obj := range objects {
			// 处理每一个字段的 omit
			// 原理参考 count 字段上的注释
			obj.omit = obj.count != length
		}
	}
	// 对象非空 添加进列表的值中
	if len(objects) != 0 {
		val = append(val, objects)
	}
	return
}

// writeObjects 向 *bytes.Buffer 写入结构体字符串
func (c *Converter) writeObjects(objects []*object) (err error) {
	c.add("struct {\n")
	c.tab++
	for _, obj := range objects {
		// 如果注释中有换行符 那么就写在字段的上方 并且每个换行都是注释
		hasNewLine := strings.Contains(obj.comment, "\n")
		if c.AddComment && hasNewLine {
			for _, comment := range strings.Split(obj.comment, "\n") {
				c.addTab()
				c.add("// ")
				c.add(comment)
				c.add("\n")
			}
		}
		// 写入正规化的字段名
		c.addTab()
		c.add(Regularize(obj.key))
		c.add(" ")
		// 根据该字段类型决定接下来如何写入
		switch obj.rtype {
		case "struct":
			err = c.writeObjects(obj.value.([]*object))
		case "slice":
			err = c.writeArray(obj.value.([]any))
		default:
			c.add(obj.rtype)
		}
		if err != nil {
			return nil
		}
		// 写入 json 标签
		c.add(" `json:\"")
		c.add(obj.key)
		if obj.omit {
			c.add(",omitempty")
		}
		c.add("\"`")
		// 写入注释
		if c.AddComment && obj.comment != "" && !hasNewLine {
			c.add(" // ")
			c.add(obj.comment)
		}
		c.add("\n")
	}
	// 细节闭合括号
	c.tab--
	c.addTab()
	c.add("}")
	return nil
}

// writeArray 向 *bytes.Buffer 写入数组字符串
func (c *Converter) writeArray(val []any) (err error) {
	c.add("[]")

	// 对象数组 []struct
	if len(val) == 1 {
		if o, ok := val[0].([]*object); ok {
			err = c.writeObjects(o)
			return
		}
	}

	// 非对象数组 []int / []string / [][]int / ...
	var rtype string
	for idx := 0; idx < len(val); idx++ {
		ntype, _ := ParseType(val[idx])
		if rtype == "" {
			rtype = ntype
		} else if rtype != ntype {
			rtype = CompareNumberType(ntype, rtype)
			if rtype == "any" {
				break
			}
		}
	}

	if rtype == "slice" {
		c.writeArray(val[0].([]any))
		return
	}

	if rtype == "" {
		rtype = "any"
	}
	if rtype == "any" && c.UseInterface {
		rtype = "interface{}"
	}
	c.add(rtype)
	return nil
}

// String 已解析内容
func (c *Converter) String() string {
	return c.buffer().String()
}

// Struct 输出格式化结构体
func (c *Converter) Struct(name string) ([]byte, error) {
	name = Regularize(name)
	if name == "" {
		name = "AutoGenerated"
	}
	code := "type " + name + " " + c.buffer().String()
	return format.Source([]byte(code))
}

// UnmarshalJSON 反序列化 JSON
func (c *Converter) UnmarshalJSON(data []byte) error {
	c.tab = 0
	c.buf = &bytes.Buffer{}

	// 避免小数被解析成整数
	data = bytes.ReplaceAll(data, []byte(".0"), []byte(".1"))
	c.dec = json.NewDecoder(bytes.NewReader(data))
	c.dec.UseNumber()

	err := assert(c.dec, '{')
	if err != nil {
		return err
	}

	objects, err := c.parseObject([]*object{})
	if err != nil {
		return err
	}

	_, err = c.dec.Token()
	if err != io.EOF {
		return err
	}

	return c.writeObjects(objects)
}

var _ json.Unmarshaler = (*Converter)(nil)

// Unmarshal 将 JSON 转换成结构体字符串
//
// 参数 name 为最外层结构体名字（会进行正规化）
func (c *Converter) Unmarshal(data []byte, name string) ([]byte, error) {
	err := c.UnmarshalJSON(data)
	if err != nil {
		return nil, err
	}
	return c.Struct(name)
}

func (c *Converter) UnmarshalAPI(api req.API) error {
	b, err := req.Content(api)
	if err != nil {
		return err
	}
	name := reflect.ValueOf(api).Type().Name()
	b, err = c.Unmarshal(b, name+"Response")
	if err != nil {
		return err
	}
	if _, err := os.Stat(name + ".txt"); err != nil {
		return os.WriteFile(name+".txt", b, os.ModePerm)
	}
	return fmt.Errorf("jsontogo: file already exists: %q", name+".txt")
}

var DefaultConverter = &Converter{}

func Unmarshal(data []byte, name string) ([]byte, error) {
	return DefaultConverter.Unmarshal(data, name)
}

func UnmarshalAPI(api req.API) error {
	return DefaultConverter.UnmarshalAPI(api)
}
