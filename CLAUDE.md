# CLAUDE.md

MarkView — Markdown の**閲覧に特化した**軽量デスクトップアプリケーション（Go + Wails v2）。

## 現状

**仕様のみが存在し、アプリケーションのコードはまだ 1 行もない。** `go.mod`・`main.go`・`internal/`・`frontend/` はこれから作る。

| 区分 | 内容 |
| --- | --- |
| **ある** | `docs/specs/`（仕様 20 文書）、`docs/tests/`（手動テスト用スクリプト）、`assets/`（アイコン）、`LICENSE` |
| **ない** | Go のコード、フロントエンド、`go.mod`、`wails.json`、`testdata/`、`scripts/`、`.github/workflows/` |

仕様は `specs-4.2.0` タグの時点で完成している。実装時は**仕様を正とし、迷ったら実装ではなく仕様を読む**。

## 仕様書の構造

`docs/specs/README.md` が索引。5 層構成で、**上位が下位を規定し、矛盾したら上位を正とする**。

| 層 | 文書 | ID | 内容 |
| --- | --- | --- | --- |
| 要求 | 01〜07 | `FR` `UI` `MD` `AR` `BR` `NFR` | 何を満たすか（163 件） |
| 実装 | 10〜13 | `IMP` | どう作るか（100 件） |
| 表示 | 20〜22 | `DSP` | どう見えるか（66 件） |
| 単体テスト | 30〜31 | `UT` | 部品が正しいか（86 件） |
| E2E テスト | 40〜41 | `E2E` | 配布物が動くか（82 件） |
| 横断 | 90 | — | 要求 ↔ 実装 ↔ 表示 の対応表 |

実装に着手するときは `docs/specs/90-traceability.md` で要求 ID から `IMP` / `DSP` を引く。必須度は RFC 2119（MUST / SHOULD / MAY）。

## 破ってはいけない規約

いずれも仕様に根拠があり、**破ると別の要求が成立しなくなる**もの。ID は根拠。

### 構造

- **`internal/` は Wails に依存しない。** Wails の API を呼ぶのは `main.go` と `app.go` だけ（IMP-012）。これが崩れると単体テストに GUI 依存が付いてくる（UT-002）。
- **判断を伴うロジックを `app.go` に置かない。** 履歴・起動時の対象解決・表示用パスの算出は `internal/session` へ（IMP-012, IMP-191, IMP-193）。
- `internal/` 同士の依存は `document → renderer` のみ。共通処理は `app.go` で組み合わせる（IMP-012）。
- フロントエンドから任意のパスを開く汎用 API を作らない。開く経路は 6 つに限る（IMP-300, IMP-192）。

### Markdown 変換

- **変換とシンタックスハイライトは必ず Go 側。** JS の Markdown パーサやハイライタを入れない（AR-031）。フロントで実行するのは Mermaid と KaTeX だけ。
- **`html.WithUnsafe()`（IMP-111）と bluemonday サニタイズ（IMP-116）は対で意味を持つ。** 片方だけ変更しない。
- **サニタイズの許可リストに `svg` を追加しない。** Alerts のアイコンは Go 側で出さず、フロントが後処理で付与する（IMP-112, IMP-225）。
- chroma は **`WithClasses(true)` 必須**。インラインスタイルを出すとテーマ切り替えのたびに再変換が要る（IMP-114, UI-105）。
- 見出し ID は goldmark の `WithAutoHeadingID` を使わず、GitHub 互換のスラッグを自前生成する（IMP-111, IMP-117, MD-021）。
- Mermaid ブロックには `data-source` を付ける。描画後に `<pre>` が SVG へ置き換わり原文が失われるため（IMP-115）。

### フロントエンド

- **Node.js・バンドラ・UI フレームワークを導入しない**（BR-001, AR-050）。`frontend/` はそのまま `go:embed` される。
- 表示・非表示は `hidden` 属性で切り替える。`style.display` を直接触らない（IMP-202）。
- **UI 文言はすべて `js/strings.js` に集約する。** 他のモジュールに文字列リテラルを書かない（IMP-290）。
- **英日併記はツールバーのツールチップだけ**（`Open / 開く`）。他の UI テキストはすべて英語（UI-024）。ロケール判定機構を持たない（NFR-062）。
- KaTeX は `.math-inline` / `.math-block` に対して**要素単位で `katex.render`** を呼ぶ。auto-render を使わない（IMP-232）。
- `outlineVisible`（利用者の意思・保存する）と `outlineSuppressed`（幅不足による一時抑制・保存しない）を別の変数で持つ（IMP-246, UI-026）。
- Go を経由しない文字列を `innerHTML` に渡さない。UI 文言は `textContent`（IMP-220）。
- 本文中のリンクは**常に `preventDefault()`**。WebView 内でページ遷移を発生させない（IMP-223, AR-060）。

### 永続化とプライバシー

- **`config.Config` にウィンドウ位置のフィールドを作らない。** 構造体になければ保存も復元も起こり得ない（IMP-150, UI-111）。
- 開いたファイルのパス・ツリールート・表示履歴・検索語を**ディスクのどこにも書かない**（NFR-042）。
- 設定の保存先はテンポラリのみ。`%APPDATA%` / `~/.config` / レジストリに書かない（UI-112, NFR-033）。
- 設定がない・壊れている・書けない場合もエラーにせず既定値で動く（UI-113）。

### エラー処理

- `panic` を使わない。番兵エラーを返し、呼び出し側は `errors.Is` で判定する（IMP-021）。
- **モーダルダイアログを使わない。** 通知先はステータス領域・本文中・本文ペインの状態画面の 3 か所（FR-110, UI-052）。
- どの異常系でも異常終了しない。`app.go` の各バインドメソッド入口と `renderer` の変換で `recover` する（FR-111, IMP-022）。

