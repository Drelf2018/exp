package svg

import (
	"fmt"
	"image"
	"math"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

// maxSide 是单张位图允许的单边像素上限。SVG 可以靠 width="1e9" 之类的写法
// 廉价地声明一个超大画布，这里做个兜底，避免解码不可信来源的文件时撑爆内存。
const maxSide = 4096

const maxPixels = maxSide * maxSide

// doc 是一份解析好的 SVG：oksvg 的图元，加上本包解析出来的视口信息。
type doc struct {
	*oksvg.SvgIcon
	box    viewBox     // viewBox 给出的用户坐标系
	aspect aspect      // 根元素的 preserveAspectRatio
	size   image.Point // 固有尺寸（像素）
}

// render 按固有尺寸开一张位图，再按 preserveAspectRatio 把 viewBox 映射上去。
func (d *doc) render() (*image.RGBA, error) {
	return d.renderAt(d.size.X, d.size.Y)
}

// renderAt 按调用方给定的尺寸开一张位图，再按 preserveAspectRatio 把 viewBox 映射上去。
// 视口（w、h）与文档的固有尺寸无关：视口定下来之后，内容怎么在里面留白、对齐、
// 或者被裁掉，仍然只由文档自己的 preserveAspectRatio 决定，见 draw。
func (d *doc) renderAt(w, h int) (*image.RGBA, error) {
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("svg: render image: non-positive size: got %dx%d", w, h)
	}
	if w > maxSide || h > maxSide || w*h > maxPixels {
		return nil, fmt.Errorf("svg: render image: too large: got %dx%d, limit %dx%d", w, h, maxSide, maxSide)
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	d.draw(img, w, h)
	return img, nil
}

func (d *doc) draw(img *image.RGBA, w, h int) {
	fw, fh := float64(w), float64(h)
	box := d.box
	var x, y, dw, dh float64
	if d.aspect.none {
		// preserveAspectRatio="none"：非等比拉伸铺满。
		dw, dh = fw, fh
	} else {
		scale := math.Min(fw/box.W, fh/box.H)
		if d.aspect.slice {
			scale = math.Max(fw/box.W, fh/box.H)
		}
		dw, dh = box.W*scale, box.H*scale
		x = alignOffset(d.aspect.x, fw-dw)
		y = alignOffset(d.aspect.y, fh-dh)
	}
	// oksvg 的 SetTarget 会做 (p - viewBox 原点) * (w/viewBox.W, h/viewBox.H) + (x, y)，
	// 让 dw/dh 与 viewBox 保持等比就是等比缩放。
	d.ViewBox.X, d.ViewBox.Y = box.X, box.Y
	d.ViewBox.W, d.ViewBox.H = box.W, box.H
	d.SetTarget(x, y, dw, dh)

	b := img.Bounds()
	scanner := rasterx.NewScannerGV(w, h, img, b)
	raster := rasterx.NewDasher(w, h, scanner)
	d.Draw(raster, 1)
}

func alignOffset(a align, slack float64) float64 {
	switch a {
	case alignMin:
		return 0
	case alignMax:
		return slack
	default:
		return slack / 2
	}
}
