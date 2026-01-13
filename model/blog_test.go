package model_test

import (
	"testing"

	"github.com/Drelf2018/exp/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func init() {
	model.MaxTextLength = -1
}

func TestBlog(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("test.db"))
	if err != nil {
		t.Fatal(err)
	}
	err = db.AutoMigrate(&model.SimplyBlog{})
	if err != nil {
		t.Fatal(err)
	}
	var blog = &model.SimplyBlog{
		Name:    "第三位博主",
		Content: "最右太能藏东西了",
		Reply: &model.SimplyBlog{
			Name:    "第二位博主",
			Content: "到底是什么，细说",
			Reply: &model.SimplyBlog{
				Name:    "第一位博主",
				Content: "最近业内发生了一件大事，只能说懂的都懂",
			},
			Comments: []*model.SimplyBlog{
				{Name: "第一位评论", Content: "我也想知道", Comments: []*model.SimplyBlog{
					{Name: "第二位评论", Content: "原来你不知道吗"},
				}},
				{Name: "第三位评论", Content: "难道你知道？"},
			},
		},
		Comments: []*model.SimplyBlog{
			{Name: "第四位评论", Content: "你也藏了？"},
		},
	}
	err = db.Save(&blog).Error
	if err != nil {
		t.Fatal(err)
	}
}
