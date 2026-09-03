# CLAUDE.md

MarkView — Markdown の**閲覧に特化した**軽量デスクトップアプリケーション（Go + Wails v2）。

## 現状

**実装はすべて揃っている。** 起動してファイルを開き、変換して表示し、ツリー・アウトライン・検索・テーマ切り替え・外部エディタで開くまで使える。CI・自動 E2E・リリースワークフローも書き終えた。**`v1.0.0-rc.1` のリリース CI は最後まで通っている**（2026-09-03）。

**ただし `v1.0.0-rc.1` は手動テストを実施せずにリジェクトした。** 外部エディタで開く機能（FR-090 系）を `v1.0.0` に含めると決めたためである。**検証対象は `v1.0.0-rc.2`** であり、そこで手動テスト 65 件を実施する（E2E-205）。41 章を改訂したため前回の結果は引き継げず、**全ケースを実施する**（E2E-201）。

| 区分 | 内容 |
| --- | --- |
| **ある** | `main.go` とルートの Go ファイル群（Wails との境界）、`internal/` 13 パッケージ、`frontend/`（HTML / CSS / JS / アイコン / 同梱資産）、`go.mod`、`wails.json`、`scripts/`（`copyicons`・`genchroma`・`genlicenses`・`smoke`・`gentestdata`・`e2e`・`pack`・`vendorupdate`）、`licenses/THIRD_PARTY.md`、`testdata/`（`showcase.md`・`smoke.md`・`e2e/`）、`docs/`（利用者向け 5 文書 + 仕様 20 文書）、`assets/`、`.github/workflows/`（`ci.yml`・`release.yml`）、ルート `README.md` |
| **未実装** | **無い。** 外部エディタで開く機能（FR-090 系）も実装済み（2026-09-03）。`internal/applog` も含め、仕様にあるものはすべて揃っている |
| **未実施** | **手動テスト 65 件**（41 章、E2E-205）と、それに向けた `v1.0.0-rc.2` のリリース CI。`v1.0.0-rc.1` は成果物こそ作られたが**手動テストせずリジェクトした**。`v0.1.0-rc.1` / `rc.2` は CI を試すための試験用タグであり、いずれも E2E-205 の検証対象ではない |

> [!IMPORTANT]
> **ルート `README.md` は配布アーカイブに同梱され、引数なし起動で最初に表示される文書でもある**（BR-020, FR-013, E2E-211）。GitHub の顔であると同時に、受け取った人が最初に読むものになる。
>
> - **配布物に無いパスへの相対リンクを書かない。** `docs/` への参照は絶対 URL にする。相対リンクが許されるのは、同梱される `LICENSE` だけ
> - 利用者向けドキュメントは `docs/`（`usage.md` / `markdown.md` / `showcase.md` / `settings.md` / `troubleshooting.md` と索引の `README.md`）。**`docs/` 内どうしは相対リンクでよい**。同じ場所に一緒に置かれるため
> - `docs/showcase.md` は「こう書くと、こう出る」の見本。**`testdata/showcase.md` とは別物**（あちらは描画を固定するゴールデンテストの入力であり、こちらは利用者が読む対訳）。記法を足したときは両方を見る
> - 日本語で書く。UI-024 が英語と定めているのは UI 文言・エラーメッセージ・`--help` / `--version` の出力であり、文書は含まない
> - **画面を説明する記述は、仕様書の図（03 章）か `frontend/index.html` を開いてから書く。** ペインの並びは `Files` → `Outline` → 本文であり、ツールバーのボタンは 7 つ（`Open` `Reload` `Theme` `Outline` `Files` `Edit` と、右端の `?`。検索と倍率のボタンは無い）。**間違えても機械が教えてくれない種類の記述**である
> - アスキーアートは組み立ててから桁を検証する。枠内は ASCII のみにする（日本語の全角幅で崩れるため）

