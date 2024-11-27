package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// アクセストークンを取得する関数
func getAccessToken(clientID, clientSecret, tenantID string) (string, error) {
	url := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", tenantID)
	data := fmt.Sprintf(
		"grant_type=client_credentials&client_id=%s&client_secret=%s&scope=https://graph.microsoft.com/.default",
		clientID, clientSecret,
	)

	req, err := http.NewRequest("POST", url, strings.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get access token: %s", resp.Status)
	}

	var tokenResponse struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResponse); err != nil {
		return "", fmt.Errorf("failed to decode token response: %w", err)
	}

	fmt.Println(tokenResponse.AccessToken)

	return tokenResponse.AccessToken, nil
}

// SharePoint サイト ID を取得する関数
func getSiteID(accessToken, hostname, sitePath string) (string, error) {
	url := fmt.Sprintf("https://graph.microsoft.com/v1.0/sites/%s:/%s", hostname, sitePath)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		fmt.Printf("Error response body: %s\n", string(body))
		return "", fmt.Errorf("failed to get site ID: %s", resp.Status)
	}

	var siteInfo struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&siteInfo); err != nil {
		return "", fmt.Errorf("failed to decode site info: %w", err)
	}

	return siteInfo.ID, nil
}

// ファイルをアップロードする関数
func uploadFileToSharePoint(accessToken, siteID, documentLibrary, filePath string) error {
	// アップロードセッションを作成するURL
	fileName := filepath.Base(filePath)
	url := fmt.Sprintf("https://graph.microsoft.com/v1.0/sites/%s/drive/root:/%s/%s:/createUploadSession", siteID, documentLibrary, fileName)

	// セッション作成リクエスト
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create upload session request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send upload session request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to create upload session: %s", resp.Status)
	}

	var sessionResponse struct {
		UploadURL string `json:"uploadUrl"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&sessionResponse); err != nil {
		return fmt.Errorf("failed to decode upload session response: %w", err)
	}

	// ファイルを開く
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// ファイルを分割してアップロード
	const chunkSize = 320 * 1024 // 320 KB
	buffer := make([]byte, chunkSize)
	offset := int64(0)
	fileInfo, _ := file.Stat()
	totalSize := fileInfo.Size()

	for {
		n, err := file.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read file chunk: %w", err)
		}

		// チャンクをアップロード
		req, err = http.NewRequest("PUT", sessionResponse.UploadURL, bytes.NewReader(buffer[:n]))
		if err != nil {
			return fmt.Errorf("failed to create chunk upload request: %w", err)
		}
		req.Header.Set("Content-Length", fmt.Sprintf("%d", n))
		req.Header.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", offset, offset+int64(n)-1, totalSize))

		resp, err = client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to send chunk upload request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
			return fmt.Errorf("chunk upload failed: %s", resp.Status)
		}

		offset += int64(n)
	}

	fmt.Println("File uploaded successfully")
	return nil
}

// メイン関数
func main() {
	clientID := os.Getenv("CLIENT_ID")
	clientSecret := os.Getenv("CLIENT_SECRET")
	tenantID := os.Getenv("TENANT_ID")
	hostname := os.Getenv("HOSTNAME")
	sitePath := os.Getenv("SITE_PATH")
	documentLibrary := os.Getenv("DOCUMENT_LIBRARY")
	filePath := "file.txt"

	// アクセストークンを取得
	accessToken, err := getAccessToken(clientID, clientSecret, tenantID)
	if err != nil {
		fmt.Printf("Error getting access token: %v\n", err)
		return
	}

	// SharePoint サイト ID を取得
	siteID, err := getSiteID(accessToken, hostname, sitePath)
	if err != nil {
		fmt.Printf("Error getting site ID: %v\n", err)
		return
	}

	// ファイルをアップロード
	if err := uploadFileToSharePoint(accessToken, siteID, documentLibrary, filePath); err != nil {
		fmt.Printf("Error uploading file: %v\n", err)
		return
	}
}
