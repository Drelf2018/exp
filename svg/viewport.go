package svg

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
	"math"
	"strconv"
	"strings"

	"golang.org/x/net/html/charset"
)

// viewBox 描述 svg 根元素 viewBox 属性给出的用户坐标系。
type viewBox struct {
	X, Y, W, H float64
}

// valid 报告这个 viewBox 是否可用（宽高必须为正数）。
// NaN 参与任何比较都是 false，所以这里的 > 0 已经把它排除了；
// NaN/Inf 在 parseViewBox 那一层就被挡掉（strconv.ParseFloat 对 "NaN"/"Inf" 是能解析成功的）。
func (v viewBox) valid() bool {
	return v.W > 0 && v.H > 0
}

// String 返回可以直接写进 viewBox 属性的形式。
func (v viewBox) String() string {
	return fmt.Sprintf("%g %g %g %g", v.X, v.Y, v.W, v.H)
}

// align 是 preserveAspectRatio 里 x/y 方向的对齐方式。
type align uint8

const (
	alignMin align = iota // Min
	alignMid              // Mid，默认
	alignMax              // Max
)

// aspect 对应根元素的 preserveAspectRatio 属性。
type aspect struct {
	none  bool  // "none"，非等比拉伸铺满
	slice bool  // "slice"，裁剪而不是留白
	x, y  align // 对齐方式
}

func defaultAspect() aspect {
	return aspect{x: alignMid, y: alignMid}
}

// header 是从 svg 根元素里解析出来的尺寸相关信息。
type header struct {
	box    viewBox // 用户坐标系，无效时会在渲染阶段补一个
	aspect aspect
	width  float64 // 固有宽度（像素），0 表示未知（百分比、em 等依赖上下文的值）
	height float64 // 固有高度（像素），0 表示未知
	// tagStart/tagEnd 是根元素起始标签在原始数据中的字节范围。
	tagStart, tagEnd int
}

// readHeader 只解析根元素，用于在真正渲染之前拿到尺寸信息。
func readHeader(data []byte) (h header, err error) {
	h.aspect = defaultAspect()
	if len(bytes.TrimSpace(data)) == 0 {
		return h, errors.New("svg: read root element: empty input")
	}
	dec := xml.NewDecoder(bytes.NewReader(data))
	// 宽容模式：这一步只是我们自己读尺寸，不该因为 &nbsp; 这类未定义实体就放弃。
	// 注意后面的 oksvg 仍然是严格解码，整份文档能不能解析最终取决于它。
	dec.Strict = false
	dec.CharsetReader = charset.NewReaderLabel
	var root *xml.StartElement
	for root == nil {
		start := int(dec.InputOffset())
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			// 一路读到结尾都没遇到根元素，多半根本不是 XML。
			return h, errors.New("svg: read root element: not found")
		}
		if err != nil {
			return h, fmt.Errorf("svg: read root element: %w", err)
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if !strings.EqualFold(se.Name.Local, "svg") {
			return h, fmt.Errorf("svg: read root element: expected <svg>, got: <%s>", se.Name.Local)
		}
		h.tagStart, h.tagEnd = start, int(dec.InputOffset())
		root = &se
	}
	for _, attr := range root.Attr {
		switch {
		case strings.EqualFold(attr.Name.Local, "width"):
			if v, ok := parseLength(attr.Value); ok {
				h.width = v
			}
		case strings.EqualFold(attr.Name.Local, "height"):
			if v, ok := parseLength(attr.Value); ok {
				h.height = v
			}
		case strings.EqualFold(attr.Name.Local, "viewBox"):
			if v, ok := parseViewBox(attr.Value); ok {
				h.box = v
			}
		case strings.EqualFold(attr.Name.Local, "preserveAspectRatio"):
			h.aspect = parseAspect(attr.Value)
		}
	}
	// 只有 width/height 没有 viewBox 时，用户坐标系就是像素坐标系。
	if !h.box.valid() && h.width > 0 && h.height > 0 {
		h.box = viewBox{W: h.width, H: h.height}
	}
	return h, nil
}