仕様は `specs-4.2.0` タグの時点で一度完成しており、その後の改訂は `docs/specs/README.md` の改訂履歴に記録する（現在 **4.8.0**）。実装時は**仕様を正とし、迷ったら実装ではなく仕様を読む**。仕様と違う判断をしたときは、同じ変更で仕様書を直し、**改訂履歴に 1 行足して版を上げる**（NFR-071）。

> [!IMPORTANT]
> **進捗の唯一の状態は `workspace/plans/implementation-progress.md`**（現在地・タスク一覧・検証ログ・決定と逸脱の記録・未解決の課題）。作業を始める前に読む。手順は同じディレクトリの `implementation-prompt.md`。
> **`workspace/` は `.gitignore` で除外されており、コミットされない。** 別クローンや `git clean -xdf` では失われるため、仕様に関わる決定はここに書いたうえで `docs/specs/` へ反映する。

## 仕様書の構造

`docs/specs/README.md` が索引。5 層構成で、**上位が下位を規定し、矛盾したら上位を正とする**。

| 層 | 文書 | ID | 内容 |
| --- | --- | --- | --- |
| 要求 | 01〜07 | `FR` `UI` `MD` `AR` `BR` `NFR` | 何を満たすか（170 件） |
| 実装 | 10〜13 | `IMP` | どう作るか（108 件） |
| 表示 | 20〜22 | `DSP` | どう見えるか（68 件） |
| 単体テスト | 30〜31 | `UT` | 部品が正しいか（90 件） |
| E2E テスト | 40〜41 | `E2E` | 配布物が動くか（88 件） |
| 横断 | 90 | — | 要求 ↔ 実装 ↔ 表示 の対応表 |

実装に着手するときは `docs/specs/90-traceability.md` で要求 ID から `IMP` / `DSP` を引く。必須度は RFC 2119（MUST / SHOULD / MAY）。

## 破ってはいけない規約

いずれも仕様に根拠があり、**破ると別の要求が成立しなくなる**もの。ID は根拠。

### 構造

- **`internal/` は Wails に依存しない。** Wails の API を呼ぶのは `main.go` と `app.go` だけ（IMP-012）。これが崩れると単体テストに GUI 依存が付いてくる（UT-002）。
- **判断を伴うロジックを `app.go` に置かない。** 履歴・起動時の対象解決・表示用パスの算出は `internal/session` へ（IMP-012, IMP-191, IMP-193）。
- **`internal/` 同士の依存は 2 系統のみ**（IMP-012）。`document → renderer` と、任意のパッケージ → **葉パッケージ**（`mdfile` / `localurl` / `applog`）。葉は標準ライブラリしか使わず、**葉に依存を追加してはならない**。それ以外の共通処理は `app.go` で組み合わせる。
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

### 外部エディタ

**この製品で唯一「任意の実行ファイルを起動する」機能**である。いずれも NFR-035 に根拠がある。

