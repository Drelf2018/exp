package main

import (
	"strings"

	"github.com/Drelf2018/req"
)

type AcrnmAPI struct {
	req.Get
}

func (AcrnmAPI) RawURL() string {
	return "http://acrnm.nana7mi.link"
}

type Variant struct {
	Color string
	Size  string
}

// 商品
type Product struct {
	Name     string
	Href     string
	Price    string
	Variants []Variant
}

func (p *Product) Equal(n *Product) bool {
	if n == nil {
		return false
	}
	if p.Href != n.Href {
		return false
	}
	if p.Price != n.Price {
		return false
	}
	m := make(map[string][]string)
	for _, v := range n.Variants {
		m[v.Color] = append(m[v.Color], v.Size)
	}
outer:
	for _, v := range p.Variants {
		for _, s := range m[v.Color] {
			if v.Size == s {
				continue outer
			}
		}
		return false
	}
	return true
}

func (p Product) String() string {
	build := &strings.Builder{}
	build.WriteString(p.Name)
	build.WriteByte('(')
	build.WriteString(p.Price)
	for _, variant := range p.Variants {
		build.WriteString(", ")
		build.WriteString(variant.Color)
		build.WriteByte(' ')
		build.WriteString(variant.Size)
	}
	build.WriteByte(')')
	return build.String()
}

func (p *Product) VariantString(hyphen, sep string) string {
	variants := make([]string, 0, len(p.Variants))
	for _, v := range p.Variants {
		variants = append(variants, v.Color+hyphen+v.Size)
	}
	return strings.Join(variants, sep)
}
