package storage

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/Abdull247/KludgeServer/internal/config"
)

type Object struct {
	Key         string `json:"key"`
	URL         string `json:"url"`
	ContentType string `json:"contentType"`
	Size        int64  `json:"size"`
	Kind        string `json:"kind"`
}

type Store struct {
	cfg *config.Config
}

func New(cfg *config.Config) *Store {
	return &Store{cfg: cfg}
}

func (s *Store) Configured() bool {
	return s.cfg != nil && s.cfg.R2AccessKeyID != "" && s.cfg.R2SecretAccessKey != "" && s.cfg.R2BucketName != "" && s.cfg.R2Endpoint != ""
}

func (s *Store) PublicURL(key string) string {
	base := strings.TrimRight(s.cfg.R2PublicURL, "/")
	if base == "" {
		base = strings.TrimRight(s.cfg.R2Endpoint, "/") + "/" + s.cfg.R2BucketName
	}
	return base + "/" + strings.TrimLeft(key, "/")
}

func (s *Store) Put(ctx context.Context, key string, body []byte, contentType string) (Object, error) {
	if !s.Configured() {
		return Object{}, fmt.Errorf("object storage is not configured")
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	endpoint := strings.TrimRight(s.cfg.R2Endpoint, "/")
	putURL := endpoint + "/" + s.cfg.R2BucketName + "/" + strings.TrimLeft(key, "/")

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, putURL, bytes.NewReader(body))
	if err != nil {
		return Object{}, err
	}
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	}
	req.ContentLength = int64(len(body))
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("x-amz-content-sha256", sha256Hex(body))
	if err := signAWS(req, s.cfg.R2AccessKeyID, s.cfg.R2SecretAccessKey, "auto", "s3"); err != nil {
		return Object{}, err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return Object{}, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(res.Body, 4<<10))
		return Object{}, fmt.Errorf("r2 put failed (%d): %s", res.StatusCode, strings.TrimSpace(string(msg)))
	}

	return Object{
		Key:         key,
		URL:         s.PublicURL(key),
		ContentType: contentType,
		Size:        int64(len(body)),
	}, nil
}

func NewKey(kind, filename string) string {
	ext := strings.ToLower(path.Ext(filename))
	if len(ext) > 8 {
		ext = ""
	}
	stamp := time.Now().UTC().Format("20060102")
	return fmt.Sprintf("%s/%s/%s%s", kind, stamp, randomID(12), ext)
}

func randomID(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return hex.EncodeToString([]byte(time.Now().UTC().Format("150405.000000000")))
	}
	return hex.EncodeToString(b)[:n]
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func hmacSHA256(key []byte, data string) []byte {
	m := hmac.New(sha256.New, key)
	_, _ = m.Write([]byte(data))
	return m.Sum(nil)
}

func signAWS(req *http.Request, accessKey, secret, region, service string) error {
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")
	payloadHash := req.Header.Get("x-amz-content-sha256")
	if payloadHash == "" {
		payloadHash = "UNSIGNED-PAYLOAD"
		req.Header.Set("x-amz-content-sha256", payloadHash)
	}
	req.Header.Set("x-amz-date", amzDate)
	host := strings.ToLower(req.Host)
	if host == "" {
		host = strings.ToLower(req.URL.Host)
	}
	req.Header.Set("Host", host)

	contentType := req.Header.Get("Content-Type")
	canonicalHeaders := "content-type:" + contentType + "\n" +
		"host:" + host + "\n" +
		"x-amz-content-sha256:" + payloadHash + "\n" +
		"x-amz-date:" + amzDate + "\n"
	signedHeaders := "content-type;host;x-amz-content-sha256;x-amz-date"
	canonicalURI := req.URL.EscapedPath()
	if canonicalURI == "" {
		canonicalURI = "/"
	}
	canonicalQuery := req.URL.RawQuery
	canonicalRequest := strings.Join([]string{
		req.Method,
		canonicalURI,
		canonicalQuery,
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")

	scope := dateStamp + "/" + region + "/" + service + "/aws4_request"
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		scope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")

	kDate := hmacSHA256([]byte("AWS4"+secret), dateStamp)
	kRegion := hmacSHA256(kDate, region)
	kService := hmacSHA256(kRegion, service)
	kSigning := hmacSHA256(kService, "aws4_request")
	sig := hex.EncodeToString(hmacSHA256(kSigning, stringToSign))

	req.Header.Set("Authorization", fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		accessKey, scope, signedHeaders, sig,
	))
	return nil
}

func SafeFilename(name string) string {
	name = path.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == "/" {
		return "upload.bin"
	}
	return name
}