- **外部プログラムを起動する経路は 3 つだけ**（NFR-035）。既定ブラウザ（FR-050）・既定アプリ（FR-053）・利用者が選んだエディタ（FR-090）。4 つ目を作らない。
- **起動処理は `internal/opener` に置く**（IMP-171, IMP-172）。**`internal/editor` を作らない。** `internal/` 同士の依存は 2 系統に限られており（IMP-012）、新パッケージからは `opener` を呼べず、プロセス起動が 2 か所に複製される。
- **エディタに渡すのは `App.target`（画面がいま対象にしているファイル）であって `current`（表示中の文書）ではない**（IMP-190）。状態画面を出している間 `current` は**前に開いていた文書のまま残る**ため、渡すと利用者が見ているのと違うファイルが静かに開く。`target` はウィンドウタイトルと常に一致する。
- **エディタに渡す引数は、その絶対パス 1 つだけ。** コマンドテンプレート・起動オプション・行番号のいずれも足さない。**一度足すと外せない。**
- **`EditorDTO` に実行ファイルのパスを載せない**（IMP-309）。フロントエンドを通るのはプリセットの ID と `custom` だけで、パスは Go の中で生まれて Go の中で消費される（IMP-300 の 3）。
- **`config.Editor` は絶対パス以外を保持しない**（IMP-153）。`Normalize` で捨てる。相対パスやコマンド名を残すと `$PATH` の内容で起動対象が変わる。
- **MarkView 自身を起動しない**（IMP-171）。`os.Executable()` と `EvalSymlinks` で解決して比較する。押すたびにウィンドウが増える。
- **エディタ選択ウィンドウに項目を追加しない**（UI-103）。決められるのは「どのエディタか」の 1 つだけ。引数や関連付けを足した時点で**設定画面**になり、「MarkView に設定画面はありません」が崩れる。
- **押すたびに選択ウィンドウを出す**（FR-091, UI-103）。保存値で直接起動するようにすると、変更手段が画面から消え、設定画面を作る以外に手がなくなる。
- **`%TEMP%\MarkView` の権限（Linux は `0700` / `0600`）を緩めない**（UI-112）。設定ファイルにエディタのパスが入った以上、これは閲覧の秘匿だけでなく**任意コード実行の防止**にも効いている。
- **端末エディタ（`vim` / `nano`）に対応しない**（IMP-172）。端末を持たずに起動され、利用者からは「押しても無反応」に見える。プリセットに入れない。

### 永続化とプライバシー

- **`config.Config` に「ウィンドウ位置・表示倍率・最大化状態」のフィールドを作らない。** 構造体になければ保存も復元も起こり得ない（IMP-150, UI-111）。倍率と最大化状態は**多重起動のために外した**（UI-115）。設定ファイルは全インスタンスで共有され、保存は構造体まるごとの後勝ちになるため、ウィンドウごとに変える値を保存対象にしてはならない。
- 開いたファイルのパス・ツリールート・表示履歴・検索語を**ディスクのどこにも書かない**（NFR-042）。
- 設定の保存先はテンポラリのみ。`%APPDATA%` / `~/.config` / レジストリに書かない（UI-112, NFR-033）。
- **`wails.Run` に `Windows: &windows.Options{WebviewUserDataPath: ...}` を必ず渡す**（AR-004, IMP-193）。渡さないと WebView2 が `%APPDATA%\MarkView.exe` に数十 MB を書き、NFR-033 に反する。**環境変数では代替できない**（go-webview2 が `WEBVIEW2_USER_DATA_FOLDER` を毎回上書きする）。Linux には指定手段が無く、既定に従うことを NFR-033 の例外として認めている。
- 設定がない・壊れている・書けない場合もエラーにせず既定値で動く（UI-113）。

### エラー処理

- `panic` を使わない。番兵エラーを返し、呼び出し側は `errors.Is` で判定する（IMP-021）。
- **モーダルダイアログを使わない。** 通知先はステータス領域・本文中・本文ペインの状態画面の 3 か所（FR-110, UI-052）。
- どの異常系でも異常終了しない。`app.go` の各バインドメソッド入口と `renderer` の変換で `recover` する（FR-111, IMP-022）。

### ビルド

- **Linux ビルドは `-tags webkit2_41` を必ず指定する**（AR-003, BR-010）。忘れると 4.0 系にリンクされ Ubuntu 24.04 で起動しない。
- **UPX 等の実行ファイル圧縮を使わない**（BR-010, NFR-052）。ウイルス対策ソフトの誤検知を招く。
- アイコンの原本は `assets/` が唯一の正。ビルド前に `build/` へ複製する（BR-013）。`assets/icon.ico` を `go:embed` しない（IMP-032）。
- 既定ではログを出さない。`MARKVIEW_DEBUG=1` のときだけ標準エラーへ（IMP-023, NFR-041）。**環境変数を読むのは `internal/applog` だけ**にする。`grep -rn MARKVIEW_DEBUG --include=*.go` が 1 ファイルしか返さないこと。判定が散ると、そのうち 1 か所が漏れて配布物が出力を始める。

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

