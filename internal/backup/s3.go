package backup

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// S3Config holds connection parameters for S3-compatible cloud object storage (AWS S3, Cloudflare R2, MinIO).
type S3Config struct {
	Endpoint       string `json:"endpoint"`
	Bucket         string `json:"bucket"`
	Region         string `json:"region"`
	AccessKey      string `json:"access_key"`
	SecretKey      string `json:"secret_key"`
	ForcePathStyle bool   `json:"force_path_style"`
}

// S3Client provides zero-external-dependency S3 operations using AWS Signature Version 4.
type S3Client struct {
	cfg        S3Config
	httpClient *http.Client
}

// NewS3Client creates a new S3Client.
func NewS3Client(cfg S3Config) *S3Client {
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}
	if cfg.Endpoint == "" {
		cfg.Endpoint = "https://s3.amazonaws.com"
	}
	return &S3Client{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// PutObject uploads data to S3 using AWS SigV4.
func (c *S3Client) PutObject(ctx context.Context, objectKey string, data io.Reader, size int64, contentType string) error {
	if c.cfg.Bucket == "" || c.cfg.AccessKey == "" || c.cfg.SecretKey == "" {
		return fmt.Errorf("s3 configuration incomplete: bucket, access key, and secret key required")
	}

	payloadBytes, err := io.ReadAll(data)
	if err != nil {
		return fmt.Errorf("failed reading payload: %w", err)
	}
	payloadHash := sha256Hex(payloadBytes)

	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")

	endpoint := strings.TrimRight(c.cfg.Endpoint, "/")
	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("invalid s3 endpoint URL: %w", err)
	}

	var reqURL string
	var hostHeader string
	canonicalURI := "/" + strings.TrimPrefix(objectKey, "/")

	if c.cfg.ForcePathStyle || strings.Contains(parsedURL.Host, "localhost") || strings.Contains(parsedURL.Host, "127.0.0.1") {
		reqURL = fmt.Sprintf("%s/%s%s", endpoint, c.cfg.Bucket, canonicalURI)
		hostHeader = parsedURL.Host
		canonicalURI = "/" + c.cfg.Bucket + canonicalURI
	} else {
		reqURL = fmt.Sprintf("%s://%s.%s%s", parsedURL.Scheme, c.cfg.Bucket, parsedURL.Host, canonicalURI)
		hostHeader = fmt.Sprintf("%s.%s", c.cfg.Bucket, parsedURL.Host)
	}

	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Canonical Headers
	canonicalHeaders := fmt.Sprintf("content-type:%s\nhost:%s\nx-amz-content-sha256:%s\nx-amz-date:%s\n",
		contentType, hostHeader, payloadHash, amzDate)
	signedHeaders := "content-type;host;x-amz-content-sha256;x-amz-date"

	canonicalRequest := strings.Join([]string{
		http.MethodPut,
		canonicalURI,
		"", // query string
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")

	algorithm := "AWS4-HMAC-SHA256"
	credentialScope := fmt.Sprintf("%s/%s/s3/aws4_request", dateStamp, c.cfg.Region)
	stringToSign := strings.Join([]string{
		algorithm,
		amzDate,
		credentialScope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")

	// Calculate Signature
	kDate := hmacSHA256([]byte("AWS4"+c.cfg.SecretKey), dateStamp)
	kRegion := hmacSHA256(kDate, c.cfg.Region)
	kService := hmacSHA256(kRegion, "s3")
	kSigning := hmacSHA256(kService, "aws4_request")
	signature := hex.EncodeToString(hmacSHA256(kSigning, stringToSign))

	authHeader := fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		algorithm, c.cfg.AccessKey, credentialScope, signedHeaders, signature)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, reqURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create http request: %w", err)
	}

	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Host", hostHeader)
	req.Header.Set("x-amz-date", amzDate)
	req.Header.Set("x-amz-content-sha256", payloadHash)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("s3 request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("s3 upload failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

