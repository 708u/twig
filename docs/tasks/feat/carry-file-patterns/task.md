# gwt add --carry ファイル指定機能

## 目的

`--carry` オプションで部分的な変更のみを新しいworktreeに移動できるようにする。
現状は全変更をcarryするため、一部の変更だけ別ブランチに持っていきたいケースに対応できない。

## 変更内容

### 仕様

```bash
gwt add <name> --carry[=<source>] [-- <glob>...]
```

使用例:

```bash
# 全変更をcarry（現状と同じ）
gwt add feat/new --carry

# 特定ファイルのみcarry
gwt add feat/new --carry -- path/to/file.go

# glob形式
gwt add feat/new --carry -- "*.go"
gwt add feat/new --carry -- "cmd/**" "internal/**"

# 別worktreeから特定ファイルのみ
gwt add feat/new --carry=@ -- config.toml
gwt add feat/new --carry=feat/a -- "**/*.go"
```

挙動:

- glob指定なし: 全変更をstash（現状と同じ）
- glob指定あり: 指定ファイルのみstash、残りは元worktreeに残る

### 実装方針

1. **GitRunner.StashPush の拡張**
   - `func (g *GitRunner) StashPush(message string, pathspecs ...string) (string, error)`
   - pathspecsが空なら現状と同じ、あれば `git stash push -u -m message -- <pathspecs>...`

2. **AddOptions/AddCommand の拡張**
   - `CarryFiles []string` フィールドを追加

3. **CLI引数パース**
   - `Args: cobra.MinimumNArgs(1)` に変更
   - `cmd.ArgsLenAtDash()` で `--` の位置を取得
   - args[0] がブランチ名、args[dashPos:] がglob

4. **エラー処理**
   - `--carry` なしで `-- <files>` を指定した場合はエラー

## 対象ファイル

- @git.go
- @add.go
- @cmd/gwt/main.go
- @docs/commands/add.md
- @add_test.go
- @add_integration_test.go
- @internal/testutil/mock_git.go

## 完了条件

- [ ] StashPush が pathspecs 引数を受け取れる
- [ ] AddOptions に CarryFiles フィールドがある
- [ ] `gwt add feat/new --carry -- "*.go"` が動作する
- [ ] `--carry` なしでファイル指定するとエラーになる
- [ ] 指定外のファイル変更は元worktreeに残る
- [ ] ユニットテストが通る
- [ ] 統合テストが通る
- [ ] ドキュメントが更新されている
