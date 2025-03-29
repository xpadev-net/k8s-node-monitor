package discord

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// WebhookClient はDiscord Webhookクライアント
type WebhookClient struct {
	WebhookURL string
	Enabled    bool
}

// WebhookPayload はDiscord Webhookに送信するペイロード
type WebhookPayload struct {
	Username  string         `json:"username,omitempty"`
	AvatarURL string         `json:"avatar_url,omitempty"`
	Content   string         `json:"content,omitempty"`
	Embeds    []WebhookEmbed `json:"embeds,omitempty"`
}

// WebhookEmbed はRich Embedコンテンツ
type WebhookEmbed struct {
	Title       string                 `json:"title,omitempty"`
	Description string                 `json:"description,omitempty"`
	Color       int                    `json:"color,omitempty"`
	Fields      []WebhookEmbedField    `json:"fields,omitempty"`
	Thumbnail   *WebhookEmbedThumbnail `json:"thumbnail,omitempty"`
	Footer      *WebhookEmbedFooter    `json:"footer,omitempty"`
	Timestamp   string                 `json:"timestamp,omitempty"`
}

// WebhookEmbedField はEmbedのフィールド
type WebhookEmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

// WebhookEmbedThumbnail はEmbedのサムネイル
type WebhookEmbedThumbnail struct {
	URL string `json:"url"`
}

// WebhookEmbedFooter はEmbedのフッター
type WebhookEmbedFooter struct {
	Text    string `json:"text"`
	IconURL string `json:"icon_url,omitempty"`
}

// NewWebhookClient は新しいDiscord Webhookクライアントを作成
func NewWebhookClient(webhookURL string, enabled bool) *WebhookClient {
	return &WebhookClient{
		WebhookURL: webhookURL,
		Enabled:    enabled,
	}
}

// SendNodeNotReadyNotification はNodeのNotReady状態を通知
func (c *WebhookClient) SendNodeNotReadyNotification(nodeName, status, duration, ip string, vmInfo string, isRestarting bool) error {
	if !c.Enabled || c.WebhookURL == "" {
		return nil // 通知無効または設定なし
	}

	// 現在時刻をISO8601形式で取得
	now := time.Now().Format(time.RFC3339)

	// 色を設定 (赤：NotReadyで再起動なし、黄：NotReadyで再起動中)
	color := 16711680 // 赤色 (0xFF0000)
	if isRestarting {
		color = 16776960 // 黄色 (0xFFFF00)
	}

	// メッセージを構築
	description := fmt.Sprintf("Node `%s` is in **%s** state for %s", nodeName, status, duration)
	if isRestarting {
		description += "\nAutomatic restart has been triggered."
	}

	// Embedを作成
	embed := WebhookEmbed{
		Title:       "Kubernetes Node NotReady Alert",
		Description: description,
		Color:       color,
		Fields: []WebhookEmbedField{
			{
				Name:   "Node",
				Value:  nodeName,
				Inline: true,
			},
			{
				Name:   "Status",
				Value:  status,
				Inline: true,
			},
			{
				Name:   "Duration",
				Value:  duration,
				Inline: true,
			},
			{
				Name:   "IP Address",
				Value:  ip,
				Inline: true,
			},
		},
		Thumbnail: &WebhookEmbedThumbnail{
			URL: "https://kubernetes.io/images/favicon.png",
		},
		Footer: &WebhookEmbedFooter{
			Text: "K8s Node Monitor",
		},
		Timestamp: now,
	}

	// VMの情報があれば追加
	if vmInfo != "" {
		embed.Fields = append(embed.Fields, WebhookEmbedField{
			Name:   "VM Info",
			Value:  vmInfo,
			Inline: false,
		})
	}

	// Payloadを作成
	payload := WebhookPayload{
		Username:  "K8s Node Monitor",
		AvatarURL: "https://kubernetes.io/images/favicon.png",
		Embeds:    []WebhookEmbed{embed},
	}

	// JSONに変換
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("JSON変換エラー: %v", err)
	}

	// POSTリクエスト送信
	resp, err := http.Post(c.WebhookURL, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("Webhook送信エラー: %v", err)
	}
	defer resp.Body.Close()

	// ステータスコードチェック
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Webhook送信失敗: ステータスコード %d, レスポンス: %s", resp.StatusCode, string(body))
	}

	return nil
}
