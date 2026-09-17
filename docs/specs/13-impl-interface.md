# 13. 実装仕様: Go ↔ フロントエンド インターフェース

> 索引: [README](README.md) | 実装仕様: [10](10-impl-overview.md) / [11](11-impl-backend.md) / [12](12-impl-frontend.md) / **13**

本文書は、Go 側（`app.go`）がフロントエンドへ公開するメソッドと、Go 側から送出するイベント、およびその間でやり取りするデータ型を定める。ここが両者の唯一の接点であり、**この境界の規約は厳密に守る**（AR-061）。

## 13.1 方針（IMP-300 系）

### IMP-300: 設計原則 **MUST**

1. **往復回数を最小化する。** 1 つの利用者操作に対する呼び出しは原則 1 回とし、必要な情報をまとめて返す（AR-061）。
2. **判断は Go 側に置く。** パスの解釈、リンク先の種類判定、拡張子の判定、サイズ判定をフロントエンドで行わない。フロントエンドは「利用者が何をしたか」を伝え、結果を描画する。
3. **フロントエンドから任意のパスを渡せる API を作らない。** ファイルを開く経路は、ダイアログ・ドロップ・引数・ツリー・リンク・履歴の 6 つに限る（IMP-192）。**書き込み（IMP-316）も例外ではない。** フロントエンドが渡すのは目印（IMP-120）と文字列だけであり、書き込み先は Go 側が持つ表示中の文書に限る（FR-143）。
4. **状態の正は Go 側にある。** フロントエンドの `state`（IMP-210）は写しである。

### IMP-301: 命名と型 **MUST**

- バインドメソッドは Go の慣習どおり大文字始まり。Wails は JavaScript 側で先頭小文字に変換する。
- DTO は `app.go` またはその近傍に定義し、`json` タグで JavaScript 側のフィールド名（キャメルケース）を明示する。
- 時刻は RFC 3339 の文字列、パスは絶対パスの文字列とする。
- `null` を返しうるフィールドはポインタ型とし、その旨を本文書に記す。

## 13.2 データ型（IMP-302 系）

### IMP-302: DocumentDTO **MUST**

```go
type DocumentDTO struct {
    Path          string             `json:"path"`          // 絶対パス
    DisplayPath   string             `json:"displayPath"`   // ステータス表示用（UI-060）
    Name          string             `json:"name"`          // ベース名（UI-013 のタイトル）
    OutsideTree   bool               `json:"outsideTree"`   // FR-052
    HTML          string             `json:"html"`          // サニタイズ済み（IMP-116）
    Headings      []renderer.Heading `json:"headings"`      // FR-040
    LineCount     int                `json:"lineCount"`     // UI-060
    Encoding      string             `json:"encoding"`      // 常に "UTF-8"
    NeedsMermaid  bool               `json:"needsMermaid"`  // AR-021
    NeedsKaTeX    bool               `json:"needsKaTeX"`    // AR-021
    NeedsPlantUML bool               `json:"needsPlantUML"` // AR-021, MD-085
    Scroll        ScrollDTO          `json:"scroll"`        // 描画後のスクロール指示
    Warnings      []string           `json:"warnings"`      // 警告の Kind（IMP-315）

    // v1.1.0（FR-120〜FR-144）
    SameDocument  bool               `json:"sameDocument"`  // 直前の表示と同じファイルか（1.7。IMP-192）
    Trigger       string             `json:"trigger"`       // "open" | "reload" | "watch" | "edit"（IMP-192）
    RefKey        string             `json:"refKey"`        // 目印の鍵（IMP-120）
    Editable      bool               `json:"editable"`      // 編集モードを開始できるか（FR-140 の表）
    EditMode      bool               `json:"editMode"`      // いま編集モードか（Go 側が正。IMP-109）
    EditSeq       uint64             `json:"editSeq"`       // 編集モードの状態の版（IMP-109）。古い値で上書きしないため
}

type ScrollDTO struct {
    Mode   string `json:"mode"`   // "top" | "anchor" | "restore" | "keep"
    Anchor string `json:"anchor"` // mode == "anchor" のとき。リンクのフラグメント（復号済み。接頭辞の有無を問わない）
    Top    int    `json:"top"`    // mode == "restore" のときの位置
}
```

| `Mode` | 動作 | 使う場面 |
| --- | --- | --- |
| `top` | 文書の先頭へ | ファイルを開く、ツリー選択、リンク遷移（アンカーなし） |
| `anchor` | `Anchor` の見出しがペイン上端付近に来る位置へ | アンカー付きリンク |
| `restore` | `Top` の値へ復元する | 履歴移動（FR-051） |
| `keep` | **フロントエンドが現在の位置をそのまま保つ**。`Top` は使わない | 再読み込み、更新の自動検知（FR-014, FR-015） |

`restore` と `keep` を分けているのは、位置の出どころが異なるためである。`restore` は Go 側が履歴に記録した値を渡すのに対し、`keep` はフロントエンドが持っている現在位置を使う。両者を 1 つのモードで表そうとすると「`Top` に 0 を入れて現在位置を維持させる」といった約束が必要になり、意味が読み取れなくなる。

`Scroll` を Go 側が決めるのは、スクロールの扱いが「どの経路で開いたか」（IMP-192）に依存するためである。フロントエンドに経路を意識させない。

**`Anchor` には、リンクのフラグメント（`#` を除き、`net/url` が復号した値。IMP-312）を入れる。** `user-content-` を Go 側で付けない。フロントエンドの `findInDocument` は生のままの値を先に試すため、復号済みの値でも見つかる。フロントエンドは IMP-223 の `findInDocument` で本文の中だけを探す（AR-053）。`LinkResultDTO.Anchor`（IMP-305）も同じ扱いとする。

`Warnings` には**文言ではなく IMP-315 の `Kind` を入れる**（例: 不正な UTF-8 を置換したときは `encoding`）。文言そのものを Go 側が組み立てると、`strings.js` の `warnEncoding`（IMP-290）が使われないまま残り、同じ文言の定義が 2 箇所になる。`ErrorDTO.Kind` と同じ扱いに揃え、フロントエンドが `Kind` から文言を選ぶ。空でも `null` ではなく空配列を返す。

同じ理由で、`Headings` も見出しがないとき空配列を返す。`null` を渡すとフロントエンドの走査が落ち、アウトラインだけでなく描画全体が止まる。

**v1.1.0 で足した 6 つは、再描画のたびに状態をどう引き継ぐかをフロントエンドに伝えるためにある**（DSP-352）。**判断はすべて Go 側で済ませて渡す**（IMP-300 の 2）。

