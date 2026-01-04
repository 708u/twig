---
description: 作業途中で発生した別タスクをmdファイルに書き出す
---

# 概要

現在の作業中に発生した別のタスクを、後で実行するためのmdファイルとして書き出します。

## 指示

1. 現在の会話コンテキストから、後回しにするタスクの内容を把握する
2. タスク内容に基づいて、適切なディレクトリ名を2-3個提案する
   - 形式: `<prefix>/<kebab-case-name>`
   - prefix: Conventional Commitsに則ったもの（例: `feat`, `fix`, `chore`, `refactor`, `docs`, `test`, `perf`, `ci` など）
3. AskUserQuestionツールを使用して、ユーザーにディレクトリ名を選択させる
4. 以下の構造でmdファイルを作成する:
   - 目的
   - 変更内容（具体的な実装方針）
   - 対象ファイル
   - 完了条件
5. ファイルは `docs/tasks/<選択されたディレクトリ名>/task.md` として保存する

## AskUserQuestionの使用例

```yaml
AskUserQuestion:
  question: "タスクのディレクトリ名を選択してください"
  header: "Dir name"
  options:
    - label: "feat/git-runner-dir-injection"
      description: "GitRunnerにディレクトリ注入機能を追加"
    - label: "refactor/config-fields"
      description: "Configフィールドのリファクタリング"
```

## 出力フォーマット

```markdown
# [タスクタイトル]

## 目的

[なぜこの変更が必要か]

## 変更内容

[具体的な変更内容・実装方針]

## 対象ファイル

- @path/to/file1.go
- @path/to/file2.go

## 完了条件

- [ ] 条件1
- [ ] 条件2
```

## 注意事項

- 現在の作業コンテキストから必要な情報を抽出する
- 後で読んでも理解できるよう、十分な情報を含める
- ディレクトリ名は `<prefix>/<kebab-case-name>` 形式で、Conventional Commitsのprefixを必ず付ける
- 対象ファイルは `@` プレフィックスで記載し、後でファイル内容を参照可能にする
