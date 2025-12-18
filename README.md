# Docker Volume Manager (dvm)

Docker ボリュームのライフサイクル管理を簡単にする Go 製 CLI ツール。

## 概要

`dvm` は Docker Compose 環境との統合を前提とし、バックアップ、リストア、アーカイブ、スワップ、クリーンアップなどの操作を簡単に実行できる CLI ツールです。

## 主な機能

- **Docker Compose 連携**: Compose ファイルから自動でボリュームを検出
- **バックアップ/リストア**: ボリュームデータの簡単なバックアップとリストア
- **アーカイブ**: 不要なボリュームをアーカイブして削除
- **スワップ**: ボリュームの内容を簡単に入れ替え（テストデータとの切り替えなど）
- **クリーンアップ**: 未使用ボリュームの自動検出と削除
- **履歴管理**: バックアップ履歴の追跡
- **メタデータ追跡**: 最終アクセス日時などをトラッキング

## インストール

### ビルド

```bash
# リポジトリをクローン
git clone https://github.com/docker-volume-manager/dvm.git
cd dvm

# ビルド
go build -o dvm ./cmd/dvm

# システムにインストール（オプション）
sudo mv dvm /usr/local/bin/
```

### 依存関係

- Go 1.21 以上
- Docker がインストールされていること

## クイックスタート

```bash
# プロジェクトディレクトリに移動（compose.yaml があるディレクトリ）
cd ~/projects/myapp

# ボリューム一覧を表示
dvm list

# 全ボリュームをバックアップ
dvm backup

# 特定のサービスのボリュームをバックアップ
dvm backup db

# 最新のバックアップからリストア
dvm restore db

# バックアップを選択してリストア
dvm restore db --select

# 未使用ボリュームをクリーンアップ（dry-run）
dvm clean --unused --dry-run

# 実際にクリーンアップ
dvm clean --unused
```

## 使い方

### グローバルオプション

```
-f, --file <path>      Compose ファイルパス
-p, --project <name>   プロジェクト名を上書き
--no-compose           Compose 連携を無効化
-v, --verbose          詳細ログ出力
-q, --quiet            出力を最小限に
--config <path>        設定ファイルパス指定
--version              バージョン表示
-h, --help             ヘルプ表示
```

### コマンド

#### `dvm list` - ボリューム一覧表示

```bash
dvm list                    # 現在のプロジェクトのボリューム
dvm list --all             # すべてのボリューム
dvm list --unused          # 未使用ボリュームのみ
dvm list --stale 30        # 30日以上アクセスなし
dvm list --format json     # JSON形式で出力
```

#### `dvm backup` - バックアップ

```bash
dvm backup                  # 全ボリュームをバックアップ
dvm backup db              # db サービスのボリュームをバックアップ
dvm backup db redis        # 複数指定
dvm backup -o /backup      # 出力先を指定
dvm backup --tag daily     # タグを付ける
dvm backup --stop          # コンテナを停止してバックアップ
```

#### `dvm restore` - リストア

```bash
dvm restore db             # 最新のバックアップからリストア
dvm restore db --select    # 対話的にバックアップを選択
dvm restore db --list      # 利用可能なバックアップ一覧
dvm restore --restart      # リストア後にコンテナ再起動
dvm restore /path/to/backup.tar.gz  # 特定のファイルからリストア
```

#### `dvm archive` - アーカイブして削除

```bash
dvm archive                # プロジェクト全体をアーカイブ
dvm archive db             # 特定のサービスのみ
dvm archive --verify       # 整合性検証後に削除
```

#### `dvm swap` - ボリューム切り替え

```bash
dvm swap db --empty --restart           # 空のボリュームに切り替え
dvm swap db test_data.tar.gz --restart  # テストデータに切り替え
dvm restore db --restart                # 元に戻す
```

#### `dvm clean` - クリーンアップ

```bash
dvm clean --unused --dry-run    # 削除対象を確認
dvm clean --unused              # 未使用ボリュームを削除
dvm clean --stale 60            # 60日以上未使用を削除
dvm clean --unused --archive    # アーカイブしてから削除
```

#### `dvm history` - バックアップ履歴

```bash
dvm history                # 現在のプロジェクトの履歴
dvm history db             # 特定のサービスの履歴
dvm history --all          # 全プロジェクトの履歴
dvm history -n 20          # 20件表示
```

#### `dvm inspect` - 詳細情報

```bash
dvm inspect db             # ボリュームの詳細情報
dvm inspect db --format json  # JSON形式
```

#### `dvm clone` - ボリューム複製

```bash
dvm clone db db_test       # テスト用に複製
```

## 設定ファイル

`~/.dvm/config.yaml` で設定をカスタマイズできます。

```yaml
# デフォルト設定
defaults:
  compress_format: tar.gz    # tar.gz | tar.zst
  keep_generations: 5        # バックアップ保持世代
  stop_before_backup: false  # バックアップ前にコンテナ停止

# パス設定
paths:
  backups: ~/.dvm/backups
  archives: ~/.dvm/archives

# プロジェクト別設定
projects:
  myproject:
    keep_generations: 10
```

## ディレクトリ構造

```
~/.dvm/
├── config.yaml              # グローバル設定
├── backups/                 # バックアップ
│   ├── myproject/
│   │   ├── db_2024-12-18_143022.tar.gz
│   │   └── redis_2024-12-18_143022.tar.gz
│   └── other-project/
├── archives/                # アーカイブ
└── meta.db                  # メタデータ（SQLite）
```

## 典型的なワークフロー

### 日常のバックアップ

```bash
cd ~/projects/myapp
dvm backup
dvm history
```

### 開発環境リセット

```bash
dvm swap db --empty --restart
# 開発作業...
dvm restore db --restart
```

### 本番データでテスト

```bash
dvm swap db ./prod_dump.tar.gz --restart
# テスト...
dvm restore db --restart
```

### プロジェクト終了時

```bash
cd ~/projects/finished-project
dvm archive --verify
```

### 定期クリーンアップ

```bash
dvm clean --stale 90 --dry-run
dvm clean --stale 90 --archive
```

## トラブルシューティング

### Docker との接続エラー

Docker が起動しているか確認してください:

```bash
docker ps
```

### 権限エラー

Docker コマンドを実行する権限があるか確認してください:

```bash
docker volume ls
```

## ライセンス

MIT

## 仕様

詳細な仕様については [spec.md](spec.md) を参照してください。
