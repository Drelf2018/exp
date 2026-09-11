package svg

// 视口模型与固有尺寸：viewBox、preserveAspectRatio 的类型定义，以及由根元素声明
// 推导输出尺寸。根元素本身的解析见 header.go；header.intrinsic 也放在这里，
// 因为它讲的是"多大"，不是"怎么读"。

import (
	"image"
	"image/color"
	"math"
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