| フィールド | フロントエンドの使い方 |
| --- | --- |
| `SameDocument` | 真なら同じ文書の再描画（1.7）として、折りたたみ・フォーカス・並べ替え・原寸表示・拡大画面を引き継ぐ（IMP-220）。偽なら解除する。**パスを比べて自分で判断しない**——同じファイルかどうかはシンボリックリンクと大文字小文字の規則を要し（`session.SameFile`。IMP-191）、**直前が状態画面なら同じファイルでも偽**になる（IMP-192） |
| `Trigger` | `edit` のときだけ、並べ替えを並べ直さずに行の並びを保つ（FR-130, IMP-229）。それ以外の用途に使わない |
| `RefKey` | 目印（`data-ref` / `data-link`。**図のブロックの `data-ref` を含む**）の値がこの鍵で始まるものだけを本物として扱う（IMP-120, IMP-260）。図の描画とコピーする原文の選択も、鍵の合うブロックに限る（NFR-030） |
| `Editable` | 偽なら編集モードのボタンを淡色にする（UI-021） |
| `EditMode` | ボタンの押下状態と本文の装飾に写す（IMP-260）。**フロントエンドが独自に保持している値より、こちらを優先する** |
| `EditSeq` | **`Editable` / `EditMode` を写すかどうかの判断にだけ使う。** フロントエンドがそれまでに受け取った版（`DocumentDTO.EditSeq` と `EditModeDTO.Seq` の大きいほう）より小さければ、`Editable` / `EditMode` を写さない（本文の再描画はする。IMP-260）。**イベントとバインドメソッドの戻り値は到着順が決まっていない**ため、`SetEditMode` の結果の後に、それより前に作られた `document:changed` が届きうる |

| 経路 | `Trigger` |
| --- | --- |
| ダイアログ・ドロップ・引数・ツリー・リンク・履歴・確認画面の `Open anyway` | `open` |
| 手動の再読み込み（FR-015） | `reload` |
| ファイル更新の自動検知（FR-014） | `watch` |
| 編集モードの書き込み・取り消し・やり直しの直後の読み直し（IMP-195 の 8）と、書き込み前の不一致による読み直し（IMP-195 の 4） | 前者は `edit`、後者は `reload` |

### IMP-303: InitialStateDTO **MUST**

```go
type InitialStateDTO struct {
    Config    ConfigDTO    `json:"config"`
    TreeRoot  string       `json:"treeRoot"`  // 絶対パス。未確定なら空文字
    Document  *DocumentDTO `json:"document"`  // 表示対象がなければ null
    StateKind string       `json:"stateKind"` // "" | "welcome" | "confirm-large" | "too-large" | "render-error"
    Error     *ErrorDTO    `json:"error"`     // StateKind が "" 以外で情報を要する場合
}

type ConfigDTO struct {
    Theme           string `json:"theme"`           // "light" | "dark"（解決済み。FR-071）
    OutlineVisible  bool   `json:"outlineVisible"`
    FileTreeVisible bool   `json:"fileTreeVisible"`
    OutlineWidth    int    `json:"outlineWidth"`
    FileTreeWidth   int    `json:"fileTreeWidth"`
}
```

**表示倍率を含めない。** 倍率は保存しない（UI-111, UI-115）ため、往路では渡すものがなく、復路でも Go 側に受け取る先がない（IMP-150）。倍率はフロントエンドの `state` だけが持つ（IMP-210, IMP-242）。ウィンドウの大きさと最大化状態も同様に含めない（サイズは Go 側が Wails のランタイムから直接読む。IMP-194）。

`Theme` は Go 側で OS 設定への追従（FR-071）まで解決済みの値を返す。OS 設定の取得は `ostheme`（IMP-175）が行う。フロントエンドで `prefers-color-scheme` を判定して上書きしない。

**`ConfigDTO` は往路（Go → JS）と復路（JS → Go, `UpdateConfig`）で同じ型を用いるが、`Theme` の意味だけが異なる。**

| 向き | `Theme` の値 | 意味 |
| --- | --- | --- |
| 往路 | `light` / `dark` | 解決済み。そのまま画面へ適用する |
| 復路 | `light` / `dark` | 利用者が明示的に切り替えた |
| 復路 | 空文字 | 利用者はまだ選んでいない。OS 設定への追従を保つ |

復路で解決済みの値を常に返すと、**ペインを開閉しただけで `config.Config.Theme` が空文字から `light` へ書き換わり、初回起動の OS 追従が最初の保存で失われる。** フロントエンドは「利用者が自分で切り替えたか」を別に持ち（IMP-210 の `state.themeExplicit`）、切り替えるまでは空文字を送る。Go 側の `Normalize`（IMP-153）は空文字を既定値（＝空文字）のまま保つため、追加の処理は要らない。

`StateKind` が `welcome` 以外の値を取るのは、**起動時の引数に大きすぎるファイルや壊れたファイルが指定された場合**である（FR-012）。この場合 `Document` は null となり、`Error` に対象パスとサイズと表示用パス（`DisplayPath` / `OutsideTree`。IMP-307）が入る。フロントエンドは通常の状態画面（IMP-250）と同じ処理でこれを描画する。起動経路のためだけの専用画面を作らない。

### IMP-304: TreeNodeDTO **MUST**

```go
type TreeNodeDTO struct {
    Name      string `json:"name"`
    Path      string `json:"path"`      // 絶対パス
    IsDir     bool   `json:"isDir"`
    HasChild  bool   `json:"hasChild"`  // 展開可能か（FR-032 の先読み結果）
    Omitted   int    `json:"omitted"`   // 件数上限で除かれた数。0 なら全件（FR-032）
}
```

子ノードは含めない。展開のたびに `ReadDir` を呼ぶ（FR-032 の遅延展開）。

`Omitted` は**その要素が属する一覧から件数上限で除かれた数**である（IMP-130）。切り詰めが起きた場合、返すすべての要素に同じ値が入る。フロントエンドは先頭の要素を見て、一覧の末尾に `… and N more` を表示する（DSP-112, IMP-290 の `treeMore`）。

`HasChild` はディレクトリかどうかと一致する。`filetree.ReadDir` が Markdown を含まないディレクトリを既に除いており（FR-031, IMP-133）、返ってきたディレクトリはすべて展開する価値があるためである。**先読みの判定を DTO 側でやり直さない。**

### IMP-305: LinkResultDTO **MUST**

```go
type LinkResultDTO struct {
    Kind     string       `json:"kind"`     // "document" | "external" | "anchor" | "error"
    Document *DocumentDTO `json:"document"` // kind == "document" のとき
    Anchor   string       `json:"anchor"`   // kind == "anchor" のとき
    Error    *ErrorDTO    `json:"error"`    // kind == "error" のとき
}
```

`external`（外部 URL・画像・その他のファイル）の場合、Go 側が既に OS へ委譲済みであり、フロントエンドは何もしない。

失敗は文言ではなく `ErrorDTO` をそのまま載せる（IMP-307）。リンク先が大きな Markdown だった場合、`Kind` は `error`、`Error.Kind` は `needs-confirm` となり、フロントエンドは他の経路と同じ確認画面を出せる（FR-016）。文言だけを渡すとサイズと上限が失われ、確認画面を組み立てられない。理由は IMP-308 と同じである。

