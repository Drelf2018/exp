package svg

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html/charset"
)

// prepare 把原始字节规整成 oksvg 能正确处理的形式。
// 前两步依赖 readHeader 给出的原始字节偏移，所以先做；
// 最后一步只做标签扫描，即使插入了字节也不影响别人。
func prepare(data []byte, h header) []byte {
	return fixSingleArgScale(rewriteRoot(hoistDefinitions(data, h.tagEnd), h))
}

// ---- 第 1 步：根标签重写 ----

// rewriteRoot 重写根元素起始标签：
// 丢掉 width/height（写成 "100%"、"10em"、"2in" 时 oksvg 的 parseFloat 会失败，
// 而它会把这类错误静默吞掉，结果 viewBox 停在 0、整张图空白，是否如此还取决于属性顺序），
// 并写入我们自己算好的 viewBox。
// 无法定位根标签（例如非 UTF-8 编码）时原样返回。
func rewriteRoot(data []byte, h header) []byte {
	if h.tagStart < 0 || h.tagStart >= h.tagEnd || h.tagEnd > len(data) {
		return data
	}
	tag := data[h.tagStart:h.tagEnd]
	if !isRootTag(tag) {
		return data
	}
	out := rewriteTag(tag, h.box)
	buf := make([]byte, 0, len(data)+len(out))
	buf = append(buf, data[:h.tagStart]...)
	buf = append(buf, out...)
	buf = append(buf, data[h.tagEnd:]...)
	return buf
}

func isRootTag(tag []byte) bool {
	if len(tag) < 2 || tag[0] != '<' {
		return false
	}
	i := 1
	for i < len(tag) && !isXMLSpace(tag[i]) && tag[i] != '>' && tag[i] != '/' {
		i++
	}
	return strings.EqualFold(string(tag[1:i]), "svg")
}

func isXMLSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

// rewriteTag 逐字节重建根元素起始标签，保留除 width/height/viewBox 以外的所有属性原样。
func rewriteTag(tag []byte, box viewBox) []byte {
	n := len(tag)
	i := 1
	for i < n && !isXMLSpace(tag[i]) && tag[i] != '>' && tag[i] != '/' {
		i++
	}
	var b bytes.Buffer
	b.Grow(n + 32)
	b.Write(tag[:i])
	selfClose := strings.HasSuffix(strings.TrimRight(string(tag), " \t\r\n"), "/>")
	wroteViewBox := false
	for i < n {
		for i < n && isXMLSpace(tag[i]) {
			i++
		}
		if i >= n || tag[i] == '>' || tag[i] == '/' {
			break
		}
		ns := i
		for i < n && !isXMLSpace(tag[i]) && tag[i] != '=' && tag[i] != '>' && tag[i] != '/' {
			i++
		}
		name := string(tag[ns:i])
		var quote byte
		valStart, valEnd := i, i
		j := i
		for j < n && isXMLSpace(tag[j]) {
			j++
		}
		if j < n && tag[j] == '=' {
			j++
			for j < n && isXMLSpace(tag[j]) {
				j++
			}
			if j < n && (tag[j] == '"' || tag[j] == 0x27) {
				quote = tag[j]
				j++
				valStart = j
				for j < n && tag[j] != quote {
					j++
				}
				valEnd = j
				if j < n {
					j++
				}
			} else {
				valStart = j
				for j < n && !isXMLSpace(tag[j]) && tag[j] != '>' {
					j++
				}
				valEnd = j
			}
		}
		i = j
		switch {
		case strings.EqualFold(name, "width"), strings.EqualFold(name, "height"):
			// 尺寸由本包自己解析，这里直接丢弃。
		case strings.EqualFold(name, "viewBox"):
			if !wroteViewBox {
				fmt.Fprintf(&b, " viewBox=%q", box.String())
				wroteViewBox = true
			}
		default:
			b.WriteByte(' ')
			b.WriteString(name)
			if quote != 0 {
				b.WriteByte('=')
				b.WriteByte(quote)
				b.Write(tag[valStart:valEnd])
				b.WriteByte(quote)
			}
		}
	}
	if !wroteViewBox && box.valid() {
		fmt.Fprintf(&b, " viewBox=%q", box.String())
	}
	if selfClose {
		b.WriteString("/>")
	} else {
		b.WriteString(">")
	}
	return b.Bytes()
}

