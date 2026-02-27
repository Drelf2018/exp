package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/Drelf2018/req"
	"github.com/Drelf2018/xpath"
	"github.com/RomainMichau/cloudscraper_go/cloudscraper"
	mapset "github.com/deckarep/golang-set/v2"
	"golang.org/x/net/html"
)

type Acrnm struct {
	// 商品信息来源接口 可使用库内的默认实现
	//
	// 	acrnm.AcrnmAPI{}
	Source req.API

	// 每次轮询间隔时间的计时器
	Delayer req.Delayer

	// 商品上新时触发
	OnNew func([]*Product)

	// 商品信息变动时触发
	OnUpdate func([]*Product)

	// 商品下架时触发
	OnRemoval func([]*Product)

	// 请求发生错误时触发
	OnError func(error)
}

func (a Acrnm) Run() error {
	if a.Source == nil {
		a.Source = AcrnmAPI{}
	}
	if a.Delayer == nil {
		a.Delayer = req.ForeverDelayer(10 * time.Second)
	}

	// 所有商品
	products := make(map[string]*Product)

	// 暂存的商品名集合
	// 当某个商品名不见时可能是商品被下架
	// 也可能是没获取到该数据
	// 为了避免错误的发送下架消息用这个集合来保存可能下架的商品名
	// 当下一次获取商品后这个商品名依然不存在
	// 才认为这个商品下架了
	victim := mapset.NewSet[string]()

	// 最近一次获取到的商品名集合
	alive := mapset.NewSet[string]()

	// 首次请求 初始化商品字典
	result, err := req.Result[[]*Product](a.Source)
	if err != nil {
		return fmt.Errorf("acrnm: initial error: %v", err)
	}
	for _, product := range result {
		alive.Add(product.Href)
		products[product.Href] = product
	}

	// 开始轮询
	for range req.WithDelay(a.Delayer) {
		result, err = req.Result[[]*Product](a.Source)
		if err != nil {
			if a.OnError != nil {
				a.OnError(err)
			}
			continue
		}

		// 请求没有错误 但是获取到的数据量太少 防止误判先跳过
		if len(result) < len(products)*3/5 {
			if a.OnError != nil {
				a.OnError(fmt.Errorf("acrnm: new products (%d) are less than existing products (%d)", len(result), len(products)))
			}
			continue
		}

		// 上新商品
		newProducts := make([]*Product, 0, len(result))

		// 信息变化商品
		updateProducts := make([]*Product, 0, len(result))

		// 下架商品
		removalProducts := make([]*Product, 0, len(result))

		// 本次获取到商品的键值 用来生成新的 alive 集合
		keys := make([]string, 0, len(result))
		for _, product := range result {
			keys = append(keys, product.Href)

			if victim.ContainsOne(product.Href) {
				// 商品未实际下架 复活进 alive
				victim.Remove(product.Href)
				alive.Add(product.Href)
			} else if !alive.ContainsOne(product.Href) {
				// 商品上新
				alive.Add(product.Href)
				products[product.Href] = product
				newProducts = append(newProducts, product)
				continue
			}

			// 判断当前商品与已保存的信息是否一致
			if !product.Equal(products[product.Href]) {
				updateProducts = append(updateProducts, product)
				products[product.Href] = product
			}
		}

		// 统计确实下架了的商品
		victim.Each(func(k string) bool {
			removalProducts = append(removalProducts, products[k])
			delete(products, k)
			return false
		})

		newSet := mapset.NewSet(keys...)
		victim = alive.Difference(newSet)
		alive = newSet

		if len(newProducts) != 0 && a.OnNew != nil {
			a.OnNew(newProducts)
		}
		if len(updateProducts) != 0 && a.OnUpdate != nil {
			a.OnUpdate(updateProducts)
		}
		if len(removalProducts) != 0 && a.OnRemoval != nil {
			a.OnRemoval(removalProducts)
		}
	}

	return nil
}