### IMP-306: AboutDTO **MUST**

```go
type AboutDTO struct {
    Version     string              `json:"version"`
    Commit      string              `json:"commit"`
    BuildTime   string              `json:"buildTime"`
    Author      string              `json:"author"`
    Repository  string              `json:"repository"`
    License     string              `json:"license"`
    Environment string              `json:"environment"`
    Vendors     []buildinfo.VendorEntry `json:"vendors"`  // UI-100 の Bundled 行
    Licenses    string              `json:"licenses"`     // THIRD_PARTY.md の全文（FR-101）
}
```

- **`Vendors` には `buildinfo.Bundled()` の結果を入れる**（IMP-181）。`Vendors()` の全体ではない。同梱物の中に含まれるもの（Viz.js / Graphviz / Expat）は `Bundled` 行に出さず、`Licenses` の中に全文として現れる（UI-100）。

### IMP-307: ErrorDTO **MUST**

```go
type ErrorDTO struct {
    Kind        string `json:"kind"`        // IMP-315 の分類
    Message     string `json:"message"`     // 表示用の英語文言
    Path        string `json:"path"`        // 対象がある場合
    Size        int64  `json:"size"`        // サイズ関連のときのみ
    Limit       int64  `json:"limit"`       // サイズ関連のときのみ
    DisplayPath string `json:"displayPath"` // 状態画面の種別のときのみ。ステータス表示用（UI-060, DSP-302）
    OutsideTree bool   `json:"outsideTree"` // 同上。ツリー外か（FR-052）
}
```

- **`DisplayPath` / `OutsideTree` は、状態画面を出す種別（`needs-confirm` / `too-large` / `render-error`）でだけ設定する**（IMP-192, IMP-193）。値は `DocumentDTO` の同名のフィールドと同じ規則（`session.DisplayPath`。IMP-191）で、**画面の対象**（`target`）について求める。
- **状態画面の間、ステータス領域の左は画面の対象を指す**（FR-016, DSP-302）。状態画面では `DocumentDTO` が届かないため、フロントエンドは表示用のパスをここから得る（IMP-250）。**パスを自分で相対化しない**（IMP-300 の 2）。**v1.0.0 はこの値を持たず、状態画面の間も前の文書のパスが残っていた**（[BUG-012](../bugs/2026-09-14-bug-012-state-screen-previous-document.md)）。
- ステータスに出す種別（`not-found` など）では空のままとする。表示を変えない失敗であり、パスの表示も直前のまま保つ（FR-110）。

### IMP-308: OpenResultDTO **MUST**

```go
type OpenResultDTO struct {
    Document *DocumentDTO `json:"document"` // 成功したとき。失敗時は null
    Error    *ErrorDTO    `json:"error"`    // 失敗したとき。成功時は null
}
```

文書を開くバインドメソッド（IMP-310）の戻り値。**失敗を Go の `error` ではなく、この構造体で返す。**

> [!IMPORTANT]
> Wails v2 は Go の `error` を**メッセージ文字列としてしか**フロントエンドへ渡せない（`dispatcher.NewErrorCallback(message string, ...)` を経て、JavaScript 側は `new Error(message)` を受け取る）。`(*DocumentDTO, error)` のまま返すと `ErrorDTO` の `Kind` / `Size` / `Limit` が失われ、**大きなファイルの確認画面（FR-016, IMP-314）を組み立てられない。** 失敗が値として渡る形にする必要がある。

`Document` と `Error` がどちらも `null` の場合は「**何も起きなかった**」を表す。フロントエンドは表示を変えない。次の 5 つがこれにあたる。

| 場面 | メソッド |
| --- | --- |
| ダイアログを取り消した | `OpenFileDialog` |
| ダイアログを開けなかった | `OpenFileDialog` |
| 履歴の端で戻る・進むを呼んだ | `HistoryBack` / `HistoryForward` |
| 画面の対象が無い状態（文書未表示）で再読み込みした | `Reload` |
| 確認待ちでないパスで `Open anyway` を押した（二度押し、確認待ちが消えた後。IMP-314） | `OpenConfirmed` |

ダイアログを開けなかった場合を失敗として扱わないのは、表示中の文書を状態画面で置き換える理由がないためである（FR-110）。利用者から見れば「ファイルが選ばれなかった」ことに変わりはない。

`ReadDir` と `CopyToClipboard` と `ReadClipboard` は `error` を返したままとする。前者はツリーの一部が読めないだけであり、後の 2 つは失敗の種類が 1 つしかない。いずれも `Kind` を伴う分岐を必要としない（IMP-315）。

### IMP-309: EditorListDTO / EditorDTO / EditorResultDTO **MUST**

エディタ選択ウィンドウ（[UI-103](03-ui.md)）とその実行の DTO。

```go
// EditorListDTO は選択ウィンドウの中身（UI-103）。
type EditorListDTO struct {
    Editors []EditorDTO `json:"editors"`
    Error   *ErrorDTO   `json:"error"`
}

// EditorDTO は一覧の 1 行。
type EditorDTO struct {
    ID        string `json:"id"`        // プリセットの ID、または "custom"
    Name      string `json:"name"`      // 画面に出す表示名
    Available bool   `json:"available"` // 選択できるか（見つかったか）
    Selected  bool   `json:"selected"`  // 初期選択（UI-116）
}

// EditorResultDTO は起動の結果。
type EditorResultDTO struct {
    Name  string    `json:"name"`  // 起動したエディタの表示名。ステータス表示に使う
    Error *ErrorDTO `json:"error"` // 失敗したとき。成功時は null
}
```

> [!IMPORTANT]
> **`EditorDTO` に実行ファイルのパスを載せてはならない**（[NFR-035](07-nonfunctional.md) の 3）。画面に出す必要がなく、載せた時点でフロントエンドをパスが通ることになる。これは IMP-300 の 3 が禁じている形そのものである。

- 一覧の末尾には常に **`ID` が `custom` の行**を置く。`Other...` にあたる。
  - まだ何も選ばれていなければ `Name` は空、`Available` は `false`。
  - `BrowseEditor` で選ばれた後、または設定のエディタがどのプリセットとも一致しない場合は、**実行ファイル名（`filepath.Base`。パスではない）**を `Name` に入れ、`Available` を `true` にする（UI-103）。
- `Selected` が真の行は高々 1 つとする。設定にエディタが無い、または見つからない場合は**どの行も真にしない**（UI-116）。
- `Editors` の順序は [IMP-172](11-impl-backend.md) の定義順に `custom` を足したものとし、**並べ替えない**（UI-103）。

## 13.3 バインドメソッド（IMP-310 系）

すべて `App`（`desktop` パッケージ）のメソッドとして定義する。Wails のバインディングにより、JavaScript からは `window.go.desktop.App.*` として呼べる（`wails build` が `frontend/wailsjs/go/desktop/App.js` を生成する）。`js/api.js` がこれを薄くラップする（IMP-201）。**`App` の公開メソッドはこの一覧に限る**——公開メソッドはすべてフロントエンドから呼べる（11 章 11.11 の前書き）。

