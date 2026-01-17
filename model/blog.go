package model

import (
	"fmt"
	"strings"
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

// Extra 扩展字段
type Extra map[string]any

// User 用户模型
type User struct {
	UID      string    `json:"uid" gorm:"primaryKey"`         // 用户标识符
	Name     string    `json:"name"`                          // 用户名
	Role     Role      `json:"role"`                          // 权限
	Title    string    `json:"title"`                         // 头衔
	Password string    `json:"-"`                             // 密码
	Created  time.Time `json:"created" gorm:"autoCreateTime"` // 建号时间
	Issued   time.Time `json:"-"`                             // 签发时间
	Unban    time.Time `json:"unban"`                         // 解封时间
	Extra    Extra     `json:"-" gorm:"serializer:json"`      // 扩展字段
}

// Blog 博文模型
type Blog struct {
	ID         uint64    `json:"id" gorm:"primaryKey;autoIncrement"`                                   // 数据库内标识符
	UID        string    `json:"uid" gorm:"index:idx_blogs;index:idx_type_blogs"`                      // 博主标识符
	Name       string    `json:"name"`                                                                 // 博主昵称
	Desc       string    `json:"desc"`                                                                 // 个人简介
	Avatar     string    `json:"avatar"`                                                               // 头像链接
	Banner     string    `json:"banner"`                                                               // 头图链接
	Follower   string    `json:"follower"`                                                             // 粉丝数量
	Following  string    `json:"following"`                                                            // 关注数量
	MID        string    `json:"mid" gorm:"column:mid;index:idx_match"`                                // 博文标识符
	URL        string    `json:"url"`                                                                  // 博文链接
	Site       string    `json:"site" gorm:"index:idx_blogs;index:idx_type_blogs;index:idx_match"`     // 发布网站
	Type       string    `json:"type" gorm:"index:idx_type_blogs;index:idx_match"`                     // 博文类型
	Time       time.Time `json:"time" gorm:"index:idx_blogs,sort:desc;index:idx_type_blogs,sort:desc"` // 发布时间
	Title      string    `json:"title"`                                                                // 博文标题
	Source     string    `json:"source"`                                                               // 博文来源
	Version    string    `json:"version" gorm:"index:idx_match"`                                       // 编辑版本
	Content    string    `json:"content"`                                                              // 原始内容
	Plaintext  string    `json:"plaintext"`                                                            // 纯文本内容
	Assets     []string  `json:"assets" gorm:"serializer:json"`                                        // 资源链接
	Reply      *Blog     `json:"reply"`                                                                // 被本文回复的博文
	ReplyID    *uint64   `json:"-"`                                                                    // 被本文回复的博文的数据库标识符
	Comments   []*Blog   `json:"comments"`                                                             // 本文的所有评论，包括二级评论
	BlogID     *uint64   `json:"-"`                                                                    // 如果本文是评论，则为根博文的数据库标识符
	Uploader   *User     `json:"uploader"`                                                             // 上传者
	UploaderID string    `json:"-" gorm:"index:idx_match"`                                             // 上传者标识符
	Extra      Extra     `json:"extra" gorm:"serializer:json"`                                         // 扩展字段
	Created    time.Time `json:"created" gorm:"autoCreateTime"`                                        // 博文创建时间
}

// Match 匹配当前博文
func (b *Blog) Match(tx *gorm.DB) *gorm.DB {
	return tx.Where("mid = ? AND site = ? AND type = ? AND version = ? AND uploader_id = ?", b.MID, b.Site, b.Type, b.Version, b.UploaderID)
}

func (b *Blog) BeforeCreate(tx *gorm.DB) error {
	if b.Plaintext == "" {
		var err error
		b.Plaintext, err = template.Plaintext(b.Content)
		if err != nil {
			return err
		}
	}
	b.ID = 0
	if b.Reply != nil {
		// 查询被回复博文是否已经保存过
		var replyID uint64
		result := tx.Select("id").Model(&Blog{}).Scopes(b.Reply.Match).Limit(1).Find(&replyID)
		if result.Error != nil {
			return result.Error
		}
		// 已经保存过，直接使用已保存的 ID 作为外键
		// 否则，向被回复博文传递上传者信息
		if result.RowsAffected != 0 {
			b.ReplyID = &replyID
			b.Reply = nil
		} else {
			b.Reply.Uploader = b.Uploader
		}
	}
	b.Comments = nil
	b.Created = time.Time{}
	return nil
}

func (b Blog) String() string {
	var text string
	if b.Plaintext != "" {
		text = b.Plaintext
	} else {
		text = b.Content
	}
	text = strings.TrimSpace(strings.ReplaceAll(text, "\n", " "))
	if b.Reply != nil {
		return fmt.Sprintf("@%s:%s//%s", b.Name, text, b.Reply)
	}
	return fmt.Sprintf("@%s:%s", b.Name, text)
}