# 生成物（コミットする。手で書き換えない）
go run ./scripts/genchroma            # frontend/css/chroma.css（IMP-114, DSP-013）
go run ./scripts/genlicenses          # licenses/THIRD_PARTY.md（BR-040, FR-101）

# 描画スモークテスト（BR-054, E2E-109）。同梱資産を更新したら必ず実行する
go run ./scripts/smoke                # Chromium 系ブラウザを自動で探す
#   MARKVIEW_SMOKE_BROWSER=... で明示指定。Node.js は要らない（BR-001）

# E2E の検証用データのうち巨大なもの（E2E-012）。コミットしない
go run ./scripts/gentestdata          # testdata/e2e/generated/ に作る
go run ./scripts/gentestdata -clean   # 消す

# 自動 E2E テスト（E2E-101〜108）。リリース CI が配布物に対して実行する
go run ./scripts/e2e archives -dir dist -version v1.0.0-rc.1
go run ./scripts/e2e binary -archive dist/MarkView-v1.0.0-rc.1-windows-amd64.zip -version v1.0.0-rc.1
#   binary は **対象 OS の上でしか実行できない**（合わなければ中断する）

# 配布アーカイブを詰める（BR-020, BR-021）。リリース CI が呼ぶ
go run ./scripts/pack -exe build/bin/MarkView.exe -os windows -arch amd64 -version v1.0.0

# 同梱資産を最新安定版へ（BR-043）。リリース CI が rc タグで呼ぶ
go run ./scripts/vendorupdate              # frontend/vendor/ を書き換える
go run ./scripts/vendorupdate -dir <path>  # 別の場所へ出して確かめる

# 手動テストの記録用 Excel を生成（E2E-200）
python docs/tests/gen_manual_test_xlsx.py --version v1.0.0-rc.1
#   -> docs/tests/results/MarkView_手動テスト結果_v1.0.0-rc.1.xlsx
#   要 openpyxl。41 章のケース定義と「テストケース一覧」節が食い違うと生成せずエラー終了する
#   スクリプトは節番号ではなく見出し（`Gn` と「テストケース一覧」）でケースを探す（E2E-200）
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
2. その成果物に対して手動テスト 65 件を実施し、結果を `docs/tests/results/` にコミット
3. 全て OK なら本タグ `v<version>` を打つ。**本タグでは資産を更新しない**ため、rc で検証した内容がそのまま配布される

> [!IMPORTANT]
> **`v` で始まるタグの push はリリース CI を起動する**（BR-050）。仕様書やドキュメントの節目に付けるタグは `v` で始めない（例: `specs-4.2.0`）。
>
> **版番号は `v1.0.0` 系を使う。** `v0.1.0-rc.1` / `v0.1.0-rc.2` は「リリースが作られるか」を試すために打った試験用のタグであり、検証対象ではない。
>
> **`v1.0.0-rc.1` は手動テストを実施せずにリジェクトした。** 外部エディタで開く機能を `v1.0.0` に含めると決めたためである。**検証対象は `v1.0.0-rc.2`**。41 章を改訂したので前回の結果は引き継げず、全 65 件を実施する（E2E-201）。

## 規約

- **コードコメントは日本語**で書く。仕様書と同じ言語にすることで、`FR-016` のような ID の前後の説明をそのまま引ける。コードは `main` と `internal/` のみで外部から import されないため、英語 godoc の読み手は実質いない。
- ただし**利用者に見えるものはすべて英語**。UI 文言・エラーメッセージ・`--help` / `--version` の出力・Go のエラー値（IMP-021, UI-024）。ツールチップの英日併記のみ例外。
- 命名は IMP-020 に従う。パッケージ名は小文字 1 語・複数形にしない、型名にパッケージ名を繰り返さない（`filetree.Node`）、エラー変数は `Err` + 内容。
- 1 ファイルは目安 400 行以内。超えたら責務で分割する（IMP-011）。
- コミットメッセージは日本語、`docs:` `feat:` のような接頭辞を付ける。