### IMP-310: 一覧 **MUST**

| メソッド | 引数 | 戻り値 | 対応要求 |
| --- | --- | --- | --- |
| `GetInitialState()` | — | `InitialStateDTO` | FR-012, FR-013, UI-110 |
| `OpenFileDialog()` | — | `OpenResultDTO` | FR-010 |
| `OpenFromTree(path string)` | 絶対パス | `OpenResultDTO` | FR-033 |
| `OpenConfirmed(path string)` | 絶対パス | `OpenResultDTO` | FR-016 |
| `FollowLink(href string)` | リンクの生値 | `LinkResultDTO` | FR-050, FR-053 |
| `HistoryBack()` / `HistoryForward()` | — | `OpenResultDTO` | FR-051 |
| `Reload()` | — | `OpenResultDTO` | FR-015 |
| `ReadDir(path string)` | 絶対パス | `([]TreeNodeDTO, error)` | FR-032, FR-035 |
| `GetTreeRoot()` | — | `string` | FR-030 |
| `SetScrollTop(top int)` | 現在のスクロール位置 | — | FR-051 |
| `UpdateConfig(patch ConfigDTO)` | 変更後の設定 | — | UI-110, UI-114 |
| `CopyToClipboard(text string)` | コピー対象 | `error` | FR-061, FR-063, AR-062 |
| `ReadClipboard()` | — | `(string, error)` | FR-063, AR-062 |
| `SetEditMode(on bool)` | 開始なら真 | `EditModeDTO` | FR-140 |
| `SetTask(ref string, checked bool)` | 目印の値、望む状態 | `EditResultDTO` | FR-141, FR-143 |
| `GetCellSource(ref string)` | 目印の値 | `CellSourceDTO` | FR-142 |
| `SetCell(ref string, text string)` | 目印の値、確定した文字列 | `EditResultDTO` | FR-142, FR-143 |
| `UndoEdit()` / `RedoEdit()` | — | `EditResultDTO` | FR-144 |
| `ListEditors()` | — | `EditorListDTO` | FR-091 |
| `BrowseEditor()` | — | `EditorListDTO` | FR-091 |
| `OpenInEditor(id string)` | プリセットの ID または `"custom"` | `EditorResultDTO` | FR-090 |
| `GetAbout()` | — | `AboutDTO` | FR-100, FR-101 |
| `Quit()` | — | — | UI-090 |

このほかに、フロントエンドから任意のパスを開く汎用メソッドを**定義しない**（IMP-300 の 3）。**エディタの 3 つも例外ではない。** 実行ファイルのパスは Go 側で生まれて Go 側で消費され、フロントエンドは識別子しか扱わない（IMP-309, NFR-035）。

- `BrowseEditor` は Go 側でファイル選択ダイアログを開き、**選ばれたパスを「確定前の候補」として 1 つだけ保持する。** `OpenInEditor("custom")` が用いてよいのは、**この候補か、設定に保存されたエディタ（UI-116）のどちらかだけ**とする。フロントエンドから受け取った値は使わない。任意の実行ファイルを無条件に起動する経路を作らないためであり、`OpenConfirmed` が確認待ちのパスを 1 つだけ保持するのと同じ考え方である（IMP-314）。
- **`ListEditors` は確定前の候補を捨てる。** 押すたびに選択ウィンドウを出す設計であり（[UI-103](03-ui.md)）、初期選択は設定に保存されたエディタだけから決まる（[UI-116](03-ui.md)）。`Browse` したまま閉じた候補が次に開いたときも残っていると、利用者には「閉じた場合は何も保存しない」（FR-091）が破れたように見える。
- **`OpenInEditor("custom")` は、確定前の候補が無ければ設定に保存されたエディタを使う**（UI-116）。`ListEditors` が候補を捨てる以上、候補だけを見ると**保存されたエディタは 2 回目以降けっして起動できない。** 一覧では選択済みとして出るのに `Open` が必ず `editor-failed` になる、という食い違いになる。用いてよいかどうかの判定は `EditorDTO.Available`（IMP-309）と同じ条件にする。**「一覧で選べる行」と「起動できる行」を一致させる。**
- `OpenInEditor` が成功したとき、**そのとき初めて設定へ保存する**（UI-116）。選択しただけ、`Browse` しただけでは保存しない。
- **`OpenInEditor` が開くファイルは `App.target`（IMP-190）である。** 「表示中の文書」（`current`）ではない。状態画面を出している間 `current` は前の文書のまま残っており、それを渡すと画面と食い違う（FR-090, NFR-035）。`target` が空（文書未表示）のときは `Error.Kind` を `editor-failed` として返す。**ボタンが淡色である以上（UI-021）通常は起こらないが、防御的に扱う。**

- **`GetAbout` は WebView のバージョンを取得して渡す**（[UI-100](03-ui.md), [IMP-181](11-impl-backend.md)）。空文字を固定で渡さない。取得手段と OS ごとの実装は IMP-181 が定める。**空文字は取得に失敗したときだけ**であり、そのとき `Environment` は当該区画を省く。
- **`Reload` は画面の対象（`target`。IMP-190）を読み直す。** 状態画面の間はその対象をもう一度開く（`confirm-large` なら確認画面、`render-error` なら変換をやり直す）。**表示中の文書（`current`）を開き直さない**——状態画面を見ながら `F5` を押すと、画面に無い前の文書が表示される（[BUG-012](../bugs/2026-09-14-bug-012-state-screen-previous-document.md)。v1.0.0 は `current` を開き直していた）。`target` が空なら何もしない（IMP-308）。
- **`ReadDir` はキャッシュを持たない。** 呼ばれるたびにディスクを読む。**いつ呼び直すかを決めるのはフロントエンドである**（FR-035 の 3 契機。[IMP-240](12-impl-frontend.md)）。

