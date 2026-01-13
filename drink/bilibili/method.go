package bilibili

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"

	"github.com/Drelf2018/req/method"
)

var ErrEmptyBiliJct = errors.New("bilibili: cookie \"bili_jct\" is empty")

type PostCSRF struct {
	ContentType string `req:"header" default:"application/x-www-form-urlencoded"`
}

func (PostCSRF) Method() string {
	return http.MethodPost
}

func (PostCSRF) Body(req *http.Request, value reflect.Value, body []reflect.StructField) (io.Reader, error) {
	biliJct, err := req.Cookie("bili_jct")
	if err != nil {
		return nil, fmt.Errorf("bilibili: %w: \"bili_jct\"", err)
	}
	if biliJct.Value == "" {
		return nil, ErrEmptyBiliJct
	}
	form := method.MakeURLValues(req.Context(), value, body)
	form.Set("csrf", biliJct.Value)
	form.Set("csrf_token", biliJct.Value)
	return strings.NewReader(form.Encode()), nil
}
