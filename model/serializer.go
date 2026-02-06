package model

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"net/url"
	"reflect"

	"github.com/Drelf2018/req"
	"gorm.io/gorm/schema"
)

// Serializer 在数据库读写时用来序列化和反序列化
type Serializer[T any, V driver.Value] struct {
	Scanner func(V) T
	Valuer  func(T) V
}

// Scan implements serializer interface
func (s Serializer[T, V]) Scan(ctx context.Context, field *schema.Field, dst reflect.Value, dbValue any) error {
	if dbValue == nil {
		return nil
	}
	if v, ok := dbValue.(V); ok {
		return field.Set(ctx, dst, s.Scanner(v))
	}
	return fmt.Errorf("model: invalid value type: %T", dbValue)
}

// Value implements serializer interface
func (s Serializer[T, V]) Value(ctx context.Context, field *schema.Field, dst reflect.Value, fieldValue any) (any, error) {
	if fieldValue == nil {
		return nil, nil
	}
	if t, ok := fieldValue.(T); ok {
		return s.Valuer(t), nil
	}
	return nil, fmt.Errorf("model: invalid field type: %T", fieldValue)
}

// ErrorSerializer 用来序列化和反序列化错误接口
var ErrorSerializer schema.SerializerInterface = Serializer[error, string]{
	Scanner: errors.New,
	Valuer:  error.Error,
}

// URLSerializer 用来序列化和反序列化 *url.URL
var URLSerializer schema.SerializerInterface = Serializer[*url.URL, string]{
	Scanner: req.MustParseURL,
	Valuer:  (*url.URL).String,
}
