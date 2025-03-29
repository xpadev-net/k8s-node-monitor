package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

// Config は全体の設定構造体
type Config struct {
	Proxmox ProxmoxConfig `yaml:"proxmox"`
	Discord DiscordConfig `yaml:"discord"`
	Nodes   []NodeMapping `yaml:"nodes"`
}

// ProxmoxConfig はProxmox APIの設定
type ProxmoxConfig struct {
	ApiUrl      string `yaml:"apiUrl"`
	Username    string `yaml:"username"`
	Password    string `yaml:"password"`
	TokenID     string `yaml:"tokenId"`     // 認証にTokenを使用する場合
	TokenSecret string `yaml:"tokenSecret"` // 認証にTokenを使用する場合
}

// DiscordConfig はDiscord Webhookの設定
type DiscordConfig struct {
	WebhookURL string `yaml:"webhookUrl"`
	Enabled    bool   `yaml:"enabled"`
}

// NodeMapping はKubernetesノード名とProxmoxの対応関係
type NodeMapping struct {
	KubernetesNodeName string `yaml:"kubernetesNodeName"`
	ProxmoxNode        string `yaml:"proxmoxNode"`
	VMID               int    `yaml:"vmid"`
}

// LoadConfig は指定されたパスから設定ファイルを読み込みます
func LoadConfig(path string) (*Config, error) {
	// 環境変数からconfig pathを上書きできるようにする
	if envPath := os.Getenv("CONFIG_PATH"); envPath != "" {
		path = envPath
	}

	// io/ioutil は非推奨なので os.ReadFile を使用
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("設定ファイル読み込みエラー: %v", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("設定ファイル解析エラー: %v", err)
	}

	return &config, nil
}

// FindNodeMapping はKubernetesのノード名からProxmoxのマッピング情報を探します
func (c *Config) FindNodeMapping(nodeName string) *NodeMapping {
	for _, node := range c.Nodes {
		if node.KubernetesNodeName == nodeName {
			return &node
		}
	}
	return nil
}
