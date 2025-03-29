# Kubernetes Node Monitor

Kubernetesクラスタのノード情報を取得して表示するツールです。

## 機能

- クラスタ内のすべてのノードの基本情報を表示
- ノードのステータス、IP、バージョン、OS情報を表示
- 割り当てリソース情報（CPU、メモリ、Pod数）の表示

## 実行方法

### ローカル実行

```bash
go mod tidy
go run main.go
```

### Dockerイメージのビルドと実行（ローカル）

```bash
# イメージのビルド
docker build -t k8s-node-monitor .

# ローカルのkubeconfigを使用して実行
docker run -v ${HOME}/.kube:/root/.kube:ro k8s-node-monitor
```

### GitHub ActionsによるDockerイメージのビルド

このリポジトリには GitHub Actions のワークフローが設定されており、以下のタイミングで自動的にDockerイメージがビルドされます：

- `main` または `master` ブランチへのプッシュ
- タグ（`v*.*.*`形式）のプッシュ
- `main` または `master` ブランチへのプルリクエスト

ビルドされたイメージは GitHub Container Registry に公開されます：

### Kubernetesクラスタ内での実行

```bash
# イメージをビルドしてレジストリにプッシュ
docker build -t your-registry/k8s-node-monitor:latest .
docker push your-registry/k8s-node-monitor:latest

# クラスタ内にデプロイ
# 適切な権限を持つServiceAccountが必要です
kubectl apply -f k8s-deployment.yaml
```
