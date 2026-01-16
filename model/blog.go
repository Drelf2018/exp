package model

import (
	"fmt"
	"time"

	"github.com/Drelf2018/req/template"
	"gorm.io/gorm"
)

// Role 用户权限
type Role uint64

const (
	Invalid Role = iota // 无效
	Normal              // 普通
	Trusted             // 信任
	Admin               // 管理员
	Owner               // 所有者
)

// IsAdmin 判断是否有管理权
func (r Role) IsAdmin() bool {
	return r == Owner || r == Admin
}

// IsOwner 判断是否为所有者
func (r Role) IsOwner() bool {
	return r == Owner
}

// User 用户模型
type User struct {
	UID      string         `json:"uid" gorm:"primaryKey"`         // 用户标识符
	Name     string         `json:"name"`                          // 用户名
	Role     Role           `json:"role"`                          // 权限
	Title    string         `json:"title"`                         // 头衔
	Password string         `json:"-"`                             // 密码
	Created  time.Time      `json:"created" gorm:"autoCreateTime"` // 建号时间
	Issued   time.Time      `json:"-"`                             // 签发时间
	Unban    time.Time      `json:"unban"`                         // 解封时间
	Extra    map[string]any `json:"-" gorm:"serializer:json"`      // 扩展字段
}

// Blog 博文模型
type Blog struct {
	ID            uint64         `json:"id" gorm:"primaryKey;autoIncrement"`                                   // 数据库内标识符
	Edited        uint64         `json:"edited"`                                                               // 编辑次数
	UID           string         `json:"uid" gorm:"index:idx_blogs;index:idx_type_blogs"`                      // 博主标识符
	Name          string         `json:"name"`                                                                 // 博主昵称
	Avatar        string         `json:"avatar"`                                                               // 头像链接
	Banner        string         `json:"banner"`                                                               // 头图链接
	Platform      string         `json:"platform" gorm:"index:idx_blogs;index:idx_type_blogs"`                 // 发布平台
	Follower      string         `json:"follower"`                                                             // 粉丝数量
	Following     string         `json:"following"`                                                            // 关注数量
	Description   string         `json:"description"`                                                          // 个人简介
	MID           string         `json:"mid" gorm:"column:mid"`                                                // 博文标识符
	URL           string         `json:"url"`                                                                  // 博文链接
	Type          string         `json:"type" gorm:"index:idx_type_blogs"`                                     // 博文类型
	Time          time.Time      `json:"time" gorm:"index:idx_blogs,sort:desc;index:idx_type_blogs,sort:desc"` // 发布时间
	Title         string         `json:"title"`                                                                // 博文标题
	Source        string         `json:"source"`                                                               // 博文来源
	Content       string         `json:"content"`                                                              // 原始内容
	Plaintext     string         `json:"plaintext"`                                                            // 纯文本内容
	Assets        []string       `json:"assets" gorm:"serializer:json"`                                        // 资源链接
	Reply         *Blog          `json:"reply"`                                                                // 被本文回复的博文
	ReplyID       *uint64        `json:"-"`                                                                    // 被本文回复的博文的数据库标识符
	Comments      []*Blog        `json:"comments"`                                                             // 本文的所有评论，包括二级评论
	BlogID        *uint64        `json:"-"`                                                                    // 如果本文是评论，则为根博文的数据库标识符
	Contributor   *User          `json:"contributor"`                                                          // 贡献者
	ContributorID string         `json:"-"`                                                                    // 贡献者标识符
	Extra         map[string]any `json:"extra" gorm:"serializer:json"`                                         // 扩展字段
	Created       time.Time      `json:"created" gorm:"autoCreateTime"`                                        // 博文创建时间
}

func (b *Blog) BeforeCreate(*gorm.DB) (err error) {
	if b.Plaintext == "" {
		b.Plaintext, err = template.Plaintext(b.Content)
		if err != nil {
			return
		}
	}
	b.ID = 0
	if b.Reply != nil {
		b.Reply.Contributor = b.Contributor
	}
	b.Comments = nil
	b.Created = time.Time{}
	return nil
}

var MaxTextLength = 18

func (b Blog) String() string {
	var text string
	if b.Plaintext != "" {
		text = b.Plaintext
	} else {
		text = b.Content
	}
	if MaxTextLength > 0 && len(text) > MaxTextLength {
		text = text[:MaxTextLength] + "..."
	}
	if b.Reply != nil {
		return fmt.Sprintf("@%s:%s//%s", b.Name, text, b.Reply)
	}
	return fmt.Sprintf("@%s:%s", b.Name, text)
}
