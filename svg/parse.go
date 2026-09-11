package svg

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"io"

	"github.com/Drelf2018/oksvg"
)

func parse(r io.Reader) (*doc, error) {
	data, err := readAll(r)
	if err != nil {
		return nil, err
	}
	h, err := readHeader(data)
	if err != nil {
		// readHeader 的报错已经带齐了类别前缀（unknown format / parse）和原因。
		return nil, err
	}
	icon, err := oksvg.ReadIconStream(bytes.NewReader(data), oksvg.IgnoreErrorMode)
	if err != nil {
		// 能走到这里说明确实是 SVG，只是 oksvg 解不开。
		return nil, fmt.Errorf("svg: parse document: %w", err)
	}
	cfg := h.intrinsic()
	d := &doc{
		SvgIcon: icon,
		box:     h.box,
		aspect:  h.aspect,
		size:    image.Point{X: cfg.Width, Y: cfg.Height},
	}
	if !d.box.valid() {
		// 完全没有尺寸信息时，用兜底出来的固有尺寸当作用户坐标系，避免除以 0。
		d.box = viewBox{W: float64(cfg.Width), H: float64(cfg.Height)}
	}
	return d, nil
}

func readAll(r io.Reader) ([]byte, error) {
	if r == nil {
		return nil, errors.New("svg: read stream: nil reader")
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("svg: read stream: %w", err)
	}
	return data, nil
}
