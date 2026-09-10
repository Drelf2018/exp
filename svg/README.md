# svg

让标准库的 `image` 包能够直接解码 SVG。

```go
import (
	"image"
	"image/png"
	"log"
	"os"

	_ "github.com/Drelf2018/exp/svg" // 空导入即完成注册
)

func main() {
	f, err := os.Open("icon.svg")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	img, format, err := image.Decode(f) // format == "svg"，img 是 *image.RGBA
	if err != nil {
		log.Fatal(err)
	}
	png.Encode(os.Stdout, img)
}
```

解析用 [oksvg](https://github.com/srwiley/oksvg)，栅格化用 [rasterx](https://github.com/srwiley/rasterx)，
本包补上它们之间缺的一环：视口尺寸计算、`preserveAspectRatio`、输出 `*image.RGBA`，并注册进 `image`。

## 安装

```bash
go get github.com/Drelf2018/exp/svg
```

要求 Go 1.18 及以上。

## 接口

只导出两个函数，就是注册进去的那两个，签名与标准库完全一致；没有别的导出类型或配置项。

| 函数 | 说明 |
| --- | --- |
| `svg.Decode(io.Reader) (image.Image, error)` | 等价于 `image.Decode`，返回 `*image.RGBA` |
| `svg.DecodeConfig(io.Reader) (image.Config, error)` | 等价于 `image.DecodeConfig`，只读尺寸不解码 |

格式名是 `svg`，识别 `<svg`、`<?xml`、`<!DOCTYPE svg` 以及带 UTF-8 BOM 的写法。

## 图像尺寸

输出尺寸取 SVG 的固有尺寸（它自己声明的尺寸）：

1. 根元素的 `width` / `height`（支持 `px`、`pt`、`pc`、`in`、`cm`、`mm`、`q`，按 96 DPI 换算）；
2. 只写一个时，另一个按 `viewBox` 的宽高比推导；
3. `100%`、`10em` 这类无法单独确定的值，退回 `viewBox` 的尺寸；
4. 都没有则按浏览器默认的 300 × 150。

`viewBox` 与固有尺寸的宽高比不一致时，按 `preserveAspectRatio` 留白或裁剪。
要别的尺寸，解码后自己缩放即可（用 [x/image/draw](https://pkg.go.dev/golang.org/x/image/draw)）：

```go
dst := image.NewRGBA(image.Rect(0, 0, 32, 32))
draw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Over, nil)
```

## 行为

- 错误信息统一是 `svg: <动作+对象>: <细节>`，例如 `svg: read stream: unexpected EOF`、
  `svg: read root element: expected <svg>, got: <rss>`；不会静默给出空图；
- 单张位图超过 4096 × 4096 像素直接报错，避免畸形 SVG 撑爆内存；
- `encoding="GB2312"` 之类的非 UTF-8 文档也能解码；
- 解析前会修掉 oksvg 的几个已知问题——定义块写在引用之后会被画成黑色、`width="100%"` 这类尺寸
  写法会让整张图空白、单参数 `scale()` 的 transform 会让图元消失——都是纯字节改写，不改动文档语义。

## 限制

底层是 oksvg，只实现了 SVG 的一个子集：

- 不支持 `<use>`、`<text>`、`<image>`、滤镜、`clipPath`/`mask`、脚本与动画；
- 不支持外部资源引用，`<style>` 只支持最基础的类选择器；
- 不支持的元素会被静默忽略；
- 解码需要把整个 SVG 读进内存，适合图标、徽标这类体量不大的矢量图。
