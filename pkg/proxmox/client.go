package proxmox

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/xpadev/k8s-node-monitor/pkg/config"
)

// Client はProxmox VE APIクライアント
type Client struct {
	apiURL      string
	username    string
	password    string
	tokenID     string
	tokenSecret string
	httpClient  *http.Client
	ticket      string
	csrfToken   string
}

// NewClient は新しいProxmox APIクライアントを作成します
func NewClient(config *config.ProxmoxConfig) *Client {
	// HTTPクライアントを設定（オプションでTLS検証をスキップ）
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	httpClient := &http.Client{
		Transport: tr,
		Timeout:   time.Second * 30,
	}

	return &Client{
		// フィールド名を正確に合わせる（大文字小文字を含む）
		apiURL:      config.ApiUrl,
		username:    config.Username,
		password:    config.Password,
		tokenID:     config.TokenID,
		tokenSecret: config.TokenSecret,
		httpClient:  httpClient,
	}
}

// Login はProxmox APIにログインします
func (c *Client) Login() error {
	// APIトークンがある場合はログイン不要
	if c.tokenID != "" && c.tokenSecret != "" {
		return nil
	}

	data := url.Values{}
	data.Set("username", c.username)
	data.Set("password", c.password)

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/access/ticket", c.apiURL), strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ログイン失敗: ステータスコード %d", resp.StatusCode)
	}

	// io/ioutil は非推奨なので io.ReadAll を使用
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var result struct {
		Data struct {
			Ticket    string `json:"ticket"`
			CSRFToken string `json:"CSRFPreventionToken"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return err
	}

	c.ticket = result.Data.Ticket
	c.csrfToken = result.Data.CSRFToken
	return nil
}

// VMStatus はVMの状態情報
type VMStatus struct {
	Status string `json:"status"` // running, stopped など
}

// GetVMStatus はVMの現在の状態を取得します
func (c *Client) GetVMStatus(node string, vmID int) (string, error) {
	// まずログインを試みる
	if err := c.Login(); err != nil {
		return "", fmt.Errorf("ログインエラー: %v", err)
	}

	url := fmt.Sprintf("%s/nodes/%s/qemu/%d/status/current", c.apiURL, node, vmID)
	
	// リクエスト作成
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("HTTPリクエスト作成エラー: %v", err)
	}
	
	// 認証情報を設定
	addAuthHeaders(req, c)
	
	// リクエスト送信
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTPリクエストエラー: %v", err)
	}
	defer resp.Body.Close()
	
	// エラーチェック
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("VM状態取得失敗: ステータスコード %d, レスポンス: %s", resp.StatusCode, string(bodyBytes))
	}
	
	// レスポンス解析
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("レスポンス読み込みエラー: %v", err)
	}
	
	var result struct {
		Data VMStatus `json:"data"`
	}
	
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("JSONデコードエラー: %v", err)
	}
	
	return result.Data.Status, nil
}

// RestartVM はVMを再起動します
func (c *Client) RestartVM(node string, vmID int) error {
	// まずVM状態を取得
	status, err := c.GetVMStatus(node, vmID)
	if err != nil {
		return fmt.Errorf("VM状態取得エラー: %v", err)
	}
	
	// VMの状態に応じて処理を分岐
	switch status {
	case "stopped":
		// 停止中の場合は起動する
		return c.startVM(node, vmID)
	case "running":
		// 実行中の場合はリセットする
		return c.resetVM(node, vmID)
	default:
		// その他の状態（paused, suspended など）
		return fmt.Errorf("対応していないVM状態: %s", status)
	}
}

// startVM は停止中のVMを起動します
func (c *Client) startVM(node string, vmID int) error {
	return c.vmAction(node, vmID, "start", "起動")
}

// resetVM は実行中のVMをリセット（再起動）します
func (c *Client) resetVM(node string, vmID int) error {
	return c.vmAction(node, vmID, "reset", "リセット")
}

// vmAction はVMに対して指定されたアクションを実行します
func (c *Client) vmAction(node string, vmID int, action, actionName string) error {
	// まずログインを試みる
	if err := c.Login(); err != nil {
		return fmt.Errorf("ログインエラー: %v", err)
	}

	url := fmt.Sprintf("%s/nodes/%s/qemu/%d/status/%s", c.apiURL, node, vmID, action)
	
	// POSTリクエスト用のJSONデータ
	data := map[string]string{}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("JSONエンコードエラー: %v", err)
	}
	
	// リクエスト作成
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("HTTPリクエスト作成エラー: %v", err)
	}
	
	// 認証情報を設定
	addAuthHeaders(req, c)
	req.Header.Add("Content-Type", "application/json")
	
	// リクエスト送信
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTPリクエストエラー: %v", err)
	}
	defer resp.Body.Close()
	
	// エラーチェック
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("VM%s失敗: ステータスコード %d, レスポンス: %s", actionName, resp.StatusCode, string(bodyBytes))
	}
	
	return nil
}

// addAuthHeaders は認証ヘッダーをリクエストに追加します
func addAuthHeaders(req *http.Request, c *Client) {
	if c.tokenID != "" && c.tokenSecret != "" {
		// APIトークンを使用
		req.Header.Add("Authorization", fmt.Sprintf("PVEAPIToken=%s=%s", c.tokenID, c.tokenSecret))
		return
	}
	
	// チケット認証を使用
	req.Header.Add("Cookie", fmt.Sprintf("PVEAuthCookie=%s", c.ticket))
	req.Header.Add("CSRFPreventionToken", c.csrfToken)
}
