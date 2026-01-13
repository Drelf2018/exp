package model

import (
	"time"

	"gorm.io/gorm"
)

// 博文筛选条件，用来描述一类博文，例如：
//
// filter1 表示所有平台为 "weibo"、类型为 "comment" 的博文
//
// filter2 表示所有由 "114" 提交的用户 "514" 的博文
//
//	var filter1 = Filter{
//		Platform: "weibo",
//		Type: "comment",
//	}
//
//	var filter2 = Filter{
//		Contributor: "114",
//		UID: "514",
//	}
type Filter struct {
	Contributor string `json:"contributor" form:"contributor"` // 博文贡献者
	Platform    string `json:"platform" form:"platform"`       // 发布平台
	Type        string `json:"type" form:"type"`               // 博文类型
	UID         string `json:"uid" form:"uid"`                 // 账户序号
	TaskID      uint64 `json:"-" form:"-"`                     // 外键
}

func (f Filter) IsZero() bool {
	return f.Contributor == "" && f.Platform == "" && f.Type == "" && f.UID == ""
}

func (f Filter) IsValid() bool {
	return !f.IsZero() && f.TaskID != 0
}

// 请求记录
type RequestLog struct {
	StartedAt time.Time `json:"started_at"`
	CreatedAt time.Time `json:"created_at"`
	BlogID    uint64    `json:"blog_id"`
	Result    any       `json:"result" gorm:"serializer:json"` // 响应为 JSON 会自动解析
	Error     error     `json:"error"`                         // 请求过程中发生的错误
	TaskID    uint64    `json:"-" gorm:"index:idx_logs_query"` // 外键
}

// 任务
type Task struct {
	ID        uint64         `json:"id" gorm:"primaryKey;autoIncrement"`
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	Public bool `json:"public"` // 是否公开
	Enable bool `json:"enable"` // 是否启用

	Name        string `json:"name"`        // 任务名称
	Description string `json:"description"` // 任务简介

	Icon   string `json:"icon"`   // 任务图标
	Banner string `json:"banner"` // 任务头图
	Readme string `json:"readme"` // 任务描述

	ForkID    uint64 `json:"fork_id"`             // 复刻来源
	ForkCount uint64 `json:"fork_count" gorm:"-"` // 被复刻次数

	UserID string `json:"user_id"` // 外键

	Template string `json:"templates"` // 请求模板

	Filters []Filter     `json:"filters"` // 筛选条件
	Logs    []RequestLog `json:"logs"`    // 请求记录
}

func (t *Task) BeforeCreate(*gorm.DB) error {
	t.ID = 0
	t.CreatedAt = time.Time{}
	t.Logs = nil
	return nil
}

func (t *Task) AfterFind(tx *gorm.DB) error {
	return tx.Model(&Task{}).Select("count(*)").Find(&t.ForkCount, "fork_id = ?", t.ID).Error
}

// func (task *Task) Run(blog *Blog) RequestLog {
// 	log := RequestLog{
// 		BlogID:    blog.ID,
// 		TaskID:    task.ID,
// 		CreatedAt: time.Now(),
// 	}
// 	json.Marshal()
// 	st := NewTemplate()
// 	r, err := t.Do(task)
// 	if err != nil {
// 		log.Error = err.Error()
// 		log.FinishedAt = time.Now()
// 		return log
// 	}
// 	err = json.Unmarshal(r, &log.Result)
// 	if err != nil {
// 		log.Result = string(r)
// 		log.Error = err.Error()
// 	}
// 	log.FinishedAt = time.Now()
// 	return log
// }

// func (t *Template) RunTasks(tasks []*Task) []RequestLog {
// 	logs := make([]RequestLog, len(tasks))
// 	wg := &sync.WaitGroup{}
// 	wg.Add(len(tasks))
// 	for idx := range tasks {
// 		idx := idx
// 		go func() {
// 			logs[idx] = t.RunTask(tasks[idx])
// 			wg.Done()
// 		}()
// 	}
// 	wg.Wait()
// 	return logs
// }

// func NextTime(hour, min, sec int) time.Time {
// 	now := time.Now()
// 	next := time.Date(now.Year(), now.Month(), now.Day(), hour, min, sec, 0, now.Location())
// 	switch {
// 	case now.Hour() > hour,
// 		now.Hour() == hour && now.Minute() > min,
// 		now.Hour() == hour && now.Minute() == min && now.Second() > sec:
// 		return next.AddDate(0, 0, 1)
// 	default:
// 		return next
// 	}
// }

// app.Get("/level", func(c *fiber.Ctx) error {
// 		// 设置缓存过期头
// 		expire := NextTime(4, 0, 0)
// 		// c.Set("Expires", expire.UTC().Format(time.RFC1123))
// 		c.Set("Cache-Control", fmt.Sprintf("max-age=%d", int(time.Until(expire).Seconds())))
// 		// 计算等级
// 		var count int64
// 		result := db.Model(&model.Blog{}).Where(
// 			"platform = ? AND uid = ? AND contributor_id = ?",
// 			c.Query("platform"), c.Query("uid"), c.Query("contributor"),
// 		).Count(&count)
// 		if result.Error != nil {
// 			return Error(-1, result.Error)
// 		}
// 		var r Response
// 		switch {
// 		case count < 0:
// 			r.Data = 0
// 		case count < 100:
// 			r.Data = 1 + count/10
// 		case count < 250:
// 			r.Data = 11 + (count-100)/15
// 		case count < 650:
// 			r.Data = 21 + (count-250)/20
// 		case count < 1150:
// 			r.Data = 41 + (count-650)/25
// 		case count < 1750:
// 			r.Data = 61 + (count-1150)/30
// 		default:
// 			r.Data = 81 + (count-1750)/50
// 		}
// 		return c.JSON(r)
// 	})
