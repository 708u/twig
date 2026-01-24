# remove コマンドの補完候補から main worktree を除外

## 目的

`twig remove` コマンドのシェル補完候補から main worktree を除外する。
main branch の削除は危険なため、補完候補に含めるべきではない。

## 作業内容

1. `cmd/twig/main.go` の `removeCmd` の `ValidArgsFunction` を修正
   - `WorktreeListBranches` から `WorktreeList` に変更
   - main worktree (index 0) と detached HEAD を除外するロジックを追加
   - clean.go と同様のパターンを採用

2. `cmd/twig/main_test.go` にテストを追加
   - `TestRemoveCmd_ValidArgs_ExcludesMain`: main が除外され、feature branch が含まれることを確認

3. テスト・lint の実行と確認
   - `go test ./cmd/twig/... -run ValidArgs` - PASS
   - `go test -tags=integration ./...` - PASS
   - `make lint` - 0 issues

## 変更ファイル

- cmd/twig/main.go:515-535
- cmd/twig/main_test.go:1295-1325

## 利用したSkill

- /commit-push-update-pr
- /export-session

## Pull Request

<https://github.com/708u/twig/pull/148>

## 未完了タスク

- [ ] .claude/settings.json を削除
- [ ] .twig-claude-prompt.sh を .twig-claude-prompt-completed.sh にリネーム
