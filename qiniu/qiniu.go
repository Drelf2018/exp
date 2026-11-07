package qiniu

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gabriel-vasile/mimetype"
	"github.com/qiniu/go-sdk/v7/storagev2/apis"
	"github.com/qiniu/go-sdk/v7/storagev2/credentials"
	"github.com/qiniu/go-sdk/v7/storagev2/http_client"
	"github.com/qiniu/go-sdk/v7/storagev2/objects"
	"github.com/qiniu/go-sdk/v7/storagev2/uploader"
	_ "golang.org/x/image/webp"
)

var (
	ErrInvalidQueryKey   = errors.New("query key contains invalid UTF-8")
	ErrInvalidQueryValue = errors.New("query value contains invalid UTF-8")
)

// QueryToHash 将 URL 查询参数规范化后计算短哈希。
//
// 将参数名升序排序，同名参数的多个值也升序排序，拼成 k1=v1&k1=v2&k2=v3 的形式，
// 使用 SHA-256 算法取哈希值的十六进制小写字符串的前 7 位。
func QueryToHash(rawQuery string) (string, error) {
	rawQuery = strings.TrimPrefix(rawQuery, "?")
	if rawQuery == "" {
		return "", nil
	}

	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return "", err
	}
	if len(values) == 0 {
		return "", nil
	}

	// 非法 UTF-8 也视为查询解析失败
	keys := make([]string, 0, len(values))
	for key, vals := range values {
		if !utf8.ValidString(key) {
			return "", ErrInvalidQueryKey
		}
		for _, value := range vals {
			if !utf8.ValidString(value) {
				return "", ErrInvalidQueryValue
			}
		}
		keys = append(keys, key)
	}

	// 参数名排序
	sort.Strings(keys)
	var normalized bytes.Buffer
	first := true
	for _, key := range keys {
		vals := values[key]
		// 参数值排序
		sort.Strings(vals)
		for _, value := range vals {
			if !first {
				normalized.WriteByte('&')
			}
			first = false
			normalized.WriteString(key)
			normalized.WriteByte('=')
			normalized.WriteString(value)
		}
	}

	sum := sha256.Sum256(normalized.Bytes())
	return hex.EncodeToString(sum[:])[:7], nil
}

// ObjectKeyError 将资源链接转成七牛云对象名时发生的错误
type ObjectKeyError struct {
	Op  string
	URL string
	Err error
}

func (e *ObjectKeyError) Unwrap() error {
	return e.Err
}

func (e *ObjectKeyError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("qiniu: %s %q", e.Op, redactURL(e.URL))
	}
	return fmt.Sprintf("qiniu: %s %q: %s", e.Op, redactURL(e.URL), unwrapURLError(e.Err))
}

// URLToKey 把资源链接转换成七牛对象名。
//
// Key 形态为 <scheme>/<host>[/<port>][/<path>][@<hash>]，不带前导 "/"。
// query 不进入 path，只通过 QueryToHash 生成的 7 位短哈希追加在后面。
// fragment 会被丢弃。默认端口（http/80、https/443）省略，IPv6 主机去掉方括号。
func URLToKey(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", &ObjectKeyError{Op: "invalid URL", URL: rawURL, Err: err}
	}

	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", &ObjectKeyError{Op: "unsupported URL scheme \"" + u.Scheme + "\" in", URL: rawURL}
	}

	host := strings.ToLower(u.Hostname())
	if host == "" {
		return "", &ObjectKeyError{Op: "missing host in", URL: rawURL}
	}

	port, err := normalizePort(u.Port(), scheme)
	if err != nil {
		return "", &ObjectKeyError{Op: "normalize port", URL: rawURL, Err: err}
	}

	// 开始构建
	var key strings.Builder
	key.WriteString(scheme)
	key.WriteByte('/')
	key.WriteString(host)
	if port != "" {
		key.WriteByte('/')
		key.WriteString(port)
	}

	// 去除空路径
	for _, segment := range strings.Split(u.EscapedPath(), "/") {
		if segment != "" {
			key.WriteByte('/')
			key.WriteString(segment)
		}
	}

	// 追加哈希值作为不同查询参数的辨识符
	if u.RawQuery != "" {
		hash, err := QueryToHash(u.RawQuery)
		if err != nil {
			return "", &ObjectKeyError{Op: "cannot hash query in", URL: rawURL, Err: err}
		}
		if hash != "" {
			key.WriteByte('@')
			key.WriteString(hash)
		}
	}

	return key.String(), nil
}

// normalizePort 校验并规范化端口，默认端口返回空字符串
func normalizePort(port, scheme string) (string, error) {
	if port == "" {
		return "", nil
	}
	for i := 0; i < len(port); i++ {
		if port[i] < '0' || port[i] > '9' {
			return "", fmt.Errorf("invalid port %q", port)
		}
	}
	n, err := strconv.ParseUint(port, 10, 16)
	if err != nil || n == 0 {
		return "", fmt.Errorf("invalid port %q", port)
	}
	normalized := strconv.FormatUint(n, 10)
	if (scheme == "http" && normalized == "80") || (scheme == "https" && normalized == "443") {
		return "", nil
	}
	return normalized, nil
}

// unwrapURLError 去掉 *url.Error 中携带的原始 URL，避免错误信息泄露隐私
func unwrapURLError(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) && urlErr.Err != nil {
		return urlErr.Err
	}
	return err
}

