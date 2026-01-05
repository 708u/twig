# gwt → twig リネーム

## 目的

ツール名 `gwt` が呼びづらいため、アナグラム + 母音で `twig` に変更する。
`twig`（小枝）は worktree（作業木）のブランチ（枝）を扱うツールとして意味的にも合致。

## 変更内容

### 1. ディレクトリリネーム

```txt
cmd/gwt/                      → cmd/twig/
.gwt/                         → .twig/
.claude/skills/gwt-guide/     → .claude/skills/twig-guide/
```

### 2. Go モジュール・パッケージ

- `go.mod`: `github.com/708u/gwt` → `github.com/708u/twig`
- 全 `.go` ファイル: `package gwt` → `package twig`
- 全 import path: `github.com/708u/gwt` → `github.com/708u/twig`

### 3. 定数・文字列

- `config.go:12`: `configDir = ".gwt"` → `configDir = ".twig"`
- `cmd/twig/main.go`: `Use: "gwt"` → `Use: "twig"`
- CLI help text、examples 内の `gwt` を `twig` に
- エラーメッセージ、出力フォーマット

### 4. ビルド設定

- `Makefile`: `cmd/gwt` → `cmd/twig`, `out/gwt` → `out/twig`

### 5. ドキュメント・設定

- 全 markdown ファイルの `gwt` → `twig` 置換
- `.gitignore`: `.gwt/` → `.twig/`
- `.claude/settings.local.json`: Bash許可リスト更新

## 対象ファイル

- @go.mod
- @Makefile
- @config.go
- @init.go
- @add.go
- @cmd/gwt/main.go
- @internal/testutil/git.go
- @README.md
- @CLAUDE.md
- @docs/configuration.md
- @docs/commands/add.md
- @docs/commands/list.md
- @docs/commands/remove.md
- @.gitignore
- @.claude/skills/gwt-guide/SKILL.md
- @.claude/settings.local.json
- 全 `*_test.go` ファイル

## 完了条件

- [ ] 全ディレクトリがリネームされている
- [ ] `go.mod` の module path が `github.com/708u/twig` になっている
- [ ] 全 `.go` ファイルの package 名が `twig` になっている
- [ ] 設定ディレクトリが `.twig/` になっている
- [ ] `make build` が成功する
- [ ] `go test ./...` が成功する
- [ ] `go test -tags=integration ./...` が成功する

## 注意事項

- GitHub リポジトリ名の変更は別途 GitHub 上で実施
- go.mod の module path はリポジトリ名変更後に合わせる