- **失敗は戻り値の DTO で伝える**（IMP-308, IMP-305）。Go の `error` を返すのは `ReadDir` と `CopyToClipboard` と `ReadClipboard` だけとする。
- **`ReadClipboard` は Wails の `runtime.ClipboardGetText` を使う**（AR-062）。WebView の `navigator.clipboard.readText` は読み取りに権限を求めるため使わない。**テキスト以外（画像など）しか入っていなければ空文字を返し、失敗にしない。**
- 各メソッドの入口で `recover` する（IMP-022, FR-111）。回復したパニックは `Error.Kind` が `render-error` の失敗として返す。**ただしエディタの 3 つは `editor-failed`、編集モードの書き込みの 4 つ（`SetTask` / `SetCell` / `UndoEdit` / `RedoEdit`）は `edit-failed` とする。** これらの結果はステータス領域に出るものであり（IMP-315）、`render-error` の文言「Failed to render this document.」は状況と合わない。利用者は「エディタを開こうとしたのに文書の変換に失敗した」と受け取ることになる。
- **文書を開くメソッド（`OpenFileDialog` / `OpenFromTree` / `OpenConfirmed` / `FollowLink` / `HistoryBack` / `HistoryForward` / `Reload`）で回復したパニックは、Go 側の状態も状態画面へ移す。** IMP-192 の「状態画面を出した」と同じ反映を行う——`showing` を偽にし、監視を外し、`edit.Left()` を呼び、確認待ちを消す。`ioMu` を取って行う。**画面の対象（`target`）は、そのメソッドが開こうとしていたパスが分かっていればそれ（ツリーの選択・確認待ちのパス・履歴のエントリ・再読み込みの対象・ダイアログで選ばれたパス）、分からなければ空とする**（`FollowLink` と、履歴を動かす前）。空のときは `ErrorDTO.DisplayPath` も空になり、ウィンドウタイトルはアプリケーション名だけになる。**フロントエンドは `render-error` の状態画面を出す**（IMP-250）ため、Go 側を「前の文書を表示中」のまま残すと、監視のイベントや `F5` で状態画面が前の文書の表示に置き換わる（BUG-012 と同じ形の食い違い）。**前の文書を `target` に残さない**——`F5` と「エディタで開く」が画面に無い文書を指す（IMP-190）。**回復が錠を取れるように、`ioMu` と `mu` は途中でパニックが起きても `defer` で解く形で取る。**
- **`SetEditMode` と `GetCellSource` は、回復したパニックを通知しない。** `edit-failed` の文言は「Failed to save: <path>」であり、保存していない操作には合わない。`SetEditMode` はその時点の `edit.On()` を返し（ボタンの表示が Go 側と揃う）、`GetCellSource` は `Stale` を真で返す（編集欄が開かないだけで済む）。
- `Quit` は `Ctrl+Q`（UI-090）の受け口である。`Alt+F4` と閉じるボタンは OS とウィンドウマネージャが処理するためこの経路を通らない。**終了処理そのものは Wails に任せ、ここで設定を保存しない。** `OnBeforeClose` / `OnShutdown`（IMP-194）を通ることで、閉じるボタンで終了した場合とまったく同じ後始末になる。
- `UpdateConfig` を**立て続けに 2 つ呼ばない**。バインドメソッドの呼び出しは Wails がメッセージごとに処理するため、到着順が入れ替わりうる。フロントエンドは `saveConfig`（IMP-210）を経由し、前の応答を待ってから次を送る。

### IMP-311: SetScrollTop の扱い **MUST**

- フロントエンドは、文書を離れる直前（リンク遷移・ツリー選択・履歴移動・再読み込みの前）に呼ぶ。
- スクロールのたびに呼ばない。呼び出し頻度を抑えるため、離脱時の 1 回に限る。
- Go 側は受け取った値を現在の履歴エントリに記録する（IMP-191）。

### IMP-312: FollowLink の判定順序 **MUST**

FR-050 の表を実装する。Go 側で以下の順に判定する。

```mermaid
flowchart TD
    A["FollowLink(href)"] --> B{"# で始まる"}
    B -->|Yes| R1["kind: anchor"]
    B -->|No| C{"スキームあり"}
    C -->|あり| D["opener.OpenURL<br/>(http / https / mailto のみ)"]
    D -->|成功| R2["kind: external"]
    D -->|失敗| R5["kind: error<br/>(open-failed)"]
    C -->|なし| F["baseDir を基準に絶対パス化 (AR-042)"]
    F --> G{"存在するか"}
    G -->|No| R3["kind: error<br/>(link-not-found)"]
    G -->|Yes| H{"拡張子 (IMP-105)"}
    H -->|Markdown| I["open(openFromLink)"] --> R4["kind: document"]
    H -->|画像| J["opener.OpenFile (FR-053)"]
    H -->|その他| J
    J -->|成功| R2
    J -->|失敗| R5
```

- `href` にアンカーが付いた Markdown（`./a.md#sec`）は、パス部分とアンカー部分を分離し、`DocumentDTO.Scroll` を `anchor` モードで返す。
- 基準ディレクトリは**表示中の文書のディレクトリ**であり、ツリールートではない（AR-042）。相対パスの解決規則は画像（IMP-118）と同一のものを使う。別々に書くと、`[x](./a.png)` が開くファイルと `![x](./a.png)` が表示するファイルが食い違いうる。
- **Windows のドライブレターをスキームと取り違えない。** `net/url` は `C:/docs/a.md` のスキームを `c` と解釈する。1 文字のスキームは存在しないため、これはローカルパスとして扱う。
- **OS への委譲の失敗は `kind: error`、`Error.Kind` は `open-failed` とする**（IMP-315）。`opener.ErrUnsupportedScheme`・`opener.ErrNotFound`・プロセスを起動できない（Linux で `xdg-open` が無いなど）のいずれも同じである。**文書を開く経路の分類（`not-found` / `render-error`）を使い回さない**——分類できないエラーを `render-error` に落とすと、フロントエンドは状態画面を出して本文が消える（[BUG-013](../bugs/2026-09-14-bug-013-link-open-failure-state-screen.md)。FR-053 は「その旨をステータス表示する」、FR-110）。v1.0.0 はこの形だった。エディタの失敗を `newEditorErrorDTO` で分けているのと同じ考え方である（IMP-315）。
- **`opener` が受け付けないスキームは `kind: error` とする**（IMP-170, NFR-030）。FR-050 は「`mailto:` 等のその他スキームは OS の既定ハンドラに委譲する」と定めるが、委譲してよいのは `http` / `https` / `mailto` に限る。文書は任意の第三者から受け取りうるため、`javascript:` や `file:` を OS へ渡さないことを優先する。

### IMP-313: ドロップの受け口 **MUST**

Wails のファイルドロップは、バインドメソッドではなくコールバックで受け取る。

```go
runtime.OnFileDrop(ctx, func(x, y int, paths []string) {
    // FR-011 の規則に従って 1 つを選び、open(openFromDrop) を呼ぶ
    // 結果は "document:opened" イベントでフロントエンドへ送る
})
```

- 複数パスから対象を選ぶ判定（FR-011）は Go 側で行う。
- ディレクトリがドロップされた場合、ツリールートを変更し、直下の README を探して開く。

> [!IMPORTANT]
> **`runtime.OnFileDrop`（Go）は購読であって、発火ではない。** 中身は `EventsOn(ctx, "wails:file-drop", ...)` だけである。**これを書いただけではパスは届かない。**
>
> | OS | 発火させるもの |
> | --- | --- |
> | **Windows**（WebView2） | **フロントエンドの `window.runtime.OnFileDrop()`**（[IMP-245](12-impl-frontend.md)）。これが `drop` リスナを取り付け、ドロップされた File オブジェクトを `postMessageWithAdditionalObjects` で Go へ渡して初めて絶対パスが得られる |
> | **Linux**（WebKitGTK） | GTK の `drag-data-received` / `drag-drop` シグナル。**JS 側の登録は要らない** |
>
> **この差があるため、Go 側だけを見て「配線した」と判断してはならない。** IMP-245 の JS 側の登録と対で成立する。片方を欠いた状態は、**オーバーレイだけが正しく出てドロップが無反応になる**という、原因を取り違えやすい形で現れる（[調査報告](../bugs/2026-09-04-bug-001-file-drop-windows.md)）。