// redactURL 去掉 URL 的 query 与 fragment，隐藏密码，并限制长度，供错误信息使用
func redactURL(rawURL string) string {
	if u, err := url.Parse(rawURL); err == nil {
		u.RawQuery = ""
		u.ForceQuery = false
		u.Fragment = ""
		u.RawFragment = ""
		rawURL = u.Redacted()
	} else if i := strings.IndexAny(rawURL, "?#"); i >= 0 {
		// URL 本身无法解析时无法可靠定位 query/fragment，退回按分隔符截断
		rawURL = rawURL[:i]
	}

	const maxRunes = 200
	runes := []rune(rawURL)
	if len(runes) > maxRunes {
		rawURL = string(runes[:maxRunes]) + "..."
	}
	return rawURL
}

// ImageToJPEG 将图片统一编码为 JPEG 格式图片，可传入空值
func ImageToJPEG(r io.Reader, options *jpeg.Options) (io.Reader, error) {
	var mime string
	if rs, ok := r.(io.ReadSeeker); ok {
		mtype, err := mimetype.DetectReader(rs)
		if err != nil {
			return nil, err
		}
		_, err = rs.Seek(0, io.SeekStart)
		if err != nil {
			return nil, err
		}
		mime = mtype.String()
	} else {
		header := bytes.NewBuffer(nil)
		// After DetectReader, the data read from input is copied into header.
		mtype, err := mimetype.DetectReader(io.TeeReader(r, header))
		if err != nil {
			return nil, err
		}
		mime = mtype.String()
		// Concatenate back the header to the rest of the file.
		// recycled now contains the complete, original data.
		r = io.MultiReader(header, r)
	}
	if strings.HasPrefix(mime, "image") && !strings.HasSuffix(mime, "jpeg") {
		img, format, err := image.Decode(r)
		if err != nil {
			return nil, fmt.Errorf("qiniu: invalid image format: %s: %w", format, err)
		}
		buf := new(bytes.Buffer)
		err = jpeg.Encode(buf, img, options)
		if err != nil {
			return nil, fmt.Errorf("qiniu: failed to encode image: %w", err)
		}
		return buf, nil
	}
	return r, nil
}

// Bucket 七牛云存储空间
type Bucket struct {
	Name      string `json:"name" yaml:"name" toml:"name"`
	AccessKey string `json:"access" yaml:"access" toml:"access"`
	SecretKey string `json:"secret" yaml:"secret" toml:"secret"`
}

func (b *Bucket) client() *http_client.Options {
	return &http_client.Options{Credentials: credentials.NewCredentials(b.AccessKey, b.SecretKey)}
}

func (b *Bucket) uploadManager() *uploader.UploadManager {
	return uploader.NewUploadManager(&uploader.UploadManagerOptions{Options: *b.client()})
}

func (b *Bucket) objectsManager() *objects.ObjectsManager {
	return objects.NewObjectsManager(&objects.ObjectsManagerOptions{Options: *b.client()})
}

func (b *Bucket) storage() *apis.Storage {
	return apis.NewStorage(b.client())
}

func (b *Bucket) bucket() *objects.Bucket {
	return b.objectsManager().Bucket(b.Name)
}

func (b *Bucket) object(name string) *objects.Object {
	return b.bucket().Object(name)
}

func (b *Bucket) objectOptions(path string) *uploader.ObjectOptions {
	return &uploader.ObjectOptions{BucketName: b.Name, ObjectName: &path, FileName: filepath.Base(path)}
}

// DeleteAfterDays 设置过期自动删除
func (b *Bucket) DeleteAfterDays(ctx context.Context, path string, days int64) error {
	if days <= 0 {
		return nil
	}
	return b.object(path).SetLifeCycle().DeleteAfterDays(days).Call(ctx)
}

// Upload 上传 io.Reader
func (b *Bucket) Upload(ctx context.Context, r io.Reader, path string) error {
	err := b.uploadManager().UploadReader(ctx, r, b.objectOptions(path), nil)
	if err != nil {
		return unwrapURLError(err)
	}
	return nil
}

// UploadURL 下载链接对应资源并上传
func (b *Bucket) UploadURL(ctx context.Context, url string, path string) error {
	if path == "" {
		generated, err := URLToKey(url)
		if err != nil {
			return err
		}
		path = generated
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("qiniu: new request with %q: %w", redactURL(url), unwrapURLError(err))
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("qiniu: request %q: %w", redactURL(url), unwrapURLError(err))
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("qiniu: upload %q: %s", redactURL(url), resp.Status)
	}
	return b.Upload(ctx, resp.Body, path)
}

// FetchURL 通过七牛服务端 Fetch 将远程资源转存到当前空间
func (b *Bucket) FetchURL(ctx context.Context, url string, path string) (*apis.FetchObjectResponse, error) {
	if path == "" {
		generated, err := URLToKey(url)
		if err != nil {
			return nil, err
		}
		path = generated
	}
	req := &apis.FetchObjectRequest{
		FromUrl: url,
		ToEntry: b.Name + ":" + path,
	}
	resp, err := b.storage().FetchObject(ctx, req, nil)
	if err != nil {
		return nil, fmt.Errorf("qiniu: fetch %q: %w", redactURL(url), err)
	}
	return resp, nil
}