### ビルド

- **Linux ビルドは `-tags webkit2_41` を必ず指定する**（AR-003, BR-010）。忘れると 4.0 系にリンクされ Ubuntu 24.04 で起動しない。
- **UPX 等の実行ファイル圧縮を使わない**（BR-010, NFR-052）。ウイルス対策ソフトの誤検知を招く。
- アイコンの原本は `assets/` が唯一の正。ビルド前に `build/` へ複製する（BR-013）。`assets/icon.ico` を `go:embed` しない（IMP-032）。
- 既定ではログを出さない。`MARKVIEW_DEBUG=1` のときだけ標準エラーへ（IMP-023, NFR-041）。

## テストの書き方

`docs/specs/30-test-policy.md` が方針、`31-test-cases.md` がケース。この仕様は「有効なテストを書くこと」と「**有効でないテストを書かないこと**」を等しく重視する。

対象は `internal/` のみ。`main.go` / `app.go` / フロントエンド / WebView / ネットワークは対象外（UT-002, IMP-042）。

特に守るもの:

- **カバレッジ率を目標にしない**（UT-060）。網羅性は「要求 ID ごとにテストがあるか」で追跡する（UT-061, 31 章 31.10）。
- 期待値は**人が書いたリテラル**。実装と同じ式で計算して比較しない（UT-031）。
- **書いたテストは一度失敗させて確かめる**（UT-033）。
- goldmark や chroma の仕様を網羅しない。「自分たちの設定が正しいか」までを見る（UT-034）。
- `time.Sleep` で待たない。チャネル + `select` で同期する（UT-037, UT-019）。
- 境界値と異常系を先に書く（UT-013）。閾値ちょうどの扱いを必ず含める。
- 各テストに対応する仕様 ID をコメントで書く（UT-010）。
- ゴールデンファイルは差分を読まずに `-update` しない（UT-039）。

## コマンド

```bash
# ビルド（BR-010）
wails build -platform windows/amd64 -ldflags "-s -w" -o MarkView.exe
wails build -platform linux/amd64 -tags webkit2_41 -ldflags "-s -w" -o MarkView

# 開発（BR-012）
wails dev

# テスト（UT-062）
go test ./...
go test -race ./...          # CI で最低限これを実行する
go test -shuffle=on ./...    # 順序依存の検出
go test ./internal/renderer -update   # ゴールデンの更新（差分を必ず読む）

# 手動テストの記録用 Excel を生成（E2E-200）
python docs/tests/gen_manual_test_xlsx.py --version v1.0.0-rc.1
#   -> docs/tests/results/MarkView_手動テスト結果_v1.0.0-rc.1.xlsx
#   要 openpyxl。41 章と 41.17 節の一覧が食い違うと生成せずエラー終了する
```

## 仕様書を変更するとき

- 機能を追加・変更したら、**同じ変更で仕様書の該当箇所も直す**（NFR-071）。乖離を残さない。
- 要求・実装・表示を増減したら `90-traceability.md` を、単体テストなら `31-test-cases.md` 31.10 節の対応表を同じ変更で更新する（UT-061, 90.1.2）。対応表は**範囲表記を使わず 1 行ずつ**書く（UT-902）。
- 各章末尾の「要求一覧」に新しい ID を追加する。
- ID は原則として章内で番号順に並べる（03 / 06 / 21 章は節構成順のため例外。各章冒頭に注記あり）。

変更後は機械的に検証する。

```bash
cd docs/specs
grep -ohE '^#{2,3} (FR|UI|MD|AR|BR|NFR|IMP|DSP|UT|E2E|UC)-[0-9]{2,3}' *.md | sed -E 's/^#+ //' | sort -u > /tmp/def.txt
grep -ohE '\b(FR|UI|MD|AR|BR|NFR|IMP|DSP|UT|E2E|UC)-[0-9]{2,3}\b' *.md | sort -u > /tmp/ref.txt
comm -3 /tmp/def.txt /tmp/ref.txt   # 何も出なければ、定義と参照が一致している
```

## リリース

**プレリリースは省略できない工程**（BR-080, E2E-205）。

1. `v<version>-rc.<n>` を打つ → CI が Mermaid / KaTeX を最新版へ更新（BR-043）し、自動 E2E テストを通してプレリリースを作る
2. その成果物に対して手動テスト 59 件を実施し、結果を `docs/tests/results/` にコミット
3. 全て OK なら本タグ `v<version>` を打つ。**本タグでは資産を更新しない**ため、rc で検証した内容がそのまま配布される

> [!IMPORTANT]
> **`v` で始まるタグの push はリリース CI を起動する**（BR-050）。仕様書やドキュメントの節目に付けるタグは `v` で始めない（例: `specs-4.2.0`）。

## 規約

- **コードコメントは日本語**で書く。仕様書と同じ言語にすることで、`FR-016` のような ID の前後の説明をそのまま引ける。コードは `main` と `internal/` のみで外部から import されないため、英語 godoc の読み手は実質いない。
- ただし**利用者に見えるものはすべて英語**。UI 文言・エラーメッセージ・`--help` / `--version` の出力・Go のエラー値（IMP-021, UI-024）。ツールチップの英日併記のみ例外。
- 命名は IMP-020 に従う。パッケージ名は小文字 1 語・複数形にしない、型名にパッケージ名を繰り返さない（`filetree.Node`）、エラー変数は `Err` + 内容。
- 1 ファイルは目安 400 行以内。超えたら責務で分割する（IMP-011）。
- コミットメッセージは日本語、`docs:` `feat:` のような接頭辞を付ける。
