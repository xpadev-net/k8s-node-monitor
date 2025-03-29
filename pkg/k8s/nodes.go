package k8s

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NodeInfo represents information about a Kubernetes node
type NodeInfo struct {
	Name              string
	Status            string
	IP                string
	KubeletVersion    string
	OSImage           string
	Architecture      string
	AllocatableCPU    string
	AllocatableMemory string
	AllocatablePods   string
	NotReadyDuration  string    // NotReadyの状態が続いている期間
	LastTransition    time.Time // 最後の状態遷移時間
}

// GetNodes retrieves information about all nodes in the cluster
func (c *Client) GetNodes() ([]NodeInfo, error) {
	// ノードリストの取得
	nodeList, err := c.clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	// 各ノードの情報を格納
	var nodes []NodeInfo
	now := time.Now()
	
	for _, node := range nodeList.Items {
		nodeInfo := processNodeInfo(node, now)
		nodes = append(nodes, nodeInfo)
	}

	return nodes, nil
}

// processNodeInfo は単一ノードの情報を処理します
func processNodeInfo(node corev1.Node, now time.Time) NodeInfo {
	// ノードステータスとその経過時間の処理
	status, lastTransitionTime, notReadyDuration := getNodeStatus(node, now)
	
	// IPアドレスの取得
	ip := getNodeIP(node)

	return NodeInfo{
		Name:              node.Name,
		Status:            status,
		IP:                ip,
		KubeletVersion:    node.Status.NodeInfo.KubeletVersion,
		OSImage:           node.Status.NodeInfo.OSImage,
		Architecture:      node.Status.NodeInfo.Architecture,
		AllocatableCPU:    node.Status.Allocatable.Cpu().String(),
		AllocatableMemory: node.Status.Allocatable.Memory().String(),
		AllocatablePods:   node.Status.Allocatable.Pods().String(),
		NotReadyDuration:  notReadyDuration,
		LastTransition:    lastTransitionTime,
	}
}

// getNodeStatus はノードのステータス情報を取得します
func getNodeStatus(node corev1.Node, now time.Time) (status string, lastTransitionTime time.Time, notReadyDuration string) {
	status = "Ready"
	
	for _, condition := range node.Status.Conditions {
		if condition.Type != "Ready" {
			continue
		}
		
		if condition.Status == "True" {
			return "Ready", time.Time{}, ""
		}
		
		// NotReadyの場合
		lastTransitionTime = condition.LastTransitionTime.Time
		duration := now.Sub(lastTransitionTime)
		notReadyDuration = formatDuration(duration)
		return "NotReady", lastTransitionTime, notReadyDuration
	}
	
	return
}

// getNodeIP はノードのIPアドレスを取得します
func getNodeIP(node corev1.Node) string {
	for _, addr := range node.Status.Addresses {
		if addr.Type == "InternalIP" {
			return addr.Address
		}
	}
	return ""
}

// formatDuration は経過時間を読みやすい形式にフォーマットします
func formatDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	
	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm %ds", days, hours, minutes, seconds)
	}
	
	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	}
	
	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	
	return fmt.Sprintf("%ds", seconds)
}
