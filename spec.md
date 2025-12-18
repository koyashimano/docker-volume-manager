# Docker Volume Manager (dvm) 仕様書

## 概要

Dockerボリュームのライフサイクル管理を簡単にするGo製CLIツール。
Docker Compose環境との統合を前提とし、バックアップ、アーカイブ、スワップ、クリーンアップなどの操作を実行できる。

## ディレクトリ構造

```
~/.dvm/
├── config.yaml              # グローバル設定
├── backups/                 # デフォルトのバックアップ先
│   ├── myproject/           # プロジェクト別
│   │   ├── db_2024-12-18_143022.tar.gz
│   │   └── redis_2024-12-18_143022.tar.gz
│   └── other-project/
├── archives/                # アーカイブ済みボリューム
└── meta.db                  # メタデータ（最終アクセス日時等）
```

---

## Docker Compose 連携

### Composeファイル自動検出

dvmはカレントディレクトリから以下の順序でComposeファイルを自動検出する:

1. `compose.yaml`
2. `compose.yml`
3. `docker-compose.yaml`
4. `docker-compose.yml`

検出されない場合、`--no-compose` モードとして動作し、ボリューム名の直接指定が必要になる。

### プロジェクト名の解決

ボリューム名のプレフィックス（プロジェクト名）は以下の優先順で決定:

1. `-p, --project` オプション
2. Composeファイル内の `name:` フィールド
3. `COMPOSE_PROJECT_NAME` 環境変数
4. Composeファイルのあるディレクトリ名

### サービス名によるボリューム指定

Composeプロジェクト内では、フルボリューム名の代わりにサービス名でボリュームを指定できる:

```bash
# 従来: フルボリューム名が必要
docker volume inspect myproject_postgres_data

# dvm: サービス名で指定（カレントディレクトリにcompose.yamlがある場合）
dvm backup db
dvm restore db

# 別プロジェクトを操作
dvm -f ~/other-project/compose.yaml backup db
```

### Composeファイル例

```yaml
# compose.yaml
name: myproject

services:
  db:
    image: postgres:16
    volumes:
      - postgres_data:/var/lib/postgresql/data

  redis:
    image: redis:7
    volumes:
      - redis_data:/data

volumes:
  postgres_data:
  redis_data:
```

この場合:

- `dvm backup db` → `myproject_postgres_data` をバックアップ
- `dvm backup redis` → `myproject_redis_data` をバックアップ
- `dvm backup` (引数なし) → プロジェクトの全ボリュームをバックアップ

---

## コマンド体系

```
dvm [global-options] <command> [command-options] [arguments]
```

## グローバルオプション

| オプション        | 短縮 | 説明                    |
| ----------------- | ---- | ----------------------- |
| `--file <path>`   | `-f` | Composeファイルパス指定 |
| `--project <n>`   | `-p` | プロジェクト名を上書き  |
| `--no-compose`    |      | Compose連携を無効化     |
| `--verbose`       | `-v` | 詳細ログ出力            |
| `--quiet`         | `-q` | 出力を最小限に          |
| `--config <path>` |      | 設定ファイルパス指定    |
| `--help`          | `-h` | ヘルプ表示              |
| `--version`       |      | バージョン表示          |

---

## コマンド一覧

### 1. `dvm list` - ボリューム一覧表示

```bash
dvm list [options]
```

**オプション:**

| オプション       | 短縮 | 説明                                   |
| ---------------- | ---- | -------------------------------------- |
| `--all`          | `-a` | 全ボリューム表示（他プロジェクト含む） |
| `--unused`       | `-u` | 未使用ボリュームのみ                   |
| `--stale <days>` |      | N日以上アクセスなし                    |
| `--size`         | `-s` | サイズ順ソート                         |
| `--format <fmt>` |      | 出力形式: table/json/csv               |

**出力例:**

```
SERVICE  VOLUME                   SIZE     LAST_USED    STATUS
db       myproject_postgres_data  2.3GB    2024-12-15   in-use
redis    myproject_redis_data     156MB    2024-12-18   in-use

# --all 指定時は他プロジェクトや孤立ボリュームも表示
```

---

### 2. `dvm backup` - バックアップ

```bash
dvm backup [service...] [options]
```

**引数:**

