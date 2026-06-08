package app

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type ossFile struct {
	FileURL    string `json:"file_url"`
	MD5        string `json:"md5"`
	FileSize   string `json:"file_size"`
	FileName   string `json:"file_name"`
	CreateTime string `json:"create_time"`
}

type prepareUploadLocalReq struct {
	MD5      string `json:"md5"`
	FileSize string `json:"file_size"`
	FileName string `json:"file_name"`
}

type prepareUploadLocalRsp struct {
	Errno    int     `json:"errno"`
	Errstr   string  `json:"errstr"`
	UploadID string  `json:"upload_id"`
	File     ossFile `json:"file,omitempty"`
}

type uploadLocalRsp struct {
	Errno  int     `json:"errno"`
	Errstr string  `json:"errstr"`
	File   ossFile `json:"file,omitempty"`
}

func (c *client) uploadLocalFile(ctx context.Context, path string) (string, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("read image %q: %w", path, err)
	}
	if stat.IsDir() {
		return "", fmt.Errorf("read image %q: is a directory", path)
	}

	sum, err := fileMD5(path)
	if err != nil {
		return "", err
	}
	prepared, err := c.prepareUploadLocal(ctx, prepareUploadLocalReq{
		MD5:      sum,
		FileSize: strconv.FormatInt(stat.Size(), 10),
		FileName: filepath.Base(path),
	})
	if err != nil {
		return "", err
	}
	if prepared.File.FileURL != "" {
		return prepared.File.FileURL, nil
	}
	if strings.TrimSpace(prepared.UploadID) == "" {
		return "", errorsFromOSS("prepare upload local", prepared.Errno, prepared.Errstr, "missing upload_id")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read image %q: %w", path, err)
	}
	uploaded, err := c.uploadLocalWithID(ctx, data, filepath.Base(path), prepared.UploadID, sum)
	if err != nil {
		return "", err
	}
	if uploaded.File.FileURL == "" {
		return "", errorsFromOSS("upload local", uploaded.Errno, uploaded.Errstr, "missing file_url")
	}
	return uploaded.File.FileURL, nil
}

func (c *client) uploadLocalData(ctx context.Context, data []byte, fileName string) (string, error) {
	sum := md5.Sum(data)
	prepared, err := c.prepareUploadLocal(ctx, prepareUploadLocalReq{
		MD5:      hex.EncodeToString(sum[:]),
		FileSize: strconv.Itoa(len(data)),
		FileName: fileName,
	})
	if err != nil {
		return "", err
	}
	if prepared.File.FileURL != "" {
		return prepared.File.FileURL, nil
	}
	if strings.TrimSpace(prepared.UploadID) == "" {
		return "", errorsFromOSS("prepare upload local", prepared.Errno, prepared.Errstr, "missing upload_id")
	}

	uploaded, err := c.uploadLocalWithID(ctx, data, fileName, prepared.UploadID, hex.EncodeToString(sum[:]))
	if err != nil {
		return "", err
	}
	if uploaded.File.FileURL == "" {
		return "", errorsFromOSS("upload local", uploaded.Errno, uploaded.Errstr, "missing file_url")
	}
	return uploaded.File.FileURL, nil
}

func (c *client) uploadLocalWithID(ctx context.Context, data []byte, fileName, uploadID, sum string) (*uploadLocalRsp, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if uploadID != "" {
		if err := writer.WriteField("upload_id", uploadID); err != nil {
			return nil, err
		}
	}
	if err := writer.WriteField("md5", sum); err != nil {
		return nil, err
	}
	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(part, bytes.NewReader(data)); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	httpReq, err := c.newOSSRequest(ctx, "/go/oss/upload_local/", &body)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	c.logRequest(httpReq, nil)
	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()

	respData, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, err
	}
	c.logResponse(httpResp, respData)
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return nil, fmt.Errorf("OSS upload local failed: %s: %s", httpResp.Status, strings.TrimSpace(string(respData)))
	}

	var rsp uploadLocalRsp
	if err := json.Unmarshal(respData, &rsp); err != nil {
		return nil, fmt.Errorf("decode OSS upload local response: %w", err)
	}
	if rsp.Errno != 0 {
		return nil, errorsFromOSS("upload local", rsp.Errno, rsp.Errstr, "")
	}
	return &rsp, nil
}

func fileMD5(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("read image %q: %w", path, err)
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("hash image %q: %w", path, err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (c *client) prepareUploadLocal(ctx context.Context, req prepareUploadLocalReq) (*prepareUploadLocalRsp, error) {
	var rsp prepareUploadLocalRsp
	if err := c.postOSSJSON(ctx, "/go/oss/prepare_upload_local/", req, &rsp); err != nil {
		return nil, err
	}
	if rsp.Errno != 0 {
		return nil, errorsFromOSS("prepare upload local", rsp.Errno, rsp.Errstr, "")
	}
	return &rsp, nil
}

func (c *client) postOSSJSON(ctx context.Context, path string, in any, out any) error {
	body, err := json.Marshal(in)
	if err != nil {
		return err
	}
	httpReq, err := c.newOSSRequest(ctx, path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	c.logRequest(httpReq, body)

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer httpResp.Body.Close()

	data, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return err
	}
	c.logResponse(httpResp, data)
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return fmt.Errorf("OSS request failed: %s: %s", httpResp.Status, strings.TrimSpace(string(data)))
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode OSS response: %w", err)
	}
	return nil
}

func (c *client) newOSSRequest(ctx context.Context, path string, body io.Reader) (*http.Request, error) {
	if strings.TrimSpace(c.ossURL) == "" {
		return nil, fmt.Errorf("missing OSS URL: pass -oss-url or set KUAIMA_OSS_URL")
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.ossURL+path, body)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Accept", "application/json")
	return httpReq, nil
}

func errorsFromOSS(action string, errno int, errstr, fallback string) error {
	message := strings.TrimSpace(errstr)
	if message == "" {
		message = fallback
	}
	if message == "" {
		message = "unknown error"
	}
	if errno != 0 {
		return fmt.Errorf("OSS %s failed: errno=%d: %s", action, errno, message)
	}
	return fmt.Errorf("OSS %s failed: %s", action, message)
}
