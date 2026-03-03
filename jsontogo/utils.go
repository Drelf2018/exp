package jsontogo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Abbreviation 是需要保持全大写的缩写词的映射，值为真时生效
var Abbreviation = map[string]bool{
	"API":   true,
	"ASCII": true,
	"CPU":   true,
	"CSS":   true,
	"DNS":   true,
	"EOF":   true,
	"GUID":  true,
	"HTML":  true,
	"HTTP":  true,
	"HTTPS": true,
	"ID":    true,
	"IP":    true,
	"JSON":  true,
	"LHS":   true,
	"QPS":   true,
	"RAM":   true,
	"RHS":   true,
	"RPC":   true,
	"SLA":   true,
	"SMTP":  true,
	"SSH":   true,
	"TCP":   true,
	"TLS":   true,
	"TTL":   true,
	"UDP":   true,
	"UI":    true,
	"UID":   true,
	"UUID":  true,
	"URI":   true,
	"URL":   true,
	"UTF8":  true,
	"VM":    true,
	"XML":   true,
	"XSRF":  true,
	"XSS":   true,
}

var numbers = map[byte]string{
	'0': "Zero",
	'1': "One",
	'2': "Two",
	'3': "Three",
	'4': "Four",
	'5': "Five",
	'6': "Six",
	'7': "Seven",
	'8': "Eight",
	'9': "Nine",
}

var (
	lowerPattern       = regexp.MustCompile(`(^|[^a-zA-Z])([a-z]+)`) // 匹配全小写单词
	capitalizedPattern = regexp.MustCompile(`([A-Z])([a-z]+)`)       // 匹配首字母大写单词
	invalidPattern     = regexp.MustCompile(`[^a-zA-Z0-9]`)          // 匹配非字母数字字符
	numberPattern      = regexp.MustCompile(`^\d+$`)                 // 匹配全数字字符串
)

// Replace 用正则表达式对字符串匹配和替换，用法与 JavaScript 中的替换函数类似
func Replace(s string, pattern *regexp.Regexp, fn func(match string, submatch []string) string) string {
	idx := pattern.FindAllStringSubmatchIndex(s, -1)
	if len(idx) == 0 {
		return s
	}
	buf := &bytes.Buffer{}
	for i, v := range pattern.FindAllStringSubmatch(s, -1) {
		if i == 0 {
			buf.WriteString(s[:idx[i][0]])
		} else {
			buf.WriteString(s[idx[i-1][1]:idx[i][0]])
		}
		buf.WriteString(fn(v[0], v[1:]))
	}
	buf.WriteString(s[idx[len(idx)-1][1]:])
	return buf.String()
}

// ToPascalCase 将字段名称转换为驼峰命名，会保持缩写词全大写并且移除所有非字母数字的字符
func ToPascalCase(name string) string {
	name = Replace(name, lowerPattern, func(match string, submatch []string) string {
		n := submatch[0]
		r := submatch[1]
		if Abbreviation[strings.ToUpper(r)] {
			return n + strings.ToUpper(r)
		} else {
			return n + strings.ToUpper(r[:1]) + strings.ToLower(r[1:])
		}
	})
	name = Replace(name, capitalizedPattern, func(match string, submatch []string) string {
		n := submatch[0]
		r := submatch[1]
		if Abbreviation[n+strings.ToUpper(r)] {
			return strings.ToUpper(n + r)
		} else {
			return n + r
		}
	})
	return invalidPattern.ReplaceAllString(name, "")
}

// Regularize 将字符串规范化为合法字段名
func Regularize(name string) string {
	if name == "" {
		return "_"
	}
	if numberPattern.MatchString(name) {
		name = "Num" + name
	} else if name[0] == '-' && numberPattern.MatchString(name[1:]) {
		name = "Neg" + name[1:]
	} else if prefix, ok := numbers[name[0]]; ok {
		name = prefix + name[1:]
	}
	name = ToPascalCase(name)
	if name != "" {
		return name
	}
	return "_"
}

// CompareNumberType 用于比较两个类型名，当任一类型不为浮点数或整数时返回 `any` ，否则返回范围更大的类型名
func CompareNumberType(type1, type2 string) string {
	switch {
	case strings.HasPrefix(type1, "float"):
		if type2 == "float64" {
			return "float64"
		} else if type2 == "float32" || strings.HasPrefix(type2, "int") {
			return type1
		}
	case strings.HasPrefix(type1, "int"):
		if strings.HasPrefix(type2, "float") {
			return type2
		} else if strings.HasPrefix(type2, "int") {
			return "int64"
		}
	}
	return "any"
}

// ParseType 用于解析词元类型，返回其类型名和值的字符串形式
func ParseType(t json.Token) (string, string) {
	switch t := t.(type) {
	case []any:
		return "slice", ""
	case []*object:
		return "struct", ""
	case json.Delim:
		switch t {
		case '[':
			return "slice", "'['"
		case '{':
			return "struct", "'{'"
		}
	case bool:
		if t {
			return "bool", "true"
		}
		return "bool", "false"
	case float64:
		return "float64", strconv.FormatFloat(t, 'f', -1, 64)
	case json.Number:
		value := t.String()
		if strings.Contains(value, ".") {
			_, err := t.Float64()
			if err != nil {
				return "json.Number", value
			}
			return "float64", value
		} else {
			i64, err := t.Int64()
			if err != nil {
				return "json.Number", value
			}
			if i64 >= -2147483648 && i64 <= 2147483647 {
				return "int", value
			}
			return "int64", value
		}
	case string:
		if new(time.Time).UnmarshalText([]byte(t)) == nil {
			return "time.Time", "\"" + t + "\""
		}
		return "string", "\"" + t + "\""
	}
	return "any", fmt.Sprint(t)
}