- 省略: プロジェクトの全ボリュームをバックアップ
- サービス名: 指定サービスのボリュームのみ

**オプション:**

| オプション        | 短縮 | 説明                   | デフォルト                  |
| ----------------- | ---- | ---------------------- | --------------------------- |
| `--output <path>` | `-o` | 出力先ディレクトリ     | `~/.dvm/backups/<project>/` |
| `--format <fmt>`  |      | tar.gz / tar.zst       | tar.gz                      |
| `--no-compress`   |      | 圧縮なし               |                             |
| `--tag <n>`       | `-t` | バックアップにタグ付け |                             |
| `--stop`          |      | 関連コンテナを停止     |                             |

**保存先:**

```
~/.dvm/backups/<project>/<service>_<YYYY-MM-DD_HHMMSS>.tar.gz
```

**実行例:**

```bash
cd ~/projects/myapp

# 全ボリュームバックアップ（引数不要）
dvm backup

# dbサービスのみ
dvm backup db

# 複数サービス
dvm backup db redis

# 別プロジェクト
dvm -f ~/other/compose.yaml backup
```

---

### 3. `dvm restore` - リストア

```bash
dvm restore [service|backup-file] [options]
```

**引数の解釈:**

- サービス名: `~/.dvm/backups/<project>/` から最新のバックアップを自動選択
- ファイルパス: 指定されたバックアップファイルを使用
- 省略: プロジェクトの全サービスを最新バックアップからリストア

**オプション:**

| オプション  | 短縮 | 説明                           |
| ----------- | ---- | ------------------------------ |
| `--select`  | `-s` | 対話的にバックアップを選択     |
| `--list`    | `-l` | 利用可能なバックアップ一覧表示 |
| `--force`   |      | 確認なしで上書き               |
| `--restart` |      | リストア後にコンテナ再起動     |

**実行例:**

```bash
# dbの最新バックアップからリストア
dvm restore db

# バックアップ一覧から選択
dvm restore db --select

# 特定のバックアップファイルを指定
dvm restore ~/.dvm/backups/myproject/db_2024-12-01_030000.tar.gz

# 全サービスを最新にリストア
dvm restore --force
```

---

### 4. `dvm archive` - アーカイブして削除

```bash
dvm archive [service...] [options]
```

バックアップを `~/.dvm/archives/` に保存し、ボリュームを削除。

**オプション:**

| オプション        | 短縮 | 説明               | デフォルト         |
| ----------------- | ---- | ------------------ | ------------------ |
| `--output <path>` | `-o` | アーカイブ先       | `~/.dvm/archives/` |
| `--verify`        |      | 整合性検証後に削除 |                    |
| `--force`         |      | 確認スキップ       |                    |

**実行例:**

```bash
# 古いプロジェクトのボリュームをアーカイブ
cd ~/old-project
dvm archive

# 特定サービスのみ
dvm archive db --verify
```

---

### 5. `dvm swap` - ボリューム切り替え

```bash
dvm swap <service> [source] [options]
```

現在のボリュームを退避し、別のデータに置き換える。

**引数:**

- `service`: 対象サービス
- `source`: 省略時は空のボリュームを作成、または:
  - バックアップファイル
  - `--empty` で明示的に空を指定

**オプション:**

| オプション    | 短縮 | 説明                         |
| ------------- | ---- | ---------------------------- |
| `--empty`     |      | 空のボリュームに置換         |
| `--no-backup` |      | 元データをバックアップしない |
| `--restart`   |      | コンテナを自動再起動         |

**実行例:**

```bash
# テストデータに一時的に切り替え
dvm swap db ./test_data.tar.gz --restart

# 空のボリュームでやり直し
dvm swap db --empty --restart

# 元に戻す（最新バックアップから）
dvm restore db --restart
```

---

### 6. `dvm clean` - クリーンアップ

```bash
dvm clean [options]
```

**オプション:**

| オプション       | 短縮 | 説明                   |
| ---------------- | ---- | ---------------------- |
| `--unused`       | `-u` | 未使用ボリュームを削除 |
| `--stale <days>` |      | N日以上未使用を削除    |
| `--dry-run`      | `-n` | 削除対象を表示のみ     |
| `--archive`      | `-a` | 削除前にアーカイブ     |
| `--force`        |      | 確認スキップ           |

