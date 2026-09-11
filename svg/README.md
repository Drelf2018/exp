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

解析用 [oksvg](https://github.com/Drelf2018/oksvg)（srwiley/oksvg 的接替版），栅格化用 [rasterx](https://github.com/srwiley/rasterx)，
本包补上它们之间缺的一环：视口尺寸计算、`preserveAspectRatio`、输出 `*image.RGBA`，并注册进 `image`。

## 安装

```bash
go get github.com/Drelf2018/exp/svg
```

要求 Go 1.18 及以上。

## 接口

`Decode` 和 `DecodeConfig` 就是注册进去的那两个函数，签名与标准库完全一致——空导入本包、
之后只用 `image.Decode` 的人看不到任何区别。需要自己指定输出尺寸时，再多用 `DecodeSize` 这一个。

| 函数 | 说明 |
| --- | --- |
| `svg.Decode(io.Reader) (image.Image, error)` | 等价于 `image.Decode`，按固有尺寸栅格化，返回 `*image.RGBA` |
| `svg.DecodeConfig(io.Reader) (image.Config, error)` | 等价于 `image.DecodeConfig`，只推导尺寸，不栅格化 |
| `svg.DecodeSize(io.Reader, width, height int) (image.Image, error)` | 按指定尺寸栅格化，返回 `*image.RGBA` |

格式名是 `svg`，识别 `<svg`、`<?xml`、`<!DOCTYPE svg` 以及带 UTF-8 BOM 的写法。

`DecodeSize` 不在 `image` 包的注册契约里：注册表只能把前缀映射到解码器，没有传尺寸的地方，
所以"按指定尺寸栅格化"只能用这个具名入口，空导入的用法不受影响。

## 图像尺寸

不指定尺寸时，输出取 SVG 的固有尺寸（它自己声明的尺寸）：

1. 根元素的 `width` / `height`（支持 `px`、`pt`、`pc`、`in`、`cm`、`mm`、`q`，按 96 DPI 换算）；
2. 只写一个时，另一个按 `viewBox` 的宽高比推导；
3. `100%`、`10em` 这类依赖外部上下文、无法单独确定的值，退回 `viewBox` 的尺寸；
4. 都没有则按浏览器默认的 300 × 150。

`viewBox` 与固有尺寸的宽高比不一致时，按文档自己的 `preserveAspectRatio` 留白或裁剪。

## 指定尺寸

要别的尺寸就用 `DecodeSize`，位图一次画到位，不必先按固有尺寸画一遍再缩小：

```go
package main

import (
	"image/png"
	"log"
	"os"

	svg "github.com/Drelf2018/exp/svg" // 具名引入同样会完成注册，不必再写一行空导入
)

func main() {
	f, err := os.Open("icon.svg")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	img, err := svg.DecodeSize(f, 32, 32) // 输出就是 32x32
	if err != nil {
		log.Fatal(err)
	}
	if err := png.Encode(os.Stdout, img); err != nil {
		log.Fatal(err)
	}
}
```

输出尺寸只由调用方决定，与文档声明的固有尺寸无关；视口定下来之后，内容仍按文档自己的
`preserveAspectRatio` 摆放：默认的 `xMidYMid meet` 等比缩放并留白居中，`slice` 裁剪，
`none` 非等比拉伸铺满。尺寸非正、或单边超过 4096 像素时直接报错。

## 行为

- 错误信息统一是 `svg: <动作+对象>: <细节>`，例如 `svg: read stream: unexpected EOF`、
  `svg: read root element: expected <svg>, got: <rss>`；不会静默给出空图；
- 单张位图单边超过 4096 像素、面积超过 4096 × 4096 像素直接报错，避免畸形 SVG 撑爆内存；
- `encoding="GB2312"` 之类的非 UTF-8 文档也能解码；
- 不对文档做任何字节级改写。上游 oksvg 的几个已知问题——定义块写在引用之后会被画成黑色、
  `width="100%"` 这类尺寸写法会让整张图空白、单参数 `scale()` 的 transform 会让图元消失——
  已在接替版里修好。

## 限制

底层是 oksvg，只实现了 SVG 的一个子集：

- 不支持 `<use>`、`<text>`、`<image>`、滤镜、`clipPath`/`mask`、脚本与动画；
- 不支持外部资源引用，`<style>` 只支持最基础的类选择器；
- 不支持的元素会被静默忽略；
- 解码需要把整个 SVG 读进内存，适合图标、徽标这类体量不大的矢量图；
- `DecodeConfig` 同样会读取整个文档：本包要先拿到根元素才能推导尺寸，而根元素可能带编码声明。

## 许可

见仓库根目录的 [LICENSE](../LICENSE)。