// ---- 第 2 步：定义块提前 ----

// hoistDefinitions 把所有定义块搬到根元素起始标签之后，保持它们之间的原有顺序。
// rootEnd 是根元素起始标签的结束偏移，也就是插入点。
// 定义块必须落在根元素内部、所有绘制内容之前，挪动位置只影响解析顺序，不影响渲染结果。
func hoistDefinitions(data []byte, rootEnd int) []byte {
	if rootEnd <= 0 || rootEnd > len(data) || !utf8.Valid(data) {
		return data
	}
	// 大多数 SVG 没有定义块，先做个便宜的排除。
	if !bytes.Contains(data, []byte("<defs")) && !bytes.Contains(data, []byte("<style")) &&
		!bytes.Contains(data, []byte("<linearGradient")) && !bytes.Contains(data, []byte("<radialGradient")) {
		return data
	}
	spans := definitionSpans(data)
	if len(spans) == 0 || spans[0].start < rootEnd {
		return data
	}
	// 定义块按原顺序拼在一起。
	defs := make([]byte, 0, 256)
	for _, s := range spans {
		defs = append(defs, data[s.start:s.end]...)
		defs = append(defs, '\n')
	}
	// 从正文里摘掉这些区间。
	body := make([]byte, 0, len(data))
	pos := 0
	for _, s := range spans {
		body = append(body, data[pos:s.start]...)
		pos = s.end
	}
	body = append(body, data[pos:]...)
	// 插到根元素起始标签之后。所有区间都在 rootEnd 之后，所以这个位置在 body 里依然有效。
	out := make([]byte, 0, len(body)+len(defs))
	out = append(out, body[:rootEnd]...)
	out = append(out, defs...)
	out = append(out, body[rootEnd:]...)
	return out
}

// definitionTags 是不会就地绘制、只靠 id 或类名被引用的元素和容器。
// oksvg 解析 fill="url(#id)" 和 class 时是即查即用的（见它的 ReadGradURL），
// 定义写在引用之后就查不到，会退化成不透明黑色，所以这里把它们提升到根元素之后。
var definitionTags = map[string]bool{
	"defs":           true,
	"lineargradient": true,
	"radialgradient": true,
	"style":          true,
}

// span 是一段需要搬走的字节区间。
type span struct{ start, end int }

// definitionSpans 扫描文档，找出需要上提的定义块区间。
// 嵌套在外层定义块里的（比如 defs 里的 linearGradient）会被外层包含，不再单独记录。
func definitionSpans(data []byte) (spans []span) {
	type frame struct {
		name    string
		start   int
		tracked bool
	}
	dec := xml.NewDecoder(bytes.NewReader(data))
	// 宽容模式：与 readHeader 保持一致，未定义实体之类的问题不应该让预处理失败。
	dec.Strict = false
	dec.CharsetReader = charset.NewReaderLabel
	var stack []frame
	for {
		start := int(dec.InputOffset())
		tok, err := dec.Token()
		if err == io.EOF {
			return spans
		}
		if err != nil {
			// 文档结构有问题时放弃预处理，交给 oksvg 自己去报错。
			return nil
		}
		switch t := tok.(type) {
		case xml.StartElement:
			name := strings.ToLower(t.Name.Local)
			stack = append(stack, frame{name: name, start: start, tracked: definitionTags[name]})
		case xml.EndElement:
			if len(stack) == 0 {
				return nil
			}
			f := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if !strings.EqualFold(t.Name.Local, f.name) {
				return nil
			}
			if !f.tracked {
				continue
			}
			// 已经被外层定义块包含的就跳过。
			nested := false
			for _, outer := range stack {
				if outer.tracked {
					nested = true
					break
				}
			}
			if nested {
				continue
			}
			if end := int(dec.InputOffset()); validSpan(data, f.start, end, f.name) {
				spans = append(spans, span{start: f.start, end: end})
			} else {
				return nil
			}
		}
	}
}