**実行例:**

```bash
# 何が消えるか確認
dvm clean --unused --dry-run

# 60日以上使っていないものをアーカイブして削除
dvm clean --stale 60 --archive

# 一括削除
dvm clean --unused --force
```

---

### 7. `dvm history` - バックアップ履歴

```bash
dvm history [service] [options]
```

**オプション:**

| オプション    | 短縮 | 説明                       |
| ------------- | ---- | -------------------------- |
| `--limit <n>` | `-n` | 表示件数（デフォルト: 10） |
| `--all`       | `-a` | 全プロジェクト             |

**出力例:**

```
SERVICE  TIMESTAMP            SIZE    TAG      PATH
db       2024-12-18 14:30:22  2.3GB   -        ~/.dvm/backups/myproject/db_2024...
db       2024-12-17 03:00:00  2.2GB   daily    ~/.dvm/backups/myproject/db_2024...
redis    2024-12-18 14:30:22  156MB   -        ~/.dvm/backups/myproject/redis_2024...
```

---

### 8. `dvm inspect` - 詳細情報

```bash
dvm inspect <service> [options]
```

**オプション:**

| オプション       | 説明                     |
| ---------------- | ------------------------ |
| `--files`        | ボリューム内ファイル一覧 |
| `--top <n>`      | サイズ上位nファイル      |
| `--format <fmt>` | json/yaml/table          |

---

### 9. `dvm clone` - ボリューム複製

```bash
dvm clone <service> <new-name> [options]
```

**実行例:**

```bash
# テスト用に複製
dvm clone db db_test
```

---

## 設定ファイル

`~/.dvm/config.yaml`

```yaml
# デフォルト設定
defaults:
  compress_format: tar.gz # tar.gz | tar.zst
  keep_generations: 5 # バックアップ保持世代
  stop_before_backup: false # バックアップ前にコンテナ停止

# パス設定（~ は $HOME に展開）
paths:
  backups: ~/.dvm/backups
  archives: ~/.dvm/archives

# プロジェクト別設定（オプション）
projects:
  myproject:
    keep_generations: 10
```

---

## 終了コード

| コード | 意味                              |
| ------ | --------------------------------- |
| 0      | 成功                              |
| 1      | 一般的なエラー                    |
| 2      | ボリューム/サービスが見つからない |
| 3      | 権限エラー                        |
| 4      | ディスク容量不足                  |
| 5      | コンテナ実行中で操作不可          |
| 6      | Composeファイルが見つからない     |

---

## 典型的なワークフロー

### 1. 日常のバックアップ

```bash
cd ~/projects/myapp

# 全部バックアップ
dvm backup

# 履歴確認
dvm history
```

### 2. 開発環境リセット

```bash
# DBを空にしてやり直し
dvm swap db --empty --restart

# 元に戻す
dvm restore db --restart
```

### 3. 本番データでテスト

```bash
# 本番からダウンロードしたバックアップでテスト
dvm swap db ./prod_dump.tar.gz --restart

# 元に戻す
dvm restore db --restart
```

### 4. プロジェクト終了時

```bash
cd ~/projects/finished-project

# アーカイブして削除
dvm archive --verify

# 確認
ls ~/.dvm/archives/finished-project/
```

### 5. 定期クリーンアップ

```bash
# 90日以上放置されてるボリュームを確認
dvm clean --stale 90 --dry-run --all

# アーカイブしてから削除
dvm clean --stale 90 --archive --all
```

---

## 実装上の考慮事項

### 技術スタック

- **Go**: シングルバイナリ、Docker SDK直接利用、クロスコンパイル容易

### バックアップ実装

```bash
# 内部的にはこれと同等の処理
docker run --rm \
  -v <volume>:/source:ro \
  -v ~/.dvm/backups/<project>:/dest \
  alpine tar czf /dest/<n>.tar.gz -C /source .
```

### 最終アクセス日時

Docker APIでは取得不可のため、`~/.dvm/meta.db`（SQLite）で独自追跡:

- バックアップ/リストア時に更新
- コンテナ起動/停止をフックして更新（オプション）

### 安全性

- 実行中コンテナのボリューム操作は警告
- `--force` なしでは破壊的操作に確認プロンプト
- バックアップ時にSHA256チェックサム記録