### IMP-314: 大きなファイルの確認 **MUST**

FR-016 を実装する。

1. 通常の `open` が `ErrNeedsConfirm`（`*SizeError`）を返す。
2. `app.go` はこれを `OpenResultDTO{Error: &ErrorDTO{Kind: "needs-confirm", Path, Size, Limit}}` に変換して返す（IMP-308）。リンクから開いた場合は `LinkResultDTO{Kind: "error", Error: ...}` となる（IMP-305）。
3. フロントエンドは状態画面 `confirm-large` を表示する（IMP-250）。
4. `Open anyway` の押下で `OpenConfirmed(path)` を呼ぶ。Go 側は `LoadOptions{Confirmed: true}` で再試行する。

`OpenConfirmed` は、直前に確認画面を出したパスに対してのみ有効とする。Go 側が「確認待ちのパス」を 1 つだけ保持し、それ以外のパスを渡された場合は拒否する。任意のサイズのファイルを無条件に開く経路を作らないため。

**拒否は失敗として返さず、「何も起きなかった」（IMP-308）とする。** 拒否が起きるのは、`Open anyway` の二度押し（1 回目で本文が出た後に 2 回目が届く）と、確認画面の間に確認以外の失敗（ツリーで消えたファイルを選ぶなど）で確認待ちが消えた後の押下である（IMP-192）。**v1.0.0 は分類できないエラーとして `render-error` を返しており**、二度押しでは出たばかりの本文が状態画面に置き換わり、Go 側は文書を表示中のまま食い違った（[BUG-012](../bugs/2026-09-14-bug-012-state-screen-previous-document.md) と同じ形）。確認待ちが消えた後は、`F5` で確認画面を出し直せる（`Reload` は画面の対象を読み直す。IMP-310）。

**確認画面を出した時点で、ツリールートと表示履歴は対象へ移す**（FR-016）。FR-016 は「確認画面を表示した時点でタイトルとパス表示を対象のものに更新し、履歴に積む。`Alt+←` で直前の文書へ戻れること」を求めており、積まないと戻る先が 1 つずれる。適用する規則は成功時と同じ表（IMP-192）に従う。**監視は張らず、前の文書の監視も外し、表示中の文書（`current`）は差し替えない。** 描画を始めていないファイルは FR-014 の対象外であり（FR-016）、前の文書は表示対象ではなくなっている（FR-014）。ステータス領域のパスは確認画面の対象を指す（`ErrorDTO.DisplayPath`。IMP-307）。

**`Open anyway` で描画した文書は、同じ文書の再描画（1.7）では確認し直さない**（FR-016）。App は同意を `currentConfirmed`（IMP-190）に持ち、同じファイルの読み直しで `LoadOptions.Confirmed` を渡す（IMP-192）。文書の切り替えと状態画面で同意は消える。

したがって `OpenConfirmed` から呼ぶ `open` は `openFromConfirm` を使い、ツリールートと履歴を二重に動かさない（IMP-192）。

### IMP-315: エラーの分類と文言 **MUST**

Go 側の番兵エラー（IMP-021）を `ErrorDTO.Kind` へ写像し、フロントエンドが `strings.js` の文言（IMP-290）を選ぶ。

| Go のエラー | `Kind` | 表示先 | 文言（英語） |
| --- | --- | --- | --- |
| `document.ErrNotFound` | `not-found` | ステータス | `File not found: <path>` |
| `document.ErrPermission` | `permission` | ステータス | `Cannot access: <path>` |
| `document.ErrNotMarkdown` | `not-markdown` | ステータス | `Not a Markdown file: <path>` |
| `document.ErrNeedsConfirm` | `needs-confirm` | 状態画面 | `This file is large.` ほか（UI-052） |
| `document.ErrTooLarge` | `too-large` | 状態画面 | `File is too large (<size> / limit <limit>)` |
| 変換エラー・パニック回復 | `render-error` | 状態画面 | `Failed to render this document.` |
| リンク先が見つからない（FR-050） | `link-not-found` | ステータス | `Link target not found: <href>` |
| リンク先を OS へ委譲できない（`opener.ErrUnsupportedScheme` / `opener.ErrNotFound` / 起動の失敗。FR-050, FR-053。IMP-312） | `open-failed` | ステータス | `Cannot open: <href>` |
| クリップボード失敗 | `clipboard` | ステータス | `Failed to copy.` |
| 監視対象が削除された | `removed` | ステータス | `File was deleted: <path>` |
| エディタを起動できない | `editor-failed` | ステータス | `Failed to start the editor.` |
| `opener.ErrSelf` | `editor-self` | ステータス | `MarkView cannot be used as an editor.` |
| 不正な文字コードを置換 | `encoding` | ステータス | `Some characters were replaced.` |
| 書き込む前にファイルが変更されていた（FR-143） | `edit-conflict` | ステータス | `The file changed on disk and was not saved.` |
| 書き込めない（`document.ErrPermission` ほか、表の形の確かめで拒んだ `document.ErrNotEditable`。FR-143, FR-142） | `edit-failed` | ステータス | `Failed to save: <path>` |
| クリップボードを読めない（FR-063） | `paste` | ステータス | `Failed to paste.` |

- **指示を作った描画の後に再描画が起きていた場合（IMP-195 の `Stale`）は、`Kind` を作らない。** 失敗ではなく、通知もしない（FR-143）。
- **`open-failed` の `Path` にはリンクの生値（`href`）を入れる。** 実行ファイルのパスは入らない（OS の既定のハンドラが起動されるため、MarkView はそれを知らない）。
- **分類できないエラーを状態画面の種別へ落とすのは、文書を開く経路（IMP-192）だけとする。** ステータスに出る操作（リンクの委譲・エディタ・書き込み・クリップボード）は、それぞれの経路で種別を決める。

- 文言の組み立てはフロントエンドで行う。Go 側は `Kind` と要素（パス・サイズ）を渡す。これにより、文言の定義が `strings.js` の 1 箇所に集約される（IMP-290）。
- **`Kind` は戻り値の DTO に載せて渡す**（IMP-308, IMP-305）。Go の `error` として返すとメッセージ文字列しか渡らず、`Kind` も `Size` / `Limit` も失われる。
- `ErrorDTO.Message` には Go 側が組み立てた英語文言も入れる。フロントエンドが未知の `Kind` を受け取った場合のフォールバックとして用いる。

### IMP-316: 編集モードの DTO **MUST**

FR-140〜FR-144 のバインドメソッド（IMP-310）の戻り値。処理の中身は IMP-195 が定める。

