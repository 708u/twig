---
description: git commit & push
allowed-tools: Bash(git status:*), Bash(git diff:*), Bash(git log:*), Bash(git add:*), Bash(git commit:*), Bash(git push:*)
---

# commit-push

変更内容をConventional Commits形式でコミットし、リモートへプッシュする。

## コンテキスト

### 現在のgit status

!`git status --short`

### ステージ済みの変更

!`git diff --cached`

### 未ステージの変更

!`git diff`

### 最近のコミット

!`git log --oneline -5`

## タスク

上記のコンテキストを基に、以下を順番に実行してください。

### コミット

1. 変更内容を分析し、適切なファイルをステージング
2. Conventional Commits形式でコミットメッセージを作成
3. コミット実行

### プッシュ

1. リモートへプッシュ
2. 失敗時は `git push -u origin <branch>` を試行

$ARGUMENTS
