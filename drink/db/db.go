package db

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var db *gorm.DB

func Open(dsn string) (err error) {
	db, err = gorm.Open(sqlite.Open(dsn))
	if err != nil {
		return
	}
	return db.AutoMigrate(&Drink{})
}

func CreateDrinks(jsonFile string) error {
	data, err := os.ReadFile(jsonFile)
	if err != nil {
		return err
	}
	var drinksSlice []string
	err = json.Unmarshal(data, &drinksSlice)
	if err != nil {
		return err
	}
	drinks := make([]*Drink, 0, len(drinksSlice))
	for _, d := range drinksSlice {
		name, _, _ := strings.Cut(strings.TrimPrefix(d, "我今天喝了"), "，")
		drinks = append(drinks, &Drink{Name: name, Value: d})
	}
	return db.Clauses(clause.OnConflict{DoNothing: true}).Create(drinks).Error
}