```go
// EditModeDTO は SetEditMode の結果。失敗を伝える欄を持たない（開始できない場合も、
// 回復したパニックも失敗としない。IMP-195, IMP-310）。
type EditModeDTO struct {
    On  bool   `json:"on"`  // 結果として編集モードか
    Seq uint64 `json:"seq"` // 編集モードの状態の版（IMP-109。DocumentDTO.EditSeq と同じ系列）
}

// EditResultDTO は書き込み・取り消し・やり直しの結果。
type EditResultDTO struct {
    Changed bool      `json:"changed"` // ファイルを書き換えたか
    Stale   bool      `json:"stale"`   // 指示を作った描画が古かった（FR-143）。通知しない
    Error   *ErrorDTO `json:"error"`   // edit-conflict / edit-failed（IMP-195 の 3〜6）。成功と Stale では null
}

// CellSourceDTO はセルの編集欄に入れるソース（FR-142）。
type CellSourceDTO struct {
    Text  string    `json:"text"`
    Stale bool      `json:"stale"` // 読めない・古い指示・編集できないセル。通知しない（IMP-195）
    Error *ErrorDTO `json:"error"` // edit-conflict のときだけ
}
```

- **`Changed` が偽で `Stale` も `Error` も無い**のは、「内容が変わらなかった」または「取り消すものが無かった」ことを表す（FR-142, FR-144）。フロントエンドは見た目を戻すだけにし、通知しない。
- **表示の更新はこれらの戻り値ではなく `document:changed`（IMP-320）で届く**（AR-061）。フロントエンドは、戻り値とイベントのどちらが先に届いても成り立つように書く（IMP-195）。
- **目印の値（`ref`）はフロントエンドが HTML から読んだ文字列をそのまま渡す。** 解くのは Go 側である（`document.ParseRef`）。フロントエンドで番号へ分解して渡さない——鍵の照合（IMP-195 の 2）を省く経路を作らないためである。

## 13.4 イベント（IMP-320 系）

Go からフロントエンドへの一方向通知。`runtime.EventsEmit` で送出し、フロントエンドは `runtime.EventsOn` で購読する。

### IMP-320: 一覧 **MUST**

| イベント名 | ペイロード | 契機 | 対応要求 |
| --- | --- | --- | --- |
| `document:opened` | `DocumentDTO` | ドロップ・引数など、フロントエンドの呼び出し以外で表示対象が変わったとき | FR-011 |
| `document:changed` | `DocumentDTO` | 表示中ファイルの更新を検知して再変換したとき。**編集モードの書き込みの直後に Go 側が読み直したとき**（IMP-195 の 8）と、書き込み前の不一致で読み直したとき（IMP-195 の 4） | FR-014, FR-143 |
| `document:removed` | `ErrorDTO` | 表示中ファイルが削除されたとき（**画面が表示している文書のものに限る**。IMP-192 の「イベントの照合」）。**編集モードも同時に終える**（IMP-195） | FR-014, FR-110, FR-140 |
| `tree:root-changed` | `string`（絶対パス） | ツリールートが変わったとき | FR-030 |
| `error` | `ErrorDTO` | 非同期処理で発生したエラー。**監視のイベントによる読み直しと、編集モードの書き込みの前後の読み直し（IMP-195 の 4 と 8）に失敗したとき**を含む | FR-014, FR-016, FR-110, FR-143 |

- **`error` の `Kind` が状態画面の種別（`needs-confirm` / `too-large` / `render-error`）なら、フロントエンドは状態画面を出す**（IMP-250。`document:opened` と同じ扱い）。Go 側は既に状態画面へ移っている（IMP-192 の `target` / `showing` / 監視の解除、IMP-109 の `Left`）。**ステータスに出すだけにすると、画面は前の文書と編集モードの表示のまま残る。**
- **フロントエンドは、`state.editable` が偽の間に届いた `EditModeDTO` の `On` が真でも写さない**（IMP-260）。状態画面へ移るときと `document:removed` は版（`EditSeq`）を運ばないため、それより前に送った `SetEditMode(true)` の結果が後から届くと、版の比較だけでは止められない。**DTO に版を足さずに済むのは、状態画面と削除の後は編集モードを始められない（`Editable` が偽）ためである。** 次に文書が届けば、その `EditSeq` が以後の比較の基準になる。

### IMP-321: document:changed の扱い **MUST**

FR-014 を実装する。

- ペイロードの `Scroll.Mode` は **`keep`** とする（IMP-302）。位置はフロントエンドが保持している現在値を用い、Go 側は `Top` を設定しない。
- 再描画時、フロントエンドは検索状態をリセットし（FR-080）、ツリーの展開状態は維持する（FR-014）。
- 再描画後、Mermaid・KaTeX・PlantUML の描画も再実行する。資産の再読み込みは行わない（AR-021）。
- **監視のイベントで読み直した内容が表示中のものと同じなら、送らない**（FR-014, IMP-192）。書き込みの直後に Go 側が送った `document:changed` の後から、同じ内容のイベントが届くためである。
- **`SameDocument` は常に真である。** `document:changed` を送るのは、画面が表示している文書（`showing` が真）を同じファイルとして読み直したときだけであり（IMP-192 の「イベントの照合」、IMP-195 の 4 と 8）、状態画面の間のイベントは読み直さずに捨てるためである（IMP-302）。フロントエンドは DSP-352 の「同じ文書の再描画」の列に従って状態を引き継ぐ（IMP-220）。
- **ペイロードの `EditMode` が偽になっていれば、フロントエンドは編集モードの表示を解く**（読み直した結果、不正なバイト列を含んでいた場合。IMP-192 の表）。

### IMP-322: イベントの購読解除 **SHOULD**

フロントエンドは単一ページであり、購読は起動時の 1 回のみ行う。動的な購読・解除を繰り返さない。

## 13.5 呼び出しの流れ（例）

### IMP-330: 文書内リンクをクリックしたとき

```mermaid
sequenceDiagram
    participant U as 利用者
    participant FE as フロントエンド
    participant APP as App (Go)
    participant DOC as document/renderer
    participant OS as OS

    U->>FE: リンクをクリック
    FE->>FE: preventDefault (AR-060)
    FE->>APP: SetScrollTop(現在位置)
    FE->>APP: FollowLink(href)
    APP->>APP: 種類を判定 (IMP-312)
    alt Markdown ファイル
        APP->>DOC: Load + Render
        DOC-->>APP: Document
        APP->>APP: 履歴に積む / ツリールートは変えない
        APP-->>FE: LinkResultDTO(kind=document)
        FE->>FE: renderDocument (IMP-220)
    else 外部 URL / 画像 / その他
        APP->>OS: 既定ブラウザ・既定アプリで開く
        APP-->>FE: LinkResultDTO(kind=external)
        FE->>FE: 何もしない
    else 見つからない・OS へ委譲できない
        APP-->>FE: LinkResultDTO(kind=error)
        FE->>FE: ステータスに表示 (link-not-found / open-failed。IMP-315)
    end
```

