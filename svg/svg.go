// Package svg 让标准库的 image 包能够直接解码 SVG。
//
// 用法就是空导入，没有别的步骤：
//
//	import _ "github.com/Drelf2018/exp/svg"
//
//	img, format, err := image.Decode(file)       // format == "svg"，img 是 *image.RGBA
//	cfg, format, err := image.DecodeConfig(file) // 只读尺寸，不栅格化
//
// 解析交给 github.com/srwiley/oksvg，栅格化用 github.com/srwiley/rasterx，
// 本包负责 oksvg 没做的那一步：算出视口尺寸、处理 preserveAspectRatio，
// 最后输出标准库的 *image.RGBA。
//
// 和 png、jpeg 一样，本包也导出 Decode 和 DecodeConfig，它们就是注册进去的
// 那两个函数，签名和 image.Decode、image.DecodeConfig 完全一致。
//
// 除此之外还多一个 DecodeSize，供需要指定输出尺寸的调用方使用。它不在 image 包的
// 注册契约里：注册表只能把前缀映射到解码器，没有传尺寸的地方，所以这一项只能具名调用。
// 空导入的用法完全不受影响。
//
// 代码结构：
//
//	svg.go       包文档、image 注册、公共入口
//	parse.go     解析流水线：读入 → 读尺寸 → 预处理 → 交给 oksvg
//	render.go    文档结构体、像素上限与光栅化
//	viewport.go  视口与尺寸：viewBox、preserveAspectRatio、单位换算
//	rewrite.go   交给 oksvg 之前的三处字节级改写
//
// 注意 oksvg 只实现了 SVG 的一个子集：use、text、image、clipPath/mask、
// 滤镜和脚本动画都不支持，遇到这些元素时会被忽略。
package svg

import (
	"image"
	"io"
)

// formatName 是注册到 image 包时使用的格式名，也是 image.Decode 返回的格式名。
const formatName = "svg"

// magics 是 image 包识别 SVG 时使用的前缀。
// image.Decode 只做前缀匹配，所以这里把常见的几种开头都列上。
var magics = []string{
	"<svg",
	"<?xml",
	"<!DOCTYPE svg",
	"<!doctype svg",
	"\xEF\xBB\xBF<svg", // 带 UTF-8 BOM
	"\xEF\xBB\xBF<?xml",
	"\xEF\xBB\xBF<!DOCTYPE svg",
}

// 空导入本包即完成注册
func init() {
	for _, magic := range magics {
		image.RegisterFormat(formatName, magic, Decode, DecodeConfig)
	}
}

// Decode 把 SVG 解码成 image.Image，签名和 image.Decode 一致。
// 返回值的动态类型是 *image.RGBA，背景保持透明。
func Decode(r io.Reader) (image.Image, error) {
	d, err := parse(r)
	if err != nil {
		return nil, err
	}
	return d.render()
}

// DecodeConfig 返回 SVG 的固有尺寸，签名和 image.DecodeConfig 一致。
func DecodeConfig(r io.Reader) (image.Config, error) {
	data, err := readAll(r)
	if err != nil {
		return image.Config{}, err
	}
	h, err := readHeader(data)
	if err != nil {
		return image.Config{}, err
	}
	return h.intrinsic(), nil
}

// DecodeSize 把 SVG 栅格化成一张 width x height 的位图，返回值的动态类型是
// *image.RGBA，背景保持透明。
//
// 和 Decode 的区别只有一处：输出位图的尺寸由调用方给定，而不是文档的固有尺寸。
// 拿到手的就是最终尺寸，不必先按固有尺寸画一遍再缩放——图标、缩略图这类场景既省掉
// 一次大位图，也避免固有尺寸偏小时被放大而发糊。
//
// 视口定了之后，内容怎么摆仍由文档自己的 preserveAspectRatio 决定：默认的
// xMidYMid meet 等比缩放并留白居中，slice 裁剪，none 非等比拉伸铺满。
//
// width 或 height 非正、或单边超过 4096 像素时返回错误。
func DecodeSize(r io.Reader, width, height int) (image.Image, error) {
	d, err := parse(r)
	if err != nil {
		return nil, err
	}
	return d.renderAt(width, height)
}