// intrinsic 返回 SVG 的固有尺寸，也就是 image.Config。
func (h header) intrinsic() image.Config {
	w, hh := h.width, h.height
	switch {
	case w > 0 && hh > 0:
	case w > 0 && h.box.valid():
		hh = w * h.box.H / h.box.W
	case hh > 0 && h.box.valid():
		w = hh * h.box.W / h.box.H
	case h.box.valid():
		w, hh = h.box.W, h.box.H
	default:
		// 什么尺寸信息都没有：用浏览器的默认视口 300x150。
		w, hh = 300, 150
	}
	return image.Config{
		ColorModel: color.RGBAModel,
		Width:      clampDim(w),
		Height:     clampDim(hh),
	}
}

// clampDim 把尺寸夹到 [1, MaxInt32]，避免后续 w*h 溢出或者被 NaN 污染。
func clampDim(v float64) int {
	if math.IsNaN(v) || v < 1 {
		return 1
	}
	if v > math.MaxInt32 {
		return math.MaxInt32
	}
	return int(math.Round(v))
}

// parseLength 把长度值换算成像素。
// 返回 false 表示这个值依赖外部上下文（百分比、em、ex 等），无法单独确定尺寸。
func parseLength(s string) (float64, bool) {
	num, unit, ok := splitNumber(strings.TrimSpace(s))
	if !ok {
		return 0, false
	}
	// splitNumber 只放行 [+-]数字[.数字][e[+-]数字]，所以这里不可能解析出 NaN/Inf：
	// "NaN"、"Inf" 这类根本没有数字前缀，会被 splitNumber 拒绝，"1e999" 则会带着 ErrRange 出错。
	v, err := strconv.ParseFloat(num, 64)
	if err != nil || v < 0 {
		return 0, false
	}
	switch strings.ToLower(unit) {
	case "", "px":
		return v, true
	case "pt":
		return v * 96 / 72, true
	case "pc":
		return v * 16, true
	case "in":
		return v * 96, true
	case "cm":
		return v * 96 / 2.54, true
	case "mm":
		return v * 96 / 25.4, true
	case "q":
		return v * 96 / 101.6, true
	default:
		// %、em、ex、rem、vw 等都需要外部上下文，交给 viewBox 兜底。
		return 0, false
	}
}

// splitNumber 把 "12.5px" 这样以数字开头的值拆成数字部分和单位部分。
func splitNumber(s string) (num, unit string, ok bool) {
	i := 0
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}
	digits := false
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
		digits = true
	}
	if i < len(s) && s[i] == '.' {
		i++
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
			digits = true
		}
	}
	if !digits {
		return "", "", false
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		j := i + 1
		if j < len(s) && (s[j] == '+' || s[j] == '-') {
			j++
		}
		k := j
		for k < len(s) && s[k] >= '0' && s[k] <= '9' {
			k++
		}
		if k > j {
			i = k
		}
	}
	return s[:i], strings.TrimSpace(s[i:]), true
}

// parseViewBox 解析 "min-x min-y width height"，分隔符可以是逗号或空白。
func parseViewBox(s string) (viewBox, bool) {
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
	if len(fields) != 4 {
		return viewBox{}, false
	}
	var v [4]float64
	for i, f := range fields {
		x, err := strconv.ParseFloat(f, 64)
		if err != nil || math.IsNaN(x) || math.IsInf(x, 0) {
			return viewBox{}, false
		}
		v[i] = x
	}
	if v[2] <= 0 || v[3] <= 0 {
		return viewBox{}, false
	}
	return viewBox{X: v[0], Y: v[1], W: v[2], H: v[3]}, true
}

// parseAspect 解析 preserveAspectRatio，例如 "xMidYMid meet"、"none"、"xMinYMax slice"。
func parseAspect(s string) aspect {
	a := defaultAspect()
	fields := strings.Fields(strings.ToLower(s))
	i := 0
	if i < len(fields) && fields[i] == "defer" {
		i++
	}
	if i < len(fields) {
		v := fields[i]
		switch {
		case v == "none":
			a.none = true
		case len(v) == 8 && v[0] == 'x' && v[4] == 'y':
			a.x, a.y = parseAlign(v[1:4]), parseAlign(v[5:8])
		}
		i++
	}
	if i < len(fields) && fields[i] == "slice" {
		a.slice = true
	}
	return a
}

func parseAlign(s string) align {
	switch s {
	case "min":
		return alignMin
	case "max":
		return alignMax
	default:
		return alignMid
	}
}