### IMP-331: エディタで開くとき

**押すたびに選択ウィンドウを出す**（FR-091）。往復は最大 3 回で、`Browse` を使わなければ 2 回で済む。

```mermaid
sequenceDiagram
    participant U as 利用者
    participant FE as フロントエンド
    participant APP as App (Go)
    participant OS as OS

    U->>FE: エディタで開くボタン / Ctrl+E
    FE->>APP: ListEditors()
    APP->>APP: プリセットを検出 (IMP-172) + 設定と突き合わせ (UI-116)
    APP-->>FE: EditorListDTO
    FE->>FE: 選択ウィンドウを表示 (IMP-252)
    opt Other... を選んで Browse
        FE->>APP: BrowseEditor()
        APP->>OS: ファイル選択ダイアログ
        OS-->>APP: 実行ファイルのパス
        APP->>APP: 確定前の候補として保持
        APP-->>FE: EditorListDTO (custom に実行ファイル名)
    end
    U->>FE: Open / Enter
    FE->>APP: OpenInEditor(id)
    APP->>APP: id を絶対パスへ解決 + 対象は App.target (IMP-190)
    APP->>APP: 起動前の検査 (IMP-171)
    alt 成功
        APP->>OS: exec.Command(editor, path).Start()
        APP->>APP: 設定へ保存 (UI-116)
        APP-->>FE: EditorResultDTO(name)
        FE->>FE: ウィンドウを閉じ、ステータスに表示 (DSP-151)
    else 失敗
        APP-->>FE: EditorResultDTO(error)
        FE->>FE: ウィンドウを閉じ、ステータスに表示 (IMP-315)
    end
```

`ListEditors` は**画面の対象があるかを見ない。** 一覧を作るだけであり、対象の有無はボタンの活性（UI-021）で表す。

**対象が無いときに呼ばない判定は、フロントエンド側の 1 か所に置く。** ツールバーのボタンとショートカット（`Ctrl+E`）の両方が同じ入口を通るようにする。ボタンは淡色で防げるが（UI-021）、ショートカットはそれだけでは止まらない。判定が抜けると、操作案内の表示中に `Ctrl+E` を押したときだけ `Failed to start the editor.` が出る、原因の分からない失敗になる。

**保存は起動できたあとに行う。** 図の順序どおり、`Start()` が成功してから設定へ書く（IMP-310, UI-116）。先に保存すると、起動に失敗したエディタが次回の初期選択として残る。

**`ListEditors` を起動時に先読みしない。** プリセットの検出はファイルシステムを触るため、押されるまで行わない（NFR-013）。また、MarkView の実行中にエディタがインストール・アンインストールされうる。

### IMP-332: 編集モードでチェックボックスを切り替えるとき

**見た目は先に変え、ファイルからの再描画は後から届く**（FR-141, NFR-012）。セルの確定（FR-142）と取り消し（FR-144）も同じ形である（取り消しは見た目を先に変えない）。

```mermaid
sequenceDiagram
    participant U as 利用者
    participant FE as フロントエンド
    participant APP as App (Go)
    participant DOC as document / renderer
    participant FS as ファイル

    U->>FE: チェックボックスをクリック / Space
    FE->>FE: 見た目を反転（100 ms 以内。IMP-261）
    FE->>APP: SetTask(ref, checked)
    APP->>APP: ioMu を取る（IMP-190）
    APP->>DOC: 編集モードか・鍵が current.RefKey と合うか（EditSession.Check。IMP-109）
    alt 鍵が合わない
        APP-->>FE: EditResultDTO(stale)
        FE->>FE: 見た目を戻す（通知しない）
    else 合う
        APP->>FS: 実体を読む
        APP->>DOC: SHA-256 が Known() と一致するか（EditSession.Plan。IMP-109）
        alt 一致しない
            APP->>DOC: openLocked で読み直し（trigger: reload。IMP-192）
            APP-->>FE: event document:changed
            APP-->>FE: EditResultDTO(edit-conflict)
            FE->>FE: 見た目を戻し、ステータスに表示
        else 一致する
            DOC->>DOC: PlanTask（IMP-106 → IMP-121）
            APP->>FS: 一時ファイル + リネーム（IMP-107）
            APP->>DOC: 履歴に記録（EditSession.Commit。IMP-109 → IMP-108）
            APP->>DOC: openLocked で読み直し（trigger: edit。内容が書き込んだものと同じなら鍵を引き継ぐ。監視を待たない）
            APP-->>FE: event document:changed
            APP-->>FE: EditResultDTO(changed)
            FE->>FE: 再描画（同じ文書として状態を引き継ぐ。DSP-352）
        end
    end
    Note over APP,FS: 150 ms 後に監視のイベントが届くが、内容が同じなので送らない（IMP-192）
```

- **イベントと戻り値の到着順は図のとおりとは限らない。** フロントエンドは、先に再描画が来てから戻り値が来ても、先に戻り値が来てから再描画が来ても、同じ結果になるように書く（IMP-261）。
- **利用者が続けて別のチェックボックスをクリックした場合**、2 つ目の `SetTask` は 1 つ目の `ioMu` が解けるまで待つ。**2 つ目は 1 つ目の書き込みの後の内容に対して位置を求め直す**（IMP-106）。**1 つ目の読み直しは鍵を引き継ぐ**（IMP-195 の 8）ため、2 つ目が 1 つ目の再描画より前の描画で作られていても `stale` にならない（FR-143 の「続けてクリックしただけで拒まない」）。

## 13.6 要求一覧

| ID | 概要 | 必須度 |
| --- | --- | --- |
| IMP-300 | 設計原則 | MUST |
| IMP-301 | 命名と型 | MUST |
| IMP-302 | DocumentDTO | MUST |
| IMP-303 | InitialStateDTO | MUST |
| IMP-304 | TreeNodeDTO | MUST |
| IMP-305 | LinkResultDTO | MUST |
| IMP-306 | AboutDTO | MUST |
| IMP-307 | ErrorDTO | MUST |
| IMP-308 | OpenResultDTO | MUST |
| IMP-309 | EditorListDTO / EditorDTO / EditorResultDTO | MUST |
| IMP-310 | バインドメソッド一覧 | MUST |
| IMP-311 | SetScrollTop の扱い | MUST |
| IMP-312 | FollowLink の判定順序 | MUST |
| IMP-313 | ドロップの受け口 | MUST |
| IMP-314 | 大きなファイルの確認 | MUST |
| IMP-315 | エラーの分類と文言 | MUST |
| IMP-316 | 編集モードの DTO | MUST |
| IMP-320 | イベント一覧 | MUST |
| IMP-321 | document:changed の扱い | MUST |
| IMP-322 | イベントの購読解除 | SHOULD |
| IMP-330 | 呼び出しの流れ（リンク遷移） | — |
| IMP-331 | 呼び出しの流れ（エディタで開く） | — |
| IMP-332 | 呼び出しの流れ（編集モードでの書き込み） | — |
