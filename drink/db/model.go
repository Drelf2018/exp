package db

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"gorm.io/gorm"
)

// Cookie 是 GORM model
type Cookie struct {
	Name      string
	Value     string
	CreatedAt time.Time
}

// FromHttpCookies 将 *http.Cookie 切片转换成 GORM model 切片
func FromHttpCookies(cookies []*http.Cookie) []Cookie {
	r := make([]Cookie, 0, len(cookies))
	for _, cookie := range cookies {
		if cookie.Name != "" {
			r = append(r, Cookie{Name: cookie.Name, Value: cookie.Value})
		}
	}
	return r
}

// ToHttpCookies 将 GORM model 切片转换成 *http.Cookie 切片
func ToHttpCookies(models []Cookie) []*http.Cookie {
	cookies := make([]*http.Cookie, 0, len(models))
	for _, m := range models {
		cookies = append(cookies, &http.Cookie{Name: m.Name, Value: m.Value})
	}
	return cookies
}

// UpdateUserCookies 根据 UID 更新表 user_{UID} 中的 Cookie
func UpdateUserCookies(uid int, cookies []*http.Cookie) error {
	if db == nil {
		return fmt.Errorf("db instance cannot be nil")
	}
	cookieModels := FromHttpCookies(cookies)
	if len(cookieModels) == 0 {
		return nil
	}
	// 创建用户 Cookie 表
	tableName := fmt.Sprintf("user_%d", uid)
	if !db.Migrator().HasTable(tableName) {
		if err := db.Table(tableName).AutoMigrate(&Cookie{}); err != nil {
			return fmt.Errorf("failed to create table \"%s\": %w", tableName, err)
		}
	}
	// 批量创建/更新
	result := db.Table(tableName).Create(&cookieModels)
	if result.Error != nil {
		return fmt.Errorf("failed to upsert cookies from table \"%s\": %w", tableName, result.Error)
	}
	return nil
}

var ErrDBNotExists = errors.New("drink/db: db instance cannot be nil")
var ErrUIDNotExists = errors.New("drink/db: uid cannot be zero")
var ErrTableNotExists = errors.New("drink/db: table not exists")

// GetUserCookies 根据 UID 从表 user_{UID} 中读取 Cookie 并转换成 []*http.Cookie
func GetUserCookies(uid int) ([]*http.Cookie, error) {
	if db == nil {
		return nil, ErrDBNotExists
	}
	if uid == 0 {
		return nil, ErrUIDNotExists
	}
	// 表不存在则返回空切片
	tableName := fmt.Sprintf("user_%d", uid)
	if !db.Migrator().HasTable(tableName) {
		return nil, ErrTableNotExists
	}
	var cookieModels []Cookie
	subquery := db.Table(tableName).Select("name, MAX(created_at)").Group("name")
	result := db.Table(tableName).Where("(name, created_at) IN (?)", subquery).Find(&cookieModels)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to query cookies from table \"%s\": %w", tableName, result.Error)
	}
	// 转换为 http.Cookie 切片返回
	return ToHttpCookies(cookieModels), nil
}

type Drink struct {
	Name      string `gorm:"primarykey"`
	Value     string
	CreatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// GetRandomDrink 从数据库随机获取一个未删除的 Drink
func GetRandomDrink() (drink Drink, err error) {
	err = db.Order("RANDOM()").First(&drink).Error
	return
}

// DrinkUp 喝掉指定名称的饮料
func DrinkUp(drinkName string) error {
	return db.Delete(&Drink{}, "name = ?", drinkName).Error
}
