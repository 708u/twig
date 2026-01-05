# feat: --carry オプションのファイルパターン指定機能

PR: <https://github.com/708u/gwt/pull/45>

## 概要

`gwt add --carry` で `--file` フラグを使用してファイルパターンを指定し、
特定のファイルのみを新しい worktree に移動できるようにした。

## 使用例

```bash
# 全変更をcarry（従来通り）
gwt add feat/new --carry

# 特定ファイルのみcarry
gwt add feat/new --carry --file "*.go"

# 全階層のGoファイル（globstar）
gwt add feat/new --carry --file "**/*.go"

# 複数パターン
gwt add feat/new --carry --file "*.go" --file "cmd/**"

# 短縮形
gwt add feat/new -c -F "*.go" -F "cmd/**"
```

## 変更ファイル

| ファイル | 変更内容 |
|----------|----------|
| git.go | `StashPush(message, pathspecs...)` に拡張、`ChangedFiles()` 追加 |
| add.go | `CarryFiles []string` フィールド追加、glob展開ロジック、重複排除 |
| cmd/gwt/main.go | `--file` / `-F` フラグ追加、Long description追加、補完機能 |
| cmd/gwt/main_test.go | `--file` フラグのユニットテスト追加 |
| add_integration_test.go | CarrySpecificFiles等のテスト追加 |
| git_integration_test.go | ChangedFiles のテスト追加 |
| docs/commands/add.md | ドキュメント更新 |

## 実装詳細

### --file フラグ

`--file` / `-F` フラグ（StringArray）でファイルパターンを指定。

- 複数回指定可能: `--file "*.go" --file "cmd/**"`
- カンマ区切りは非対応（ファイル名にカンマが含まれる場合の問題を回避）
- タブ補完で変更ファイルを候補に表示

### StashPush の拡張

```go
func (g *GitRunner) StashPush(message string, pathspecs ...string) (string, error)
```

pathspecs が指定された場合、`git stash push -u -m <message> -- <pathspecs...>`
を実行する。

### ChangedFiles

```go
func (g *GitRunner) ChangedFiles() ([]string, error)
```

`git status --porcelain -uall` を使用して変更ファイル一覧を取得。
`--file` フラグの補完で使用。

### glob展開と重複排除

doublestar ライブラリを使用してパターンをファイルパスに展開。
複数パターンで同じファイルにマッチした場合の重複を排除。

```go
seen := make(map[string]bool)
for _, pattern := range c.CarryFiles {
    matches, err := c.FS.Glob(c.CarryFrom, pattern)
    for _, match := range matches {
        if !seen[match] {
            seen[match] = true
            pathspecs = append(pathspecs, match)
        }
    }
}
```

### Long description

`gwt add --help` で `--sync` と `--carry` の違いを説明。

## テスト

```bash
# 統合テスト
go test -tags=integration -run "CarrySpecificFiles|CarryMultiplePatterns|CarryGlobstarPattern|ChangedFiles" ./...

# ユニットテスト
go test -run "TestAddCmd/file" ./cmd/gwt/...
```

| テスト名 | 内容 |
|----------|------|
| CarrySpecificFiles | `*.go` で Go ファイルのみ carry |
| CarryMultiplePatterns | `*.go` と `cmd/**` の複数パターン |
| CarryGlobstarPattern | `**/*.go` でルート含む全階層の Go ファイル |
| ChangedFiles | staged/unstaged/untracked/renamed ファイルの検出 |
| file_requires_carry | `--file` は `--carry` 必須 |
| file_with_carry | `--file` で CarryFiles に値が渡る |

## 設計判断

### `--` ではなく `--file` を採用した理由

- `gwt add` の操作対象は worktree であり、`-- <files>` の意味が暗黙的
- `--file` なら明示的で意図が伝わりやすい
- タブ補完を設定可能
- 将来 `--` を別の用途に使える余地が残る

### カンマ区切りを非対応にした理由

- ファイル名にカンマが含まれる場合に正しく動作しない
- StringArray で複数回指定すれば各フラグで補完が効く

## 注意事項

- `--sync` と併用不可
- `--file` は `--carry` 必須
- パターンにマッチするファイルがない場合、stash は空になる可能性あり
- globstar (`**`) は doublestar の仕様に従う（ルートも含む）
