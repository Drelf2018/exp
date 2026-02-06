package model

import (
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type Database string

func (d Database) Open(dst ...any) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(string(d)))
	if err != nil {
		return nil, err
	}
	return db, db.AutoMigrate(dst...)
}
