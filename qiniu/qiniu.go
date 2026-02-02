package qiniu

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gabriel-vasile/mimetype"
	"github.com/qiniu/go-sdk/v7/storagev2/credentials"
	"github.com/qiniu/go-sdk/v7/storagev2/http_client"
	"github.com/qiniu/go-sdk/v7/storagev2/objects"
	"github.com/qiniu/go-sdk/v7/storagev2/uploader"
	_ "golang.org/x/image/webp"
)

// UploadHandler 上传前处理器
type UploadHandler func(ctx context.Context, r io.Reader, path string) (io.Reader, error)

// EncodeJPEGImage 将图片统一编码为 JPEG 格式，可传入空值
func EncodeJPEGImage(options *jpeg.Options) UploadHandler {
	return func(ctx context.Context, r io.Reader, path string) (io.Reader, error) {
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
			if options == nil {
				options = &jpeg.Options{Quality: 80}
			}
			err = jpeg.Encode(buf, img, options)
			if err != nil {
				return nil, fmt.Errorf("qiniu: failed to encode image: %w", err)
			}
			return buf, nil
		}
		return r, nil
	}
}

var _ UploadHandler = EncodeJPEGImage(nil)

// TemporaryUploader 七牛云临时上传器
type TemporaryUploader struct {
	AccessKey       string `json:"access_key" yaml:"access_key" toml:"access_key" long:"access_key" description:"七牛云 AccessKey"`
	SecretKey       string `json:"secret_key" yaml:"secret_key" toml:"secret_key" long:"secret_key" description:"七牛云 SecretKey"`
	BucketName      string `json:"bucket_name" yaml:"bucket_name" toml:"bucket_name" long:"bucket_name" description:"七牛云空间名称"`
	DeleteAfterDays int64  `json:"delete_after_days" yaml:"delete_after_days" toml:"delete_after_days" long:"delete_after_days" description:"七牛云临时上传有效天数"`
}

// Upload 临时上传 io.Reader
func (t *TemporaryUploader) Upload(ctx context.Context, r io.Reader, path string, handlers ...UploadHandler) (err error) {
	// 上传前处理
	for _, handler := range handlers {
		r, err = handler(ctx, r, path)
		if err != nil {
			return
		}
	}
	// 上传 io.Reader
	clientOpts := http_client.Options{Credentials: credentials.NewCredentials(t.AccessKey, t.SecretKey)}
	uploadManager := uploader.NewUploadManager(&uploader.UploadManagerOptions{Options: clientOpts})
	objectOpts := &uploader.ObjectOptions{BucketName: t.BucketName, ObjectName: &path, FileName: filepath.Base(path)}
	err = uploadManager.UploadReader(ctx, r, objectOpts, nil)
	if err != nil {
		return err
	}
	// 设置删除时间
	if t.DeleteAfterDays <= 0 {
		return nil
	}
	bucket := objects.NewObjectsManager(&objects.ObjectsManagerOptions{Options: clientOpts}).Bucket(t.BucketName)
	return bucket.Object(path).SetLifeCycle().DeleteAfterDays(t.DeleteAfterDays).Call(ctx)
}

// UploadURL 下载链接对应资源后临时上传
func (t *TemporaryUploader) UploadURL(ctx context.Context, url string, path string, handlers ...UploadHandler) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return t.Upload(ctx, bytes.NewReader(b), path, handlers...)
}
