package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/xpadev/k8s-node-monitor/pkg/config"
	"github.com/xpadev/k8s-node-monitor/pkg/discord"
	"github.com/xpadev/k8s-node-monitor/pkg/k8s"
	"github.com/xpadev/k8s-node-monitor/pkg/proxmox"
)

const DEFAULT_CONFIG_PATH = "config.yaml"

func main() {
	// コマンドライン引数の処理
	configPath := flag.String("config", DEFAULT_CONFIG_PATH, "設定ファイルのパス")
	enableRestart := flag.Bool("restart", false, "NotReadyノードの自動再起動を有効にする")
	flag.Parse()

	// 設定ファイルの読み込み
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("設定ファイル読み込みエラー: %v", err)
	}

	// ProxmoxクライアントとK8sクライアントの作成
	proxmoxClient := proxmox.NewClient(&cfg.Proxmox)
	k8sClient, err := k8s.NewClient()
	if err != nil {
		log.Fatalf("K8sクライアント作成エラー: %v", err)
	}

	// Discord Webhookクライアントの作成
	discordClient := discord.NewWebhookClient(cfg.Discord.WebhookURL, cfg.Discord.Enabled)

	// ノードリストの取得
	nodes, err := k8sClient.GetNodes()
	if err != nil {
		log.Fatalf("ノード取得エラー: %v", err)
	}

	// ノード情報の表示
	fmt.Println("Kubernetes Cluster Nodes:")
	fmt.Println("=========================")

	for _, node := range nodes {
		displayNodeInfo(node, cfg, proxmoxClient, discordClient, *enableRestart)
	}
}

// displayNodeInfo はノード情報を表示し、必要に応じて再起動処理を行います
func displayNodeInfo(node k8s.NodeInfo, cfg *config.Config, proxmoxClient *proxmox.Client, discordClient *discord.WebhookClient, enableRestart bool) {
	fmt.Printf("Name: %s\n", node.Name)

	// Readyの場合は単純に表示して終了
	if node.Status == "Ready" {
		fmt.Printf("  Status: %s\n", node.Status)
	} else {
		// NotReadyの場合、期間も表示
		fmt.Printf("  Status: %s (for %s)\n", node.Status, node.NotReadyDuration)
		
		// 自動再起動が有効でない場合は再起動処理をスキップ
		if !enableRestart {
			fmt.Printf("  Action: Automatic restart disabled\n")
			
			// Discord通知（再起動なし）
			notifyDiscord(node, cfg, discordClient, false)
		} else if time.Since(node.LastTransition).Minutes() <= 1 {
			// 1分未満の場合は再起動しない
			fmt.Printf("  Action: Node is NotReady but for less than 1 minute, no restart needed\n")
			
			// Discord通知（再起動なし）
			notifyDiscord(node, cfg, discordClient, false)
		} else {
			// 1分以上NotReadyなので再起動処理
			handleNodeRestart(node, cfg, proxmoxClient, discordClient)
		}
	}

	// 共通情報の表示
	fmt.Printf("  IP: %s\n", node.IP)
	fmt.Printf("  Kubelet Version: %s\n", node.KubeletVersion)
	fmt.Printf("  OS/Arch: %s/%s\n", node.OSImage, node.Architecture)
	fmt.Printf("  Allocatable Resources:\n")
	fmt.Printf("    CPU: %s\n", node.AllocatableCPU)
	fmt.Printf("    Memory: %s\n", node.AllocatableMemory)
	fmt.Printf("    Pods: %s\n", node.AllocatablePods)
	fmt.Println()
}

// notifyDiscord はDiscordにノード状態を通知します
func notifyDiscord(node k8s.NodeInfo, cfg *config.Config, discordClient *discord.WebhookClient, isRestarting bool) {
	if discordClient == nil || !discordClient.Enabled {
		return
	}

	vmInfo := ""
	nodeMapping := cfg.FindNodeMapping(node.Name)
	if nodeMapping != nil {
		vmInfo = fmt.Sprintf("Proxmox Node: %s, VM ID: %d", nodeMapping.ProxmoxNode, nodeMapping.VMID)
	}

	err := discordClient.SendNodeNotReadyNotification(node.Name, node.Status, node.NotReadyDuration, node.IP, vmInfo, isRestarting)
	if err != nil {
		fmt.Printf("  Discord通知エラー: %v\n", err)
	} else {
		fmt.Printf("  Discord通知: 送信成功\n")
	}
}

// handleNodeRestart はNotReadyノードの再起動処理を行います
func handleNodeRestart(node k8s.NodeInfo, cfg *config.Config, proxmoxClient *proxmox.Client, discordClient *discord.WebhookClient) {
	// 対応するProxmoxノードの情報を取得
	nodeMapping := cfg.FindNodeMapping(node.Name)
	if nodeMapping == nil {
		fmt.Printf("  Action: No mapping found for node '%s' in config\n", node.Name)
		
		// Discord通知（再起動なし、マッピングなし）
		notifyDiscord(node, cfg, discordClient, false)
		return
	}

	// まず状態を取得
	status, err := proxmoxClient.GetVMStatus(nodeMapping.ProxmoxNode, nodeMapping.VMID)
	if err != nil {
		fmt.Printf("  Status Error: VM状態取得失敗: %v\n", err)
		
		// Discord通知（再起動なし、エラー）
		notifyDiscord(node, cfg, discordClient, false)
		return
	}
	
	fmt.Printf("  Current VM Status: %s\n", status)
	fmt.Printf("  Action: Restarting node via Proxmox (Node: %s, VMID: %d)\n", 
		nodeMapping.ProxmoxNode, nodeMapping.VMID)

	// Discord通知（再起動開始）
	notifyDiscord(node, cfg, discordClient, true)
	
	// VMの再起動
	err = proxmoxClient.RestartVM(nodeMapping.ProxmoxNode, nodeMapping.VMID)
	if err != nil {
		fmt.Printf("  Restart Error: %v\n", err)
		return
	}
	
	// 成功時は状態に応じたメッセージ表示
	if status == "stopped" {
		fmt.Printf("  Restart: VM was stopped, started successfully\n")
	} else {
		fmt.Printf("  Restart: Requested successfully\n")
	}
}
