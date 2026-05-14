package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/Drelf2018/exp/xpath"
	"github.com/RomainMichau/cloudscraper_go/cloudscraper"
)

var BaseURL, _ = url.Parse("https://acrnm.com/")

// 商品款式
type Variant struct {
	Color string `xpath:"./div/span/text()"`
	Size  string `xpath:"./span/text()"`
}

// 商品
type Product struct {
	Name     string    `xpath:"./td[1]/a/span/text()"`
	Href     string    `xpath:"./td[1]/a/@href"`
	Price    string    `xpath:"./td[4]/span/text()"`
	Variants []Variant `xpath:"./td[3]/div/span"`
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

func (p Product) URL() string {
	return BaseURL.JoinPath(p.Href).String()
}

var scraper, _ = cloudscraper.Init(false, false)

// 商品列表
type Products []*Product

func (Products) XPath() string {
	return `//*[@id="main"]/div/table/tbody/tr[./td[4]/span/text() != ""]`
}

func GetProducts() ([]*Product, error) {
	resp, err := http.Get("http://serverless.nana7mi.link/api/acrnm")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	var products Products
	err = json.Unmarshal(b, &products)
	return products, err
}

type ProductImage string

func (ProductImage) XPath() string {
	return "/html/body/div[1]/main/div/div[2]/div/img/@src"
}

func GetProductImage(url string) (string, error) {
	resp, err := scraper.Get(url, make(map[string]string), "")
	if err != nil {
		return "", err
	}
	var image ProductImage
	err = xpath.UnmarshalText(resp.Body, &image)
	if err != nil {
		return "", err
	}
	return string(image), nil
}