// validSpan 校验区间确实是一个完整的标签区间。
// 偏移量是解码器给出的，非 UTF-8 文档会经过 charset 转换而对不上原始字节，这里兜底。
func validSpan(data []byte, start, end int, name string) bool {
	if start < 0 || start >= end || end > len(data) || data[start] != '<' || data[end-1] != '>' {
		return false
	}
	rest := data[start+1:]
	return len(rest) >= len(name) && strings.EqualFold(string(rest[:len(name)]), name)
}

// ---- 第 3 步：单参数 scale 补第二个参数 ----

// fixSingleArgScale 把属性值里的单参数 scale(x) 改写成 scale(x x)。
//
// oksvg 解析 transform 时，单参数 scale 的分支是错的（icon_cursor.go 的 readTransformAttr）：
//
//	case "scale":
//		if ln == 1 {
//			m1 = m1.Scale(c.points[0], 0)   // 第二个参数应该是 c.points[0]
//		}
//
// 纵轴因此被压成 0，transform="scale(1.6)" 的图元会退化成一条零高度的线、整个画不出来
// （双参数写法 scale(1.6,1.6) 是正常的）。SVG 规范里 scale(sx) 等价于 scale(sx, sx)，
// 所以这里在交给 oksvg 之前把缺的第二个参数补上。
//
// 只改标签内部（也就是属性值里）出现的 scale(...)，正文文本、注释和 <style> 内容都不动。
func fixSingleArgScale(data []byte) []byte {
	if !bytes.Contains(data, []byte("scale(")) {
		return data
	}
	var (
		out     = make([]byte, 0, len(data)+16)
		changed bool
	)
	for i := 0; i < len(data); {
		if data[i] != '<' {
			j := bytes.IndexByte(data[i:], '<')
			if j < 0 {
				out = append(out, data[i:]...)
				break
			}
			out = append(out, data[i:i+j]...)
			i += j
			continue
		}
		end := tagEnd(data, i)
		if end < 0 {
			out = append(out, data[i:]...)
			break
		}
		tag := data[i:end]
		if fixed := fixScaleInTag(tag); !bytes.Equal(fixed, tag) {
			changed = true
			out = append(out, fixed...)
		} else {
			out = append(out, tag...)
		}
		i = end
	}
	if !changed {
		return data
	}
	return out
}

// tagEnd 返回从 data[i]（必须是 '<'）开始的那个标签结束（含 '>'）的下标，找不到返回 -1。
func tagEnd(data []byte, i int) int {
	var quote byte
	for j := i; j < len(data); j++ {
		switch c := data[j]; {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '"' || c == 0x27:
			quote = c
		case c == '>':
			return j + 1
		}
	}
	return -1
}

// fixScaleInTag 在一个标签内部找 scale(...)，把单参数的补成两参数。
func fixScaleInTag(tag []byte) []byte {
	const key = "scale("
	var out []byte
	pos := 0
	for {
		k := bytes.Index(tag[pos:], []byte(key))
		if k < 0 {
			break
		}
		start := pos + k + len(key)
		rel := bytes.IndexByte(tag[start:], ')')
		if rel < 0 {
			break
		}
		end := start + rel
		if num, ok := singleNumber(tag[start:end]); ok {
			if out == nil {
				out = make([]byte, 0, len(tag)+8)
			}
			out = append(out, tag[pos:start]...)
			out = append(out, num...)
			out = append(out, ' ')
			out = append(out, num...)
		} else if out != nil {
			out = append(out, tag[pos:end]...)
		}
		pos = end
	}
	if out == nil {
		return tag
	}
	return append(out, tag[pos:]...)
}

// singleNumber 判断 scale 的括号里是不是只有一个数字，是就返回它。
func singleNumber(inner []byte) ([]byte, bool) {
	fields := strings.FieldsFunc(string(inner), func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
	if len(fields) != 1 {
		return nil, false
	}
	num, unit, ok := splitNumber(fields[0])
	if !ok || unit != "" {
		return nil, false
	}
	return []byte(num), true
}
