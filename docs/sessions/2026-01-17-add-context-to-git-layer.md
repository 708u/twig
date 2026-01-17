# Git層にcontext.Contextを追加するリファクタリング

## 目的

GoのベストプラクティスとしてI/O操作を伴う関数は第一引数に`context.Context`を取るべき。
Git層の全メソッドにcontextを追加し、キャンセル対応とタイムアウト制御を可能にする。

## 作業内容

### 完了した作業

1. **GitExecutor interface更新**
   - `Run(args ...string)` → `Run(ctx context.Context, args ...string)`

2. **osGitExecutor実装更新**
   - `exec.Command` → `exec.CommandContext`を使用

3. **GitRunner全メソッドにcontext追加**
   - 23+ のpublicメソッド全てに`ctx context.Context`を第一引数として追加
   - privateメソッド（worktreeAdd, worktreeRemove等）も同様に更新

4. **戻り値の変更**
   - `LocalBranchExists`: `bool` → `(bool, error)` - context cancelを返せるように
   - `FindRemotesForBranch`: `[]string` → `([]string, error)` - 同上

5. **MockGitExecutor更新**
   - テスト用モックもcontext対応に更新

6. **git_test.go更新**
   - `t.Context()`を使用（Go 1.21+のテスト用context）

7. **コミット＆プッシュ**
   - `2ffb7e2` - refactor(git): add context.Context to all GitRunner methods

8. **コマンド層の対応開始**（途中）
   - add.go: `Run`, `createWorktree`にcontext追加済み
   - remove.go: `Run`にcontext追加開始

### 意思決定

- テストでは `context.Background()` ではなく `t.Context()` を使用
  - ユーザーの指示により変更

- コマンド層も含めて全対応することを決定
  - 理由: git.goだけ変更するとパッケージがコンパイルできず、テストも実行できないため

## 変更ファイル

### 完了

- @git.go
- @git_test.go
- @internal/testutil/mock_git.go

### 進行中

- @add.go
- @remove.go

## 利用したSkill

- /commit-push-update-pr

## Pull Request

<https://github.com/708u/twig/pull/119>

## 未完了タスク

- [ ] remove.go の残りのメソッド更新（Check, removePrunable, checkSkipReason, checkPrunableSkipReason, getCleanReason）
- [ ] clean.go にcontext追加
- [ ] list.go にcontext追加
- [ ] main.go のインターフェースと呼び出し更新
  - AddCommander, RemoveCommander, CleanCommander, ListCommander
  - cmd.Context()を使用してcobraからcontextを取得
- [ ] テストファイルの更新（add_test.go, remove_test.go, clean_test.go, list_test.go, main_test.go）
- [ ] 全テスト実行して確認
- [ ] PRのdescription更新
- [ ] .claude/settings.json から defaultMode 設定を削除
- [ ] .twig-claude-prompt.sh を .twig-claude-prompt-completed.sh にリネーム
