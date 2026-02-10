package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Storage interface {
	Upload(ctx context.Context, bucket, path string, data io.Reader, contentType string) error
	Download(ctx context.Context, bucket, path string) (io.ReadCloser, error)
	Delete(ctx context.Context, bucket, path string) error
	GetPublicURL(bucket, path string) string
}

type SupabaseStorage struct {
	baseURL    string
	serviceKey string
	httpClient *http.Client
}

func NewSupabaseStorage(supabaseURL, serviceKey string) *SupabaseStorage {
	return &SupabaseStorage{
		baseURL:    supabaseURL + "/storage/v1",
		serviceKey: serviceKey,
		httpClient: &http.Client{Timeout: 2 * time.Minute},
	}
}

func (s *SupabaseStorage) Upload(ctx context.Context, bucket, path string, data io.Reader, contentType string) error {
	url := fmt.Sprintf("%s/object/%s/%s", s.baseURL, bucket, path)

	buf := &bytes.Buffer{}
	if _, err := io.Copy(buf, data); err != nil {
		return fmt.Errorf("read upload data: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, buf)
	if err != nil {
		return fmt.Errorf("create upload request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.serviceKey)
	req.Header.Set("Content-Type", contentType)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("upload file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload failed (%d): %s", resp.StatusCode, string(body))
	}

	return nil
}

func (s *SupabaseStorage) Download(ctx context.Context, bucket, path string) (io.ReadCloser, error) {
	url := fmt.Sprintf("%s/object/%s/%s", s.baseURL, bucket, path)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create download request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.serviceKey)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download file: %w", err)
	}

	if resp.StatusCode >= 400 {
		resp.Body.Close()
		return nil, fmt.Errorf("download failed (%d)", resp.StatusCode)
	}

	return resp.Body, nil
}

func (s *SupabaseStorage) Delete(ctx context.Context, bucket, path string) error {
	url := fmt.Sprintf("%s/object/%s/%s", s.baseURL, bucket, path)

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("create delete request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.serviceKey)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("delete file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("delete failed (%d)", resp.StatusCode)
	}

	return nil
}

func (s *SupabaseStorage) GetPublicURL(bucket, path string) string {
	return fmt.Sprintf("%s/object/public/%s/%s", s.baseURL, bucket, path)
}
