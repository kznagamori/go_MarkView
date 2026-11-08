# 11. 実装仕様: Go 側

> 索引: [README](README.md) | 実装仕様: [10](10-impl-overview.md) / **11** / [12](12-impl-frontend.md) / [13](13-impl-interface.md)

本文書は `internal/` 配下の各パッケージと `app.go` の実装仕様を定める。型定義・シグネチャは実装の指針であり、同等の結果が得られる範囲での変更を妨げない（IMP-002）。

> [!NOTE]
> 本文書の節はパッケージの依存順（葉から先）に並べており、ID の番号順とは一致しない。`mdfile`（IMP-105）を先頭に置いているのは、**依存を持たない葉パッケージ**であり `document` / `filetree` / `session` / `assetsrv` が参照するためである（IMP-012）。

## 11.1 mdfile パッケージ（IMP-105）

責務: **開けるファイル（[1.7](01-overview.md)）の拡張子と種類の判定**。**依存を一切持たない葉パッケージ**であり、`document` / `filetree` / `session` / `assetsrv` / `desktop` のいずれからも直接参照してよい（IMP-012）。**パッケージ名は v1.2.0 でも変えない**——Markdown 以外も扱うようになった（FR-025〜FR-027）が、変えると import がすべて変わる。

### IMP-105: 拡張子と種類の判定 **MUST**

```go
package mdfile

// Kind は開けるファイルの種類（1.7 の「開けるファイル」）。
type Kind string

const (
    KindNone     Kind = ""         // 開けない
    KindMarkdown Kind = "markdown" // FR-010
    KindPlantUML Kind = "plantuml" // FR-025
    KindMermaid  Kind = "mermaid"  // FR-026
    KindImage    Kind = "image"    // FR-027
)

// 拡張子の定義は、ここだけに置く（1.7 の「開けるファイル」の正）。
var (
    Extensions         = []string{".md", ".markdown", ".mdown", ".mkd"} // Markdown（FR-031）
    PlantUMLExtensions = []string{".puml", ".pu", ".wsd", ".plantuml"} // FR-025
    MermaidExtensions  = []string{".mmd", ".mermaid"}                  // FR-026
)

// ImageType は画像の拡張子と、配信するときの Content-Type（FR-022, IMP-161, IMP-162）。
type ImageType struct {
    Ext         string
    ContentType string
}

// ImageTypes は 9 つの画像の拡張子（FR-022）。並びは FR-037 の文言の並びとする。
var ImageTypes = []ImageType{
    {".png", "image/png"}, {".jpg", "image/jpeg"}, {".jpeg", "image/jpeg"},
    {".gif", "image/gif"}, {".svg", "image/svg+xml"}, {".webp", "image/webp"},
    {".avif", "image/avif"}, {".bmp", "image/bmp"}, {".ico", "image/x-icon"},
}

// KindOf は拡張子から種類を返す。比較は常に小文字化して行う。開けなければ KindNone。
func KindOf(path string) Kind

func IsMarkdown(path string) bool // KindOf(path) == KindMarkdown
func IsDiagram(path string) bool  // 図のファイル（PlantUML か Mermaid。1.7）
func IsImage(path string) bool    // 画像ファイル（1.7）
func IsText(path string) bool     // 文字のファイル（Markdown と図。1.7）。外部エディタで開けるもの（FR-090）
func IsOpenable(path string) bool // 開けるファイル（文字のファイルと画像。1.7）

// ImageContentType は、画像の拡張子なら配信する Content-Type を返す（IMP-161）。
func ImageContentType(path string) (string, bool)
```

この 1 箇所を、次のすべてが参照する。**定義を分散させない。**

| 使う所 | 使う判定 |
| --- | --- |
| 読み込み（IMP-102） | `KindOf`（`KindNone` なら `ErrNotSupported`） |
| ファイル選択ダイアログのフィルタ（FR-010） | 3 つの拡張子の一覧と `ImageTypes`（IMP-310 の `OpenFileDialog`） |
| ドロップ（FR-011, IMP-313）・起動引数（FR-012）・リンク（FR-050, IMP-312） | `IsOpenable` |
| ツリーの絞り込み（FR-031, IMP-134）とツリーの操作の検査（FR-036, IMP-135） | `KindOf` |
| README の探索（FR-013, IMP-193） | `IsMarkdown`（**Markdown のまま**） |
| ローカル画像の配信（AR-041, IMP-161） | `ImageContentType` |
| フィルターの文言の拡張子（FR-037, IMP-303） | 3 つの拡張子の一覧と `ImageTypes` |
| 外部エディタの対象（FR-090, IMP-196） | `IsText` |

- **`IsText` と `IsOpenable` の違いは画像だけである。** 画像はエディタで開かない（FR-027, FR-090）。`IsText` はエディタで開けるかの判断にだけ使う。
- **名前全体が拡張子の文字と一致するもの（`.md` / `.puml` / `.png` など）は `KindNone` とする**（v1.1.0 の `IsMarkdown` と同じ規則を 3 種類に当てはめる。4.77.0 で明記した。UT-105 のケース 4）。拡張子しか持たない名前を、その種類のファイルとして扱わない。
- **`.iuml` は含めない**（FR-025）。取り込み用の断片は単体で図にならない。開けないファイルとして扱い、リンク先なら OS の既定のアプリケーションへ渡す（FR-050）。
- **画像の拡張子と Content-Type の表は、v1.1.0 まで `assetsrv` が持っていた**（`allowedImageExt`。IMP-161）。**画像ファイルを開けるようになって（FR-027）、配信する拡張子と開ける拡張子が同じ一覧になった**ため、ここへ移した（4.75.0）。2 か所に置くと、片方だけ足したときに、開けるのに描けない画像や、描けるのに開けない画像が生まれる。`assetsrv` → `mdfile` は葉への依存であり、IMP-012 が許す。

> [!NOTE]
> 独立したパッケージとしているのは、`filetree`（IMP-132）と `session`（IMP-193）と `assetsrv`（IMP-161）がこの判定を必要とする一方、IMP-012 が `internal/` 同士の依存を禁じているためである。判定を `document` に置くとそれらが `document` を経由して `renderer` に依存し（`document` 自身も、読み込みの種類を決めるためにこの判定を使う。IMP-102）、拡張子を調べるだけのために goldmark と chroma をテストバイナリへ持ち込むことになる。依存を持たない葉パッケージに置くことで、「定義は 1 箇所」（本項）と「内部パッケージ同士を絡ませない」（IMP-012）の双方を満たす。

## 11.2 document パッケージ（IMP-100 系）

責務: 開けるファイル（Markdown・図のファイル・画像ファイル）の読み込み、文字コードの正規化、サイズ判定、`renderer` の呼び出し、結果の組み立て。**編集モードの書き換え（チェックボックス・セル・コードブロックと図・修飾）の判断と、ファイルへの書き込み**（IMP-106〜IMP-109, IMP-122, IMP-124）。

### IMP-100: 型定義 **MUST**

```go
package document

// Document は表示対象の 1 文書を表す。
type Document struct {
    Path          string             // 絶対パス（IMP-025）
    Kind          mdfile.Kind        // 種類（IMP-105）。変換の分け方と編集の可否を決める（4.75.0）
    Size          int64              // ファイルの実バイト数
    HTML          string             // サニタイズ済みの本文 HTML
    Headings      []renderer.Heading // アウトライン（FR-040）
    LineCount     int                // 総行数（UI-060 の表示に使う）
    NeedsMermaid  bool               // Mermaid の遅延ロード判定（AR-021）
    NeedsKaTeX    bool               // KaTeX の遅延ロード判定（AR-021）
    NeedsPlantUML bool               // PlantUML の遅延ロード判定（AR-021, MD-085）
    Warnings      []Warning          // 描画は継続するが利用者に伝える事象
    Digest        [sha256.Size]byte  // 読み込んだ生バイト列の要約（FR-143 の「把握している内容」）
    RefKey        string             // この描画の目印の鍵（IMP-120）。描画のたびに作り直す
    TextRefs      bool               // 修飾の目印（text:<n>）を付けて描画したか（IMP-120。編集モードの間だけ真）
    NoDiagrams    bool               // 図のファイルで、図が 1 つも無い（FR-025, FR-026。IMP-126, IMP-127）
}

// Editable は編集モードを開始できる文書かを返す（FR-140 の表）。
// 不正なバイト列を置き換えた文書は、表示とファイルの内容が一致しないため偽とする。
// 画像ファイル（Kind が KindImage）も偽とする（書き換える対象を持たない。FR-027）。
func (d *Document) Editable() bool

// Warning は FR-110 のうち「描画を継続する」事象を表す。
type Warning struct {
    Kind   WarningKind
    Detail string
}

type WarningKind int

const (
    WarnInvalidEncoding WarningKind = iota // 不正な UTF-8 を置換した（FR-021）
    WarnTruncatedTree                      // ツリーの件数上限に達した（FR-032）
)
```

### IMP-101: サイズ閾値 **MUST**

```go
const (
    // FR-016 の閾値。「10 MB」「50 MB」は 2 進接頭辞（specs/README.md の「単位の表記」）。
    ConfirmThreshold int64 = 10 << 20 // 10 MiB = 10,485,760 バイト
    MaxSize          int64 = 50 << 20 // 50 MiB = 52,428,800 バイト
)
```

利用者への表示は小数第 1 位までの MB 表記（`12.4 MB`）とし、`1 MB = 1024 × 1024` で換算する（DSP-181）。**仕様書全体の約束と同じである**（[README](README.md) の「単位の表記」）。

### IMP-102: 読み込み **MUST**

```go
type LoadOptions struct {
    // Confirmed が true の場合、ConfirmThreshold を超えていても描画する。
    // FR-016 の「Open anyway」と、確認して描画した文書を同じファイルとして
    // 読み直すとき（IMP-192 の currentConfirmed）に渡す。
    Confirmed bool

    // RefKey が空でなく、読んだ生バイト列の要約が ExpectDigest と一致すれば、
    // 新しい鍵を作らずにこの値を使う（IMP-120）。
    // 編集モードの書き込みの直後の読み直しだけが渡す（IMP-195 の 8, FR-143）。
    RefKey       string
    ExpectDigest [sha256.Size]byte // RefKey を使ってよい内容の要約（書き込んだ内容の SHA-256）

    // TextRefs が true なら、修飾の目印（text:<n>）も付けて描画する（IMP-120, renderer.Options）。
    // 編集モードの間の、同じ文書の読み直しだけが渡す（IMP-192）。
    TextRefs bool
}

// Load はファイルを読み込み、変換して Document を返す。
// 返しうるエラー: ErrNotFound / ErrPermission / ErrNotSupported /
//                 ErrTooLarge / ErrNeedsConfirm / 変換エラー
func Load(r *renderer.Renderer, path string, opts LoadOptions) (*Document, error)
```

処理順序を以下に固定する。

```mermaid
flowchart TD
    A["Load(path)"] --> B{"拡張子が開けるものか<br/>(mdfile.KindOf)"}
    B -->|No| E1["ErrNotSupported"]
    B -->|Yes| C["os.Stat"]
    C -->|存在しない| E2["ErrNotFound"]
    C -->|権限なし| E3["ErrPermission"]
    C -->|ディレクトリ・パイプ・デバイス・ソケット| E1
    C --> D{"サイズ判定 (IMP-101)"}
    D -->|"> MaxSize"| E4["ErrTooLarge"]
    D -->|"> ConfirmThreshold かつ !Confirmed"| E5["ErrNeedsConfirm"]
    D -->|それ以外| J{"画像か"}
    J -->|Yes| L["版を作る (localurl.Version)<br/>renderer.RenderImageFile (IMP-128)"]
    L --> I
    J -->|No| F["os.ReadFile"]
    F --> K["Digest を求め、鍵（RefKey）を決める"]
    K --> G["Normalize (IMP-103)"]
    G --> H["種類ごとの変換<br/>Render / RenderPlantUMLFile / RenderMermaidFile"]
    H --> I["Document を組み立てて返す"]
```

**種類ごとの変換**（4.75.0。FR-025〜FR-027）

| `Kind` | 変換 | 読み込み | `Digest` と鍵 |
| --- | --- | --- | --- |
| `markdown` | `renderer.Render`（IMP-110） | 上の図のとおり | 作る |
| `plantuml` | `renderer.RenderPlantUMLFile`（IMP-126） | 同上（FR-021 の正規化も同じ） | 作る（図の書き換え。FR-145） |
| `mermaid` | `renderer.RenderMermaidFile`（IMP-127） | 同上 | 作る（同上） |
| `image` | `renderer.RenderImageFile`（IMP-128） | **中身を読まない。** `os.Stat` の値だけを使う | **`Digest` は版（`localurl.Version` の文字列）の SHA-256 とする**（版が同じなら同じ内容とみなす。FR-014）。**鍵は作らない**（空文字。編集しない。FR-027） |

- **画像ファイルは中身を読まない。** 大きさの規則（FR-016）は `os.Stat` の大きさで当てはめる（FR-027）。**`Digest` を版から作るのは、監視の読み直しで「同じ内容なら送らない」（IMP-192）を画像にも当てはめるためである**——零値のままにすると常に「同じ」になり、外部で画像を書き換えても再描画が送られない（FR-014, FR-027）。`LineCount` は 0、`Editable()` は偽とする。**版（`localurl.Version`。IMP-118）は `os.Stat` の更新時刻と大きさから作る**——外部で書き換えられたら URL が変わり、WebView が取り直す（FR-027, BUG-026）。
- 画像の寸法（`1920 × 1080 px`）は Go 側で求めない。フロントエンドが読み込んだ後の `naturalWidth` / `naturalHeight` を使う（IMP-128, DSP-150）。ファイルの大きさは `Size` を `DocumentDTO.FileSize` に載せる（IMP-302）。
- **ディレクトリ・名前付きパイプ・デバイス・ソケットは、種類を問わず `ErrNotSupported` とする**（`x.md` / `x.png` という名前のディレクトリ。`os.Stat` の後で判定し、存在しなければ `ErrNotFound` のまま。4.77.0 までは画像だけ `ErrNotFound` とし、図のファイルは決めていなかった。v1.1.0 の Markdown と同じ扱いにそろえた。4.78.0。利用者の判断。UT-106 のケース 5・6）。**名前付きパイプ（FIFO）を読むと書き手が現れるまで戻らず、デバイスは中身を読んでしまう**（Linux。UT-106 のケース 7）。**`Mode().IsRegular()` では判定しない**——Go 1.23 から、Windows の再解析ポイント（シンボリックリンクと重複除去以外。OneDrive のクラウドのファイルなど）は `ModeIrregular` になり、v1.1.0 で開けた文書が開けなくなる（P24 の独立したレビューの指摘。利用者の判断。4.87.0。それまでは「通常のファイルでなければ（ディレクトリなど）」と書いていたが、実装はディレクトリだけを拒んでいた）。
- **`TextRefs` は `renderer.Options` へそのまま渡し、`Document.TextRefs` に写す**（IMP-120）。Markdown 以外では目印を付けないため常に偽とする（FR-146 の対象外）。

- **`Digest` は `os.ReadFile` で得た生バイト列（正規化の前）の SHA-256 とする**（FR-143, FR-144）。内容の写しは保持しない。
- **`RefKey` は `crypto/rand` の 8 バイトを 16 進 16 文字にしたものとし、`Load` のたびに作り直して `renderer.Render` へ渡す**（IMP-120）。同じファイルを読み直しても値は変わる。**鍵が変わること自体が「その描画の後に再描画が起きた」ことの印になる**（FR-143 の「指示を作った描画に対してだけ有効」）。
- **例外は `LoadOptions.RefKey` が空でなく、`Digest` が `LoadOptions.ExpectDigest` と一致するときで、その値をそのまま使う。** 編集モードの書き込みの直後の読み直し（IMP-195 の 8）だけが、表示中の文書の鍵と、書き込んだ内容の要約を渡す。**編集モードを始めたときの描き直し（IMP-195。4.75.0）は鍵を渡さない**（新しい鍵になる。FR-143 の「自分の書き込み以外を契機とする再描画の後の指示は拒む」に従う）。編集モードでない間に作られた指示は、もともと Go 側が拒む（IMP-109 の 1）。**自分の書き込みでは、それより前の描画で作った指示を有効なまま保つ**（FR-143）——作り直すと、続けてクリックした 2 つ目が拒まれる。
  - **要約が一致しなければ、新しい鍵を作る。** 書き込み（IMP-195 の 7）と読み直し（8）の間に外部の書き込みが入ると、読み直した内容は自分の書き込みではない。鍵を引き継ぐと、古い描画で作った指示が `Check`（IMP-109）を通り、`Loaded` が把握している内容を外部の内容で作り直すため要約の照合も通る。**外部で項目が挿入されていれば、別のチェックボックスを書き換える**（FR-143 の「自分の書き込み以外による再描画の後の指示は拒む」）。
  - 要約と鍵は `os.ReadFile` の直後、変換の前に決める（上の図）。
- **`Confirmed` が真でも、`MaxSize` を超えれば `ErrTooLarge` とする。** 同意が効くのは `ConfirmThreshold` だけである（FR-016）。
- `ErrTooLarge` と `ErrNeedsConfirm` は、サイズ情報を添えて返す。呼び出し側が状態画面（UI-052）に表示できるよう、`*SizeError` 型でラップする。**画像ファイルも同じである**（FR-027, UI-052）。
- **`ErrNotSupported` は v1.1.0 の `ErrNotMarkdown` を改名したもの**である（4.75.0。IMP-021, IMP-315 の `not-supported`）。開けるファイルが 3 種類になり、「Markdown でない」では理由を表せなくなった。

```go
type SizeError struct {
    Path  string
    Size  int64
    Limit int64
    Err   error // ErrTooLarge または ErrNeedsConfirm
}

func (e *SizeError) Error() string { ... }
func (e *SizeError) Unwrap() error { return e.Err }
```

### IMP-103: 文字コードの正規化 **MUST**

FR-021 を実装する。

```go
// Normalize は生バイト列を UTF-8 テキストへ正規化する。
// 戻り値の bool は、不正なバイト列を置換したかどうかを示す。
func Normalize(raw []byte) (text []byte, replaced bool)
```

処理内容は以下の順とする。

1. UTF-8 BOM（`EF BB BF`）を先頭にのみ検出し、除去する。
2. 改行コードを LF に統一する。`CRLF` → `LF`、単独の `CR` → `LF` の順で置換する。
3. `utf8.Valid` が false の場合、`strings.ToValidUTF8` 相当の処理で不正バイトを U+FFFD に置換し、`replaced = true` を返す。
4. UTF-16 の BOM（`FF FE` / `FE FF`）を検出した場合も、変換は行わずそのまま 3 の処理へ進む。結果は文字化けするが、読み込み自体は成功させる（FR-021 の「失敗させない」）。

### IMP-104: 行数の算出 **MUST**

`LineCount` は正規化後のテキストの LF の個数に 1 を加えた値とする。末尾が LF で終わる場合は加算しない。

### IMP-106: 書き換え位置の対応 **MUST**

FR-141 / FR-142 / FR-145 / FR-146 / FR-147 / AR-031 を実装する。**書き換える位置を、ファイルの生バイト列の上で求める。** 配置は `internal/document/edit.go`。

**書き換え処理を置くパッケージは `document` とする。** 位置を求めるには `renderer` と同じ goldmark の構成が要り（IMP-121）、`internal/` 同士の依存で許されているのは `document` → `renderer` だけである（IMP-012）。**`internal/editor` のような新しいパッケージを作らない**——`renderer` を呼べず、構成が 2 か所に複製される。

```go
// RefKind は書き換え対象の種類。
type RefKind int

const (
    RefTask     RefKind = iota // タスクリストのチェックボックス（FR-141）
    RefCell                    // 表のセル（FR-142。修飾の対象にもなる。FR-146）
    RefCode                    // フェンス付きのコードブロック（FR-145。4.75.0）
    RefMermaid                 // Mermaid のブロックと Mermaid ファイル（FR-145）
    RefPlantUML                // PlantUML のブロックと PlantUML ファイルの図（FR-145）
    RefText                    // 修飾できるブロック（FR-146, FR-147）
)

// Ref はフロントエンドから届く書き換え対象の指示を解いたもの（IMP-120 の目印の値）。
type Ref struct {
    Key   string  // 描画の鍵（Document.RefKey と照合する）
    Kind  RefKind
    Index int     // RefTask: 文書の中で何番目のタスクか（0 起点）。RefCell: 何番目の表か。
                  // RefCode / RefMermaid / RefPlantUML / RefText: その種類の中で何番目か
    Row   int     // RefCell: 行（0 が見出し行、1 以降が本体の行。ソース上の順）
    Col   int     // RefCell: 列（0 起点）
}

var (
    ErrBadRef      = errors.New("malformed edit reference")
    ErrRefNotFound = errors.New("edit target not found")
    ErrNotEditable  = errors.New("document is not editable")
    ErrChanged      = errors.New("file changed on disk")
    ErrRejected     = errors.New("edit would change the document structure") // 確かめで拒んだ（FR-143。edit-rejected）
    ErrLocked       = errors.New("code block cannot be edited")              // 前置きが行ごとに違う（FR-145。code-locked）
    ErrCannotFormat = errors.New("formatting cannot be applied")             // 修飾を分けられない・確かめで拒んだ（FR-146。format-rejected）
)

// ParseRef は data-ref 属性の値（IMP-120）を解く。形が違えば ErrBadRef。
// 書き込みの対象（task / cell / code / mermaid / plantuml / text）以外の目印（table。IMP-120）も ErrBadRef とする。
// 数は IMP-116 の正規表現と同じ形（符号なしの 10 進数）だけを受け付け、int に収まらなければ ErrBadRef。
func ParseRef(s string) (Ref, error)

// Patch は生バイト列の 1 か所の置き換え。取り消し（IMP-108）にもそのまま使う。
type Patch struct {
    Offset int    // 生バイト列の上の位置
    Old    []byte // 置き換える前のバイト列
    New    []byte // 置き換えた後のバイト列
}

// Apply は raw に p を適用した新しいバイト列を返す。
// raw[Offset:Offset+len(Old)] が Old と一致しなければ ErrChanged を返す。
func (p Patch) Apply(raw []byte) ([]byte, error)

// Inverse は p を打ち消す Patch を返す（Old と New を入れ替える）。
func (p Patch) Inverse() Patch

// PlanTask は raw の中で ref のチェックボックスを checked にする Patch を返す（FR-141）。
// 既にその状態なら changed は false。
func PlanTask(r *renderer.Renderer, raw []byte, ref Ref, checked bool) (p Patch, changed bool, err error)

// PlanCell は raw の中で ref のセルを text に置き換える Patch を返す（FR-142）。
func PlanCell(r *renderer.Renderer, raw []byte, ref Ref, text string) (p Patch, changed bool, err error)

// CellSource は raw の中の ref のセルのソースを返す（FR-142 の編集欄の初期値）。
func CellSource(r *renderer.Renderer, raw []byte, ref Ref) (string, error)

// コードブロックと図（PlanCode / CodeSource）は IMP-122、修飾（PlanFormat）は IMP-124。
```

**処理の順序を固定する。**

1. `raw` を IMP-103 と同じ規則で正規化し、**正規化後の位置から生バイト列の位置への対応表**を作る。対応が崩れるのは次の 2 つだけである。
   - 先頭の UTF-8 BOM（3 バイト）の除去
   - `CRLF` と単独の `CR` の `LF` への置き換え（除いた `CR` の位置を昇順に持ち、二分探索で数える）
   
   **不正なバイト列の置き換え（IMP-103 の 3）が起きる場合は `ErrNotEditable` を返す。** 置き換えは長さを変え、位置を対応させられない。FR-140 はこの文書で編集モードを始めさせないが、**書き込みの直前に読み直した内容で改めて確かめる**（その間に外部で壊されうる）。
2. 正規化後のテキストを `r.Locate`（IMP-121）に渡し、ソース上の位置を得る。**`Render` と `Locate` は同じ数え方を共有しており**、`Ref.Index` / `Row` / `Col` は描画時の目印（IMP-120）と一致する。
3. 位置を 1 の対応表で生バイト列へ移し、`Patch` を組み立てる。**生バイト列のそれ以外の部分には触れない**（FR-143 の「1 バイトも変えない」）。

**チェックボックス**（`PlanTask`）

- 置き換えるのは括弧の中の 1 バイトだけとする。`checked` が真なら `x`、偽なら半角空白を書く。
- 現在の文字が `x` / `X` なら「オン」、それ以外（goldmark のタスクの正規表現の `\s` に当たる半角空白・タブ・改ページ）なら「オフ」とみなす。**`X` をオフにすると半角空白になる**（FR-141）。
- 指示された状態と既に同じなら `changed` を偽にする。**反転ではなく「この状態にする」で受け取る**のは、同じ指示が 2 度届いても元へ戻らないようにするためである。
- **求めた位置の前後が `[` と `]` であり、中の 1 バイトが半角空白・タブ・改ページ・`x`・`X` のいずれかであることを確かめる。** 違えば `Patch` を返さず `ErrNotEditable` とする。位置の求め方（IMP-121）が構文木と食い違ったときに、別のバイトを書き換えないための防御である（リストの字下げにタブを含むと、goldmark は字下げを桁に換算して扱うため、行の中の位置の数え方がずれうる）。

**セル**（`PlanCell`）

- 入力は次の順で整える。
  1. 改行（`\r\n` / `\r` / `\n`）を半角空白 1 つへ置き換える（FR-142）。
  2. 前後の空白を除く。
  3. **直前の 1 文字が `\` でない `|`** を `\|` にする。**`\` の個数は数えない**——goldmark v1.8.5 の表の解析は、`|` の直前の 1 文字だけを見て区切りかどうかを決める（`extension/table.go`）。`\` を数えて「偶数ならエスケープされていない」とすると、パーサと食い違う。
  4. **新しい内容が `\` で終わり、セルの内容の直後が区切りの `|`（間に空白が無い）なら、内容の末尾に半角空白を 1 つ足す。** そのままでは `\` が区切りの `|` をエスケープし、隣のセルと結合して表の形が変わる。空白があれば何も足さない。
- 置き換える範囲はセルの**内容**（前後の空白を除いた範囲。IMP-121 の `Content`）とする。区切りの `|` と、その内側の前後の空白は残す（FR-142）。
- **空のセルに書き込むときは、区切りの間の空白（`Between`）の 1 文字目の直後へ入れる。** 空白が無ければ前の区切りの直後へ入れる。`|  |` は `| x |` になり、区切りの間の空白の数は変わらない。
- 整えた結果が元の内容と同じなら `changed` を偽にする（FR-142 の「内容が変わっていなければ書き込まない」）。
- **組み立てた `Patch` を適用した生バイト列を、1 と同じ規則で正規化してから `r.Locate` で解き直し**、次のすべてを確かめる。どれかが違えば `Patch` を返さず `ErrRejected` とする（書き込まない。IMP-195 が `edit-rejected` で通知する）。**エスケープの規則はパーサの実装に依存し、手で書いた規則だけでは形が変わらないことを保証できない**（4.43.0 で、`\` の個数を数える規則が goldmark と食い違っていた）。
  - 文書の中の表の数とタスクの数が変わらない（**見出し行の書き換えで表として解析されなくなると、後ろの表とセルの番号がずれる**）
  - その表の行数と、各行のセルの数が変わらない
  - 書き換えたセル以外のセルについて、`Content` の区間が指すバイト列が変わらない（**区間の位置そのものは、書き換えたセルより後ろでずれる。** 比べるのは位置ではなく中身である）
  - 書き換えたセルの `Content` の区間が指すバイト列が、整えた文字列（4 で足した空白を除く）と一致する
- **確かめで拒んだときは `ErrRejected` とする**（4.75.0。v1.1.0 は `ErrNotEditable` を返し、`Failed to save: <path>` で知らせていた）。拒んだのは書き込めなかったからではなく、表の形が変わるためである（IMP-315 の `edit-rejected`）。**`ErrNotEditable` は、不正なバイト列（上の 1）と、求めた位置の中身が食い違ったとき（下のチェックボックス）だけに使う。**
- 表の番号・行・列が文書の範囲を超える場合は `ErrRefNotFound` とする。panic しない。
- **ソース上に存在しないセル**（IMP-121 で `nil`）は `ErrRefNotFound` とする（FR-142）。

> [!IMPORTANT]
> **位置は書き込みのたびに、その時点のファイルの内容から求め直す。** 描画時に求めた位置を持ち回らない。**続けて編集すると前の書き込みで後ろのセルの位置がずれる**（FR-143）。`Ref` が持つのは「何番目か」だけであり、構造を変えない編集（[2.14](02-functional.md) の IMPORTANT）では何番目かは変わらない（セル・コードブロックと図・修飾の確かめ（本項・IMP-122・IMP-124）が、それを書き込みのたびに保証する）。

### IMP-107: 置き換えによる書き込み **MUST**

FR-143 / NFR-031 / NFR-033 を実装する。配置は `internal/document/write.go`。

```go
// Replace は path の実体を data で置き換える。
// 返しうるエラー: ErrNotFound / ErrPermission / 書き込み時のエラー
func Replace(path string, data []byte) error
```

処理の順序を固定する。**どこで失敗しても、元のファイルは変わらず、一時ファイルは残らない。**

| # | 処理 | 失敗したとき |
| --- | --- | --- |
| 1 | `filepath.EvalSymlinks(path)` で実体のパスを得る。以降はすべて実体に対して行う | `ErrNotFound` |
| 2 | `os.Stat` で権限ビットを控える | `ErrNotFound` |
| 3 | **`os.OpenFile(実体, os.O_WRONLY, 0)` で開けるかを確かめ、すぐ閉じる**（切り詰めない） | `ErrPermission` |
| 4 | `os.CreateTemp(filepath.Dir(実体), "." + ベース名 + ".markview-*.tmp")` | `ErrPermission`（ディレクトリに書けない） |
| 5 | 一時ファイルへ `data` を書き、`Sync` して閉じる | 一時ファイルを消して返す |
| 6 | Windows 以外では、一時ファイルの権限ビットを 2 で控えた値にする（`Chmod`） | 一時ファイルを消して返す |
| 7 | `os.Rename(一時ファイル, 実体)` | 一時ファイルを消して返す |

- **表の番兵は、その段階の既定である。** どの段階でも、失敗の原因が「存在しない」（`fs.ErrNotExist`）なら `ErrNotFound`、「権限が無い」（`fs.ErrPermission`）なら `ErrPermission` とし、それ以外は表の番兵で包む（4.67.0）。**1 で上位のディレクトリを辿れない場合も `ErrPermission` になる**——`ErrNotFound` にすると、利用者への通知（FR-143）が実態と違う。IMP-102 の `classifyError` と同じ分け方である。実体が通常のファイルでなければ `ErrNotFound` とする。
- **3 を省かない。** ディレクトリに書き込めればリネームは成功するため、**読み取り専用のファイルを置き換えてしまう**（FR-143 の「元のファイルが書き込み可能でなければ置き換えない」）。Windows でも、読み取り専用属性とアクセス制御の両方をこの 1 回で確かめられる。
- **一時ファイルの名前を `.` で始める。** ファイルツリーは `.` で始まる名前を出さない（FR-031）。書き込みの途中でツリーを読み直しても現れない。
- **一時ファイルを実体と同じディレクトリに置く。** 別のボリュームへのリネームは置き換えにならない（コピーと削除になり、途中の状態が残りうる）。`%TEMP%` に置かない（NFR-033 の例外の範囲。FR-143）。
- **リンクそのものを置き換えない。** 1 で実体を得てから 7 で実体へリネームするため、シンボリックリンクは残る（FR-143）。
- 監視（IMP-141）は実体のディレクトリを見ており、リネームは `Rename` / `Create` として届く。IMP-142 が 150 ms 後に存在を確かめて `Modified` にするため、**削除とは誤認しない**（FR-140 の「保存に伴う一時的な削除」）。
- ハードリンクが切れること、Windows でファイル個別に設定したアクセス制御が引き継がれないことは許容する（FR-143）。

### IMP-108: 取り消し履歴 **MUST**

FR-144 を実装する。配置は `internal/document/editlog.go`。**ファイルに触れない純粋なデータ構造**とし、単体テストでファイルを要さない。

```go
const MaxUndo = 100 // FR-144

// EditLog は編集モードの 1 回分の書き込みの記録。編集モードを始めたときに作り、
// 終えたときに捨てる（FR-140）。
type EditLog struct {
    known [sha256.Size]byte // 把握している内容の要約（FR-143）
    undo  []Patch           // 古い順。末尾が直前の書き込み
    redo  []Patch           // 取り消した書き込み。末尾が直前に取り消したもの
}

func NewEditLog(known [sha256.Size]byte) *EditLog

// Known は把握している内容の要約を返す。
func (l *EditLog) Known() [sha256.Size]byte

// Record は書き込みを記録する。known を after にし、redo を捨て、undo が MaxUndo を超えたら先頭を捨てる。
func (l *EditLog) Record(p Patch, after [sha256.Size]byte)

// Undo は取り消しに使う Patch（直前の書き込みの Inverse）を返す。無ければ ok は偽。
// 返しただけでは履歴を動かさない。書き込みに成功してから CommitUndo を呼ぶ。
// CommitUndo は known を after にし、undo の末尾を redo へ移す。
func (l *EditLog) Undo() (p Patch, ok bool)
func (l *EditLog) CommitUndo(after [sha256.Size]byte)

// Redo / CommitRedo は Undo / CommitUndo の逆向き。
func (l *EditLog) Redo() (p Patch, ok bool)
func (l *EditLog) CommitRedo(after [sha256.Size]byte)
```

- **履歴に文書全体の写しを持たない。** `Patch` は書き換えた箇所の前後の文字列だけを持ち、内容の一致は要約で確かめる（FR-144, NFR-020）。50 MB の文書でも、1 回の記録は書き換えた範囲の大きさで済む——チェックボックスとセルは数文字、修飾は最初の変更から最後の変更まで、**コードブロックはそのブロックの大きさ、Mermaid ファイルはファイル全体**である（FR-144 の例外。IMP-122, IMP-124。4.76.0）。
- **「取り出す」と「確定する」を分ける。** 書き込みに失敗した取り消しで履歴が動くと、次の `Ctrl+Z` が 1 つ先を取り消してしまう。
- `Undo` が返す `Patch` を適用できるのは、ファイルの内容が `Known()` と一致しているときだけである。**位置（`Offset`）は直前の書き込みの後の内容に対するもの**であり、他者の変更の上に適用すると別の箇所を壊す。一致を確かめるのは呼び出し側（IMP-109 の `Plan`）である。

### IMP-109: 編集モードの判断 **MUST**

FR-140〜FR-147 / NFR-030 を実装する。配置は `internal/document/editsession.go`。

**編集モードの状態と、書き込んでよいかの判断をすべてここに置く。** ファイルにも Wails にも触れず、錠も持たない。**バインドメソッドの側（IMP-195）は、錠を取り、ファイルを読み書きし、イベントを送るだけにする。** 判断を `desktop` に置くと単体テストの対象外になり（UT-002, IMP-012）、**書き込みの安全性を担う部分が検証されない。**

```go
// ErrStale は、指示を作った描画の後に再描画が起きていたことを表す（FR-143）。通知しない。
var ErrStale = errors.New("edit instruction is stale")

// OpKind は書き込みの種類。
type OpKind int

const (
    OpTask OpKind = iota // SetTask（FR-141）
    OpCell               // SetCell（FR-142）
    OpUndo               // UndoEdit（FR-144）
    OpRedo               // RedoEdit（FR-144）
    OpCode               // SetCode（FR-145。4.75.0）
    OpFormat             // ApplyFormat（FR-146, FR-147。4.75.0）
)

// Op はフロントエンドから届いた 1 回の書き込みの指示。
type Op struct {
    Kind    OpKind
    Ref     string   // OpTask / OpCell / OpCode / OpFormat: data-ref の値そのもの（IMP-316）
    Checked bool     // OpTask
    Text    string   // OpCell / OpCode
    Format  FormatOp // OpFormat（IMP-124）
}

// EditSession は編集モードの状態（FR-140）。ゼロ値は「編集モードでない」。
type EditSession struct {
    on      bool
    removed bool     // 監視が表示中のファイルの削除を送った。次に読み込めたら偽に戻す
    log     *EditLog // on の間だけ非 nil（IMP-108）
    seq     uint64   // 状態を変えうる呼び出しのたびに増やす（IMP-302 の EditSeq）
}

func (s *EditSession) On() bool
func (s *EditSession) Seq() uint64

// CanStart は doc で編集モードを始められるかを返す（DocumentDTO.Editable）。
// showing は、画面が doc を表示していること（状態画面・文書未表示でない。IMP-190 の showing）。
func (s *EditSession) CanStart(doc *Document, showing bool) bool

// Start は CanStart が真なら編集モードを始めて真を返す。偽なら何も変えない。
func (s *EditSession) Start(doc *Document, showing bool) bool

// Stop は編集モードを終え、履歴を捨てる。
func (s *EditSession) Stop()

// Loaded は文書を開く処理（IMP-192）で読み込みに成功したときに呼ぶ。same は SameDocument。
func (s *EditSession) Loaded(doc *Document, same bool)

// Left は状態画面を出したときに呼ぶ（文書の切り替えとして終える。IMP-192）。
func (s *EditSession) Left()

// Removed は監視が表示中のファイルの削除を送ったときに呼ぶ（IMP-195, IMP-320）。
func (s *EditSession) Removed()

// Deleted は削除の印が立っているか（Removed の後、まだ読み込めていないか）を返す（IMP-192）。
func (s *EditSession) Deleted() bool

// Discard は取り消し・やり直しの履歴だけを捨てる。把握している内容（Known）は変えない（IMP-195 の 4）。
func (s *EditSession) Discard()

// Check は、指示がいまの描画に対して有効かを確かめる（IMP-195 の 1, 2）。ファイルを読む前に呼ぶ。
func (s *EditSession) Check(doc *Document, showing bool, op Op) error

// Plan は読んだ生バイト列 raw に対する Patch を作る（IMP-195 の 4, 5）。kind は表示中の文書の種類。
// widened は、手書きの取り消し線を全体で外した（FR-146。IMP-124）。
func (s *EditSession) Plan(r *renderer.Renderer, raw []byte, kind mdfile.Kind, op Op) (p Patch, changed, widened bool, err error)

// Commit は書き込みに成功した後に呼び、履歴を確定する（IMP-195 の 7）。after は書き込んだ内容。
func (s *EditSession) Commit(op Op, p Patch, after []byte)

// CellSource は raw の中の ref のセルのソースを返す（GetCellSource。IMP-195）。
func (s *EditSession) CellSource(r *renderer.Renderer, raw []byte, ref string) (string, error)

// CodeSource は raw の中の ref のコードブロック・図のソースを返す（GetCodeSource。IMP-122, IMP-195）。
func (s *EditSession) CodeSource(r *renderer.Renderer, raw []byte, kind mdfile.Kind, ref string) (string, error)
```

**開始できる条件**（`CanStart`。FR-140 の表）

次のすべてを満たすときだけ真とする。`doc` が nil でない、`showing` が真、`doc.Editable()` が真（画像ファイルでは偽。IMP-100）、`removed` が偽。**`Start` はこの関数だけで判断する**——`DocumentDTO.Editable` とボタンの淡色（UI-021）と、開始の可否を 1 つの式に揃える。

**読み込みの後の扱い**（`Loaded` / `Left`。FR-140, FR-144）

| 呼び出し | 条件 | 編集モード | 取り消し履歴 |
| --- | --- | --- | --- |
| `Loaded` | `on` が偽 | 変えない（始めない） | —（持っていない） |
| `Left` | 状態画面を出した（文書の切り替え） | **終える** | 捨てる |
| `Loaded` | `same` が偽（文書の切り替え） | **終える** | 捨てる |
| `Loaded` | `same` が真で、`doc.Editable()` が偽（読み直したら不正なバイト列を含んでいた） | **終える** | 捨てる |
| `Loaded` | `same` が真で、`doc.Digest` が `log.Known()` と一致しない | 保つ | **捨てて、新しい `Digest` で作り直す**（外部で変更された。FR-144） |
| `Loaded` | `same` が真で、一致する | 保つ | 保つ |

- 表は上から順に当てはめる。
- `Loaded` は `removed` を偽に戻す（読み込めた以上、ファイルはある）。`Left` は戻さない。
- `Start` は編集モードを始めるとき、`NewEditLog(doc.Digest)` で履歴を作る（IMP-108）。
- **表示を変えない失敗**（`not-found` など。IMP-192 の `target` の表）では、どちらも呼ばない。

**書き込みの判断**（`Check` / `Plan`）

| # | 確かめること | 当てはまらないとき |
| --- | --- | --- |
| 1 | `on` が真、`doc` が nil でない、`showing` が真 | `ErrStale` |
| 2 | `OpTask` / `OpCell` / `OpCode` / `OpFormat` なら、`ParseRef` で解け、**`Ref.Key` が `doc.RefKey` と一致し、`Ref.Kind` が `Op.Kind` に合う**（下の表。取り消し・やり直しは鍵を持たないため省く）。**`OpFormat` は、さらに `doc.Kind` が Markdown で、`doc.TextRefs` が真であること**（修飾の目印を付けた描画に対する指示であること。IMP-120） | `ErrStale`——指示を作った描画の後に、文書の切り替えか、自分の書き込み以外による再描画が起きている（FR-143）。**種類が食い違う指示（セルの目印でタスクを書き換える）も拒む** |
| 4 | **`raw` の SHA-256 が `log.Known()` と一致する** | `ErrChanged`（書き込まない。FR-143） |
| 5 | `OpTask` は `PlanTask`、`OpCell` は `PlanCell`、**`OpCode` は `PlanCode`（IMP-122）、`OpFormat` は `PlanFormat`（IMP-124）**、`OpUndo` は `log.Undo()`、`OpRedo` は `log.Redo()` | `ErrBadRef` / `ErrRefNotFound` は `ErrStale`。**`ErrNotEditable` / `ErrRejected` / `ErrLocked` / `ErrCannotFormat` はそのまま返す**（IMP-195 がそれぞれ `edit-failed` / `edit-rejected` / `code-locked` / `format-rejected` で通知する）。変わらない・履歴が空なら `changed` が偽 |

**指示と目印の種類の対応**（2 の「合う」。4.75.0）

| `Op.Kind` | 受け付ける `Ref.Kind` |
| --- | --- |
| `OpTask` | `RefTask` |
| `OpCell` | `RefCell` |
| `OpCode` | `RefCode` / `RefMermaid` / `RefPlantUML` |
| `OpFormat` | `RefText` / `RefCell`（表のセルも修飾できる。FR-146） |

- 番号は IMP-195 の手順の番号と揃えている（3 のファイルの読み込みは IMP-195 が行う）。
- **`Commit` は、`OpTask` / `OpCell` / `OpCode` / `OpFormat` なら `log.Record`、`OpUndo` なら `log.CommitUndo`、`OpRedo` なら `log.CommitRedo` を、`after` の SHA-256 で呼ぶ。** 書き込みに失敗したときは呼ばない（IMP-108 の「取り出す」と「確定する」を分ける）。
- **5 で `ErrNotEditable` / `ErrRejected` / `ErrLocked` / `ErrCannotFormat` を `ErrStale` にしない。** 確かめで拒んだのは、書き込む内容が構造を変える場合であり、古い指示ではない。黙って戻すと、**確定した入力が通知も無く消える**（FR-142, FR-145, FR-110）。
- **`OpFormat` の見える文字の食い違い（IMP-124 の 2）は `ErrRefNotFound` で返り、`ErrStale` になる**（FR-146 の「古い指示として扱う」。通知しない）。
- `CodeSource` は `CellSource` と同じ確かめ（`ErrChanged`）を行い、`document.CodeSource`（IMP-122）の結果を返す。`ErrBadRef` / `ErrRefNotFound` / `ErrNotEditable` は `ErrStale` とする。**`ErrLocked` はそのまま返す**（IMP-195 が `code-locked` の警告を出す。FR-145）。呼ぶ前に `Check`（`OpCode`）を通す。
- `CellSource` は 4 と同じ確かめを行い（一致しなければ `ErrChanged`）、`document.CellSource` の結果を返す。`ErrBadRef` / `ErrRefNotFound` / `ErrNotEditable` は `ErrStale` とする（編集欄が開かないだけで済み、書き込みではないため通知しない）。呼ぶ前に `Check`（`OpCell`）を通す。
- **`seq` は、`Stop` / `Loaded` / `Left` / `Removed` のたびと、`Start` が編集モードを始めたとき（偽を返した `Start` と、既に編集モードの間の `Start` を除く）に 1 増やす。** フロントエンドは、先に届いた新しい値を後から届いた古い値で上書きしない（IMP-260）。
- **`Removed` は編集モードを終え、履歴を捨て、削除の印を立てる**（FR-140 の表の「削除された」）。
- **`Discard` は `seq` を増やさない**（編集モードの状態を変えない）。**把握している内容を作り直さない**——作り直すと、読み直しに失敗して描画が古いまま（鍵も古いまま）なのに、次の書き込みが 4 を通ってしまう。次に読み込めたときに `Loaded` が作り直す。
- **既に編集モードの間に `Start` を呼んでも、何も変えずに真を返す**（履歴を保つ。ボタンの二重押しで履歴が消えないようにする）。

### IMP-122: コードブロックと図の書き換え **MUST**

FR-145 を実装する（4.75.0）。配置は `internal/document/editcode.go`（**図のファイルの書き換えは `editdiagram.go`**。400 行の目安（IMP-011）により分けた。4.87.0）。**位置は `renderer` が求め（IMP-121, IMP-126）、ここは生バイト列への対応・`Patch` の組み立て・確かめだけを行う**（IMP-106 と同じ分け方）。

```go
// CodeSource は ref のコードブロック・図の、書き換える範囲のソースを返す（編集欄の初期値。FR-145）。
// 前置き（引用の `> ` やリストの字下げ）は除き、改行は LF にそろえる。前置きが行ごとに違えば ErrLocked。
func CodeSource(r *renderer.Renderer, raw []byte, kind mdfile.Kind, ref Ref) (string, error)

// PlanCode は ref のコードブロック・図の中身を text に置き換える Patch を返す（FR-145）。
func PlanCode(r *renderer.Renderer, raw []byte, kind mdfile.Kind, ref Ref, text string) (p Patch, changed bool, err error)
```

**対象と書き換える範囲**（FR-145 の表）

| 文書の種類（`kind`） | 目印（IMP-120） | 位置を求めるもの | 書き換える範囲 |
| --- | --- | --- | --- |
| Markdown | `code:<n>` / `mermaid:<n>` / `plantuml:<n>` | `r.Locate` の `Codes` / `Mermaids` / `PlantUMLs`（IMP-121。どれも `FenceLocation`） | 中身の行。**囲いを延ばすときは、開きの囲いの文字の並びから閉じの囲いの文字の並びまで**（下の 5） |
| PlantUML ファイル | `plantuml:<n>` | `renderer.SplitPlantUML`（IMP-126） | `@start…` の行の頭から `@end…` の行の終わり（改行の手前）まで。`@end…` の行が無い図は、ファイルの終わりまで（MD-086 の 4） |
| Mermaid ファイル | `mermaid:0` | 求めない | ファイル全体（先頭の BOM を除く。末尾の改行を含む） |

**編集欄の値（`CodeSource`）**（4.77.0 で明記した。利用者の判断）

| 文書の種類 | 返す値（LF にそろえる） |
| --- | --- |
| Markdown | **中身の各行（`Lines`。前置きの後ろから改行の手前まで）を LF でつないだもの。最後の行の改行を含めない**（` ```go ` / `a` / `b` / ` ``` ` なら `"a\nb"`）。中身の行が無いブロックと、空の行 1 つだけのブロックは、どちらも空文字になる |
| PlantUML ファイル | 図の `Span` の中身（`@start…` の行から `@end…` の行の改行の手前まで） |
| Mermaid ファイル | ファイル全体（先頭の BOM を除き、末尾の改行を含む） |

- **Markdown の中のブロックでは、書き戻すときに各行の後ろに改行を付ける**（下の 4）。編集欄の値に最後の改行を含めると、何も変えずに確定しただけで最後に空の行が 1 つ増える。**含めないので、そのまま確定すれば 8 で「変わらない」になる。** 利用者が入力の末尾に改行を書けば、最後の空の行として残る（下の 3）。

**Markdown の中のブロック**

1. IMP-106 の 1 と同じく正規化し、位置の対応表を作る。不正なバイト列があれば `ErrNotEditable`。
2. `r.Locate` で、目印の種類と番号に当たる `FenceLocation`（IMP-121）を得る。番号が範囲を超えれば `ErrRefNotFound`。**`Locked` が真なら `ErrLocked`**（前置きが行ごとに違うブロック。下の「前置き」）。
3. **入力を行に分ける。** 改行（`\r\n` / `\r` / `\n`）で区切る。**入力の末尾の改行は、最後の空の行として書いたとおりに残す。**
4. **各行の前にそのブロックの前置きを付け、各行の後ろにそのブロックの改行コードを付ける**（最後の行も。範囲の最後に改行が無かったときは 7。4.77.0 で「つなぐ」から言い換えた）。改行コードは開きの囲いの行の終わりのもの（CRLF / CR / LF）とし、無ければ（文書の最後の行）LF とする（FR-145）。**空の行には、前置きから末尾の半角空白とタブを除いたものを付ける**（引用の中の空の行は `>` だけになる。行の末尾に空白を残さない）。
5. **囲いの延長**（FR-145。利用者の判断）: 入力の各行の先頭の空白を除いた後ろが、囲いと同じ文字（`` ` `` か `~`）で始まる行のうち、**先頭の空白が半角空白 3 つまでの行と、先頭の空白にタブを含む行**について、その文字の並びの長さを数える（**タブを含む行も数えるのは、引用やリストの中では前置きの後ろのタブが 4 桁にならず、囲いを閉じうるためである**。`> ` の後ろのタブは 2 桁。最上位では延ばしすぎになるが害は無い。4.78.0）。**最も長い並びが囲いの長さ以上なら、開きと閉じの囲いをその長さより 1 つ長くする。** 閉じの囲いが無いブロック（文書や引用の終わりで閉じたもの）は、開きの囲いだけを延ばす。**行の途中の並びは囲いを閉じない（CommonMark）ため数えない。**
6. **言語指定（info string）は変えない**（FR-145）。延ばすときも、開きの囲いの文字の並びの後ろ（言語指定と改行）はそのまま残す。
7. **`Patch` は 1 つにする。** 延ばさないときは中身の行の範囲（`Body`。IMP-121）、延ばすときは開きの囲いの文字の並びの頭から閉じの囲いの文字の並びの終わりまで（閉じが無ければ中身の行の終わりまで）。**延ばすときは、閉じの囲いの行の頭から閉じの囲いの文字の並びの手前まで（その行の前置き。`Body` の終わりから `Close` の頭まで）を、元のバイト列のまま書き戻してから延ばした閉じの囲いを書く**——範囲に入るため、書き戻さないと引用やリストの中で閉じの囲いが前置きを失い、9 の確かめで拒まれる（4.76.0）。**範囲の最後のバイトが改行でないとき（閉じの囲いが無く、文書の最後の中身の行に改行が無いとき）は、新しい中身の最後の行にも改行を付けない。** 範囲が空のとき（中身の行が無いブロック）は、最後の行にも改行を付ける（4.77.0 で言い換えた）。**ただし開きの囲いの行に改行が無い（文書が開きの囲いの行で終わる）ときは、書き込む中身の前に LF を 1 つ足し、最後の行には改行を付けない**（4.77.0。利用者の判断）。そのまま差し込むと、中身の 1 行目が言語指定とつながる。**どちらの場合も、最後の行が空なら（入力の末尾に改行があったら）改行を付ける**（付けないと空の行が消え、9 の確かめで拒まれる。4.78.0。利用者の判断。UT-118 のケース 27）。 範囲の外のバイト列には触れない（FR-143）。
8. 新しい中身（LF にそろえたもの）が `CodeSource` の値と同じなら `changed` を偽にする（FR-145 の「内容が変わっていなければ書き込まない」）。
9. **確かめ**（FR-143）: `Patch` を適用した生バイト列を正規化して `r.Locate` で解き直し、次のすべてを確かめる。どれかが違えば `ErrRejected`（書き込まない。IMP-195 が `edit-rejected` で通知する）。
   - **ブロックの並び**（`Locations.Blocks`。構文木の順のブロックの種類）が変わらない
   - タスクの数、表の数と各表の形、コードブロック・Mermaid・PlantUML のブロックの数、修飾できるブロックの数（`Texts`）が変わらない
   - **書き換えたブロックの中身**（前置きを除き、LF にそろえたもの）が入力と一致し、言語指定が変わらない
   - **他のコードブロックと図の中身**が変わらない（位置ではなく中身を比べる。IMP-106 のセルと同じ）

**前置き**（FR-145。IMP-121 が求め、ここで使う）

- **中身の各行について、行の頭から中身の区間の始まりまでのバイト列を、その行の前置きとする。** goldmark の中身の行の区間は、引用の `> ` やリストの字下げの後ろから始まる（実測。実現性検討の付録 A.1）。
- **最初の空でない中身の行の前置きを、そのブロックの前置きとする**（4.78.0。1 行目が空の行だと、その前置きは末尾の空白の無い `>` になり、2 行目と違って `Locked` になっていた）。**中身の行が無いブロックと、空の行だけのブロックは、閉じの囲いの行の頭から囲いの文字の手前までを前置きとする**（開きの囲いの行は、リストの中ではリストの記号 `- ` を含み、前置きにすると項目が 2 つに割れる）。**閉じの囲いも無ければ、開きの囲いの行の頭から囲いの文字の手前までとし、そこに半角空白・タブ・`>` 以外の文字（リストの記号）があれば `Locked` とする**（4.78.0。UT-118 のケース 30・31、UT-222 のケース 17）。
- **次のどれかに当たるブロックは `Locked` とし、編集できないものとする**（FR-145。編集のボタンを淡色にし、ダブルクリックでは `code-locked` の警告を出す。IMP-264）。
  - 前置きがブロックの前置きと違う行がある。**ただし空の行は、前置きから末尾の半角空白とタブを除いたものと一致してもよい**（引用の中の `>` だけの行、リストの中の何も無い行。どちらもよく書かれ、書き込みの規則は上の 4 で決まっている）
  - 中身の行の区間がタブの途中から始まる（goldmark の `Segment.Padding` が 0 でない。番号付きリストの中でタブを使った場合など。実測）
  - **前置きを決められない**: 中身の行も閉じの囲いも無く、開きの囲いの行の頭から囲いの文字の手前までに、半角空白・タブ・`>` 以外の文字（リストの記号）がある（上の 2 つ目の箇条。UT-118 のケース 30。4.78.0 で決め、4.81.0 でこの一覧に書き写した）
- **書き換えの規則を、この 3 つ以上に手で増やさない。** 増やすほどパーサと食い違いうる（IMP-106 のセルの `\` の件と同じ）。食い違いは 9 の確かめが捕まえる。

**PlantUML ファイルの図**（FR-025, MD-086）

- `renderer.SplitPlantUML`（IMP-126）で図に分け、`plantuml:<n>` の図の `Span` を書き換える範囲とする。**`@start…` と `@end…` の行も編集欄に出し、書き換えられる**（図の名前を直せるように。FR-145）。前置きは無い。
- 入力の改行は、ファイルの最初の改行コード（無ければ LF）にそろえる。
- **図の範囲が `@end…` の行の改行の手前で終わるとき（`Closed` が真）は、入力の末尾の改行をすべて除いて書く**（4.77.0。利用者の判断）。図の外に空行を足さず、下の確かめで拒まない。`@end…` の無い図（範囲がファイルの終わりまで）は、入力をそのまま書く。
- **確かめ**: 書き込んだ内容を `SplitPlantUML` で分け直し、**図の数が変わらない**こと（`@enduml` を消して 2 つの図がつながる、などを拒む）、書き換えた図の原文が書いた内容（上で末尾の改行を除いたもの）と一致すること（**比べるときは両方の末尾の改行を除く**——`@end…` を足す・消す書き込みでは、書いた後の図の範囲が改行の手前で終わるかが変わり、最後に `Enter` を押したかどうかで拒まれる。4.78.0。UT-119 のケース 26・27）、他の図の原文が変わらないことを確かめる。違えば `ErrRejected`。

**Mermaid ファイル**（FR-026, MD-087）

- 書き換える範囲はファイル全体（先頭の BOM を除く）であり、位置を求めない。編集欄には先頭の空行も含めて出す（MD-087 の 2。描画のときだけ除く）。
- 入力の改行は、ファイルの最初の改行コード（無ければ LF）にそろえる。
- **確かめ**: 入力が空白だけなら `ErrRejected`（図が無くなる。MD-087 の 3）。**Mermaid の構文は解かない**（AR-031）。構文の誤りは描画のときに処理系が示す（FR-023）。

- **図のファイルも、IMP-106 の 1 と同じく正規化して位置の対応表を作り、不正なバイト列があれば `ErrNotEditable` とする**（4.77.0 で明記した。PlantUML ファイルの切り分け（IMP-126）は正規化後のテキストで行い、BOM と CR の分を生バイト列の位置へ戻す）。FR-140 はこの文書で編集モードを始めさせないが、書き込みの直前に読み直した内容で改めて確かめる（Markdown と同じ）。

- **取り消しの記録は、書き換えた範囲の前後の文字列になる**（IMP-108 の `Patch`）。コードブロックと図の書き換えでは、1 回の記録がそのブロックの大きさになり、**Mermaid ファイルではファイル全体になる**（FR-144 の例外。NFR-020。上限は設けない）。
- 書き込みの指示は IMP-109 の `Check` と `Plan` を通る（鍵の照合と、把握している内容との一致）。

### IMP-124: 修飾の書き込み **MUST**

FR-146 / FR-147 を実装する（4.75.0）。配置は `internal/document/editformat.go`（**差し込み位置の決め方は `editformatpos.go`**。400 行の目安（IMP-011）により分けた。4.87.0）。**書くのは `<del>` と決めた色の `<span>` を差し込む・取り除くことだけであり、既存の記法を組み立て直さない**（2.14 の IMPORTANT の 4）。見える文字とソースの対応は `renderer` が求める（IMP-123）。

```go
// FormatStyle は修飾の操作。Strike と Color のどちらか一方だけを使う。
type FormatStyle struct {
    Strike bool   // 取り消し線のトグル（FR-146）
    Color  string // 文字色の名前（FR-147。renderer.TextColors の名前か "default"）
}

// FormatOp はフロントエンドから届く修飾の指示（FR-146, FR-147。IMP-316）。
type FormatOp struct {
    Start, End int    // ブロックの見える文字の中の、Unicode の符号位置での位置（半開区間）
    Visible    string // そのブロックの見える文字の全体（照合用。IMP-123, IMP-266）
    Style      FormatStyle
}

// PlanFormat は ref のブロックの選んだ文字に修飾を付ける・外す Patch を返す。
// widened は、手書きの取り消し線を全体で外した（FR-146。IMP-195 が format-widened の警告を出す）。
func PlanFormat(r *renderer.Renderer, raw []byte, ref Ref, op FormatOp) (p Patch, changed, widened bool, err error)
```

**読み取るもの**（選んだ文字が取り消し線か、何色か。FR-146, FR-147）

| 種類 | MarkView の形（一部だけ外す・替えるもできる） | 手書きのもの（全体を外すことだけできる） |
| --- | --- | --- |
| 取り消し線 | `<del>…</del>` | `~~…~~` / `~…~`（goldmark の `Strikethrough`）、`<s>…</s>`、`<strike>…</strike>` |
| 文字色 | `<span style="color: <一覧の値>">…</span>`（`style` が `color` だけで、値が `renderer.TextColors` にあるもの。サニタイズが通す揺れ（大文字と小文字・コロンの後の空白の有無）を含む。手で書いたものも同じに扱う。FR-147） | 無い（一覧に無い色・他の `style` はサニタイズで落ちる。色として読まない） |

- **生 HTML のタグの対は、同じブロックの中で開きと閉じがそろっているものだけを読む**（goldmark は生 HTML のタグを入れ子にせず、兄弟の `RawHTML` として並べる。実測）。対の見つけ方は IMP-123 の `Inlines`。

**手順**（順序を固定する）

1. IMP-106 の 1 と同じく正規化し、位置の対応表を作る。不正なバイト列があれば `ErrNotEditable`。
2. `r.Visible`（IMP-123）でブロックの見える文字を求める。**`op.Visible` と 1 文字でも違えば `ErrRefNotFound`**（IMP-109 が `ErrStale` にする。FR-146 の「古い指示として扱う」）。`Start` / `End` が範囲の外、または `Start >= End` なら `ErrRefNotFound`。**`Style` が「`Strike` が真で `Color` が空」か「`Strike` が偽で `Color` が `TextColors` の名前か `default`」のどちらでもなければ `ErrRefNotFound`**（フロントエンドが送るのはこの 2 つの形だけであり、ほかは信じない指示である。通知しない。IMP-300。4.76.0）。
3. **選択を整える**（FR-146）: 両端の空白を除く（除いて空になれば `changed` が偽）。インラインコード・数式・画像・脚注の参照・絵文字・自動リンクの一部だけにかかっていたら、その部品（`Atoms`）の全体まで広げる。**ただし脚注の戻りリンク（ソースに無い 2 文字。IMP-123）にかかっていたら、広げずに選択から外し、改めて両端の空白を除く**（外して空になれば `changed` が偽。4.78.0。利用者の判断。UT-120 のケース 23）。
4. **境界ごとに分ける**: 選択を、太字・斜体・取り消し線・リンク・既存のタグの対など、**インラインの要素の境界**で区切り、同じ要素の中に収まる「区間」の並びにする。差し込みは区間ごとに行うので、タグが既存の要素の境界をまたがない。**区間が太字・斜体・取り消し線・リンクの中身の全体と一致するときは、その要素の外側に差し込む**（`日本語<del>**重要**</del>です`。FR-146。日本語に接した区切り記号の内側にタグを置くと、太字などが効かなくなる）。**この取り消し線には MarkView の形の `<del>` も含め、色の `<span>` は含めない**——`<del>` の中身の全体に色を付けると `<span style="color: …"><del>…</del></span>`、色の `span` の中身の全体に取り消し線を付けても `<span style="color: …"><del>…</del></span>` になり、**入れ子の順が操作の順に関わらず「色が外、取り消し線が内」にそろう**（4.80.0。利用者の判断。UT-120 のケース 24・25）。**境界を作るのは、インラインコードと画像以外の要素である**（部品は 3 で全体まで広げてあり、区間の内側に丸ごと入る。`` <del>`code` と</del>文字 ``。UT-120 のケース 9）。**区間の端が部品のときは、その記号の外側に差し込む**——インラインコードのバッククォートと、**角括弧の自動リンクの `<` と `>`**（IMP-123 は自動リンクの中の文字だけに区間を付ける。内側に差し込むと自動リンクでなくなる。UT-120 のケース 26。4.87.0）。**改行で終わる区間の閉じは、次の行の前置き（引用の `> ` やリストの字下げ）の後ろに置く**——行の頭に置くとその行が前置きを失って遅延継続行になり（`>` が文字になる）、改行の手前に置くと改行が取り消し線の外に出る（UT-120 のケース 28。P24 の独立したレビューの指摘。4.87.0）。
5. **取り消し線**（`Strike`）: **選択（3 で整えたもの）の文字**（空白を除く）が**すべて取り消し線なら外し、そうでなければ付ける**（トグル。FR-146）。**判定は選択の全体で行い、差し込みは区間ごとに行う**（区間ごとに判定すると、一部が取り消し線の選択で、取り消し線の区間を外し、ほかに付けることになる。4.76.0）。
   - **付ける**: 取り消し線でない区間の両端に `<del>` と `</del>` を差し込む。既に取り消し線の区間は何もしない。
   - **外す（MarkView の形）**: 区間が `<del>` の中身の全体ならタグを取り除く。一部なら、区間の前に `</del>`、後ろに `<del>` を差し込んで分ける。**区間が `<del>` の先頭や末尾に接しているときは、空のタグを作らないよう、元のタグを取り除いて区間の反対側に差し込み直す。**
   - **外す（手書き）**: その取り消し線の**全体**の記号・タグを取り除く（記法を書き換えない）。選んだ範囲より広く外れたら `widened` を真にする（FR-146）。
6. **文字色**（`Color`）: 一覧の名前ならその色にし、`default` なら色を外す（FR-147）。
   - **付ける**: 色の無い区間の両端に `<span style="color: #xxxxxx">` と `</span>` を差し込む。**書く形を 1 つに固定する**（コロンの後に半角空白 1 つ、小文字の 16 進。値は一覧の Light の値）。
   - **替える・外す（MarkView の形の `span` の中）**: 区間が中身の全体なら、`style` の値を決めた形で書き直す（**ほかの属性は残す**。書き直すのは名前が `style` の属性であり、`data-style` のような名前や、ほかの属性の値の中の `style=` ではない——属性を頭から順に読む。外すときはタグを取り除く。UT-120 のケース 14・27。4.87.0）。一部なら、取り消し線と同じく閉じタグと開きタグを差し込んで分ける（端に接するときはタグを動かす）。
7. **分けられない場合**: 区間と、外したい（替えたい）タグの間に別の要素がある（`<del>あ**いう**え</del>` の「う」だけを外す。`</del>` を `**` の内側に置くと入れ子が壊れる）とき、**取り消し線と、色を外す操作は `ErrCannotFormat`**（書き込まない）。**色を替えるときだけは、区間を新しい `span` で囲む入れ子で替える**（内側の色が勝つ。FR-147）。**日本語に接した太字などの一部だけを選んだとき**（内側に差し込むと太字が効かなくなる）も、9 の確かめで `ErrCannotFormat` になる。
8. **差し込み・取り除きを 1 つの `Patch` にまとめる**（最初の変更から最後の変更まで）。その間のバイト列は変えない。何も変えない（同じ色を選ぶ・色の無い文字に `default` を選ぶ）なら `changed` を偽にする（FR-143 の「内容が変わらない書き込みはしない」）。
9. **確かめ**（FR-143）: `Patch` を適用した生バイト列を正規化して解き直し、次のすべてを確かめる。どれかが違えば `ErrCannotFormat`（IMP-195 が `format-rejected` で通知する）。
   - ブロックの並び（`Locations.Blocks`）と、タスク・表・コードブロック・図・修飾できるブロックの数が変わらない
   - **そのブロックの見える文字が 1 文字も変わらない**
   - そのブロックの**インラインの要素の並び**（太字・斜体・リンク・コード・画像など。MarkView の形の `<del>` と `<span>` を除く）と、**ハード改行の数**が変わらない——**太字などの記号の組み合わせが変わる**のを捕まえる（`**重要**と**重要**` の `要` に取り消し線を付けると、閉じの `**` の直前が `>` になって対にならず、2 つの太字が 1 つの外側の太字に組み替わる。見える文字は同じままなので、2 番目では捕まらない。UT-121 のケース 14）。**日本語に接した太字の記号が対にならず文字として現れる場合は、見える文字が変わり、2 番目でも捕まる**（UT-121 のケース 11・13。4.87.0 で直した。それまでは「見える文字は同じまま」と書いていたが、実測では `**` が文字として現れる）
   - **選んだ文字の取り消し線と色が意図どおりで、選んでいない文字の取り消し線と色が変わらない**
   - **手書きの取り消し線を全体で外したとき（5）は、外れた範囲の全体を「選んだ文字」とみなし、外した取り消し線（`~~` / `~` / `<s>` / `<strike>`）をインラインの要素の並びの比べから除く**（4.78.0。除かないと、並びが変わり選んでいない文字の取り消し線も変わるため、手書きの取り消し線を一度も外せない。下の例の `~~あいう~~`。FR-143, FR-146）

**例**（`<red>` は `<span style="color: #d1242f">`、`<blue>` は `<span style="color: #0969da">` の略。実現性検討の 7.6 で、MarkView の変換器で解き直して確かめたもの）

| 元のソース | 選んだ文字 | 操作 | 書き込み後 |
| --- | --- | --- | --- |
| `今日は会議です` | 会議 | 取り消し線 | `今日は<del>会議</del>です` |
| `**重要**な点` | 要な | 取り消し線 | `**重<del>要</del>**<del>な</del>点` |
| `<del>あいう</del>` | い | 取り消し線（外す） | `<del>あ</del>い<del>う</del>` |
| `<del>あいう</del>` | あ | 取り消し線（外す） | `あ<del>いう</del>` |
| `~~あいう~~` | い | 取り消し線（外す） | `あいう`（`widened`） |
| `<red>赤い文字</span>` | 文字 | 青 | `<red>赤い</span><blue>文字</span>` |
| `<red>赤い文字</span>` | 赤い | `default` | `赤い<red>文字</span>` |
| `` `code` と文字 `` | de と | 取り消し線 | `` <del>`code` と</del>文字 `` |
| `日本語**重要**です` | 重要 | 取り消し線 | `日本語<del>**重要**</del>です`（外側に差し込む） |
| `<del>項目</del>` | 項目 | 紫 | `<span style="color: #8250df"><del>項目</del></span>`（`<del>` の外側に差し込む。4.80.0。利用者の判断） |
| `<red>赤い</span>` | 赤い | 取り消し線 | `<red><del>赤い</del></span>`（色の `span` の内側に差し込む。4.80.0） |

- **取り消し線を付けた直後に続けて色を付けられるよう、書き込みは見える文字を変えない**（9 の 2 番目）。フロントエンドは再描画の後に同じ目印・同じ位置で選択を当て直す（IMP-265）。
- **目印の番号は変わらない。** 差し込むのはインラインのタグだけで、ブロックの数と種類は変わらない（9 の 1 番目）。自分の書き込みの直後の再描画で、それより前の描画の指示を有効なまま保てる（FR-143）。

## 11.3 renderer パッケージ（IMP-110 系）

責務: goldmark パイプラインの構築と実行。Markdown → サニタイズ済み HTML への変換と、見出しの抽出。**図のファイルと画像ファイルの変換**（IMP-126〜IMP-128）と、**編集モードのための位置と見える文字の対応付け**（IMP-121, IMP-123）。

### IMP-110: 型定義 **MUST**

```go
package renderer

type Heading struct {
    Level int    `json:"level"` // 1..6
    Text  string `json:"text"`  // インライン記法を除去したプレーンテキスト
    ID    string `json:"id"`    // 見出しの id 属性の値。user-content- 付き（MD-021, AR-053）
}

type Result struct {
    HTML          string
    Headings      []Heading
    NeedsMermaid  bool
    NeedsKaTeX    bool
    NeedsPlantUML bool // AR-021, MD-085
    NoDiagrams    bool // 図のファイルで、図が 1 つも無い（IMP-126, IMP-127。FR-025, FR-026）
}

// Options は変換の指定（4.75.0 で位置引数から改めた）。
type Options struct {
    BaseDir  string // 相対パス解決の基準ディレクトリ（表示中ファイルのディレクトリ。AR-042）
    RefKey   string // 目印の鍵（IMP-120）。空文字なら目印を付けない
    TextRefs bool   // 修飾の目印（text:<n>）も付けるか（IMP-120。編集モードの間だけ真）
}

type Renderer struct {
    md       goldmark.Markdown
    policy   *bluemonday.Policy
}

func New() *Renderer

// Render は Markdown を変換する。
func (r *Renderer) Render(source []byte, opts Options) (Result, error)

// 図のファイルと画像ファイルの変換（4.75.0。FR-025〜FR-027）。
func (r *Renderer) RenderPlantUMLFile(source []byte, opts Options) (Result, error) // IMP-126
func (r *Renderer) RenderMermaidFile(source []byte, opts Options) (Result, error)  // IMP-127
func (r *Renderer) RenderImageFile(path, version string) (Result, error)          // IMP-128

// Locate は変換を行わずに構文木だけを作り、書き換え位置を返す（IMP-121）。
func (r *Renderer) Locate(source []byte) (Locations, error)

// Visible は修飾するブロックの見える文字と、ソースとの対応を返す（IMP-123）。
func (r *Renderer) Visible(source []byte, target TextTarget) (*TextBlock, error)
```

- `Renderer` は状態を持たず、複数のゴルーチンから同時に `Render` / `Locate` を呼べる（IMP-024）。
- **`Options.RefKey` を空にできるのはテストのためである。** ゴールデンテスト（IMP-041）は固定の鍵を渡して目印も含めて比べ、目印を見ないテストは空文字を渡す。**アプリケーションは必ず鍵を渡す**（IMP-102）。鍵を渡さないと、並べ替え（FR-130）とリンクのコピー（FR-063）も働かない。
- **`Render` を呼ぶのは `internal/document`（IMP-102）と、描画スモークテストの `scripts/smoke`（BR-054）である。** 引数を足すときは両方を直す（4.75.0 で `Options` にまとめた）。**描画スモークは、見える文字の照合（BR-054）のために `VisibleTexts`（IMP-123）も呼び、`TextRefs` を真にして描く。****描画スモークテストも、アプリと同じく実行のたびに作った鍵を渡す**——目印の照合（BR-054）を検査するためである。
- `Render` 内でパニックが発生した場合は `recover` し、エラーとして返す（IMP-022）。

### IMP-111: goldmark の構成 **MUST**

```go
goldmark.New(
    // 既定の構文解析器のリンクだけを包んだもの（下記。IMP-123）。拡張と構文解析器の設定より前に置く
    // ——後に置くと、それまでに足した設定ごと置き換わる。
    goldmark.WithParser(newParser()),
    goldmark.WithExtensions(
        // GFM の内訳を個別に登録する。表の桁揃えを style 属性ではなく align 属性で
        // 出させ、出力から style を一掃するため（MD-024, MD-072, IMP-116）。
        extension.NewTable(
            extension.WithTableCellAlignMethod(extension.TableCellAlignAttribute),
        ),
        extension.Strikethrough, // 打消し線（MD-024）
        extension.TaskList,      // タスクリスト（MD-022）
        extension.Linkify,       // 裸の URL の自動リンク（MD-070）
        extension.NewFootnote(  // 脚注（MD-050）。戻りリンクの記号を指定する
            extension.WithFootnoteBacklinkHTML("&#x21a9;"),
            extension.WithFootnoteIDPrefix("user-content-"), // AR-053
        ),
        emoji.Emoji,            // 絵文字ショートコード（MD-051）
        meta.Meta,              // Front Matter（MD-073）
        alertExtension{},       // GitHub Alerts（IMP-112）
        mathExtension{},        // 数式の保護（IMP-113）
        mermaidExtension{},     // Mermaid ブロックの取り出し（IMP-115）
        plantUMLExtension{},    // PlantUML ブロックの取り出し（IMP-119）
        refExtension{},         // 編集・並べ替え・リンクの目印（IMP-120）。変換の優先度は 1000（脚注の 999 より後。BUG-027）
        // ハイライトは数式・Mermaid・PlantUML より後に置く（下記）
        highlighting.NewHighlighting(...), // IMP-114
    ),
    goldmark.WithParserOptions(
        // parser.WithAutoHeadingID() は使用しない（理由は後述）。
        parser.WithASTTransformers(
            util.Prioritized(headingTransformer{}, 100), // 見出し ID と一覧（IMP-117）
            util.Prioritized(imageTransformer{}, 95),    // 画像 URL の書き換え（IMP-118）
            util.Prioritized(rawHTMLTransformer{}, 90),  // 生 HTML の有無。サニタイズの後処理を省いてよいかの判断（IMP-116）
            util.Prioritized(codeAttrTransformer{}, 85), // 言語指定の後ろの属性を読ませない（IMP-114。4.86.0）
        ),
    ),
    goldmark.WithRendererOptions(
        html.WithUnsafe(),      // 生 HTML を通し、後段の bluemonday で除去する
        // インデント形式のコードブロックもラッパで包む（IMP-115）。
        // 既定の描画器（優先度 1000）より小さい値で上書きする
        gmrenderer.WithNodeRenderers( // goldmark の renderer パッケージ
            util.Prioritized(codeBlockRenderer{}, 500),
        ),
    ),
)
```

- **ハイライト（IMP-114）は、数式（IMP-113）・Mermaid（IMP-115）・PlantUML（IMP-119）の拡張より後に登録する。** これらは先に専用のノードへ差し替わり、chroma に渡らない。先に登録すると、`mermaid` や `plantuml` のフェンスがハイライト済みのコードブロックとして出力される（v1.0.0 の `renderer.go` と同じ並び）。
- **`html.WithUnsafe()` を有効にする。** 生 HTML を goldmark 段階で落とすと、`<details>` 等の許可要素（MD-072）まで失われるため。安全性の担保は後段のサニタイズ（IMP-116）に一元化する。この 2 つは必ず対で実装する。
- **見出し ID は goldmark の `WithAutoHeadingID` を使わない。** GitHub 互換のスラッグ規則（MD-021）と生成結果が異なるため、独自の AST 変換で付与する（IMP-117）。上のコードブロックが `parser.WithASTTransformers` を渡しているのはこのためであり、`WithAutoHeadingID` を併用してはならない（後から付与される ID に上書きされる）。
- **脚注の id には `extension.WithFootnoteIDPrefix("user-content-")` で接頭辞を付ける**（MD-050, AR-053）。goldmark はこの値を `id` 属性とリンクの `href`（`#user-content-fn:1`）の両方に付ける。フロントエンドは接頭辞付きのフラグメントもそのまま探せる（IMP-223）。
- **脚注の戻りリンクは `extension.NewFootnote(extension.WithFootnoteBacklinkHTML("&#x21a9;"))` で指定する。** goldmark の既定は `&#x21a9;&#xfe0e;` で、異体字セレクタにより白黒の記号として描かれる。GitHub はセレクタを付けないため見た目が変わる（MD-050, MD-002）。
- TOML の Front Matter（`+++`）は `meta.Meta` が扱わないため、`Render` の前段で文字列として除去する。
- **目印の変換（`refTransformer`）は、すべての AST 変換の後に走らせる**（優先度 1000。4.75.0。IMP-120, [BUG-027](../bugs/2026-11-08-bug-027-footnote-edit-ref-order.md)）。goldmark は AST 変換を優先度の**小さい**順に走らせ、脚注の変換（999）は定義を文書の末尾へ移し、参照されていない定義を除き、参照の順に並べ替える。**v1.1.0 は目印を 80 で付けており、移す前の木で番号を振っていた。**
- **フェンス付きのコードブロックの開きと閉じの囲いの行の位置を、構文木に残す**（IMP-121。4.75.0）。goldmark の `FencedCodeBlock` は囲いの行の区間を持たない（中身の行と言語指定だけ）。記録する手段は問わない（例: 標準のフェンスの構文解析器を包み、`Open` と `Close` で読んだ行の区間をノードに持たせる）が、**`Render` と `Locate` で同じ構成を使う**。**開きの囲いの位置にも goldmark の `Pos()` を使わない**（4.86.0。独立したレビューの指摘。UT-222 のケース 19）——`Pos()` は行の区間の頭に字下げの桁数を足した値であり、行頭にタブの残り（`Segment.Padding`。`>` の直後のタブなど）があると、その桁数だけ後ろへずれる（引用の記号 `>` の直後のタブで 2 桁）。開きの囲いを読んだ時点で、goldmark が言語指定の位置に使う式（区間の頭 − `Padding` + 行の中の位置）で記録する。ずれると囲いの文字を取り違え、閉じも記録できず、`data-locked` まで誤る。
- **リンクの構文解析器だけを包んだ、既定の構文解析器を使う**（`goldmark.WithParser(newParser())`。4.86.0。IMP-123）。goldmark はリンクと画像の開きの位置（`[` / `![` の頭）を `Pos()` に置くが、閉じの括弧 `]` と構文の終わり（`)` や参照の `]` の後ろ）を残さない。リンクを作った時点の読み位置を、変換ごとの文脈に記録する。**標準のリンクの構文解析器は置き換えて包む（並べて登録しない）**——`]` で失敗したときに開きの括弧の記録を取り除く副作用があり、並べると同じ `]` を 2 度読んで外側のリンクを壊す（フェンスの構文解析器は `Open` に副作用が無いため並べて登録できる）。配置は `internal/renderer/linkend.go`。

### IMP-112: GitHub Alerts 拡張 **MUST**

MD-040 を実装する。goldmark の `ASTTransformer` として実装する。

```go
type alertKind int

const (
    alertNote alertKind = iota
    alertTip
    alertImportant
    alertWarning
    alertCaution
)

// alertTransformer は Blockquote の最初の Paragraph の先頭テキストが
// "[!NOTE]" 形式であれば、Blockquote を Alert ノードへ差し替える。
type alertTransformer struct{}

func (t *alertTransformer) Transform(doc *ast.Document, reader text.Reader, pc parser.Context)
```

判定規則:

1. **`ast.Blockquote` の親が文書（`*ast.Document`）であること**（MD-040。4.75.0。[BUG-025](../bugs/2026-11-08-bug-025-nested-alerts.md)）。リストの項目・引用・脚注の中の引用は、先頭行が `[!NOTE]` の形でも通常の引用として残す（GitHub と同じ）。**v1.0.0 から、すべての `Blockquote` を見ていた。**
2. `ast.Blockquote` の最初の子が `ast.Paragraph` であること。
3. その先頭行が `[!` で始まり `]` で終わること。大文字小文字は区別しない。
4. 括弧内が 5 種のいずれかに一致すること。一致しない場合は変換せず、通常の引用として残す（MD-040）。
5. 一致した場合、その行を Blockquote から取り除き、Alert 種別を属性として保持する。

出力する HTML の構造は以下に固定する。

```html
<div class="markdown-alert markdown-alert-warning">
  <p class="markdown-alert-title">Warning</p>
  <p>本文…</p>
</div>
```

- 種別は `markdown-alert-<種別小文字>` のクラスで示す（`note` / `tip` / `important` / `warning` / `caution`）。
- ラベル（`Note` 等）は Go 側が出力する。英語表記に固定する（MD-040）。
- **アイコンは Go 側で出力しない。** サニタイズ（MD-072）は `svg` 要素を除去対象としており、Go 側が出力したインライン SVG は必ず落ちる。アイコンはフロントエンドが後処理で付与する（IMP-220 の 3'、DSP-260）。
- サニタイズの許可クラスに `markdown-alert` の接頭辞を含める（IMP-116）。

> [!IMPORTANT]
> 「アイコンは SVG で表示する」（MD-040）と「生 HTML の `svg` は除去する」（MD-072）は、Go 側で SVG を出力しようとすると衝突する。アイコンの付与をフロントエンドに寄せることで、サニタイズの許可リストを緩めずに両立させる。**サニタイズの許可リストに `svg` を追加して解決してはならない。**

### IMP-113: 数式の保護 **MUST**

MD-060 を実装する。KaTeX は**フロントエンドで実行する**（AR-031）ため、Go 側の役割は「数式部分を Markdown の他の記法から保護し、フロントエンドが識別できる形で出力する」ことに限る。

```go
// mathExtension は $...$ / $$...$$ / ```math を検出し、
// 内容をエスケープした上で <span class="math-inline"> /
// <div class="math-block"> として出力する。
type mathExtension struct{}
```

- 数式の中身は Markdown として解釈させない。`_` や `*` が強調として処理されることを防ぐため、インラインパーサの段階でテキストノードとして取り込む。
- `$` の判定は GitHub の規則に従う（MD-060）。開始の `$` の直後が空白の場合、および終了の `$` の直前が空白の場合は数式としない。これにより `$100 と $200` が数式にならない。
- コードブロック・インラインコードの内部は対象外とする。goldmark のインラインパーサはコードスパンを先に処理するため、優先度を適切に設定すれば自然に満たされる。
- 数式ノードを 1 つ以上出力した場合、`Result.NeedsKaTeX = true` とする。

### IMP-114: シンタックスハイライト **MUST**

MD-030 を実装する。

```go
highlighting.NewHighlighting(
    highlighting.WithFormatOptions(
        chromahtml.WithClasses(true),   // 重要（後述）
        chromahtml.WithLineNumbers(false), // 行番号なし（MD-032）
    ),
    highlighting.WithWrapperRenderer(codeBlockWrapper), // IMP-115
)
```

- **`WithClasses(true)` を必ず指定し、インラインスタイルを出力させない。** chroma がインラインの `style` 属性で色を書き込むと、テーマ切り替え（FR-070）のたびに Markdown の再変換が必要になり、「ちらつきなく即座に切り替える」（UI-105）が満たせなくなる。クラス名のみを出力し、配色は CSS 側で Light / Dark を切り替える（DSP-250）。
- 配色 CSS は**あらかじめ生成して `frontend/css/chroma.css` に置く**（`go run ./scripts/genchroma`）。ビルド時に作らず、実行時にも chroma のスタイルを走査しない。
- **生成に使うのはクラス名だけとし、色は DSP-013 の表から与える。** chroma 同梱の `github` / `github-dark` は 2015 年ごろの GitHub の配色（キーワードが黒の太字、文字列が `#dd1144`）であり、DSP-013 が定める現在の Primer の配色とは別物である。色までそちらから採ると MD-002 の「GitHub と並べて比較する」が成り立たない。一方でクラス名を手で並べると chroma が型を増やしたときに取りこぼすため、`chroma.StandardTypes` を走査し、系統（`Keyword` / `LiteralString` / `LiteralNumber` / `Comment` など）ごとにまとめて色を与える。DSP-013 の表に無い系統には色を与えない。
- **`WithLineNumbers(false)` だけでは足りない。** goldmark-highlighting は info string の属性（```` ```go {linenos=table} ````）から行番号を有効にできる。文書側から MD-032 を破れてしまうため、`WithCodeBlockOptions` で `WithLineNumbers(false)` を返し、属性由来の設定を打ち消す（属性より後に適用される）。
- **言語指定の後ろの属性（```` ```go {nohl=true} ```` など）は、鍵の有無にかかわらず常に読まない**（4.86.0。利用者の判断。UT-206 のケース 6）。GitHub は言語指定の最初の語だけを使う（MD-002）。goldmark-highlighting は、ノードに属性があればそれを使い、無いときだけ言語指定の後ろの `{…}` を読む（v2 の `getAttributes`）。v1.1.0 はノードに属性を置かず `{nohl=true}` でハイライトが消えていたが、v1.2.0 でコードブロックに目印（IMP-120）を置いたことで鍵があるときだけ読まなくなり、**鍵が空のとき（テストだけ）は言語指定の後ろに書いた `data-ref` がそのまま出力されていた**（独立したレビューの指摘。NFR-030）。**すべてのフェンス付きのコードブロックに、出力しないノードの属性を置く**（`codeAttrTransformer`。ラッパ（IMP-115）が書くのは `data-ref` と `data-locked` だけ）。上の `WithCodeBlockOptions` は、これがあっても残す。
- 登録する言語は MD-031 の一覧に限定してよい（AR-033）。限定する場合、`lexers.Get` が nil を返した言語はハイライトなしで出力する。
- 言語名のエイリアス解決は chroma の機能に委ねる。

### IMP-115: コードブロックのラッパと Mermaid **MUST**

FR-060 / MD-080 を実装する。すべてのコードブロックを共通のラッパで包み、フロントエンドがコピーボタンと Mermaid 描画の対象を識別できるようにする。

出力する構造:

```html
<div class="code-block" data-lang="go">
  <pre class="chroma"><code>…ハイライト済み…</code></pre>
</div>

<div class="code-block" data-lang="mermaid" data-mermaid="1"
     data-ref="0123456789abcdef:mermaid:0"
     data-source="graph TD&#10;  A--&gt;B">
  <pre class="mermaid-source">graph TD
  A--&gt;B</pre>
</div>
```

- **Mermaid ブロックにのみ `data-source` 属性を付け、原文を重複して持たせる。** Mermaid は描画後に `<pre>` が SVG へ置き換わり、DOM から原文が失われる。これがないと、描画後にコピーボタン（FR-060）がソースを取得できず、テーマ切り替え時の再描画（IMP-231）もできない。
- `data-source` の値は HTML 属性としてエスケープする（改行は `&#10;`）。Base64 等の追加のエンコードは行わない。デバッグ時に目視できる形を保つため。ただし最後段のサニタイズ（IMP-116）が数値文字参照を実体へ戻すため、**最終的な出力では改行がそのまま現れる**。要求は「値として改行が保たれること」であり、表記の形ではない。
- **Mermaid ブロックには目印 `data-ref="<鍵>:mermaid:<n>"` を付ける**（IMP-120, NFR-030）。n は Mermaid ブロックを**描画される順**（脚注の中のものは、脚注が末尾へ移った後の位置。BUG-027）に数えた番号（0 起点。描画に失敗するものも数える。FR-120 の「同じ対象」と同じ数え方）。**フロントエンドは鍵の合うブロックだけを描画し、鍵の合うブロックの `data-source` だけを使う**（IMP-230, IMP-221）。生 HTML で `class="code-block"` と `data-mermaid` と `data-source` を書けば同じ形を作れる（サニタイズは形しか見られない。IMP-116）。**目印で見分けないと、Go 側を経ていない図が描かれ、見えている内容と違う原文がコピーされる**（[BUG-014](../bugs/2026-09-14-bug-014-diagram-marker-spoofing.md)）。鍵が空なら付けない（IMP-110）。
- 通常のコードブロックには `data-source` を付けない。原文は `pre code` の `textContent` から取得できる（IMP-221）。**コピーされるのは見えている文字列そのもの**であり、偽装の余地が無い。
- **フェンス付きのコードブロック（図と数式を除く）には、目印 `data-ref="<鍵>:code:<n>"` を付ける**（4.75.0。FR-145）。編集モードで書き換える対象を、生 HTML で装ったものと見分けるためである（NFR-030）。番号は IMP-120 の `walkRefs` が振る。**前置きが行ごとに違うブロック（IMP-121 の `Locked`）には `data-locked="1"` も付ける**（編集のボタンを淡色にする。UI-053, IMP-264）。字下げ形式のコードブロック・` ```math `・生 HTML の `<pre>` には付けない（FR-145 の対象外）。鍵が空なら付けない。
- **Mermaid と PlantUML のブロックの目印は、書き換え（FR-145）の対象を指す目印にもなる**（IMP-122）。前置きが行ごとに違うものには、同じく `data-locked="1"` を付ける。
- ラッパが出すのは `<div class="code-block">` だけであり、内側の `<pre>` / `<code>` は chroma が出力する。ハイライトできない場合（言語指定なし・未知の言語）は chroma を通らないため、ラッパ側で `<pre><code>` を補う。`chroma` クラスが付くのは `<pre>` であり、ハイライトされたブロックに限る。
- `mermaid` ブロックを 1 つ以上出力した場合、`Result.NeedsMermaid = true` とする。
- `math` 言語のコードブロックは Mermaid ではなく数式として扱う（IMP-113）。
- `plantuml` / `puml` 言語のコードブロックは PlantUML として扱う（IMP-119）。ラッパの形は同じであり、属性だけが違う。

### IMP-116: サニタイズ **MUST**

MD-072 を実装する。変換パイプラインの最後段に固定で置き、迂回経路を作らない（AR-031）。

```go
// Policy は MD-072 の許可リストを実装した bluemonday ポリシーを返す。
func Policy() *bluemonday.Policy
```

ポリシーの要点:

- 許可要素は MD-072 の一覧をハードコードする。設定で緩められるようにしない。**4.74.0 で `strike` が加わった**（MD-072。手書きの取り消し線として読む。IMP-124）。
- `img` には `src` / `alt` / `title` / `width` / `height` を許可する。`src` は `http`, `https`, および内部アセットサーバのパス（`/__local/`）のみ許可する。
- `abbr` には `title` を許可する。これがないと許可要素として意味を持たない。
- `a` には `href` / `title` を許可する。`href` は `http`, `https`, `mailto`, 相対パス、`#` アンカーのみ許可する。`javascript:` 等は除去する。
- `class` 属性は `a` / `p` / `div` / `span` / `pre` / `code` / `ol` / `li` / `sup` に許可する。**値は、次の語を空白で区切って並べたものだけを通す**（語の完全一致。接頭辞で許可しない）。**「など」で濁さず、自前で出力するものをすべて列挙する。** 任意のクラス名を通さない。
  - `code-block` / `mermaid-source` / `plantuml-source`（IMP-115, IMP-119）
  - `markdown-alert` / `markdown-alert-title` / `markdown-alert-note` / `markdown-alert-tip` / `markdown-alert-important` / `markdown-alert-warning` / `markdown-alert-caution`（IMP-112）
  - `math-inline` / `math-block`（IMP-113）
  - `footnotes` / `footnote-ref` / `footnote-backref`（goldmark の脚注。MD-050）
  - `image-file`（画像ファイルの表示。IMP-128。4.75.0）
  - chroma のクラス（`chroma` とトークンクラス。下記）
  - **文字色のクラス（`md-color-red` など）は入れない**（下の `style` の後処理だけが付ける）
- `div` の `data-lang` / `data-mermaid` / **`data-plantuml`** / **`data-puml-error`** / `data-source` / `data-ref`（コードブロックと図のブロックの目印。下記）/ **`data-locked`**（値は `1` だけ。IMP-115）と、`id`（見出しアンカーと脚注）を許可する。**PlantUML の 2 つを落とすと、描画対象と拒んだブロックがフロントエンドから見えなくなる**（IMP-119, IMP-233）。
- 表の桁揃えのため `th` / `td` の `align`（`left` / `center` / `right`）を許可する。goldmark に align 属性で出力させることで、`style` 属性を許可せずに MD-024 を満たす（IMP-111）。
- **`style` 属性は、`span` の `color` だけを、値が文字色の一覧（`TextColors`。FR-147）の Light の値であるときに限って通す**（MD-072。4.75.0）。値の大文字と小文字、コロンの後の空白の有無の違いは許す（bluemonday は `color:#D1242F` を `color: #D1242F` に整える。実現性検討の付録 A.3）。一覧に無い値・`color` 以外の指定・`span` 以外の要素の `style` は落とす。
  - **`color` 以外の指定を含む `style` は、`color` の指定も含めて落とす**（4.77.0。利用者の判断。MD-072）。**bluemonday は宣言ごとに絞り、許可した宣言だけを残す**（v1.0.27 の `sanitizeStyles`）ため、そのままでは `color: #d1242f; font-weight: bold` が赤で描かれ、Go が色と読むもの（IMP-123 と IMP-124 の「MarkView の形」）と食い違う——画面は赤いのに、`Default` を選んでも色が外れない。**生 HTML がある文書では、サニタイズの前に 1 回走査し、`style` が「一覧の色の `color` の宣言 1 つだけ」でない `span` の `style` を取り除く。** 末尾の `;` は許し、`!important` は含めない。**判定は IMP-123 と同じ関数で行う。**
  - **サニタイズの後の走査（下の `id` と `input` と同じ 1 回の走査）で、残った `style` を外し、`class` に `md-color-<名前>` を足す**（`md-color-red` など。既に `class` があれば語を足す）。色は CSS が与える——Light は一覧の Light の値、Dark は一覧の Dark の値で描く（FR-147, DSP-010, DSP-121）。**インラインの `style` のままでは、Dark で別の色に描けない**（CSS より `style` が勝つ）。
  - **`md-color-*` を `class` の許可リストに入れない。** 生 HTML で直接書いた `class="md-color-red"` は落ちる。色が付くのは許可した `style` を書いた `span` だけになり、**画面で色が付くものと、Go が色として読むもの（IMP-124）が一致する。** 入れると、書き手が色だけ付いて Go が色と読まない `span` を作れ、`Default` を選んでも色が外れない。
  - **生 HTML が無い文書では、この後処理も要らない**（`style` は生 HTML からしか来ない）。下の「走査を省いてよい」はそのまま成り立つ。
- タスクリスト（MD-022）のため `input` の `type="checkbox"` / `checked` / `disabled` を許可する。MD-072 の許可要素に `input` はないが、これがないとタスクリストが描画されない。**bluemonday では属性を許可した要素が許可要素になる**ため、要素の一覧には足さない。**`disabled` は編集モードでも出力し続ける。** 外すのはフロントエンドであり、目印（IMP-120）の鍵が合う要素に限る（IMP-261）。
- **生 HTML の `id` 属性は、サニタイズの後に `user-content-` を前に付けた値へ書き換える**（MD-072, AR-053）。既に `user-content-` で始まる値には付けない。**書き換えは下の `input` の後処理と同じ 1 回の走査で行い**、`id` 属性を持つ**開始タグと自己終了タグ（`StartTagToken` / `SelfClosingTagToken`）**だけを組み立て直す（トークンの種類、つまり末尾の `/>` の有無は保つ）。それ以外のトークンは `Raw()` のバイト列をそのまま書く。**bluemonday は `<div id="x"/>` を自己終了タグのまま出力し、ブラウザはこれを開始タグとして扱う**（bluemonday v1.0.27 の `sanitize.go`）。開始タグだけを見ると、接頭辞の無い `id` が本文に残る。**見出しと脚注の id は、出力の時点で接頭辞が付いている**（IMP-117, IMP-111）ため、この走査では変わらない。
  - **文書に生 HTML が 1 つも無い（構文木に `HTMLBlock` / `RawHTML` が無い）なら、この走査を省いてよい**（NFR-011）。生 HTML が無ければ、出力の `id` と `input` はすべて自分で出したものである。
- **`type="checkbox"` を持たない `input` を残さない。** bluemonday は、許可した属性が 1 つでも残れば要素を残す。**`<input disabled>` や `<input data-ref="…">` と書くと、`type` が落ちて文字の入力欄として本文に出る**（`type` の既定は `text`。bluemonday v1.0.27 で確認）。**属性指定だけでは例外を閉じ込められない。** サニタイズの後に `golang.org/x/net/html` のトークナイザで `input` の開始タグと自己終了タグ（`<input disabled/>`）を調べ、`type="checkbox"` を持たないものを取り除く。**それ以外のトークンは `Raw()` のバイト列をそのまま書き、出力を変えない**（ゴールデンテストに差分を出さない）。
  - **生 HTML がある場合は、数を数えて走査を省いてはならない。** bluemonday は `object` / `noscript` / `style` / `title` などの**中身ごと捨てる**（bluemonday v1.0.27 の `policy.go` の `addDefaultSkipElementContent`）。`<object>` の中に Markdown の見出しを置くと Go が出した `id` が 1 つ消え、同じ数だけ生 HTML の `id="statusbar"` を足せば数が一致する。**`id` の数も `<input` の数も、書き手が合わせられる**（AR-053 の [BUG-011](../bugs/2026-09-14-bug-011-document-id-collision.md) の再発、`<input disabled>` が入力欄として残る）。
  - **組み立て直すタグの属性値は、HTML の属性値としてエスケープし直す。**
  - **「既に `user-content-` で始まる値には付けない」はこの走査に必須の規則である**——見出しと脚注の id は出力の時点で接頭辞が付いており、付け直すと二重になる。
- **目印の属性（IMP-120）を、値の形を正規表現で縛って許可する。** `data-ref` は `input` / `table` / `th` / `td` に限り `^[0-9a-f]{16}:(task:[0-9]+|table:[0-9]+|cell:[0-9]+:[0-9]+:[0-9]+)$`、`div` に限り `^[0-9a-f]{16}:(?:code|mermaid|plantuml):[0-9]+$`、**`p` / `h1`〜`h6` / `li` に限り `^[0-9a-f]{16}:text:[0-9]+$`**（4.75.0）、`data-link` は `a` に限り `^[0-9a-f]{16}:` で始まる値だけを通す。**形を縛っても偽装は防げない**（書き手は同じ形を書ける）。**防ぐのは鍵である**——鍵は変換のたびに作られ、書き手は知りようがない（IMP-120）。形を縛るのは、それ以外の用途で属性を通さないためである。
- 脚注の `role`（`doc-*`）を許可する。
- **chroma のトークンクラス（`k` `s2` `nf` など）は接頭辞を持たない。** 値を書き写すと chroma の更新で取りこぼし、コードが無色になる。`chroma.StandardTypes` から許可リストを組み立て、一覧の維持を不要にする。
- **`data:` の判定に bluemonday の `AllowDataURIImages` を使わない。** あれは `image/svg+xml` を許可する（MD-072 参照）。許可する種別を自前で `gif` / `jpeg` / `png` / `webp` に限る。
- サニタイズ後に、想定したクラスや属性が失われていないことをユニットテストで確認する（IMP-040）。

- **図のファイル・画像ファイルの変換（IMP-126〜IMP-128）の出力も、このサニタイズを最後に通す**（4.75.0）。Go が組み立てた HTML でも迂回経路を作らない（AR-031）。

> [!IMPORTANT]
> `html.WithUnsafe()`（IMP-111）とこのサニタイズは対で意味を持つ。片方だけを変更してはならない。

### IMP-117: 見出しアンカーの生成 **MUST**

MD-021 を実装する。

```go
type slugger struct {
    used map[string]int
}

// Slug は GitHub 互換のアンカー文字列を返す。同一 slugger 内で
// 重複した場合、2 つ目以降に "-1", "-2" … を付加する。
func (s *slugger) Slug(text string) string
```

処理順序:

1. 見出しの AST からインライン記法を除いたプレーンテキストを組み立てる。**生 HTML のタグ（`RawHTML`）は除いて中の文字だけを残し、エスケープと文字参照は解く**（MD-021 の 1。`## A &amp; B` は `a--b`、`Heading.Text` は `A & B`。4.78.0 で明記した。UT-202 のケース 14・15）。**インラインコード（`CodeSpan`）の中の文字は、文字参照を解かずにそのまま使う**（`` ## `&amp;` `` の `Heading.Text` は `&amp;`、スラッグは `amp`。本文で文字どおりに表示される文字と同じにする。4.84.0。利用者の判断。UT-202 のケース 16）。ただし**行の改行は半角空白 1 つにする**（本文の `code` の中と同じ。CommonMark のコードスパン。4.86.0。UT-203 のケース 9）。**数値の文字参照は描画（goldmark の Writer）と同じ規則で解く**——10 進は 1〜7 桁を基数 10、16 進は 1〜6 桁で読み、0 と符号位置にならない値は U+FFFD とする（4.86.0。独立したレビューの指摘。UT-203 のケース 9）。goldmark の `util.ResolveNumericReferences` は 10 進を基数 0 で読み（`&#065;` が 8 進の `5`）、16 進の桁数の上限も無いため使わない。**goldmark の文字のノードは、エスケープ（`\*`）も文字参照（`&amp;`）も書かれたとおりの綴りで持ち、描画のときに解く**（goldmark v1.8.5 で実測。IMP-123 の表と同じ）。**両方をここで、1 回の走査で解く**（2 段に分けると `\&amp;` を取り違える。見える文字の対応付け（IMP-123）と同じ解き方にする）。`## a\*b` の `Heading.Text` は `a*b`（UT-203 のケース 8）。
2. Unicode の小文字化を行う（`strings.ToLower`）。
3. **前後の空白を落としたうえで、残った空白（`unicode.IsSpace`）1 つにつき `-` を 1 つ書く。** **連続をまとめない**（MD-021 の 3）。まとめると、記号を除いた跡に並んだ空白が 1 つのハイフンになり、GitHub と食い違う。
4. **英数字・`-`・`_` と、`unicode.IsLetter` / `unicode.IsDigit` に当たる文字だけを残す。** それ以外は除去する（MD-021 の 4）。**「非 ASCII なら残す」と書かない。** 全角の括弧・中黒・矢印まで残り、GitHub と違うアンカーになる。
5. 重複時に連番を付与する。**スラッグが空なら連番を付けない**（空の見出しが 2 つあっても、どちらも空。6。4.78.0 で明記した）。
6. **`id` 属性の値は、スラッグの前に `user-content-` を付けたものとする**（MD-021, AR-053）。`Heading.ID` も同じ値にする（アウトラインが本文の要素を探すため。IMP-224）。重複の判定と連番はスラッグで行う（`test` / `test-1`）。
   - **スラッグが `user-content-` で始まっていても、必ず付ける**（`## user-content-foo` の id は `user-content-user-content-foo`）。「二重に付けない」は生 HTML の `id` だけの規則である（IMP-116）。
   - **スラッグが空なら、`id` 属性も `Heading.ID` も空とする**（接頭辞だけの `user-content-` を出さない。空の見出しがいくつあっても同じ id が並ばない）。

同じ処理で得たプレーンテキストを `Heading.Text` にも用いる（FR-040）。

> [!IMPORTANT]
> **接頭辞を `Slug` の中で付けない。** `Slug` は MD-021 の 1〜5 を実装する関数であり、UT-202 はスラッグそのものを見る。接頭辞は `id` 属性へ書くときに 1 か所で付ける。**2 か所で付けると、`user-content-user-content-` が生まれる。**

### IMP-118: 画像 URL の書き換え **MUST**

FR-022 / FR-027 / AR-040 / AR-042 を実装する。

```go
// rewriteImageURL は Markdown 内の画像 src を、WebView から取得できる形へ変換する。
//   http:// https://        → そのまま（MD-071）
//   data:image/…            → そのまま
//   絶対パス・相対パス       → /__local/<エスケープ済み絶対パス>?v=<版>（AR-040。版は下記）
func rewriteImageURL(src, baseDir string) string
```

- 相対パスは `baseDir` を基準に `filepath.Join` して絶対化する。先頭が `/` のパスは、OS を問わず絶対パスとして扱う（Markdown の URL は POSIX 形式で書かれるが、Windows の `filepath.IsAbs` はドライブレターを要求するため）。
- URL の組み立ては `internal/localurl` の `Encode` を使う（IMP-012）。**接頭辞とエスケープ規則を `renderer` 側に書かない。**解く側（IMP-161）と規則が食い違えば、ローカル画像がすべて 404 になる。
- 宛先は URL であるため、`%20` のような百分率エンコードを解いてからパスとして解決する。
- リンク（`a href`）は書き換えない。クリック時にフロントエンドが捕捉して Go 側へ渡すため（AR-060）、元の値のまま保持する。
- **ローカルの画像の URL には、ファイルの版をクエリで付ける**（`?v=<版>`。4.75.0。FR-022, FR-027。[BUG-026](../bugs/2026-11-08-bug-026-stale-embedded-image.md)）。版は `localurl.Version`（更新時刻と大きさ。IMP-160）で作り、`os.Stat` に失敗したら付けない（404 になるだけである）。
  - **WebView2 も WebKitGTK も、ページの中で一度読んだ画像を、`Cache-Control: no-store`（IMP-162）でも使い回す**（実測。BUG-026 は W1 / L1 の実機で確かめた）。MarkView はページを遷移しない（AR-060）ため、URL がパスだけで決まると、画像を書き換えて `F5` を押してもウィンドウを閉じるまで古い画像が出続ける。**版が変われば URL が変わり、取り直される。** 変わらなければ URL も同じなので、再描画のたびに取り直さない（ちらつかない）。
  - 配信側（IMP-161）はクエリを見ない（`localurl.Decode` は `EscapedPath` を受け取る）。版は取り直させるためだけのものであり、検査には使わない。
  - 変換のたびに、ローカルの画像ごとに `os.Stat` を 1 回呼ぶ。`Locate` と `Visible` では呼ばない（URL を出さない）。

### IMP-119: PlantUML ブロックの取り出し **MUST**

FR-024 / MD-083 / MD-084 を実装する。IMP-115 のラッパの上に乗り、Mermaid と同じ形で出力する。

```html
<div class="code-block" data-lang="plantuml" data-plantuml="1"
     data-ref="0123456789abcdef:plantuml:0"
     data-source="@startuml&#10;Alice -&gt; Bob&#10;@enduml">
  <pre class="plantuml-source">@startuml
Alice -&gt; Bob
@enduml</pre>
</div>
```

- 対象は言語指定が `plantuml` または `puml` のフェンス。**大文字小文字を区別しない**（chroma の言語名解決と揃える）。**`uml` は対象としない**（MD-083）。
- **`data-source` を付ける**。描画後に `<pre>` が SVG へ置き換わり原文が失われるためであり、理由は IMP-115 と同じである（コピーボタンとテーマ切り替え時の再描画）。
- **目印 `data-ref="<鍵>:plantuml:<n>"` を、描画するブロックにも、下の検査で拒んだブロック（`data-puml-error`）にも付ける**（IMP-120）。n は PlantUML ブロックを**描画される順**（脚注の中のものは末尾。BUG-027）に数えた番号（0 起点。拒んだものも数える。FR-120 の「同じ対象」と同じ数え方。番号は IMP-120 の `walkRefs` が振る）。**フロントエンドは鍵の合うブロックだけを描画と理由の表示の対象にし、資産を読むかどうかも鍵の合うブロックの有無で決める**（IMP-233, NFR-013）。**目印が無いと、生 HTML で `data-plantuml` と `data-source` を書いたブロックが、下の検査を経ずに描画へ回る**（MD-084, NFR-032。[BUG-014](../bugs/2026-09-14-bug-014-diagram-marker-spoofing.md)）。鍵が空なら付けない（IMP-110）。
- **`data-plantuml` を付けたブロック**を 1 つ以上出力した場合、`Result.NeedsPlantUML = true` とする（下記の検査で拒んだブロックは数えない）。

**取り込み指令の検査**（MD-084, NFR-032）

```go
// hasIncludeDirective は PlantUML ソースが外部を取り込む指令を含むかを返す。
func hasIncludeDirective(source string) bool
```

| 規則 | 内容 |
| --- | --- |
| 対象の指令 | `!include` / `!includeurl` / `!includesub` / `!import` と、**`from` を伴う `!theme`** |
| 位置 | **行頭**（前に空白があってもよい）にあるものだけを見る。PlantUML のプリプロセッサは行頭でしか効かない |
| 大文字小文字 | 区別しない |
| `!includesub` の前方一致 | **`!include` の検出で巻き込めない。** `!includesub` は `!include` で始まるが、`!includeurl` とともに別の指令である。どれも拒むため結果は同じだが、**将来 `!include` だけを許すことになったときに壊れる** |
| `!theme` | **`from` を伴わない `!theme plain` は拒まない**（組み込みテーマ。MD-083 で使えると定めている） |
| コメント | `'` で始まる行は検査の対象外とする |

- 検査に引っかかったブロックは、`data-plantuml` を**付けず**、`data-puml-error="include"` を付けて出力する。フロントエンドはこれを見て理由を表示し（FR-110）、**描画を試みない**。
- **引っかかったブロックは `NeedsPlantUML` を立てない。** 全部のブロックが拒まれた文書で 5 MiB の資産を読むのは無駄である（NFR-013）。

> [!IMPORTANT]
> **判定を Go 側に置くのは、描画処理系の振る舞いに依存しないためである**（MD-084, AR-031）。資産は BR-043 で自動更新されるため、上流がリモート取得を有効化してもこちらは気づかない。
>
> **フロント側で `XMLHttpRequest` を一時的に潰す方式は採らない。** 描画中だけ差し替える実装は副作用が読みにくく、描画が非同期（IMP-233）であるため元に戻す契機も定まらない。

### IMP-120: 編集・並べ替え・リンクの目印 **MUST**

FR-061 / FR-063 / FR-130 / FR-141 / FR-142 / FR-145〜FR-147 / MD-084 / NFR-030 を実装する。配置は `internal/renderer/editref.go`（**フェンス付きのコードブロックの囲いの行の記録と `FenceLocation` の組み立ては `fence.go`**。4.85.0。コードブロックと図のブロックの目印は IMP-115 / IMP-119 の描画器が出す）。

**GFM の構文に由来する要素にだけ、変換時に目印の属性を付ける。** フロントエンドは目印を見て、編集できる要素・並べ替えられる表・書かれたとおりのリンク先・**描画してよい図と、コピーに使ってよい原文（`data-source`）**を知る。**生 HTML で書かれた要素には付かない。**

| 対象（goldmark のノード） | 出力する属性 | 値 |
| --- | --- | --- |
| `TaskCheckBox`（タスクリストの項目） | `input` の `data-ref` | `<鍵>:task:<n>`。n は文書の中で何番目のタスクか |
| `Table`（GFM の表） | `table` の `data-ref` | `<鍵>:table:<t>`。t は文書の中で何番目の GFM の表か |
| `TableCell`（**ソース上に存在するセル**） | `th` / `td` の `data-ref` | `<鍵>:cell:<t>:<r>:<c>`。r は 0 が見出し行、c は列 |
| `Link` / `AutoLink` | `a` の `data-link` | `<鍵>:<書かれたとおりのリンク先>` |
| Mermaid のブロック（IMP-115） | `div.code-block` の `data-ref` | `<鍵>:mermaid:<n>`。n は文書の中で何番目の Mermaid ブロックか。**Mermaid ファイルでは `mermaid:0`**（IMP-127） |
| PlantUML のブロック（IMP-119。検査で拒んだものを含む） | `div.code-block` の `data-ref` | `<鍵>:plantuml:<n>`。n は文書の中で何番目の PlantUML ブロックか。**PlantUML ファイルでは何番目の図か**（IMP-126） |
| **フェンス付きのコードブロック**（図と数式を除く。IMP-115。4.75.0） | `div.code-block` の `data-ref` | `<鍵>:code:<n>`。n は文書の中で何番目のコードブロックか。前置きが行ごとに違うものには `data-locked="1"` も付ける（FR-145） |
| **修飾できるブロック**（`Paragraph`・`Heading`・リストの項目の `TextBlock`。**`Options.TextRefs` が真のときだけ**。4.75.0） | `p` / `h1`〜`h6` / `li` の `data-ref` | `<鍵>:text:<n>`。n は文書の中で何番目の修飾できるブロックか（FR-146, FR-147） |

- **鍵（`refKey`）は変換のたびに作る乱数である**（IMP-102）。文書の書き手は変換より前に文書を書くため、**鍵を知りようがなく、生 HTML で目印を偽装できない**（FR-141, FR-063, NFR-030）。サニタイズ（IMP-116）が属性の形を通しても、鍵の合わない目印はフロントエンドが無視し（IMP-260）、Go 側も拒む（IMP-195）。
- 番号は 0 起点で、**描画される順**（脚注の変換の後の木の順。脚注の中のものは末尾。BUG-027）とする。数え方は IMP-121 の `Locate` と**同じ関数で**決める（`walkRefs`）。**2 か所に書くと、描画で付けた番号と書き込みで探す番号が食い違い、別のセルを書き換える。**
- **番号は、すべての AST 変換を終えた後の木の順で振る**（4.75.0。[BUG-027](../bugs/2026-11-08-bug-027-footnote-edit-ref-order.md)）。目印の変換（`refTransformer`）の優先度を 1000 とし、goldmark の脚注の変換（999）より後に走らせる（IMP-111）。**脚注の変換は、定義を文書の末尾へ移し、参照されていない定義を除き、参照の順に並べ替える。** v1.1.0 は目印を移す前の木（優先度 80）で振り、`Locate` は移した後の木で数えていた——**脚注の定義の中にタスクや表があると、押したものと別のチェックボックスやセルを書き換えた**（定義が本文より前・参照されていない定義・定義と参照の順の違い、の 3 つの形。BUG-027 の 1.1）。`Locate` も同じ構成で構文解析するため、同じ木で数える。
  - **図のブロックの番号も `walkRefs` で振る**（4.75.0）。v1.1.0 は図の変換器（`mermaidTransformer` / `plantUMLTransformer`）が番号を振り、`walkRefs` と共有しなかった——図が書き換えの対象でなかったためである。**図も書き換えの対象になった（FR-145）ため、描画で付けた番号と書き込みで探す番号を同じ関数で決める。** 図の変換器はノードを差し替えるだけとし、番号と目印の値は `refTransformer` がノードに持たせ、描画器（IMP-115, IMP-119）はそれを書く（描画器は複数の変換で共有されゴルーチンをまたぐため、変換ごとの状態を持てない。4.67.0 の理由はそのまま）。描画に失敗するブロック・拒んだブロックも数える（FR-120 の「同じ対象」）。
- **補われたセルには `data-ref` を付けない。** 行のセル数が見出し行より少ないとき、goldmark は空の `TableCell` を補う（ソース上の位置を持たない。`Lines().Len() == 0`）。付けないことで、フロントエンドは編集できないセルとして扱う（FR-142）。
- **`TaskCheckBox` は描画器を差し替える。** goldmark 標準の描画器は属性を出力しない。**差し替えた描画器は、標準の描画器と同じ文字列（属性の順と、末尾の半角空白 `> ` を含む）に `data-ref` を足しただけにする。** 違えると、ゴールデンテスト（IMP-041）の差分が目印以外にも出る。`Table` / `TableCell` / `Link` / `AutoLink` は、AST 変換で属性を与えれば標準の描画器が `data-` 属性として出力する（`html.RenderAttributes` は `data-` で始まる属性をフィルタに関わらず出す。goldmark v1.8.5 で確認）。
- **リンク先の「書かれたとおり」は `Link.Destination` の値とする**（FR-063）。goldmark は参照リンクを解決した後の宛先（定義に書かれた形）を `Destination` に持ち、`href` へ出すときにエスケープと実体参照を解いて百分率エンコードする。**`Destination` は原文そのものではない。** 山括弧で囲んだ宛先（`<./a b.md>`）は括弧が外れ（`./a b.md`）、バックスラッシュのエスケープと実体参照は解かれずに残る（`./a\_b.md` はそのまま）。**これを「書かれたとおり」とする。**
- **`AutoLink` は `Label(source)` の値とする。** `URL(source)` は `www.` で始まる裸の URL（MD-070）に `http://` を足すため、書かれた形ではない（`www.example.com/p` が `http://www.example.com/p` になる）。メールアドレスの `mailto:` は `href` にだけ付き、どちらにも入らない。
- **生 HTML の `<a>` には付かないため、フロントエンドは `href` を使う**（FR-063）。
- 属性値は HTML 属性としてエスケープする（IMP-115 の `data-source` と同じ扱い）。

**修飾の目印**（`text:<n>`。FR-146, FR-147。4.75.0）

- **`Options.TextRefs` が真のときだけ付ける**（編集モードの間だけ。利用者の判断）。全ブロックに付けると、HTML が 3〜5 割、変換の時間が 4〜12 % 増えた（実現性検討の付録 A.12）。**付けないときも、番号は同じに数える**（`Locate` と `Visible` は付けるかどうかを知らない）。
- **対象のブロック**: `Paragraph`（引用・Alerts・脚注・広いリストの項目・`<details>` の中で Markdown として解かれたものを含む）、`Heading`（ATX と設定見出し）、詰まったリストの項目の `TextBlock`（**`ListItem` の最初の子であるものだけ**。`TextBlock` は要素を出さないため、目印は親の `li` に付ける。対象は `li` の中の最初のブロック要素の手前まで。**2 つ目以降の `TextBlock`**（字下げしたフェンスの後ろに続く文字など）**と、最初の子が `TextBlock` でない項目は数えない**——`li` には属性を 1 つしか書けない。4.78.0。IMP-123）。
- **対象にしないもの**: Alerts の見出し（`p.markdown-alert-title`。MarkView が出す文字。IMP-112）、表のセル（`cell:` の目印を使う）、コードブロック・数式・図・生 HTML のブロック（FR-146 の対象外）。
- 修飾は見える文字もブロックの数も変えない（IMP-124 の 9）ため、**自分の書き込みの後も番号は変わらない**（FR-143）。

**コードブロックの目印**（`code:<n>`。FR-145。4.75.0）

- **常に付ける**（編集モードでなくても）。数は文書のコードブロックの数だけであり、重さは問題にならない。タスクとセルの目印と同じ扱いである。
- 前置きが行ごとに違うブロック（IMP-121 の `Locked`）には `data-locked="1"` も付ける。値の形は IMP-116 が縛る。偽装しても、鍵の合わない要素はフロントエンドが無視する（IMP-260）。

> [!IMPORTANT]
> **目印を「生 HTML か GFM か」を区別する唯一の手段にする。** サニタイズを通った後の HTML では、GFM のタスクリストのチェックボックスと、生 HTML で書かれた `<input type="checkbox" disabled>` は**区別がつかない**（MD-072 の NOTE）。属性の有無や形で判断すると、書き手が同じ属性を書くだけで偽装できる。**鍵が合うかどうかだけで判断する。**
>
> **図のブロックも同じである。** `data-mermaid` / `data-plantuml` / `data-source` の有無で判断すると、生 HTML で書いたブロックが Go 側の検査（IMP-119）を経ずに描画され、見えている内容と違う原文がコピーされる（[BUG-014](../bugs/2026-09-14-bug-014-diagram-marker-spoofing.md)。v1.0.0 から残っていた）。

### IMP-121: 書き換え位置の特定 **MUST**

FR-141 / FR-142 / FR-145 / AR-031 を実装する。配置は `internal/renderer/locate.go`（数え方の `walkRefs` は IMP-120 の `editref.go`。4.85.0 で 400 行の目安（IMP-011）により分けた）。

```go
// Span はソース上の半開区間 [Start, Stop)。
type Span struct{ Start, Stop int }

// CellSpan は 1 つのセルの位置。
type CellSpan struct {
    Content Span // 前後の空白を除いた内容（空のセルでは Start == Stop）
    Between Span // 前後の区切り（|）の間。空白を含む
}

type TableLocation struct {
    Cells [][]*CellSpan // [行][列]。0 行目が見出し行。nil はソース上に存在しないセル
}

// FenceLocation はフェンス付きのコードブロック・図のブロックの位置（IMP-122。4.75.0）。
type FenceLocation struct {
    Open   Span   // 開きの囲いの文字の並び（「```」の部分）
    Close  Span   // 閉じの囲いの文字の並び（Closed が偽なら中身の終わりの位置で、長さ 0）
    Closed bool   // 閉じの囲いがあるか（文書や引用の終わりで閉じたものは偽）
    Body   Span   // 中身の行の全体（開きの囲いの行の次の行の頭から、閉じの囲いの行の頭まで。
                  // 閉じが無ければ最後の中身の行の終わりまで。開きの囲いの行に改行が無ければ、文書の終わりで長さ 0）
    Lines  []Span // 中身の各行の区間（前置きの後ろから、改行の手前まで）
    Info   string // 言語指定（確かめに使う）
    Prefix string // ブロックの前置き（IMP-122）
    Locked bool   // 前置きが行ごとに違う・タブの途中から始まる（FR-145。IMP-122）
}

type Locations struct {
    Tasks     []Span          // 各タスクの括弧の中の 1 文字
    Tables    []TableLocation // GFM の表
    Codes     []FenceLocation // code:<n>（4.75.0）
    Mermaids  []FenceLocation // mermaid:<n>
    PlantUMLs []FenceLocation // plantuml:<n>
    Texts     int             // 修飾できるブロックの数（text:<n>。位置は IMP-123 が求める）
    Blocks    []string        // 構文木の順のブロックの種類（ノードの Kind の名前）。書き込みの確かめに使う（IMP-122, IMP-124）
}
```

- **`Render` と同じ goldmark の構成で構文木だけを作り、HTML を出力しない。** サニタイズも通さない。位置を求めるのに要らない。
- **前処理も `Render` と同じにする。** 前処理（Front Matter の扱い。IMP-111）は位置を次の 2 通りでずらすため、ずれた分を位置から戻す。**前処理とずれの量は 1 つの関数が返し、`Render` と `Locate` の 2 か所に書かない。**
  - TOML の Front Matter（`+++`）を文字列として除いた場合: 除いた長さを足す。
  - **1 行目が `---` で閉じの無い YAML の場合: 前処理が先頭に改行を 1 バイト足しているため、1 を引く。**
  - 閉じた YAML は `meta.Meta` がパーサの中で読み飛ばすため、位置はずれない。
- 番号付けは IMP-120 と共有する `walkRefs` で行う。**すべての AST 変換を終えた後の木で数える**（IMP-120。BUG-027）。
- **タスクの位置**は、`TaskCheckBox` の親（`TextBlock` または `Paragraph`）の先頭行の開始位置から、GFM のタスクリストの正規表現（`^\[([\sxX])\]\s*`）で括弧の中の 1 文字を求める。
- **セルの位置**は `TableCell.Lines().At(0)` の区間とする。goldmark は前後の空白を除いた区間を持つ。`Between` は、その区間を区切りの `|` の直後・直前まで広げたものとする。**引用やリストの中の表**でも、区間はソース上の実際の位置を指す（行頭の `> ` などは区間の外にある）。
  - **空のセル（`|  |`）の区間は `Start == Stop` で、位置は閉じる `|` の位置（空白の後ろ）にある。** `Between` は前の `|` の直後からその位置までとなる。
  - **行頭に `|` が無い行の最初のセル**の `Between` は、行の内容の先頭（引用の `> ` などの後ろ）から始まる。
  - **`Between` へ広げるときに越えるのは、半角空白とタブだけ**とし、改行は越えない。**左は行の内容の先頭（行のノードの位置）、右は行末（改行の手前）で止まる**——行末に `|` が無い行の最後のセルは、行の終わりまでとなる（4.67.0。UT-218 のケース 8・9 の期待はこの規則による）。
  - **エスケープした `\|` を含むセルの区間は `\` を含む。** `CellSource` はそれをそのまま返す（編集欄には書かれたとおりに出る）。
  - **見出し行より多いセルは、goldmark が構文木から捨てる。** 描画されず、目印も位置も持たない。
- **コードブロックと図の位置**（`FenceLocation`。4.75.0）は、IMP-111 で構文木に残した囲いの行の区間と、goldmark の中身の行の区間（`Lines()`）から求める。`Prefix` と `Locked` の決め方は IMP-122 の「前置き」に従い、**描画（IMP-115 の `data-locked`）と書き込み（IMP-122）が同じ値を使う。** 字下げ形式のコードブロックと ` ```math ` は数えない（IMP-120）。
- **`Blocks` は、すべてのブロックのノードの種類を、構文木の順（深さ優先の行きがけ順）に並べたもの**とする。書き込みの前後で比べ、ブロックの数と種類が変わらないことを確かめる（2.14 の IMPORTANT。IMP-122, IMP-124）。
- 変換と同じく、パニックは `recover` してエラーとして返す（IMP-022）。

> [!NOTE]
> **位置は「正規化後のテキスト」の上の値である。** 生バイト列の上の位置へ移すのは `document` の役目である（IMP-106）。`renderer` は BOM や改行コードの正規化を知らない（IMP-103 は `document` にある）。

### IMP-123: 見える文字の対応付け **MUST**

FR-146 / FR-147 / BR-054 を実装する（4.75.0）。配置は `internal/renderer/visible.go`（**走査は `visiblebuild.go`、記法の記号の区間は `visiblemarks.go`、生 HTML のタグの対は `visibletags.go`、エスケープと文字参照の解き方は `decode.go`（IMP-117 と共有）、色の判定 `ColorOfStyle` は `colorstyle.go`**（IMP-116 / IMP-124 と共有）、リンクの閉じの位置の記録は `linkend.go`（IMP-111）。4.85.0 と 4.86.0 で 400 行の目安（IMP-011）により分けた）。**画面の選択をソースの位置に対応付けるのは Go 側である**（AR-031, FR-146）。フロントエンドは見える文字を数えて送るだけにし（IMP-266）、Go 側は構文木から同じ数え方で見える文字を組み立てて照合する（IMP-124 の 2）。

```go
// TextTarget は修飾するブロックの指定（document の Ref から作る）。
type TextTarget struct {
    Cell            bool // 表のセルなら真（Table / Row / Col を使う）
    Index           int  // text:<n> の n
    Table, Row, Col int
}

// TextBlock は 1 つのブロックの見える文字と、ソースとの対応。
type TextBlock struct {
    Visible    []rune       // 見える文字（下の表の数え方）
    Source     []Span       // 見える文字 1 つずつの、ソース上の区間（正規化後のテキストの上）
    Atoms      []Span       // 丸ごとにしか囲めない部品の、見える文字の上の区間（下の表）
    Inlines    []InlineSpan // インラインの要素（入れ子の外側から順）
    HardBreaks int          // ハード改行の数（IMP-124 の 9）
}

// InlineSpan はインラインの要素 1 つ。
type InlineSpan struct {
    Kind  string // "strong" / "emphasis" / "strike"（記号）/ "link" / "code" / "image" /
                 // "del" / "s" / "strike-tag" / "color"（生 HTML のタグの対）/ "html"（その他のタグの対）
    Text  Span   // 見える文字の上の区間（中身）
    Open  Span   // ソース上の開きの記号・タグの区間（link は `[`、image は `![`、code は開きのバッククォートの並び）
    Close Span   // ソース上の閉じの記号・タグの区間（link と image は `]` から構文の終わりまで（`](…)` / `][…]` / `]`）、
                 // code は開きと同じ長さの閉じの並び。4.86.0）
    Color string // Kind が "color" のときの色の名前（TextColors）
}

// TextColors は文字色の一覧（FR-147）。名前・ファイルに書く値（Light）・Dark で描く値の順。
var TextColors = []struct{ Name, Light, Dark string }{
    {"red", "#d1242f", "#f85149"}, {"orange", "#bc4c00", "#db6d28"},
    {"yellow", "#9a6700", "#d29922"}, {"green", "#1a7f37", "#3fb950"},
    {"blue", "#0969da", "#4493f8"}, {"purple", "#8250df", "#ab7df8"},
    {"gray", "#59636e", "#9198a1"},
}

func (r *Renderer) Visible(source []byte, target TextTarget) (*TextBlock, error)

// VisibleTexts は、修飾できるすべてのブロック（text と cell）の見える文字を、目印から鍵を除いた値
// （"text:3" / "cell:0:1:2"）ごとに返す（描画スモークの照合。BR-054）。
func (r *Renderer) VisibleTexts(source []byte) (map[string]string, error)
```

**数え方**（フロントエンドの IMP-266 と同じにする。実現性検討の 7.4 と付録 A.11）

| 本文の中のもの | 数え方 |
| --- | --- |
| 文字 | `Text` の区間の文字を（インラインコードの中を除く）、**エスケープ（`\*` → `*`）と文字参照（`&amp;` → `&`）を解いたもの**として 1 符号位置ずつ数える。1 文字ごとにソースの区間を持つ（エスケープと文字参照は 1 つの `Text` の区間の中に生のまま入っている。実測）。**数値の文字参照は描画と同じ規則で解く**（IMP-117 の 1。4.86.0）。**NUL は U+FFFD**（描画の Writer が置き換える）、**`&#13;`（CR）は LF**（ブラウザが HTML を読むときにそろえる）。ただし **CR の直後に LF が来る形（行末の `&#13;`、`&#13;&#10;`）は、ブラウザが 2 つを LF 1 つにまとめるため一致しない**（そのブロックの修飾は古い指示として断られる。IMP-124 の 2） |
| 改行（ソフト・ハード） | `\n` 1 つ（ハード改行の印は、その行の最後の `Text` に付く。行末がインラインの要素（`**a**` の後ろに半角空白 2 つ など）のときだけ、区間が空の `Text` に付く。実測。4.78.0 で直した） |
| インラインコード | 中の文字。**変換器が `code` の中に書く文字と同じにする**——エスケープも文字参照も解かず（`\*` は 2 文字、`&amp;` は 5 文字）、行の改行は半角空白 1 つ（CommonMark のコードスパン。子の `Text` は `IsRaw()` が真。実測。4.78.0）。**部品**（丸ごとにだけ囲める。中にタグを入れても文字として出るだけのため） |
| 数式（IMP-113） | **1 文字（U+FFFC）**。部品。`Source` は前後の `$` 1 つずつを含む（続けて書いた数式（`$x$$y$`）の区間を重ねない。4.86.0。UT-220 のケース 30） |
| 画像（生 HTML の `<img>` と、読み込みに失敗した画像を含む） | **1 文字（U+FFFC）**。部品 |
| 脚注の参照 | 1 文字（U+FFFC）。部品 |
| 脚注の戻りリンク | **2 文字（U+00A0 と `↩`（U+21A9））**（変換器が前に `&#160;` を 1 つ書く。構文木には無い）。**部品**とし、`Source` はどちらも長さ 0（段落の終わり）。**選択にかかったら選択から外す**（IMP-124 の 3。4.78.0。利用者の判断） |
| 絵文字ショートコード | 絵文字の値の符号位置。部品 |
| 自動リンク（`<https://…>`、`www.…` などの裸の URL） | 中の文字。部品（中にタグを入れるとリンクの範囲が変わる） |
| 生 HTML のタグ | 0（中の文字は数える） |
| タスクのチェックボックス | 0 |
| 入れ子のブロックを持つリストの項目 | 文字の後ろの改行を数えない（変換器が書く。構文木には無い） |

- **1 文字と数える部品は、U+FFFC（オブジェクト置換文字）で表す**（4.77.0。利用者の判断。実現性検討の試作と同じ）。見える文字は文字列として 1 文字ずつ照合する（IMP-124 の 2、BR-054）ため、**数だけでなく文字も画面側（IMP-266）と同じにする。** 脚注の戻りリンクは、画面に出ている文字（U+00A0 と `↩`）そのものとする（変換器の `WithFootnoteBacklinkHTML("&#x21a9;")`。IMP-111）。本文に U+FFFC そのものを書いた場合も同じ文字になるが、位置の対応（`Source`）には影響しない。
- **対象のブロック**は IMP-120 の修飾できるブロックと、表のセルである。リストの項目は、**最初の子の `TextBlock`** の中だけを数える（入れ子のリストやコードブロックと、2 つ目以降の `TextBlock` を含まない。IMP-120。4.78.0）。
- 生 HTML のタグの対（`<del>…</del>`・`<span style="color: …">…</span>` など）は、**同じブロックの中で開きと閉じがそろっているものだけ**を `Inlines` に入れる。そろわないタグは対として扱わない（0 文字のまま）。`color` は、`style` が `color` だけで、値が `TextColors` の Light の値（大文字と小文字・コロンの後の空白の揺れを許す）であるものに限る（IMP-116 が通すものと同じ。IMP-124）。**末尾の `;` は許し、`!important` は含めない。判定は IMP-116 のサニタイズの前の走査と同じ関数で行う**（4.77.0。そろわないと、画面で色が付くものと Go が色と読むものが食い違う）。
- **サニタイズで中身ごと消える生 HTML**（`<script>…</script>` など）は、Go 側では数え、画面には無い。そのブロックは食い違いとして検出され、修飾できないだけで済む（IMP-124 の 2。FR-146）。**数え方を合わせるために、サニタイズの結果を解き直さない。**
- **生 HTML で、画面側の数え方の対象と同じ形を作ったもの**（行の中の `<input type="checkbox">`、`a.footnote-ref` を含む `sup` など）も、Go 側はタグを 0 文字・中身と後ろの文字をそのまま数えるため、画面側（IMP-266）と食い違う（`visible.js` は鍵を照合しない）。**`<script>` と同じく設計どおりの食い違い**であり、そのブロックが修飾できないだけで済む（4.78.0。UT-220 のケース 18）。
- **`Inlines` の `Open` / `Close` は、子の要素のソース上の範囲から下から順に求める**（4.86.0。独立したレビューの指摘。UT-220 のケース 15・26・27）。強調と取り消し線の記号は、中身の範囲（子の要素の記号・タグを含む）のすぐ外に並ぶ（goldmark は開きの記号の並びの**後ろ**と、閉じの並びの**前**を使う）。記号の数は、強調なら段、取り消し線なら前後に並ぶ `~` の数（どちらも 2 つ以上なら 2、そうでなければ 1）。リンクと画像の開きは goldmark の `Pos()`、閉じは IMP-111 の記録（`]` から構文の終わりまで）、インラインコードは開きのバッククォートの並びと、その後ろで最初に現れる同じ長さの並びとする。**中身の最初の文字の直前の記号を数える方法では、中身の端がリンク・コード・生 HTML のタグ・別の強調のときに別の記号を指す**（`~~**a**~~` の取り消し線が `**` を指し、手書きの取り消し線を外すと太字の記号が消える。IMP-124 の 4・5 がこの区間を使う）。**`Inlines` は入れ子の外側から並べ、中身が同じなら、記号・タグがソースで先に出る方を外側とする**（`***a***` の斜体は太字の外、`<del>**a**</del>` の `del` は太字の外）。
- **閉じの記号の残りの文字は、使われた記号の後ろに置く**（4.86.0。UT-220 のケース 28）。goldmark は閉じの記号の並び（`*a**` の `**`）の前を強調に使い、残りを文字のノードにするが、その区間を並びの頭に置いたままにする（開きの残りは正しい）。そのままでは残りの文字の `Source` が強調の閉じの記号と重なり、そこへの差し込みが記号を壊す。
- **前処理（Front Matter の扱い）は `Render` と `Locate` と同じ関数を使い、`Source` はずれを戻したソース上の位置とする**（IMP-121。4.78.0 で明記した。UT-220 のケース 24）。違えると、TOML や閉じの無い YAML の文書で `text:` の番号か位置がずれ、修飾が働かない。
- **描画スモーク（BR-054）が、全ブロックについて画面側の数え方（IMP-266）と `VisibleTexts` が一致することを確かめる**（Chromium だけ。WebKitGTK との差は手動テスト）。実現性検討の試作では、`testdata/showcase.md` ほか 3 文書の 348 ブロックのうち 347 で一致し、残る 1 つは `<script>` を含む段落だった（設計どおりの食い違い）。
- `TextColors` は `renderer` に置く。サニタイズ（IMP-116）と書き込み（IMP-124）の両方が使い、`document` → `renderer` の向きでしか参照できない（IMP-012）。**フロントエンドの色の一覧（IMP-265）と CSS のトークン（DSP-010）にも同じ値を書く**——名前と値は FR-147 の表が正である。
- 変換と同じく、パニックは `recover` してエラーとして返す（IMP-022）。

### IMP-126: PlantUML ファイルの変換 **MUST**

FR-025 / MD-086 を実装する（4.75.0）。配置は `internal/renderer/plantumlfile.go`。

```go
// Diagram は PlantUML ファイルの中の図 1 つ（MD-086）。
type Diagram struct {
    Name   string // 見出し（MD-086 の 3）
    Span   Span   // @start の行の頭から、@end の行の終わり（改行の手前）まで。@end が無ければファイルの終わりまで
    Closed bool   // @end の行があったか
}

// SplitPlantUML はファイルの中身（正規化後）を図に切り分ける（MD-086）。
func SplitPlantUML(text []byte) []Diagram

func (r *Renderer) RenderPlantUMLFile(source []byte, opts Options) (Result, error)
```

**切り分け**（MD-086）

1. 行頭の空白・タブを除いた後ろが `@start<種類>` で始まる行で図が始まる（大文字小文字を区別しない。`<種類>` は英字と数字の並び）。同じ `<種類>` の `@end<種類>` で始まる行で終わる。**図の中に別の `@start…` があっても、その図の `@end…` まで図とする。**
2. 図の外の行は無視する。
3. `@end…` が無いまま終わった図は、ファイルの終わりまでを 1 つの図とする（`Closed` が偽。MD-086 の 4。描けるかは処理系次第で、シーケンス図は描けない表示になる。MD-083）。
4. **見出し**: `@start…` の後ろの名前（`@startuml name` の空白の後ろ、`@startuml(id=name)` の `id` の値。前後の空白を除く）→ 図の中の最初の `title <文字>` の行（行頭の空白を除き、大文字小文字を区別しない。複数行の `title` … `end title` の形は使わない）→ `Diagram <n>`（n は 1 起点の図の番号）。

**出力**（図ごとに、見出しと図のブロックを上から並べる）

```html
<h2 id="user-content-sequence">sequence</h2>
<div class="code-block" data-lang="plantuml" data-plantuml="1"
     data-ref="0123456789abcdef:plantuml:0"
     data-source="@startuml sequence&#10;Alice -&gt; Bob&#10;@enduml">
  <pre class="plantuml-source">@startuml sequence
Alice -&gt; Bob
@enduml</pre>
</div>
```

- **図のブロックの形は、Markdown の中の PlantUML のブロック（IMP-119）と同じにする。** フロントエンドの描画・コピー・原寸表示・拡大画面・テーマへの追従・目印の照合（IMP-260）が、そのまま働く（FR-025）。
- **`data-source` は図の `Span` の中身**（`@start…` と `@end…` の行を含む）とする。**処理系へは図ごとに渡す**——処理系は 1 回の描画で先頭の図しか描かず、先頭に空行があるだけで描けない図になる（MD-086。実測）。
- **取り込み指令は図ごとに検査する**（`hasIncludeDirective`。IMP-119）。拒んだ図は `data-plantuml` を付けず `data-puml-error="include"` を付ける（見出しは付ける）。`NeedsPlantUML` は、`data-plantuml` を付けた図が 1 つ以上あるときだけ立てる（IMP-119 と同じ）。
- 目印は `<鍵>:plantuml:<n>`（n は図の番号。0 起点）。**拒んだ図も数える**（IMP-119 と同じ）。PlantUML ファイルの図の書き換え（IMP-122）が同じ番号で引く。
- **見出し**は `h2` とし、`Heading{Level: 2}` をアウトライン（FR-040）へ返す。アンカーは IMP-117 の `slugger` で作る（同じ名前の図には `-1` … を付け、`user-content-` を付ける。AR-053）。見出しの文字は HTML としてエスケープする（**Markdown として解かない**）。
- **図が 1 つも無いファイル**は、ソースの全体を等幅のコードブロック（`<div class="code-block" data-lang="plantuml"><pre><code>…</code></pre></div>`。目印と `data-plantuml` は付けない）として出し、`Result.NoDiagrams` を真にする（FR-025）。**`No diagrams found.` の文言はフロントエンドが出す**（IMP-220, IMP-290。UI 文言を Go 側に書かない）。
- 修飾の目印（`text:<n>`）は付けない（FR-146 の対象外）。`Options.TextRefs` は無視する。
- 出力は IMP-116 のサニタイズを通す（迂回経路を作らない。AR-031）。パニックは `recover` してエラーとして返す（IMP-022）。

### IMP-127: Mermaid ファイルの変換 **MUST**

FR-026 / MD-087 を実装する（4.75.0）。配置は `internal/renderer/mermaidfile.go`。

```go
func (r *Renderer) RenderMermaidFile(source []byte, opts Options) (Result, error)
```

- **ファイル全体を 1 つの図とし**、Markdown の中の Mermaid のブロック（IMP-115）と同じ形の `div.code-block` を 1 つだけ出す（`data-lang="mermaid"`・`data-mermaid="1"`・`data-ref="<鍵>:mermaid:0"`・`data-source`）。`NeedsMermaid` を立てる。
- **`data-source` は、先頭の空行（空白とタブだけの行）を除いたもの**とする（MD-087 の 2）。除くのは表示のときだけで、ファイルは変えない。書き換え（IMP-122）の編集欄にはファイル全体を出す。
- **中身が空白だけのファイル**は、図を出さず（`HTML` は空）、`Result.NoDiagrams` を真にする（MD-087 の 3）。
- 見出しは付けない（`Headings` は空配列。MD-087 の 4）。修飾の目印は付けない。
- **Go 側は Mermaid の構文を解かない**（AR-031）。図が 2 つある・MultiMarkdown の文書である、などは処理系が構文エラーとして示す（FR-023, FR-026）。
- 出力は IMP-116 のサニタイズを通す。パニックは `recover` してエラーとして返す（IMP-022）。

### IMP-128: 画像ファイルの変換 **MUST**

FR-027 を実装する（4.75.0）。配置は `internal/renderer/imagefile.go`。

```go
// RenderImageFile は画像ファイルを 1 枚だけ表示する HTML を返す。
// version は localurl.Version で作ったファイルの版（IMP-118）。
func (r *Renderer) RenderImageFile(path, version string) (Result, error)
```

```html
<div class="image-file"><img src="/__local/<エスケープ済み絶対パス>?v=<版>" alt="<ファイル名>"></div>
```

- URL は `localurl.Encode`（IMP-118 と同じ）で組み立て、**版をクエリで付ける**（IMP-118）。外部で画像を書き換えたら、監視の再描画（FR-014）で版の変わった URL が出て、WebView が取り直す（FR-027, BUG-026）。
- **`alt` はファイル名（ベース名）**とし、HTML の属性値としてエスケープする。読み込みに失敗したら、埋め込み画像と同じ壊れた画像の表示に代替テキストとして出る（IMP-226, DSP-123）。
- 中身は読まない（IMP-102）。寸法も求めない（フロントエンドの `naturalWidth` / `naturalHeight`。IMP-228）。
- **画像ファイルかどうかの判断は `DocumentDTO.kind` で行い、`class="image-file"` の有無では行わない**（IMP-228）。生 HTML でも同じ `class` を書ける（IMP-116 が通す）。害はボタンが出るだけだが、目印と同じ規則（IMP-260）を崩さない。
- 出力は IMP-116 のサニタイズを通す（`/__local/` の `src` は許可されている）。見出し・目印は持たない。

## 11.4 filetree パッケージ（IMP-130 系）

### IMP-130: 型定義 **MUST**

```go
package filetree

type Node struct {
    Name     string `json:"name"`
    Path     string `json:"path"`     // 絶対パス
    IsDir    bool   `json:"isDir"`
    Kind     mdfile.Kind `json:"kind"` // ファイルの種類（IMP-135 の Classify）。ディレクトリは空（4.75.0。UI-030 のアイコン）
    Children []Node `json:"children"` // 未読込のディレクトリでは nil
    Loaded   bool   `json:"loaded"`   // 子を読み込み済みか
    Omitted  int    `json:"omitted"`  // 件数上限で除かれた数。0 なら全件（FR-032）
}
```

- `Omitted` は、**その要素が属する一覧から件数上限で除かれた数**である。切り詰めが起きた場合、`ReadDir` は返すすべての要素に同じ値を入れる。
  一覧に対する値を要素側に持たせているのは、`ReadDir` が返すのが子の並びだけで、親を表す値を返さないためである。すべてに入れるので、並べ替えても値が失われない。フロントエンドは先頭の要素を見て、一覧の末尾に `… and N more` を表示する（FR-032, DSP-112）。
  真偽値ではなく件数を持つのは、表示に N が要るためである。切り詰めの有無だけを返すと、フロントエンドは省略された件数を組み立てられない。

### IMP-131: 読み込み **MUST**

```go
const MaxEntriesPerDir = 1000 // FR-032

// ReadDir は dir の直下のみを読み込む。再帰しない（FR-032）。
// show の種類だけを並べる（FR-037。IMP-134）。件数の上限は絞り込み後の件数に対して適用する。
func ReadDir(dir string, show Show) ([]Node, error)

// PathTo は root から target に至る経路上のディレクトリを順に返す。
// 表示中ファイルまでの自動展開（FR-032）に用いる。target 自身は含めない。
// target がツリー外にある場合は ErrOutsideRoot を返す。
var ErrOutsideRoot = errors.New("target is outside the tree root")

func PathTo(root, target string) ([]string, error)
```

- `PathTo` はファイルシステムに触れない。与えられたパスを絶対パスとみなし、`filepath.Clean` だけを行う。存在しないパスでも経路を計算できるほうが、呼び出し側で扱いやすい。
- **要素の種類（ディレクトリか、どの種類のファイルか）は `Classify`（IMP-135）で決める**（4.75.0）。ツリーの操作の検査と同じ判定を使い、**ツリーに出ているものは開ける・展開できる**ようにする（FR-036）。**`Classify` が失敗した項目（宛先の無いシンボリックリンク、リンク先を調べる権限が無いもの）はツリーに出さず、ディレクトリの他の項目は返す**（リンク 1 つでディレクトリ全体を失敗にしない。v1.1.0 は名前だけで出していた。4.78.0。利用者の判断。UT-301 のケース 12）。

### IMP-132: フィルタ規則 **MUST**

FR-031 を実装する。

```go
var excludedDirs = map[string]bool{
    "node_modules": true, "vendor": true, ".git": true,
    "target": true, "dist": true, "build": true,
}

// include は名前の規則でエントリを表示対象とするか判定する（種類は見ない）。
func include(name string, isDir bool) bool
```

- 名前が `.` で始まるものを除外する。
- `excludedDirs` に一致するディレクトリを除外する。
- ファイルは、**種類（`Classify`。IMP-135）を `Show`（IMP-134）が含むもののみ**含める（4.75.0。FR-031, FR-037）。v1.1.0 は `mdfile.IsMarkdown` が真のものだけだった。
- **同じ規則をツリーの操作の検査（IMP-135 の 3）でも使う。** 2 か所に書くと、ツリーに出ているのに開けないもの、出ていないのに開けるものが生まれる。
- 並び順はディレクトリ優先、次に名前の昇順（大文字小文字を区別しない比較）。

### IMP-133: 空ディレクトリの判定 **SHOULD**

FR-031 の「表示している種類のファイルを 1 つも含まないディレクトリは表示しない」を、FR-032 の遅延展開と両立させる。

```go
// hasShownWithin は dir 直下と、その 1 階層下までを調べ、
// show の種類のファイルが存在する可能性があるかを返す（4.75.0。v1.1.0 の hasMarkdownWithin）。
// 全階層は走査しない（FR-032 の NOTE）。
func hasShownWithin(dir string, show Show) bool
```

- **先読みの判定は、ファイルを並べる判定（IMP-132）と同じ `Show` で行う**（FR-037）。画像を表示していないときに、画像しか無いディレクトリを出さない。**先読みの深さ（1 階層下まで）は変えない。**

判定できなかった深い階層のディレクトリは表示し、展開したときに空であることが分かる状態を許容する。

### IMP-134: 表示する種類 **MUST**

FR-037 を実装する（4.75.0）。

```go
// Show はツリーに表示する種類（FR-037）。Markdown は常に表示する（外せない）。
type Show struct {
    Diagrams bool // 図のファイル（PlantUML・Mermaid）
    Images   bool // 画像ファイル
}

// Includes は種類 k のファイルを表示するかを返す。KindNone は常に偽、KindMarkdown は常に真。
func (s Show) Includes(k mdfile.Kind) bool
```

- **絞り込みは Go 側で行い、フロントエンドでは行わない。** 理由は 3 つある。
  1. ディレクトリを出すかの先読み（IMP-133）が種類に依存する。フロントエンドは、読み込んでいないディレクトリの中身を知らない。
  2. 1000 件の上限（FR-032）は、絞り込んだ後の件数で数えなければならない（画像を隠しているのに、画像の数で上限に掛からないように）。
  3. ツリーの操作の検査（IMP-135）が「ツリーに出しているものだけを受け付ける」ためには、検査する Go がフィルターを知っている必要がある。
- **`desktop` は `App.cfg` の値から `Show` を作って渡す**（IMP-196）。フロントエンドから種類を受け取らない。フィルターの切り替えは設定（`UpdateConfig`。IMP-303）として届き、その後の `ReadDir` から効く。
- 既定は `Diagrams` が真、`Images` が偽（UI-110。IMP-151 の `Default()`）。

### IMP-135: ツリーの操作の検査 **MUST**

FR-032 / FR-033 / FR-036 を実装する（4.75.0）。**ツリーからパスを受け取る操作は、すべてこの関数を通す**（開く・展開・エディタで開く・ファイルマネージャで開く・親ディレクトリを開く・ツリールートにする・パスのコピー。IMP-196）。フロントエンドが渡す値は、形が正しくても信じない（NFR-030, NFR-035）。

```go
// Want は操作が求める種類（FR-036）。
type Want int

const (
    WantOpenable Want = iota // 開く（FR-033、メニューの Open）。開けるファイル
    WantText                 // Open in editor...。文字のファイル（画像を含めない）
    WantDir                  // 展開・Open in file manager・Set as tree root。ディレクトリ
    WantAny                  // Copy path。表示しているファイルかディレクトリ（Open containing folder はファイルなので WantOpenable）
)

var (
    ErrNotInTree = errors.New("path is not shown in the tree") // 何もしない（通知しない）
    ErrMissing   = errors.New("path no longer exists")          // not-found を出す（FR-036）
)

// Check は path がいまのツリーに出ているもので、want の種類であるかを確かめ、
// filepath.Clean したパスを返す。allowRoot が真なら、ツリールートそのものも受け付ける。
func Check(root, path string, show Show, want Want, allowRoot bool) (string, error)

// Classify は path の種類を、ツリーの表示（IMP-131）と同じ規則で決める。
func Classify(path string) (isDir bool, kind mdfile.Kind, err error)
```

**検査の順序**（FR-036。どこで落ちても何も起動しない）

| # | 確かめること | 落ちたとき |
| --- | --- | --- |
| 1 | `root` が空でなく、`path` が絶対パスである | `ErrNotInTree` |
| 2 | **ツリー上のパス**（`filepath.Clean` だけ。シンボリックリンクを解決しない）が `root` の中にある（`filepath.Rel` が `..` で始まらない）。`root` そのものは `allowRoot` が真のときだけ受け付ける | `ErrNotInTree` |
| 3 | **`root` からの相対パスの各部分が、ツリーに表示する規則（IMP-132 の `include`）に当てはまる**——名前が `.` で始まらない、既定で隠すディレクトリでない。**`root` 自身の名前と、`root` より上には当てはめない**。**最後の部分も、ディレクトリとして当てはめる**（`include(name, true)`。ここではファイルシステムに触れず、ディレクトリかどうかを知らない。既定で隠すディレクトリと同じ名前のファイルは拡張子が無く開けないため、結果は変わらない。4.78.0。UT-308 のケース 24） | `ErrNotInTree` |
| 4 | `Classify(path)` で種類を決める。存在しない（`fs.ErrNotExist`）/ それ以外の失敗（権限が無い等。4.76.0） | `ErrMissing` / その `err` を包んで返す（IMP-196 が `permission` にする） |
| 5 | 種類が `want` に合い（下の表）、ファイルならその種類を `show` が含む（IMP-134。フィルターで隠した種類を受け付けない）。**ディレクトリかファイルかはリンク先の実体で見る** | `ErrNotInTree` |

| `want` | 合う種類 |
| --- | --- |
| `WantOpenable` | ファイルで、種類が `KindNone` でない |
| `WantText` | ファイルで、種類が Markdown か図（`mdfile.IsText`。画像は合わない。FR-027, FR-090） |
| `WantDir` | ディレクトリ |
| `WantAny` | ディレクトリか、種類が `KindNone` でないファイル |

**`Classify` の規則**（ツリーの表示とツリーの操作で同じ判定を使う。4.74.0 の利用者の判断）

- ディレクトリかファイルかは、**`os.Stat`（シンボリックリンクを解決した実体）**で決める。**名前付きパイプ・デバイス・ソケットは種類なしとする**（IMP-102 の読み込みの判定と同じ。**`Mode().IsRegular()` では判定しない**——Go 1.23 から、Windows の再解析ポイント（シンボリックリンクと重複除去以外。OneDrive のクラウドのファイルなど）は `ModeIrregular` になり、v1.1.0 でツリーに出ていたファイルが消える。4.88.0 で 4.87.0 の利用者の判断にそろえた。それまでは「通常のファイルでもディレクトリでもないもの（デバイスなど）」と書いていた）。**ファイルマネージャで開く経路は、ここでディレクトリと分かったものだけを OS へ渡す**（NFR-035。ファイルを渡すと、実行ファイルなら実行される）。
- ファイルの種類は、ツリー上の名前の拡張子（`mdfile.KindOf`）とする。**ただしシンボリックリンクなら、実体（`filepath.EvalSymlinks`）の拡張子の種類が同じであるときに限る**（違えば種類なし）。名前と実体の種類が食い違うリンクを、開ける種類として扱わない。
- **ツリールートの外を指すシンボリックリンクも、ツリーに出ているものなら受け付ける**（2 と 3 はツリー上のパスで見る。4.74.0 の利用者の判断）。実体のパスで見ると、v1.1.0 まで開けていたリンクが開けなくなる。

- **ファイルが既に無い（`ErrMissing`）ときは、何もしないのではなく `not-found` を出す**（FR-036, FR-110）。ツリーは自動では更新されないため（FR-035）、消したファイルが並んでいることがある。`ErrNotInTree` は何もしない（通知しない）。
- 検査してから使うまでの間に、ディレクトリがファイルへすり替わる余地は残る。利用者自身のディスク上の操作であり、許容する（NFR-030 が想定するのは文書の書き手である）。ファイルマネージャで開く経路は、起動の直前にもう一度ディレクトリであることを確かめる（IMP-173）。
- **ファイルシステムに触れるのは 4 だけ**とし、1〜3 はパスの文字列だけで決める（単体テストで、パスの規則と種類の規則を分けて確かめられる）。

## 11.5 watcher パッケージ（IMP-140 系）

### IMP-140: 型定義とライフサイクル **MUST**

FR-014 / AR-070 を実装する。

```go
package watcher

type EventKind int

const (
    Modified EventKind = iota
    Removed
)

type Event struct {
    Path string
    Kind EventKind
}

type Watcher struct { ... }

// New は監視を開始する。ctx のキャンセルで内部ゴルーチンを終了する（IMP-024）。
func New(ctx context.Context) (*Watcher, error)

// Watch は監視対象を path 1 つに切り替える。以前の対象は必ず解除する。
// 監視対象は常に 1 つ以下であること（NFR-020）。
func (w *Watcher) Watch(path string) error

// Unwatch は監視を解除する。
func (w *Watcher) Unwatch()

// Events は通知チャネルを返す。デバウンス後のイベントのみが流れる。
// バッファは 1 とし、受け手が取り出す前に次のイベントが出たら、新しい値で置き換える。
func (w *Watcher) Events() <-chan Event

func (w *Watcher) Close() error
```

- **送信で止まらない。** 通知を受ける側（IMP-192 の読み直し、IMP-195 の削除）は `ioMu` を待つため、取り出しが秒単位で遅れうる。送信で止まると、監視のゴルーチンが fsnotify のイベントを読まなくなる。**fsnotify v1.10.1 の Windows 実装は、イベントの送信が詰まっている間、`Add` / `Remove` の要求も処理しない**（同じゴルーチンで処理する）。その間に文書を開く処理が `ioMu` を持ったまま `Watch` を呼ぶと止まり、`mu` を取るバインドメソッドがすべて止まる（FR-111）。**v1.0.0 はバッファの無いチャネルで送っていた**（読み込みと変換が `ioMu` の外にあり、窓が狭かった）。
- 置き換えてよいのは、監視対象が常に 1 つで（NFR-020）、受け手が必要とするのは「いまファイルがあるか」の最新の状態だけだからである。`Modified` の後の `Removed` も、その逆も、後の値だけで正しく扱える。
- `Watch` / `Unwatch` は `App.mu` の外で呼ぶ（IMP-192）。

### IMP-141: 監視方式 **MUST**

- fsnotify では**対象ファイルの親ディレクトリを監視し**、イベントのファイル名が対象と一致するものだけを拾う。ファイル単体の監視は、エディタの「一時ファイル作成 → リネーム」保存で監視ハンドルが外れるため採用しない（FR-014）。
- 親ディレクトリの監視で拾ったイベントのうち、対象ファイル以外のものは破棄する。ツリーの更新契機には使わない（FR-035）。**編集モードの一時ファイル（`.<名前>.markview-*.tmp`。IMP-107）もここで捨てられる。**
- **監視するのは、シンボリックリンクを解決した実体のあるディレクトリとする**（AR-070）。`Watch` は受け取ったパスを `filepath.EvalSymlinks` で解決してから親ディレクトリを求め、イベントのファイル名も実体の名前と比べる。リンクの置き場所を監視すると、実体への書き込み（外部のエディタ、IMP-107）を拾えない。解決に失敗した場合は、受け取ったパスのまま監視する。**v1.0.0 の実装は解決していない**（4.41.0 で足した規則）。
- **`Event.Path` は `Watch` に渡したパスとする**（リンクを開いていればリンクのパス）。実体のパスを載せると、`openFromReload` で開き直したときに画面の対象（`target`）とウィンドウタイトルがリンクのパスから実体のパスへ変わる（IMP-190）。

### IMP-142: デバウンス **MUST**

```go
const debounceInterval = 150 * time.Millisecond // FR-014
```

- 対象のファイル名のイベントを受けるたびに、**種類を問わず**タイマをリセットする。
- 最後のイベントから 150 ms 追加のイベントがなければ、**その時点で対象ファイルの存在を確かめ**、存在すれば `Modified`、存在しなければ `Removed` を 1 件送出する。これにより、リネーム型の保存（`Remove` / `Rename` の後に `Create`）を削除と誤認しない。

## 11.6 config パッケージ（IMP-150 系）

### IMP-150: 型定義 **MUST**

UI-110 を実装する。

```go
package config

type Config struct {
    Theme           string `json:"theme"`           // "light" | "dark"
    OutlineVisible  bool   `json:"outlineVisible"`
    FileTreeVisible bool   `json:"fileTreeVisible"`
    OutlineWidth    int    `json:"outlineWidth"`
    FileTreeWidth   int    `json:"fileTreeWidth"`
    WindowWidth     int    `json:"windowWidth"`
    WindowHeight    int    `json:"windowHeight"`
    Editor          string `json:"editor"`          // 実行ファイルの絶対パス（UI-116）

    // ツリーのフィルター（FR-037。4.75.0）。Markdown は常に表示するため項目を持たない
    TreeShowDiagrams bool `json:"treeShowDiagrams"` // 既定 true
    TreeShowImages   bool `json:"treeShowImages"`   // 既定 false
}
```

`Editor` だけ性格が異なる。**他の項目は「表示の快適性に関わる値」だが、これは「起動されるプログラム」である**（NFR-035）。絶対パス以外を保持しないことを `Normalize` で保証する（IMP-153）。

**次のフィールドを定義しない。** 構造体に存在しなければ、保存も復元も起こり得ない（UI-111）。

| 定義しないもの | 根拠 |
| --- | --- |
| ウィンドウ位置（`X`, `Y`） | UI-111。起動時は常にプライマリモニタの中央 |
| 表示倍率（`Zoom`） | UI-111, UI-115。セッション内の値であり、フロントエンドだけが持つ（IMP-242） |
| 最大化状態（`WindowMaximized`） | UI-111, UI-115。起動時は常に通常状態 |
| 編集モードの状態（`EditMode`） | UI-111, FR-140。起動時は常に編集モードでない。多重起動で後勝ちになる値にしない |
| 取り消し履歴・表の並べ替え・原寸表示・拡大画面の状態 | UI-111, NFR-042。取り消し履歴は文書の断片を含む。いずれも Go 側（IMP-108）かフロントエンドのメモリ上だけに持つ |
| ツリールート・ツリーの展開の状態 | FR-030, NFR-042。**ツリーについて保存するのは、ペインの表示と幅と、フィルターの 2 つ（種類ごとの表示・非表示）だけ**である（4.75.0）。パスでも閲覧の痕跡でもない |

倍率と最大化状態は「保存しないだけ」であり、セッション内では機能する。**保存しないことを構造で保証する**という点で、ウィンドウ位置と同じ扱いにする。

### IMP-151: 読み書き **MUST**

```go
// Default は UI-110 の既定値を返す。Theme は空文字とし、
// OS のテーマ設定に追従させる判断は呼び出し側が行う（FR-071）。
// TreeShowDiagrams は true、TreeShowImages は false（FR-037）。
func Default() Config

// Load は設定を読み込む。**エラーを返さない。**
// ファイルがない・壊れている・読めない場合は Default() を返す（UI-113）。
func Load() Config

// Save は設定を保存する。失敗しても動作は継続させるため、
// 呼び出し側はエラーを無視してよい（UI-113）。
func Save(c Config) error

// Normalize は範囲外の値を既定値へ丸める（UI-113）。
func (c *Config) Normalize()
```

- `Save` は同一ディレクトリの一時ファイルへ書き出し、`os.Rename` で置き換える（UI-112）。
- `Load` は JSON の部分的な欠落を許容する。`Default()` を初期値とした構造体へ `json.Unmarshal` することで、未指定項目に既定値が残る。**v1.1.0 までの設定ファイルにはツリーのフィルターの項目が無く、既定値（図は表示、画像は非表示）で始まる**（UI-113。4.75.0）。

### IMP-152: 保存先の解決 **MUST**

UI-112 を実装する。OS 差異はファイル名サフィックスで分ける（IMP-031）。

```go
// Dir は設定ディレクトリの絶対パスを返す。
//   Windows: %TEMP%\MarkView
//   Linux:   $TMPDIR/MarkView-<uid>  （TMPDIR 未設定時は /tmp/MarkView-<uid>）
func Dir() (string, error)

func Path() (string, error) // Dir() + "/config.json"
```

- Linux では `os.Getuid()` をディレクトリ名に含め、ディレクトリを `0700`、ファイルを `0600` で作成する。
- ディレクトリの作成に失敗した場合、`Dir` はエラーを返す。`Load` はそれを握り潰して `Default()` を返す。

### IMP-153: 値の範囲 **MUST**

```go
const (
    MinPaneWidth = 160  // UI-030, UI-040
    MinWindowW   = 640  // UI-011
    MinWindowH   = 480
)
```

**倍率の範囲（50〜300、10 刻み）は `config` に置かない。** 倍率は保存されず（UI-111）、`Normalize` の対象にならない。範囲と刻みは操作の上限・下限としてフロントエンドだけが持つ（IMP-242, FR-081）。

`Normalize` は、範囲外・ゼロ値・負値をすべて `Default()` の対応する値へ置き換える。**最小値・最大値へ切り詰めない。** 範囲外の値が保存されているのはファイルが壊れた場合であり、その値を元に復元するより既定値から始めるほうが確実である。

`Editor` は、**絶対パスでなければ空にする**（UI-116, NFR-035）。相対パスやコマンド名を残すと、`$PATH` や作業ディレクトリの内容によって起動されるプログラムが変わる。存在するかどうかはここでは見ない。設定を読むのは起動時の 1 回だけであり（UI-115）、その後にアンインストールされうるためである。**存在の確認は起動の直前に行う**（IMP-171）。

対象は数値と文字列に限る。**真偽値は対象としない**（ツリーのフィルターの 2 つも同じ。4.75.0）。 真偽値にはゼロ値と「利用者が false を選んだ状態」の区別がなく、ゼロ値を既定値へ戻すと、閉じたペインが毎回開くことになる（UI-113 の「一部の項目のみが存在する場合、欠けている項目は既定値を用いる」は、`Default()` を初期値として `json.Unmarshal` することで満たす。IMP-151）。ペイン幅の上限（ウィンドウ幅の 40 %）は実行時のウィンドウ幅に依存するため、フロントエンド側で制限する（IMP-240）。

## 11.7 assetsrv パッケージ（IMP-160 系）

### IMP-160: ハンドラ **MUST**

AR-040 / AR-041 を実装する。

```go
package assetsrv

const (
    LocalPrefix = "/__local/"
    AppIconPath = "/appicon.png" // アプリケーションアイコン（IMP-032）
)

type Handler struct {
    embedded fs.FS
    appIcon  []byte
}

func New(embedded fs.FS, appIcon []byte) *Handler

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request)

// URL の組み立てと解読は internal/localurl が持つ（IMP-012）。
//   localurl.Prefix                  = "/__local/"
//   localurl.Encode(absPath) string  // IMP-118 が使用
//   localurl.Decode(urlPath) (string, bool)  // 本パッケージが使用
//   localurl.Version(fi fs.FileInfo) string  // 画像の URL に付ける版（IMP-118, IMP-128。4.75.0）
//     更新時刻（UnixNano）と大きさを小文字の 16 進で "-" でつないだもの（"1000-a" など。4.77.0 で小文字と明記した）。配信の検査には使わない
```

配信するパスは以下の 3 系統に限る。

| パス | 配信内容 |
| --- | --- |
| `/appicon.png` | `go:embed` したアプリケーションアイコン（UI-025）。情報ダイアログが参照する |
| `/__local/<絶対パス>` | ローカルの画像ファイル（IMP-161 の検査を通す）。埋め込み画像（FR-022）と画像ファイルの表示（FR-027）が同じ経路を使う。クエリ（`?v=<版>`）は見ない |
| それ以外の `/` 配下 | 埋め込みのアプリ資産（HTML / CSS / JS / vendor） |

### IMP-161: ローカル配信の検査順序 **MUST**

`/__local/` へのリクエストは、以下の順で検査する。1 つでも失敗したら 404 を返し、理由を本文に含めない。

1. `localurl.Decode` でパスを取り出す（`r.URL.Path` ではなく `r.URL.EscapedPath()` を渡す）。失敗したら 404。
2. `filepath.Clean` と `filepath.Abs` で正規化する。
3. `filepath.EvalSymlinks` でシンボリックリンクを解決する。
4. **解決後のパス**の拡張子が画像のものかを `mdfile.ImageContentType`（IMP-105）で検査する。
5. `os.Stat` で通常ファイルであることを確認する。ディレクトリ・デバイスファイル等は拒否する。

- **拡張子と Content-Type の表は `mdfile.ImageTypes` に置く**（4.75.0。v1.1.0 は本パッケージの `allowedImageExt` が持っていた）。画像ファイルの表示（FR-027）で、配信する拡張子と開ける拡張子が同じ一覧になったためである（IMP-105）。**配信する範囲は広がらない**——拡張子は FR-022 の 9 つのまま、SVG も `<img>` からだけ参照するので中のスクリプトは動かない（AR-041）。

### IMP-162: 応答ヘッダ **MUST**

| ヘッダ | 値 | 根拠 |
| --- | --- | --- |
| `Content-Type` | 拡張子から決定（`mdfile.ImageTypes`。IMP-105。4.75.0 で `assetsrv` から移した） | AR-041 |
| `X-Content-Type-Options` | `nosniff` | AR-041 |
| `Content-Security-Policy` | `sandbox` | AR-041（SVG 対策） |
| `Cache-Control` | `no-store` | ローカルの画像を WebView のディスクのキャッシュに残さない。**古い画像が出続けるのを防ぐのは、URL の版である**（IMP-118。下の NOTE） |

埋め込み資産（`/` 配下）には `Cache-Control: public, max-age=31536000` を付けてよい。内容は実行ファイルに固定されているため。

- 埋め込み資産の `Content-Type` は**拡張子から自前の表で決める**。`mime.TypeByExtension` に任せない。Windows では拡張子と種別の対応をレジストリから引くため、`.js` が `text/plain` になる環境がある。CSS と JS の種別を誤ると画面が成り立たない。
- 埋め込み資産の配信に `http.FileServer` を使わない。`/index.html` を `/` へ 301 で書き換え、ディレクトリ一覧も返すためである。パスから直接ファイルを引き、見つからなければ 404 とする。

> [!NOTE]
> **v1.1.0 までは、`no-store` の根拠を「更新した画像が古いまま表示されるのを防ぐ」としていた。この前提は成り立たなかった**（[BUG-026](../bugs/2026-11-08-bug-026-stale-embedded-image.md)。4.75.0）。WebView2 も WebKitGTK も、ページの中で一度読んだ画像を、`no-store` でも同じ URL なら使い回す（ヘッドレスの Edge で測り、W1 / L1 の実機でも起きた）。MarkView はページを遷移しないため、画像を書き換えて `F5` を押しても、ウィンドウを閉じるまで古い画像が出続けた。**URL にファイルの版を付けて直す**（IMP-118, IMP-128）。`no-store` は外さない。

## 11.8 opener パッケージ（IMP-170 系）

### IMP-170: 外部委譲 **MUST**

FR-050 / FR-036 を実装する。Wails に依存しない（IMP-012）。**利用者が選んだエディタの起動（FR-090）と、ディレクトリをファイルマネージャで開くこと（FR-036）も本パッケージに置く**（IMP-171, IMP-173）。プロセス起動の実装を 1 か所に保つためである。`internal/` 同士の依存は 2 系統に限られており（IMP-012）、別パッケージからは呼べない。

```go
package opener

// 番兵エラー（IMP-021）。
var (
    ErrUnsupportedScheme = errors.New("unsupported URL scheme") // http / https / mailto 以外（NFR-030）
    ErrNotFound          = errors.New("file not found")          // 開く対象、またはエディタの実行ファイルが無い
)

// OpenURL は既定ブラウザで URL を開く。http/https/mailto のみ受け付ける。
func OpenURL(rawurl string) error

// OpenFile は既定のアプリケーションでファイルを開く（FR-050 の「その他のファイル」）。
func OpenFile(path string) error
```

**外部プログラムを起動する経路は、この 4 つに限る**（NFR-035。4.73.0 で 3 つから 4 つになった）。**OS の設定の問い合わせ（Linux の `gsettings`。IMP-175）はこの数に入れない**——コマンドも引数も固定であり、NFR-035 が例外として認めている（4.81.0）。

| 経路 | 関数 | 契機 |
| --- | --- | --- |
| 既定のブラウザ | `OpenURL` | 本文中の `http` / `https` / `mailto` のリンク（FR-050） |
| 既定のアプリケーション | `OpenFile` | 本文中の、**開けないファイル**へのリンク（FR-050。`.iuml` を含む）。**画像へのリンクは 4.73.0 から MarkView で開き、ここを通らない**（FR-053） |
| 既定のファイルマネージャ | `OpenDir`（IMP-173） | ファイルツリーの右クリックメニュー（FR-036）。**ディレクトリだけ** |
| 利用者が選んだエディタ | `OpenWith`（IMP-171） | エディタで開く（FR-090, FR-036） |

| OS | 実装 |
| --- | --- |
| Windows | `rundll32.exe url.dll,FileProtocolHandler <target>` を `exec.Command` で起動する。`cmd /c start` はクォート処理の差異で誤動作しうるため用いない |
| Linux | `xdg-open <target>` を起動する。存在しない場合はエラーを返し、呼び出し側がステータス表示する |

- 引数は必ず `exec.Command` の可変長引数として渡し、シェルを経由しない。文字列連結でコマンドを組み立てない。
- URL は事前にスキームを検査し、`http` / `https` / `mailto` 以外を `ErrUnsupportedScheme` で拒否する。
- **失敗（`ErrUnsupportedScheme`・`ErrNotFound`・プロセスを起動できない）は、リンクの経路とファイルマネージャの経路では `open-failed` として伝える**（IMP-312, IMP-315, IMP-196）。**文書の変換の失敗（`render-error`）と同じ種別にしない**——フロントエンドは `render-error` を状態画面として出し、本文が消える（[BUG-013](../bugs/2026-09-14-bug-013-link-open-failure-state-screen.md)。FR-050 と FR-110 はステータス表示を求める）。

### IMP-171: エディタの起動 **MUST**

FR-090 と [NFR-035](07-nonfunctional.md) を実装する。

```go
// 番兵エラー（IMP-021）。ErrNotFound は IMP-170 のものを使う。
var (
    ErrNotAbsolute = errors.New("editor path must be absolute")
    ErrSelf        = errors.New("MarkView cannot be used as an editor")
)

// OpenWith は指定した実行ファイルでファイルを開く（FR-090）。
//
// editor / path はいずれも絶対パスであること。渡す引数は path 1 つだけとする。
func OpenWith(editor, path string) error
```

検査の順序は次のとおり。**どれかで落ちたら起動しない。**

| # | 検査 | 返すエラー | 根拠 |
| --- | --- | --- | --- |
| 1 | `editor` が絶対パスである | `ErrNotAbsolute` | NFR-035 の 5 |
| 2 | `editor` が存在する | `ErrNotFound` | UI-116 |
| 3 | `editor` が MarkView 自身でない | `ErrSelf` | NFR-035 の 6 |
| 4 | `path` が絶対パスで、存在する | `ErrNotFound` | — |
| 5 | `exec.Command(editor, path).Start()` が成功する | 起動時のエラー | — |

`path` は **エディタ選択ウィンドウを開いた時点で `App` が控えた対象**（`editorTarget`。IMP-190。ツールバーからなら画面の対象 `target`、ファイルツリーからなら IMP-135 で確かめたファイル。4.75.0）であり、**`OpenInEditor` の呼び出しでフロントエンドから受け取らない**（NFR-035 の 2 と 3）。**相対パスは、実在していても拒む。**

- **3 の比較は `filepath.EvalSymlinks` で両者を解決してから行う。** シンボリックリンクやジャンクション経由の指定を素通しさせない。`os.Executable()` を基準とする。
- 大文字小文字の扱いは IMP-025 に従う（Windows では区別しない）。ただし **`session.SamePath` は使えない。** `internal/` 同士の依存は 2 系統に限られており（IMP-012）、`opener` から `session` を呼べないためである。**同じ規則を `opener` の非公開関数として持つ。** 判定の内容が食い違わないよう、変更するときは両方を見る。
- 起動後は待たない。`OpenURL` / `OpenFile` と同じく、別のゴルーチンで `Wait` を呼んでゾンビを回収する（NFR-020）。
- **`OpenFile`（FR-050）と統合しない。** あちらは OS の既定アプリケーションへ委譲するもので、何が起動されるかを MarkView が知らない。検査すべき対象も内容も異なる。

### IMP-172: プリセットの検出 **MUST**

[UI-103](03-ui.md) の一覧を作る。

```go
// Editor は既知のエディタ 1 件。
type Editor struct {
    ID   string // "vscode" 等。**設定には保存しない**（UI-116）
    Name string // 画面に出す表示名。英語（UI-024）
    Path string // 見つかった絶対パス。見つからなければ空
}

// Editors は既知のエディタを定義順に返す。
// **見つからなかったものも Path が空の状態で含める**（UI-103）。
func Editors() []Editor
```

- **順序を定義順に固定する。** 見つかったものを前へ並べ替えない（UI-103）。並びが環境や起動のたびに変わると、位置で覚えられなくなる。
- **検出は `os.Stat`（Windows）と `exec.LookPath`（Linux）だけで行う。レジストリを読まない。** 既知のパスで足りるため、読む経路を増やさない。**NFR-033 が禁じるのはレジストリへの書き込みであり、読むことではない**（OS のテーマの取得は読む。IMP-175。4.81.0 で理由を改めた。4.80.0 までは「レジストリに触らない（NFR-033）」としていた）。
- **`ID` に `custom` を使わない。** フロントエンドとの間で「任意指定」を表す予約語である（IMP-309）。
- **探索処理は変数として差し替えられるようにする**（`runCommand` と同じ流儀。UT-705）。実際にエディタがインストールされているかどうかに依存するテストを書かないためである（UT-035）。

#### Windows（`presets_windows.go`）

| ID | 表示名 | 探す場所（先に見つかったものを採る） |
| --- | --- | --- |
| `notepad` | Notepad | `%WINDIR%\system32\notepad.exe` |
| `vscode` | Visual Studio Code | `%LOCALAPPDATA%\Programs\Microsoft VS Code\Code.exe`、`%PROGRAMFILES%\Microsoft VS Code\Code.exe` |
| `notepadpp` | Notepad++ | `%PROGRAMFILES%\Notepad++\notepad++.exe`、`%PROGRAMFILES(X86)%\Notepad++\notepad++.exe` |
| `hidemaru` | Hidemaru | `%PROGRAMFILES%\Hidemaru\Hidemaru.exe`、`%PROGRAMFILES(X86)%\Hidemaru\Hidemaru.exe` |
| `sakura` | sakura editor | `%PROGRAMFILES(X86)%\sakura\sakura.exe`、`%PROGRAMFILES%\sakura\sakura.exe` |

**環境変数が空の環境では、その候補ごと落とす。** そのまま連結すると相対パスになり、作業ディレクトリに置かれた実行ファイルを起動しうる（NFR-035 の 5）。`PROGRAMFILES(X86)` が存在しない構成はありうる。

#### Linux（`presets_other.go`）

`exec.LookPath` で `$PATH` から探す。返る絶対パスをそのまま `Path` とする。**絶対パスで返らなかった候補は採らない。** `$PATH` に相対パスの項目が含まれていると `exec.LookPath` は相対パスを返し、作業ディレクトリの内容で起動対象が変わる（NFR-035 の 5）。

| ID | 表示名 | コマンド |
| --- | --- | --- |
| `gnome-text-editor` | GNOME Text Editor | `gnome-text-editor` |
| `gedit` | gedit | `gedit` |
| `kate` | Kate | `kate` |
| `mousepad` | Mousepad | `mousepad` |
| `vscode` | Visual Studio Code | `code` |
| `gvim` | gVim | `gvim` |

> [!IMPORTANT]
> **端末エディタ（`vim` / `nano` / `emacs -nw`）をプリセットに入れない。** `exec.Command("vim", path)` は端末を持たないため、GUI から起動しても何も起きないまま終わる。**利用者からは「押しても無反応」に見える。** 引数を渡さない設計（NFR-035 の 2）のため `Other...` でラッパを指定することもできず、**この用途には対応しない。**

> [!NOTE]
> **「OS の既定アプリケーション」をプリセットに入れない。** 利用者が `.md` を MarkView に関連付けている場合（`docs/usage.md` がその手順を案内している）、押すたびに新しい MarkView が開く。NFR-035 の 6 が拒めるのは実行ファイルの直接指定だけであり、既定アプリ経由はその検査をすり抜ける。

### IMP-173: ディレクトリをファイルマネージャで開く **MUST**

FR-036 と [NFR-035](07-nonfunctional.md) を実装する（4.75.0）。

```go
// ErrNotDir は、ディレクトリでないものをファイルマネージャで開こうとしたことを表す（NFR-035）。
var ErrNotDir = errors.New("not a directory")

// OpenDir は OS の既定のファイルマネージャでディレクトリを開く（FR-036）。
// dir は絶対パスで、ディレクトリであること。
func OpenDir(dir string) error
```

| # | 検査・処理 | 返すエラー |
| --- | --- | --- |
| 1 | `dir` が絶対パスである | `ErrNotFound` |
| 2 | **`os.Stat`（リンク先の実体）でディレクトリである** | 無ければ `ErrNotFound`、ディレクトリでなければ `ErrNotDir` |
| 3 | `OpenFile` と同じコマンドに `dir` を渡して起動する（Windows は `rundll32.exe url.dll,FileProtocolHandler <dir>`、Linux は `xdg-open <dir>`） | 起動時のエラー |

- **ディレクトリだけを OS へ渡す。** 既定のアプリケーションの経路にファイルを渡すと、**実行ファイルなら実行される**（Windows の `FileProtocolHandler` も Linux の `xdg-open` も、対象の種類で動きを変える。NFR-035）。呼び出し側（IMP-196）はツリーの操作の検査（IMP-135）でディレクトリと確かめてから呼ぶが、**ここでも起動の直前にもう一度確かめる**（検査と起動の間のすり替わりを狭める）。
- **Windows で `explorer.exe` を使わない。** `explorer.exe` は引数の `,` を区切りとして読むなど解釈が特殊である。**v1.2.0 の実現性検討で、W1 / L1 の実機で、名前に空白・`,`・`&`・`.exe` の終わり・日本語・`%20`・`;`・`'`・`^`・括弧を含む 11 個のディレクトリがすべて正しく開き、W1 では UNC パス（`\\localhost\C$\…`）も開くことを確かめた**（付録 A.17）。
- **ファイルを選んだ状態では開かない**（FR-036 の `Open containing folder` は親ディレクトリを開くだけ）。`explorer.exe /select,` のような OS ごとの手段を持たない。
- **Linux では `xdg-open` が `inode/directory` の既定のアプリケーションを起動する。** エディタが既定になっている環境ではエディタが開く——利用者の環境の設定であり、MarkView 側では扱わない。`xdg-open` が無い・起動できないときは、呼び出し側が `open-failed` でステータスに出す（FR-110）。
- 起動後は待たない。別のゴルーチンで `Wait` を呼んでゾンビを回収する（IMP-170 と同じ）。
- 引数は `exec.Command` の可変長引数として渡し、シェルを経由しない（IMP-170 と同じ）。

## 11.9 ostheme パッケージ（IMP-175 系）

### IMP-175: OS のテーマ設定の取得 **MUST**

FR-071 を実装する。Wails に依存しない（IMP-012）。

```go
package ostheme

const (
    Light   = "light"
    Dark    = "dark"
    Unknown = ""       // 判定できなかった
)

// Detect は OS のテーマ設定を返す。判定できない場合は Unknown を返す。
func Detect() string
```

**Wails v2 のランタイムには OS のテーマを取得する API がない**（`WindowSetDarkTheme` などの設定側のみ）。FR-071 の追従を実現するには、OS ごとの設定を直接読むほかない。

| OS | 実装 |
| --- | --- |
| Windows | レジストリ `HKCU\Software\Microsoft\Windows\CurrentVersion\Themes\Personalize` の `AppsUseLightTheme`（`REG_DWORD`）を読む。**0 がダーク、1 がライト**。`golang.org/x/sys/windows/registry` を用いる |
| Linux | `gsettings get org.gnome.desktop.interface color-scheme` を実行し、`'prefer-dark'` / `'prefer-light'` を解釈する。`'default'` または取得に失敗した場合は `gtk-theme` を読み、名前が `-dark` で終わるものだけをダークとみなす |

- `AppsUseLightTheme` は真偽の `REG_DWORD` として読み、**0 ならダーク、0 以外ならライト**とする（UT-703 のケース 1〜3）。**判定できないのは、キーや値が無い・読めない・型が違うとき、`gsettings` が使えないときと、`color-scheme` / `gtk-theme` から決められないとき**である（4.81.0 で書き分けた）。
- **判定できない場合に Light を返さない。** 「OS がライトである」ことと「OS の設定を読めなかった」ことは別であり、呼び出し側が区別できる必要がある。FR-071 の「判定できない場合は Light テーマとする」の適用は呼び出し側（`resolvedTheme`）の責務とする。
- `SystemUsesLightTheme`（タスクバー等の配色）ではなく `AppsUseLightTheme` を見る。アプリケーションの配色に対応するのは後者である。
- Linux では外部コマンドを起動するため、応答を待つ上限を 500 ms とする。GNOME が入っていない環境では実行そのものが失敗し、待ち時間は発生しない。引数は `exec.Command` の可変長引数として渡し、シェルを経由しない（IMP-170 と同じ理由）。
- **OS へ実際に問い合わせる部分と、値を解釈する部分を分ける。** 解釈（`fromAppsUseLightTheme` / `fromColorScheme` / `fromGTKTheme`）はプラットフォームに依存しないファイルへ置き、ビルドタグ付きのテストを書かずに済むようにする（UT-035, UT-703）。
- **呼び出しは設定にテーマが記録されていないときだけとする**（IMP-303）。通常は初回起動の 1 回に限られるため、結果を保持する仕組みは持たない。

## 11.10 buildinfo パッケージ（IMP-180 系）

### IMP-180: バージョン情報 **MUST**

BR-030 を実装する。

```go
package buildinfo

// ldflags -X で上書きする。既定値は開発ビルド用。
var (
    Version   = "dev"
    Commit    = "unknown"
    BuildTime = "unknown"
)
```

### IMP-181: 同梱資産の情報 **MUST**

BR-042 / UI-100 の `Bundled` 行を実装する。

```go
type VendorEntry struct {
    Name      string `json:"name"`
    Version   string `json:"version"`             // 分からない場合は空（BR-042）
    SPDX      string `json:"spdx"`                // ライセンス種別。NFR-051 の判定に使う
    License   string `json:"license"`             // 全文の位置。frontend/vendor/ からの相対パス
    Source    string `json:"source"`
    Fetched   string `json:"fetched"`
    BundledIn string `json:"bundledIn,omitempty"` // 同梱元の資産名。最上位なら空
}

// SetVendorJSON は埋め込んだ vendor.json を登録する。main.go が起動時に
// 1 度だけ呼ぶ。go:embed はパッケージのディレクトリより上を参照できず、
// frontend/vendor/vendor.json をこのパッケージから直接は読めないため。
func SetVendorJSON(data []byte)

// Vendors は登録された vendor.json を解析して返す。**常に非 nil を返す。**
// 記録をそのまま返し、絞り込まない。
func Vendors() []VendorEntry

// Bundled は Bundled 行（UI-100）に出す資産だけを返す。BundledIn が空のもの。
func Bundled() []VendorEntry

// Environment は "windows/amd64  Go 1.24.0  WebView2 120.x" 形式の文字列を返す。
// WebView のバージョンは呼び出し側（desktop）が渡す。**取得に失敗した
// ときだけ空文字を渡し**、その場合は区画ごと省く。
func Environment(webviewVersion string) string
```

`webviewVersion` の取得は `desktop` パッケージに置く（`desktop/webview_windows.go` / `webview_other.go`）。**OS ごとにビルドタグで分ける**（ルートの `console_windows.go` / `console_other.go` と同じ形）。

```go
// webview_windows.go   //go:build windows
func webviewVersion() string   // go-webview2 の webviewloader から取る
// webview_other.go     //go:build !windows
func webviewVersion() string   // WebKitGTK の版を取る
```

- `vendor.json` の形式は `VendorEntry` の配列とする（BR-042）。**全エントリが同じ形を持つ。** 同梱物の中に含まれるもの（Viz.js / Graphviz / Expat）だけを別の形にしない。
- **`SPDX` と `License` はアプリケーションの実行時には使わない。** 全文は `THIRD_PARTY.md` へ取り込み済みであり（FR-101）、この 2 つを読むのは `genlicenses`（BR-040）と `vendorupdate`（BR-043）である。**それでも同じ構造体に持たせるのは、`vendor.json` の形を 1 つに保つためである。**
- **`Bundled` の絞り込みをここ 1 か所に置く。** フロントエンドで絞ると、絞り方が 2 つに分かれる（IMP-306）。
- `Vendors` は解析に失敗しても**空スライスを返し、エラーにしない**（FR-111）。情報表示が欠けるだけで、文書の閲覧は続けられる。JSON の `null` は解析に成功したうえで nil を書き込むため、そこも空スライスへ揃える。
- `Environment` の WebView 名は OS で異なる（Windows: `WebView2`、Linux: `WebKitGTK`）。バージョンが空のときに「`WebView2 `」とだけ書かれた区画は情報として役に立たないため、区画ごと省く。
- **`Environment` に空文字を渡してよいのは、取得に失敗したときだけである。** UI-100 は `Environment` 行に WebView の版を出すことを **MUST** で定めており、**常に空を渡す実装はこの要求を満たさない。**
- **WebView の版は取得できる。** Wails v2 の公開ランタイム（`pkg/runtime`）は返さないが、それは「取得できない」という意味ではない。
  - **Windows**: `github.com/wailsapp/go-webview2/webviewloader` の `GetAvailableCoreWebView2BrowserVersionString("")` を使う。**Wails 自身が内部で使っているのと同じパッケージ**であり、`go-webview2` は既に依存関係に入っている（新しいモジュールは増えない）。
  - **Linux**: WebKitGTK の `webkit_get_major_version()` / `_minor_` / `_micro_` を cgo で呼ぶ。Linux ビルドは既に cgo と WebKitGTK にリンクしている（AR-003, BR-010 の `-tags webkit2_41`）。
- **取得は `internal/` に置かない**（IMP-012）。`internal/` を Wails 非依存・OS 非依存に保つ規約であり、ここへ入れると単体テストに OS 依存が持ち込まれる（UT-002）。
- 取得に失敗しても**起動を止めず、エラーも出さない**（FR-111）。区画が 1 つ減るだけである。

> [!IMPORTANT]
> **「Wails から取得できないことがある」を、実装しない理由に使わない。** 省略の規定は**異常系のための救済**であり、常に通る経路にしてはならない。
>
> **Linux こそ表示する価値が大きい。** `docs/troubleshooting.md` の最初の項目は「WebKitGTK 4.1 が入っていません」であり、`README.md` も 4.0 系との取り違えに繰り返し触れている。**利用者に「あなたの環境の WebKitGTK は何版か」を示せるのは、この情報ウィンドウだけである**（[調査報告](../bugs/2026-09-04-bug-005-about-webview-version.md)）。

## 11.11 App と session（IMP-190 系）

`App`（`desktop/app.go`）は Wails にバインドされる唯一の型であり、**`main.go` と `desktop` パッケージ（IMP-011）が Wails に依存する。** **判断を伴うロジックは `internal/session` / `internal/document` に置き、`desktop` からは呼ぶだけにする**（IMP-012）。

**`App` の公開メソッドはすべてフロントエンドから呼べる**（Wails の `Bind`）。**公開メソッドは IMP-310 の一覧に限る。** ライフサイクルの関数（`onStartup` / `onBeforeClose` / `onShutdown`）は非公開のまま `desktop.LifecycleOf` で取り出し、`main.go` が起動オプションへ渡す。Wails は起動オプションに渡された関数だけを関数名で照合してバインドから外すため、**公開メソッドにすると、`main.go` で包んで渡しただけで JavaScript から呼べるメソッドになる**（IMP-300。4.60.0 で `wails generate module` により確かめた）。この分離により、履歴・起動解決・パス算出・同じファイルの判定を Wails なしでテストできる（UT-803〜UT-805, UT-809）。

| 責務 | 置き場所 |
| --- | --- |
| Wails のバインドメソッド | `desktop/` の `bind.go` / `clipboard.go` / `editor.go` / `link.go` / `editmode.go` / **`tree.go`**（IMP-011） |
| イベント送出、ウィンドウ操作、ライフサイクル | `desktop/app.go`（ライフサイクルの関数の取り出しは `desktop.go`。**`history:changed` の送出は `history.go`**。IMP-323） |
| ファイルツリーの操作（錠・検査の呼び出し・起動・ツリールートの変更。IMP-196） | `desktop/tree.go`（検査の判断は `internal/filetree`。IMP-135） |
| アプリケーション状態の保持と排他制御 | `desktop/app.go` |
| 文書を開く共通処理（IMP-192） | `desktop/open.go` / `open_route.go` |
| 監視のイベントの受け取り（IMP-192）、ドロップ（IMP-313）、パニックの回復（IMP-022） | `desktop/watch.go` / `drop.go` / `recover.go` |
| 表示履歴の操作（IMP-191） | `internal/session` |
| 起動時の対象解決（IMP-193） | `internal/session` |
| 表示用パスの算出、パスの比較、**同じファイルの判定**（IMP-025, IMP-191） | `internal/session` |
| 書き換え位置の対応・書き込み・取り消し履歴・**編集モードの状態と判断**（IMP-106〜IMP-109） | `internal/document` |
| 編集モードのバインドメソッド・錠・ファイルの読み書きの呼び出し・イベント送出（IMP-195） | `desktop/editmode.go` |
| **ツリーの操作の検査（IMP-135）と、表示する種類の絞り込み（IMP-134）** | `internal/filetree` |
| **ツリーの操作のバインドメソッド**（開く・展開・エディタ・ファイルマネージャ・ツリールート・パスのコピー。IMP-196） | `desktop/tree.go` |

### IMP-190: 保持する状態 **MUST**

```go
type App struct {
    ctx      context.Context
    mu       sync.Mutex

    renderer *renderer.Renderer
    watcher  *watcher.Watcher
    cfg      config.Config

    treeRoot string      // ツリールートの絶対パス（FR-030）
    current  *document.Document   // 表示中の文書。未表示なら nil
    history  *session.History     // 表示履歴（IMP-191）

    // target は画面がいま対象にしているファイルの絶対パス。
    // 本文を表示していればその文書、状態画面を出していればその対象。
    // 文書未表示（welcome）なら空。再読み込みと、エディタ選択ウィンドウを開いた時点の控え（editorTarget）が使う（FR-090, FR-015）。
    target string

    // showing は、画面が current を表示しているか（状態画面・文書未表示なら偽。IMP-192）。
    // target と current.Path を比べて代わりにしない（同じファイルの状態画面がありうる）。
    showing bool

    // currentConfirmed は、current を確認画面の Open anyway で開いたか、
    // その同意を引き継いで読み直したか（FR-016）。同じファイルの読み直しで
    // LoadOptions.Confirmed を渡すために持つ（IMP-192）。
    currentConfirmed bool

    // 確認画面を表示中のファイル（FR-016）。OpenConfirmed が受け付ける
    // 対象をこの 1 つに限定するために保持する（IMP-314）。
    pendingConfirm string

    // pendingEditor は BrowseEditor で選ばれた「確定前の候補」（IMP-310）。
    // 実行ファイルの絶対パスであり、フロントエンドへ渡さない（IMP-309, NFR-035）。
    pendingEditor string

    // editorTarget はエディタ選択ウィンドウを開いた時点で控えた、エディタで開くファイル（FR-091。4.75.0）。
    // ListEditors なら target（文字のファイルのときだけ）、ListEditorsForTree なら IMP-135 で確かめたファイル。
    // OpenInEditor はこれを開き、起動の成否に関わらず終わったら捨てる。ウィンドウを閉じたら（CloseEditor）も捨てる。フロントエンドへ渡さない
    editorTarget string

    // historySeq は history:changed（IMP-323）の版。送るたびに 1 増やす（4.75.0）
    historySeq uint64

    // editBoxOpen は、フロントエンドで編集欄（セル・コードブロック）が開いているか（SetEditBoxOpen。4.75.0）。
    // ドロップを拒むためにだけ使う（FR-140, IMP-313）。書き込みの判断には使わない（IMP-109 が正）
    editBoxOpen bool

    // ioMu は表示中ファイルの読み直しと書き込みを 1 つずつ順に行うための錠（FR-143）。
    // 取る順序は常に ioMu → mu とする。
    ioMu sync.Mutex

    // edit は編集モードの状態と判断（IMP-109）。Go 側が正（IMP-300 の 4）。
    // ioMu の内側でだけ触る。
    edit document.EditSession
}
```

- すべての状態変更は `mu` で保護する（IMP-024）。**`edit` だけは例外で、`ioMu` で保護する**（`edit` を触る経路はすべて `ioMu` を取るため。`Plan` の構文解析の間 `mu` を持ち続けない）。
- **`ioMu` は、文書を開く処理（IMP-192）、編集モードの切り替えと書き込み（IMP-195）、削除の受け取り（IMP-195）の全体を包む。** 読み込みと変換は `mu` の外で行う（IMP-192）が、`ioMu` の内側で行う。**`mu` だけでは、読み直しの途中に書き込みが割り込み、自分の書き込みを外部の変更と取り違える**（FR-143, FR-144）。**錠を取る順序を逆にしない**（デッドロックを避ける）。
- **`sync.Mutex` は再入できない。** `ioMu` を持ったまま文書を開く処理を呼ぶ経路（IMP-195 の 4 と 8、**編集モードを始めたときの描き直し（IMP-195）、戻る・進む（IMP-191 の手順）**。4.75.0）は、錠を取らない `openLocked`（IMP-192）を呼ぶ。`open` を呼ぶと止まる。
- **ファイルパスの履歴やツリールートをディスクへ書き出す経路を持たない**（NFR-042）。`config.Config` にそれらのフィールドが存在しないことで構造的に保証する（IMP-150）。

> [!IMPORTANT]
> **`target` と `current` を取り違えない。** `current` は「読み込みと変換に成功した文書」であり、状態画面（`confirm-large` / `too-large` / `render-error`）を出している間は**前に開いていた文書のまま残る**（IMP-192）。一方 `target` は画面が示している対象であり、**ウィンドウタイトル（UI-013）およびステータスのパス表示（DSP-302）と常に一致する。** 画面が `current` を表示しているかは `showing` で表す——**`target == current.Path` で代えない。** 表示中の文書を読み直したら 50 MB を超えていた場合、状態画面の対象は `current` と同じファイルになる。
>
> 「エディタで開く」（FR-090）は `target` を使う。ここで `current` を渡すと、利用者が `big.md — too large` の画面を見ながら押したのに前の文書が開き、**エラーも出ないため気づけない**（NFR-035 の 2）。
>
> **4.75.0 で、エディタ選択ウィンドウを開いた時点の対象を `editorTarget` に控え、`Open` ではそれを開くようにした**（FR-091。利用者の判断）。ウィンドウを開いている間にドロップで文書が切り替わっても（UI-090）、ウィンドウに出ている名前のファイルが開く。ファイルツリーから開いた場合（FR-036）も同じ欄を使う。

### IMP-191: 表示履歴 **MUST**

FR-051 を実装する。**配置は `internal/session`**（IMP-012）。

```go
package session

type Entry struct {
    Path      string
    ScrollTop int    // フロントエンドから受け取るスクロール位置
    Anchor    string // アンカー付きリンクで開いた場合のフラグメント（復号済み。IMP-302）
}

type History struct {
    entries []Entry
    index   int // 現在位置
}

const MaxHistory = 50 // FR-051

func NewHistory() *History
func (h *History) Push(e Entry)              // 前方履歴を破棄して追加
func (h *History) SetScrollTop(top int)      // 現在位置のスクロール位置を更新

// 戻る・進む（4.75.0。v1.1.0 の Back / Forward は、開く前に位置を動かしていた。BUG-023）
func (h *History) Peek(delta int) (Entry, bool) // 位置を動かさずに、delta（-1: 戻る / +1: 進む）先の項目を返す
func (h *History) Step(delta int)               // 位置を delta 動かす。開けた後（状態画面を含む）に呼ぶ
func (h *History) Drop(delta int)               // delta 先の項目を取り除く。位置が指す文書は変えない。開けなかった後に呼ぶ（4.76.0）
func (h *History) CanBack() bool                // 戻れるか（FR-051 のボタンの淡色。IMP-323）
func (h *History) CanForward() bool             // 進めるか
```

- 履歴はメモリ上のみ。プロセス終了で破棄する（FR-051, NFR-042）。
- `Push` の前に、現在位置のエントリのスクロール位置を更新する。戻ったときに元の位置へ復元できるようにするため（FR-050）。
- 上限を超えた場合、先頭から捨てる。

**戻る・進むの手順**（FR-051。4.75.0。[BUG-023](../bugs/2026-11-08-bug-023-history-step-on-open-failure.md)）

1. `Peek(delta)` で移動先の項目を得る。無ければ何もしない（履歴の端。IMP-308）。
2. 項目を開く（`openFromHistory`。IMP-192）。
3. **開けた（文書を表示した）、または状態画面になった**なら `Step(delta)`。状態画面は「開けなかった」に入れない——画面がその文書を対象にしている（UI-013。BUG-024 で状態画面も履歴に積むことにした）。
4. **表示を変えない失敗**（`not-found` / `permission` / `not-supported`。IMP-192 の `target` の表）なら、`Drop(delta)` で移動先の項目を取り除き、**位置は動かさない**。理由はステータス領域に出す（FR-110）。

- **`Drop` は、位置より前の項目（`delta` が負）を取り除いたら、位置を 1 つ前へずらす**（位置が指す文書を変えない）。位置より後ろ（`delta` が正）なら位置は変わらない。**何を取り除き、位置をどこに置くかを 1 つの呼び出しにまとめ、`desktop` に添字の計算を置かない**（IMP-012。BUG-023 の調査報告の「判断は `internal/session` に置く」。4.75.0 は `Remove(Index() + delta)` と書いており、組み合わせが単体テストの外に残っていた。4.76.0）。`delta` が 0（位置そのもの）なら、位置は 1 つ前（先頭なら 0）を指す（App はこの使い方をしないが、境界として定める。UT-804）。空の履歴・範囲外の `delta` では何もしない。**すべて取り除いたら空の履歴に戻る**（位置が空の履歴を指したままだと `SetScrollTop` が panic する。UT-804 のケース 13。4.88.0）。**`Step` も、範囲外の `delta` では何もしない**（App は `Peek` が通った後にしか呼ばないが、位置が範囲の外を指すと次の `SetScrollTop` が panic する。FR-111。UT-804 のケース 14。4.88.0 で書いた——それまでは決めていなかった）。
- **v1.1.0 は開く前に位置を動かし（`Back` / `Forward`）、開けなくても戻さなかった。** 画面は前の文書のまま、位置だけが消えた文書を指し、その間に別の文書を開くと、位置より後ろを捨ててから積むため、**見ていた文書が履歴から消えて戻れなくなった**（BUG-023）。
- **1 回の操作で 1 つ戻る**ことは変えない（開けない項目を飛ばして、その先の文書を開かない。FR-051）。
- **1〜4 は 1 つの `ioMu` の中で行う**（IMP-310 の `HistoryBack` / `HistoryForward`）。2 回続けて押したとき、2 回目が 1 回目の `Step` より前の位置から移動先を求めると、同じ項目を 2 回開いて位置だけが 2 つ進む。

表示用パスの算出も同じパッケージに置く（IMP-025, UT-805）。

```go
// DisplayPath はステータス領域に出すパスと、ツリー外かどうかを返す
// （UI-060, FR-052）。ツリールートの内側なら相対パス、外側なら絶対パス。
func DisplayPath(root, target string) (display string, outside bool)
```

- ファイルシステムには触れない。`target` は絶対パスであることを前提とし、`filepath.Clean` だけを行う。
- 区切り文字は OS のものをそのまま使う。ツリー外で絶対パスを出すときと表記を揃えるため。
- ツリールートが定まっていない（空文字）場合は絶対パスを返し、**`outside` は false とする**。ツリーがない状態で `(outside tree)` と表示しても意味を持たないため。

パスの比較も同じパッケージに置く（IMP-025）。

```go
// SamePath は 2 つのパスが同じ場所を指すかを返す。Windows では大文字小文字を
// 区別せず、Linux では区別する。シンボリックリンクは解決しない。
func SamePath(a, b string) bool
```

`app.go` はツリールートの変更判定にこれを使う（IMP-192）。単純な `!=` で比べると、Windows で大文字小文字だけが違うパスを別のルートとみなし、同じ場所を指しているのに `tree:root-changed` を送ってツリーを組み直してしまう。

**同じファイルかどうか**（[1.7](01-overview.md)）の判定も同じパッケージに置く（UT-809）。

```go
// SameFile は a と b が同じファイル（1.7）かを返す。両方を filepath.EvalSymlinks で
// 解決し、SamePath で比べる。解決に失敗した側は、解決前のパスのまま比べる。
func SameFile(a, b string) bool
```

- **`DocumentDTO.SameDocument` の値を決める判断そのもの**であり、編集モードを終えるか（FR-140 の MUST）の入力になる（IMP-192 → IMP-109 の `Loaded`）。**`desktop` に置くと単体テストの対象外になる**（UT-002）。4.44.0 で `package main`（4.60.0 から `desktop`）の `sameFile` から移した。
- 標準ライブラリだけで書ける（`path/filepath`）。`session` の依存（IMP-012）は増えない。
- ファイルシステムに触れる（`EvalSymlinks`）。`DisplayPath` と違い、存在しないパスでも失敗にせず、解決前のパスで比べる。

### IMP-192: 文書を開く共通処理 **MUST**

FR-010 / FR-011 / FR-012 / FR-033 / FR-050 / FR-051 のすべてが、この 1 つの内部処理を通る（AR-060）。

```go
type openSource int

const (
    openFromDialog openSource = iota // FR-010
    openFromDrop                     // FR-011
    openFromArgs                     // FR-012, FR-013（起動時の表示。引数なしで見つけた README も同じ経路）
    openFromTree                     // FR-033, FR-036（メニューの Open）。開く前に IMP-135 の検査を通す（IMP-196）
    openFromLink                     // FR-050
    openFromHistory                  // FR-051
    openFromReload                   // FR-014, FR-015
    openFromConfirm                  // FR-016 の Open anyway
)

// openRequest は open への指示。アンカー（FR-050）と復元位置（FR-051）を
// 渡す必要があるため、位置引数ではなく構造体で受ける。
type openRequest struct {
    path        string
    src         openSource
    anchor      string // アンカー付きリンクを踏んだときのフラグメント（復号済み。IMP-302）
    scrollTop   int    // openFromHistory で復元する位置
    confirmed   bool   // FR-016 の Open anyway。同じファイルの読み直しでは下の規則で決める
    trigger     string // DocumentDTO.Trigger（IMP-302）。"open" | "reload" | "watch" | "edit"
    refKey      string // 空でなければ LoadOptions.RefKey に渡す（IMP-102）。IMP-195 の 8 だけが渡す
    expectDigest [sha256.Size]byte // refKey と対で LoadOptions.ExpectDigest に渡す（IMP-102）
}

// open は ioMu を取って openLocked を呼ぶ。バインドメソッドと監視のイベントはこちらを呼ぶ
// （ioMu を自分で取るもの——戻る・進むと編集モードの処理——を除く）。
func (a *App) open(req openRequest) (*DocumentDTO, error)

// openLocked は ioMu を持った呼び出し元から呼ぶ（IMP-195 の 4 と 8 と編集モードの開始、IMP-191 の戻る・進む）。錠を取らない。
func (a *App) openLocked(req openRequest) (*DocumentDTO, error)
```

**呼び出し元ごとの `src` / パス / `trigger`** は次のとおりとする。

| 呼び出し元 | `src` | 開くパス | `trigger` |
| --- | --- | --- | --- |
| ダイアログ・ドロップ・引数・ツリー・リンク・履歴 | それぞれの経路 | 受け取った（解決した）パス | `open` |
| 確認画面の `Open anyway`（`OpenConfirmed`） | `openFromConfirm` | `pendingConfirm` | `open` |
| 手動の再読み込み（`Reload`） | `openFromReload` | **`target`**（状態画面の間はその対象。IMP-310） | `reload` |
| 監視のイベント（IMP-140） | `openFromReload` | `current.Path`（下の「イベントの照合」を通ったもの） | `watch` |
| 書き込み前の不一致による読み直し（IMP-195 の 4） | `openFromReload` | `current.Path` | `reload` |
| 書き込みの直後の読み直し（IMP-195 の 8） | `openFromReload` | `current.Path` | `edit` |
| **編集モードを始めたときの読み直し**（修飾の目印を付けて描き直す。IMP-195。4.75.0） | `openFromReload` | `current.Path` | `reload` |

ツリールート・履歴・スクロールについて、各 `openSource` による差異は以下に限る。

| source | ツリールート | 履歴 | スクロール |
| --- | --- | --- | --- |
| `openFromDialog` / `openFromDrop` / `openFromArgs` | 親ディレクトリへ変更 | 積む | 先頭 |
| `openFromTree` / `openFromLink` | **変更しない**（FR-030, FR-052） | 積む | 先頭（アンカー指定時はその位置） |
| `openFromHistory` | 変更しない | 積まない | `restore`: 記録された位置を復元 |
| `openFromReload` | 変更しない | **積まない**（FR-051） | `keep`: 現在位置を維持 |
| `openFromConfirm` | **変更しない**（確認時に変更済み） | **積まない**（確認時に積み済み） | 先頭 |

ツリールート・履歴・スクロールについては、この表以外の差異を持ち込まない。分岐が増えると FR-030 の不変条件（リンク遷移でツリールートが動かない）を壊しやすくなる。`trigger` による違い（同じ内容なら送らない、鍵の引き継ぎ、同意の引き継ぎ）は、下の規則だけで定める。

実装上の規約を以下に定める。

- **読み込みと変換は `mu` の外で行う。** 10 MB 近い文書では時間がかかり、その間ほかのバインドメソッド（ツリー・設定・情報など）を止める理由がない。`renderer` は状態を持たず同時に呼んでよい（IMP-024）。状態への反映だけを `mu` の内側で行う。**`ioMu` の内側では行う**——文書を開く処理と書き込みどうしは 1 つずつ順に行う（IMP-190。`ioMu` を取らないバインドメソッドは止まらない）。
- **Wails の呼び出し（`tree:root-changed` の送出）もロックの外で行う。** ツリールートが変わったかどうかはロックの内側で判定し、送出は解いた後に行う。
- ツリールートが変わるのは `filepath.Dir` を取った結果が現在の値と異なる場合に限る。比較は `session.SamePath` で行う（IMP-191）。
- 確認待ちのパス（`pendingConfirm`）はこの処理の中でのみ更新する。`ErrNeedsConfirm` で立て、**開けたときと、確認以外の失敗のときに消す**（IMP-314）。残したままにすると、確認画面を閉じたあとの操作で開けてしまう。
- **画面の対象（`target`。IMP-190）もこの処理の中でのみ更新する。** 規則はウィンドウタイトル（UI-013）とまったく同じで、**画面の対象が変わったときだけ**書き換える。

| 結果 | `target` |
| --- | --- |
| 読み込みに成功した | 開いた文書の絶対パス |
| 状態画面を出した（`confirm-large` / `too-large` / `render-error`） | **その対象の絶対パス。** 描画していなくても書き換える |
| 表示を変えない失敗（`not-found` / `permission` / `not-supported`） | **変えない。** FR-110 が「直前の内容を維持」と定めており、画面の対象も変わっていない |

  **判定はタイトル更新と同じ条件で行い、2 か所に分けて書かない。** 片方だけ直すと、タイトルと「エディタで開く」の対象が食い違う。
- **`showing` と監視も、`target` と同じ 3 つの結果で決める。**

  | 結果 | `showing` | 監視（`watcher`） | `currentConfirmed` |
  | --- | --- | --- | --- |
  | 読み込みに成功した | 真 | 開いた文書へ切り替える（`Watch`） | `req.confirmed` の値 |
  | 状態画面を出した | **偽** | **外す（`Unwatch`）** | 偽 |
  | 表示を変えない失敗 | 変えない | 変えない | 変えない |

  **状態画面を出したら、前の文書の監視を外す**（FR-014 の「表示対象を切り替えたときは、以前のファイルの監視を解除する」、FR-016 の「確認画面を表示しているだけの状態では監視しない」）。**v1.0.0 は外していなかった**——確認画面の間に前の文書が外部で更新されると、`ev.Path` をそのまま開き直して `document:changed` を送り、**確認画面が前の文書の表示に置き換わっていた**（[BUG-012](../bugs/2026-09-14-bug-012-state-screen-previous-document.md)）。
- 監視は常に 1 つ以下とし、`Watch` の失敗でも開く操作は成功とする。自動更新が効かなくなるだけで、利用者は再読み込みできる（FR-014, FR-015, FR-111）。**`Watch` / `Unwatch` は `mu` の外で呼ぶ**（IMP-140。`ioMu` の内側ではある）。
- **状態画面の失敗を返すときは、`ErrorDTO.DisplayPath` / `OutsideTree` に画面の対象の表示用パスを入れる**（IMP-307。`session.DisplayPath(treeRoot, target)`。ツリールートは、状態画面でツリーを移した後の値）。**`ErrorDTO.TargetKind` に画面の対象の種類（`mdfile.KindOf(target)` の文字列）を入れる**（状態画面の間の「エディタで開く」の淡色。UI-021, IMP-202。4.76.0）。フロントエンドは状態画面の間、ステータス領域の左にこれを出す（DSP-302, FR-016）。**v1.0.0 は渡しておらず、状態画面の間も前の文書のパスが残っていた**（BUG-012）。
- **処理の全体を `ioMu` で包む**（IMP-190, FR-143）。包むのは `open` であり、中身は `openLocked` に書く。
- **状態画面を出したときも、積む経路（ダイアログ・ドロップ・引数・ツリー・リンク）なら、その対象を履歴に積む**（FR-051。4.75.0。[BUG-024](../bugs/2026-11-08-bug-024-state-screen-not-in-history.md)）。**v1.1.0 は確認画面（`confirm-large`）だけを積み**（IMP-314）、上限超過とレンダリングエラーの状態画面から `Alt+←` を押すと直前の文書を飛ばした。積むかどうかは上の表の「履歴」の列に従い、**経路の規則は変えない**（履歴・再読み込み・確認からは積まない）。**ツリールートも、状態画面の 3 種類とも上の表のとおり動かす**（ダイアログ・ドロップ・引数なら親ディレクトリへ。FR-010, FR-011, FR-030。4.76.0。利用者の判断）。**v1.1.0 は確認画面のときだけ移し、上限超過とレンダリングエラーでは移さなかった**——起動時の引数では移しており（IMP-193）、経路によって結果が違っていた。`Open anyway`（`openFromConfirm`）は確認のときに移し済みであり、二重に動かさない（IMP-314）。
- **履歴を変えたら（`Push`・`Step`・`Drop`。IMP-191。4.77.0 で `Remove` から直した）、錠を解いた後で `history:changed`（IMP-323）を送る。**
- **修飾の目印**（IMP-120）: `LoadOptions.TextRefs` は、**編集モードの間（`edit.On()`）で、直前の `showing` が真で、`session.SameFile(current.Path, 開くパス)` が真のとき**だけ真にする（同じ文書の再描画。4.75.0）。文書の切り替えでは編集モードが終わるため付けない（FR-140）。

**同じファイルかどうか**（[1.7](01-overview.md)）もこの処理の中で 1 度だけ判定し、`DocumentDTO.SameDocument`（IMP-302）として返す。判定は `session.SameFile`（IMP-191）で行う。

| 条件 | `SameDocument` |
| --- | --- |
| 読み込みに成功し、**直前の `showing` が真**で、`session.SameFile(直前の current.Path, 開いた文書のパス)` が真 | 真（**開き直し・再読み込み・更新検知・書き込みの後の読み直しのいずれも**） |
| それ以外（別のファイル、直前が状態画面や未表示だった） | 偽 |

- **直前が状態画面なら、同じファイルでも偽とする。** 状態画面へ移った時点で文書の切り替えが起きており（DSP-352, IMP-109 の `Left`）、引き継ぐ状態が残っていない。`current` は前の文書のまま残っているため、`current` だけを見ると真になってしまう。

**同意の引き継ぎ**（FR-016）

- **読み込みの前に、`showing` が真で、`currentConfirmed` が真で、`session.SameFile(current.Path, 開くパス)` が真なら、`confirmed` を真にして読む。** 監視・再読み込み・書き込みの後の読み直し・同じファイルの開き直し（ツリーやドロップ）のいずれも同じに扱う。**確認して描画した 10 MB 超の文書で、保存や `F5` のたびに確認画面へ戻さないため**であり、**書き込みの直後の読み直しが `ErrNeedsConfirm` になって、裏で編集モードが終わるのに画面は編集モードのまま残る**ことを防ぐ（FR-140, FR-143）。
- 50 MB を超えれば `ErrTooLarge`（IMP-102）。状態画面を出した時点で `currentConfirmed` は偽になり、以後の読み直しは確認し直す。

**イベントの照合**（監視のイベントを受けたとき）

- **`ioMu` を取った後で、`showing` が真で、`session.SameFile(ev.Path, current.Path)` が真であることを確かめる。** 違えば何もしない（読み直さず、送らない）。`ioMu` を待つ間に別の文書を開く処理が済んでいると、古いイベントが後から届く。**確かめないと、表示中の B に対して A を開き直し、B の表示が A に戻る。** 削除のイベント（IMP-195）も同じ照合を通す。

**編集モードと取り消し履歴**（FR-140, FR-144）もこの処理の中で決める。**フロントエンドに判断させない**（IMP-300 の 2）。判断そのものは `document.EditSession`（IMP-109）が持ち、この処理は呼ぶだけにする。

| 結果 | 呼ぶもの |
| --- | --- |
| 読み込みに成功した | `a.edit.Loaded(doc, SameDocument)` |
| 状態画面を出した（`confirm-large` / `too-large` / `render-error`） | `a.edit.Left()` |
| 表示を変えない失敗（`not-found` など。上の `target` の表） | 呼ばない |

- **監視のイベント（`trigger` が `watch`）で読み直した内容が、表示中の `current.Digest` と同じで、かつ `edit.Deleted()` が偽なら、`document:changed` を送らない**（FR-014）。書き込み（IMP-195）の直後に届くイベントで、同じ再描画を繰り返さないためである。**このとき `current` を差し替えず、`edit.Loaded` も呼ばず、`openLocked` は `(nil, nil)` を返す**（呼び出し側は何も送らない）——差し替えると鍵（`RefKey`）だけが新しくなり、画面の目印と食い違って以後の指示がすべて `Stale` になる。**手動の再読み込み（FR-015。`trigger` が `reload`）は、同じでも送る。**
  - **削除の印（`edit.Deleted()`）が立っていれば、同じ内容でも送り、`edit.Loaded` を呼ぶ。** 削除した後に同じ内容で戻る場合がある（`git checkout`、150 ms を超える削除と作成の保存）。送らないと、Go 側は削除の印が残って編集モードを二度と始められず、フロントエンドは `document:removed` で淡色にしたボタンとステータスの表示が戻らない。
- 返す `DocumentDTO` の `EditMode` / `Editable` / `EditSeq` は、上の呼び出しの**後の**値（`edit.On()` / `edit.CanStart(current, true)` / `edit.Seq()`）とする。

### IMP-193: 起動シーケンス **MUST**

FR-012 / FR-013 / AR-080 を実装する。**判定部分の配置は `internal/session`**（IMP-012）。`main.go` はコマンドライン引数を渡すだけとし、探索の判断をここへ集約する。

```go
package session

// Startup は起動時に決定した表示対象とツリールートを表す。
type Startup struct {
    TreeRoot  string // 絶対パス
    Initial   string // 表示対象の絶対パス。なければ空文字
    Requested string // 引数で指定されたパスの絶対形。解決に失敗した場合も残す
}

// ResolveStartup は起動時の表示対象とツリールートを決定する。
// cwd と exeDir を引数で受け取ることで、テストから差し替えられる（UT-803）。
func ResolveStartup(args []string, cwd, exeDir string) (Startup, error)
```

`os.Getwd()` と `os.Executable()` の呼び出しは `main.go` 側で行い、結果を引数として渡す。**実行環境に依存する値を関数内で直接取得しない**（UT-035）。

1. `--version` / `--help` は Wails を起動する前に処理し、標準出力へ書いて終了する（FR-012）。

   > [!IMPORTANT]
   > **Windows では、標準出力へ書く前に親プロセスのコンソールへ繋ぎ直す必要がある。**
   > Wails が作る Windows の実行ファイルは GUI サブシステムであり、コンソールから
   > 起動されてもコンソールに接続されない（`GetStdHandle` が NULL を返す）。
   > そのまま書いても行き先がなく、利用者がターミナルで `MarkView --version` と
   > 打っても何も表示されない。`AttachConsole(ATTACH_PARENT_PROCESS)` を呼び、
   > `CONOUT$` を開き直してから出力する。実装は `console_windows.go` に置き、
   > Windows 以外は何もしない実装を `console_other.go` に置く（IMP-031）。
   >
   > ただし**標準出力がすでに有効な場合（リダイレクトやパイプ）は繋ぎ直さない。**
   > 繋ぎ直すと、リダイレクト先ではなくコンソールへ出力してしまう。
   > また、繋ぎ直すのは `--version` / `--help` / 未知のオプションを処理する経路に
   > 限る。ウィンドウを開く経路でコンソールを掴み続けると、起動元のターミナルを
   > 閉じられなくなる。
   >
   > **シェルはプロセスの終了を待たずにプロンプトを返す。** GUI サブシステムの
   > 実行ファイルに対する Windows の既定の挙動であり、出力はプロンプトが出た後に
   > 現れる。見え方は不格好だが、出力そのものは届いており、リダイレクトや
   > パイプで起動した場合はシェルが待つため終了コードも正しく取れる（E2E-102,
   > E2E-103, E2E-105 は CI で出力を捕捉して実行するため成立する）。
   > これを解消するにはコンソールサブシステムでビルドすることになるが、その場合は
   > エクスプローラからのダブルクリック起動（UC-01。本アプリの主たる使い方）で
   > 黒いコンソール窓が開いてしまう。**主たる使い方を優先し、この見え方を許容する。**
2. 引数にパスがある場合、ファイルなら「親ディレクトリ + そのファイル」、ディレクトリなら「そのディレクトリ + 直下の README」。
3. 引数がない場合、以下の順で README（拡張子を除いた名前が `README` の Markdown。大文字小文字を区別しない）を探す（FR-013）。**探すのは Markdown のままである**（`mdfile.IsMarkdown`。図や画像のファイルを探さない。4.75.0）。
   1. `os.Getwd()`
   2. `os.Executable()` を `filepath.EvalSymlinks` で解決したディレクトリ
4. どちらにも見つからない場合、ツリールートをカレントディレクトリとし、表示対象なしとする。

```go
// FindReadme はディレクトリ直下から README を探す。
// 完全一致 "README.md" を優先し、次に大文字小文字を無視した一致のうち
// 名前の昇順で先頭のものを返す（FR-013）。配置は internal/session。
func FindReadme(dir string) (string, bool)
```

**設定は `main.go` が Wails の起動前に読む**（`config.Load`）。ウィンドウの初期サイズ（UI-110）が `wails.Run` のオプションとして必要であり、`App` の生成より先に確定していなければならないためである。**`WindowStartState` は指定しない。** 最大化状態は復元しないため、常に既定の `options.Normal` で開く（UI-111, UI-115）。`Load` はエラーを返さない。設定がない・壊れている場合も既定値で起動する（UI-113, IMP-151）。

**Windows では WebView2 のユーザデータ領域をテンポラリへ寄せる**（AR-004, NFR-033）。`options.App` に次を渡す。

```go
// AR-004。この下に WebView2 が EBWebView を作る。
// 設定ファイル（IMP-152）と同じ %TEMP%\MarkView の下にまとめる
Windows: &windows.Options{
    WebviewUserDataPath: filepath.Join(os.TempDir(), "MarkView", "webview2"),
},
```

- **指定しないと `%APPDATA%\MarkView.exe` が使われる。** 既定値は go-webview2 が `filepath.Join(os.Getenv("AppData"), <実行ファイル名>)` で組み立てており、NFR-033 に反する。
- **環境変数では代替できない。** go-webview2 は環境生成の直前に `preventEnvAndRegistryOverrides` で `WEBVIEW2_USER_DATA_FOLDER` を自身の計算値へ上書きするため、外から与えた値もレジストリの設定も効かない。このオプションが唯一の手段である。
- パスは `config.Dir()`（IMP-152）を使わず `os.TempDir()` から直接組み立てる。`config.Dir()` はディレクトリを作成しエラーを返しうるが、ここは `wails.Run` のオプション値であり、失敗しても起動を止めてはならない（FR-012）。ディレクトリは WebView2 が自分で作る。
- Linux には対応するオプションが無い。`linux.Options` には渡さない（AR-004）。

**WebView の標準の右クリックメニューを出さない**（AR-060 の MUST, FR-063）。

- `options.App.EnableDefaultContextMenu` は**指定しない（`false` のまま）**。Wails v2.15.0 は、リリースビルドではこれが偽のとき、Windows では `AreDefaultContextMenusEnabled` を偽にし、Linux では WebKitGTK のメニューを抑止する。**`true` にしてはならない**——標準のメニューには「戻る」「再読み込み」「検証」が含まれる（AR-060）。
- **開発ビルド（`wails dev`、`-debug`）では、Wails はこの指定に関わらず標準のメニューを出す。** そのためフロントエンドは `contextmenu` を常に `preventDefault()` する（IMP-249）。**リリースビルドと開発ビルドで右クリックの振る舞いを変えない。**

> [!IMPORTANT]
> **指定したパスが使えないと、WebView2 の環境生成に失敗して `os.Exit(1)` する**（go-webview2 の `Embed` → `errorCallback`）。これは FR-111 が禁じる異常終了に当たるが、**指定しない場合も `%APPDATA%` が書けなければ同じ結果になる**ため、この変更が新たな失敗経路を作るわけではない。むしろ書き込み先が UI-112 と同じテンポラリに揃い、前提が 1 つ減る。

起動時の表示対象が読み込めない場合も、**ウィンドウは必ず開く**（FR-012）。エラーの種類に応じて `InitialStateDTO.StateKind` を設定し、状態画面を初期表示とする（IMP-303）。

| 起動時の事象 | `StateKind` | `Document` |
| --- | --- | --- |
| 正常に読み込めた | `""` | 文書 |
| 表示対象が見つからない | `"welcome"` | null |
| 10 MB 超（`ErrNeedsConfirm`） | `"confirm-large"` | null（`Error` にパスとサイズ） |
| 50 MB 超（`ErrTooLarge`） | `"too-large"` | null（同上） |
| 変換に失敗した | `"render-error"` | null（`Error` に内容） |
| 存在しない・読めないパス、**開けない種類のファイル**（FR-012。ツリールートはその親ディレクトリ）を指定された | `"welcome"` | null（加えてエラーをステータス表示。`not-found` / `permission` / `not-supported`） |

`confirm-large` で起動した場合、`pendingConfirm`（IMP-190）にそのパスを設定し、`OpenConfirmed` を受け付けられる状態にする。

**`StateKind` が `welcome` 以外のとき、`target`（IMP-190）にもそのパスを設定する。** 起動直後に「エディタで開く」を押せる状態にするためであり、規則は IMP-192 と同一である（`showing` は偽、監視は張らない、`Error.DisplayPath` / `OutsideTree` / `TargetKind` を入れる）。`welcome` のときは空のままとする。

### IMP-194: 終了処理 **MUST**

- **書き込みの途中なら、終わるまで上限つきで待つ。** `OnShutdown` は `ioMu` を最大 2 秒待ってから（`TryLock` を短い間隔で試す）後の処理へ進む。待たずに終わると、`document.Replace`（IMP-107）の途中でプロセスが終わり、**一時ファイルが残りうる**（FR-143, NFR-031）。上限を設けるのは、読み込みと変換（大きな文書では秒単位）で終了を止めないためである。**取れた `ioMu` は解かずに持ったまま後の処理へ進む**（4.67.0）——解くと、その直後に遅れて届いた書き込み（`SetTask` など）が始まり、その途中でプロセスが終わって一時ファイルが残りうる。`OnShutdown` の後にプロセスは終わるため、持ち続けて困る呼び出し元は無い。
- `watcher` を停止し、ゴルーチンを終了させる。
- `config.Save` を呼ぶ（UI-114）。失敗しても終了を妨げない。
- 履歴・表示中パスは保存しない（NFR-042）。

**ウィンドウのサイズは Go 側で取得する。** フロントエンドから通知されない（`ConfigDTO` に含まれない。IMP-303）ため、保存の直前に Wails のランタイムから読み出す。最大化しているかどうかも同時に読むが、これは**保存しないと決めるための判定**であって、保存する値ではない（UI-111）。

> [!IMPORTANT]
> **取得は `OnBeforeClose` で行う。`OnShutdown` では取得できない。**
> `OnShutdown` が呼ばれる時点でウィンドウは既に破棄されており、
> `runtime.WindowGetSize` は DPI を 0 として除算し、**panic する**
> （Wails v2.15.0 / Windows で実測。`winc.ScaleToDefaultDPI`）。
> 終了のたびにアプリケーションが異常終了することになり、FR-111 に反する。
>
> 役割を次のように分ける。
>
> | 契機 | 行うこと |
> | --- | --- |
> | `OnBeforeClose` | `captureWindowState` のみ。`false` を返して閉じる操作を通す |
> | 保存の予約（UI-114 の 1 秒） | `captureWindowState` + `config.Save`。ウィンドウは生きている |
> | `OnShutdown` | 保存の予約を止め、`ioMu` を上限つきで待ち、`watcher` を閉じ、`config.Save`。**Wails のランタイムを呼ばない** |
>
> `captureWindowState` 自体にも `recover` を置く。ウィンドウの状態を読む API は
> ウィンドウの生存に依存しており、取りこぼしても保存は続けるべきである
> （FR-111, IMP-022）。

```go
// OnBeforeClose と、保存の予約から呼ぶ
func (a *App) captureWindowState() {
    // 最大化中は画面いっぱいの値が返るため、サイズを取り込まない。
    // 最大化していること自体は保存しない（UI-111, IMP-150）
    if runtime.WindowIsMaximised(a.ctx) {
        return
    }
    a.cfg.WindowWidth, a.cfg.WindowHeight = runtime.WindowGetSize(a.ctx)
}
```

- 最大化状態で終了した場合、**そのときの画面いっぱいのサイズを保存しない。** 保存すると、次回のウィンドウが画面いっぱいの大きさで開く。幅と高さは最大化する前の値を保つ。**最大化状態そのものは保存しないため、次回は通常状態で開く**（UI-111, UI-115）。
- ウィンドウ位置と最大化状態は保存しない。構造体にフィールドが存在しない（IMP-150, UI-111）。

### IMP-195: 編集モードの処理 **MUST**

FR-140〜FR-147 を実装する。**バインドメソッドの入口は `desktop/editmode.go` に置く。ここに書くのは、錠を取ること、ファイルを読み書きする関数を呼ぶこと、イベントを送ることだけとする。** 状態と判断は `document.EditSession`（IMP-109）が持つ（IMP-012）。`editor.go`（外部エディタ。IMP-331）と混ぜない。

**編集モードの開始と終了**（`SetEditMode`。IMP-316）

- **`ioMu` を取る**（IMP-190）。取らないと、文書を開く処理が `edit` を決めてから `document:changed` を送るまでの間に割り込み、状態が食い違う。
- `mu` の内側で `current` と `showing`（IMP-190）を控える。
- 開始なら `a.edit.Start(current, showing)`、終了なら `a.edit.Stop()` を呼ぶ。**開始できなかった場合も失敗として通知しない**（ボタンは淡色のはずである）。
- **編集モードを始めたら（`Start` が編集モードでない状態から始めたとき）、表示中の文書が Markdown なら、同じ `ioMu` の内側で、修飾の目印付きで描き直す**（4.75.0。IMP-120。利用者の判断）。`openLocked`（`trigger: "reload"`）を呼び、`document:changed` を送る。**鍵は引き継がない**（FR-143。描き直しは自分の書き込みではない）。描き直しが届くまでの間に押したチェックボックスは `Stale` で戻る（通知しない）。**図のファイルでは描き直さない**——修飾の目印を付けない（FR-146 の対象外。IMP-126, IMP-127）ため、描き直しても図を描き直すだけである。読み直しに失敗したら、書き込みと同じく `error` イベントを送る（下記）。**図と数式も描き直しになる**（同じ文書の再描画。DSP-352）。
- **編集モードを終えても描き直さない**（利用者の判断）。描画に残った修飾の目印は、次の再描画で消える。**フロントエンドは編集モードでない間は修飾の操作を出さず（IMP-265）、Go 側も編集モードでない指示を拒む**（IMP-109 の 1）ため、残っていても使われない。
- `EditModeDTO{On: a.edit.On(), Seq: a.edit.Seq()}` を返す。描き直した `document:changed` との到着順は決まっていない（IMP-260 が `Seq` で扱う）。

**書き込み**（`SetTask` / `SetCell` / `SetCode` / `ApplyFormat` / `UndoEdit` / `RedoEdit`。IMP-316）

処理の順序を固定する。**全体を `ioMu` で包む**（IMP-190）。番号は IMP-109 の判断の番号と揃えている。

| # | 処理 | 当てはまらないとき |
| --- | --- | --- |
| 1, 2 | `mu` の内側で `current` と `showing` を控え、`a.edit.Check(current, showing, op)` を呼ぶ（IMP-109） | `ErrStale` なら `Stale` を返す（何もしない） |
| 3 | 実体のパス（`filepath.EvalSymlinks`）を求め、`os.Stat` の大きさが `document.MaxSize` 以下なら `os.ReadFile` で読む | 解決・読み込みの失敗は `edit-failed`。**`MaxSize` を超えていれば読まずに `ErrChanged` として 4 へ進む**（把握している内容は `MaxSize` 以下で読み込んだものであり、一致しえない） |
| 4, 5 | `a.edit.Plan(a.renderer, raw, current.Kind, op)` を呼ぶ（IMP-109） | **`ErrChanged` なら `edit-conflict`。** 書き込まず、**`a.edit.Discard()` を呼んでから**、`openLocked`（`trigger: "reload"`）で読み直して `document:changed` を送る。取り消し履歴は、読み直しに成功すれば `edit.Loaded` が作り直す（FR-143, FR-144）。**読み直しに失敗したら `error` イベント（`ErrorDTO`）を送る**（下記）。`ErrStale` なら `Stale`。**`ErrNotEditable` なら `edit-failed`**（パスを添える。不正なバイト列・位置の食い違い）。**`ErrRejected` なら `edit-rejected`、`ErrLocked` なら `code-locked`、`ErrCannotFormat` なら `format-rejected`**（4.75.0。確かめで拒んだ。IMP-106, IMP-122, IMP-124）。`changed` が偽なら `Changed: false`（何も書かない） |
| 6 | `Patch.Apply` で新しい内容を作り、**3 で解決した実体のパス**を `document.Replace` に渡して書き込む | `edit-failed`（パスを添える）。**`Commit` を呼ばない**（履歴を動かさない）。`Patch.Apply` の `ErrChanged` は 4 と同じく `edit-conflict` として扱う |
| 7 | `a.edit.Commit(op, patch, newContent)` を呼ぶ（IMP-109）。`Plan` が `widened` を返していたら、戻り値の `Notice` を `format-widened` にする（FR-146。警告） | — |
| 8 | **監視のイベントを待たずに `openLocked`（`trigger: "edit"`、`refKey: current.RefKey`、`expectDigest: sha256(newContent)`）を呼び、`document:changed` を送る**（FR-014, AR-061）。**内容が書き込んだものと一致すれば鍵を引き継ぐ**（IMP-102, FR-143）。同意は IMP-192 の規則で引き継ぐ（FR-016） | 書き込みは成功として返す。**読み直しに失敗したら `error` イベント（`ErrorDTO`）を送る**（下記） |

- **読み直し（4 と 8）に失敗したら、監視のイベントと同じく `error` イベントで `ErrorDTO` を送る**（IMP-320）。状態画面になる失敗（しきい値をまたいだ・50 MB を超えた・変換に失敗した）では、`openLocked` が `edit.Left()` と `target` の書き換えと監視の解除を済ませており（IMP-192）、**送らないと Go 側だけが状態画面へ移り、画面は編集モードのまま残る**（以後の指示はすべて `Stale` で黙って戻り、セルは `.cell-pending` のまま残る。IMP-262）。表示を変えない失敗（削除・権限）では、フロントエンドはステータスに出すだけでよい。
- **3 で解決した実体のパスを 6 に渡す。** `Replace` の 1（IMP-107）で改めて解決すると、その間にリンク先が変わった場合に、読んだファイルと書くファイルが食い違う。
- **1 と 2 を省かない。** フロントエンドは見た目を先に変えてから呼ぶ（IMP-261, IMP-262）ため、呼び出しは描画より遅れて届きうる。**その間にドロップなどで文書が切り替わっていると、`current` は既に別の文書を指している**——鍵の照合がそれを止める（FR-143）。
- **`Stale` は失敗として通知しない**（FR-143 の「拒んだことは通知しない」）。フロントエンドは見た目を戻すだけにする。
- **4 と 8 は、1〜7 と同じ `ioMu` の内側で `openLocked` を呼ぶ。** 錠を解いてから読み直すと、その間に次の書き込みが入り、自分の書き込みを外部の変更と取り違える（FR-143）。**`open` を呼ばない**——`ioMu` を二重に取って止まる（IMP-190）。
- **4 と 5 の構文解析（`Plan`）は `mu` の外で行う。** 50 MB の文書でも、その間ほかのバインドメソッドを止めない（IMP-192 と同じ考え方）。
- **書き込みの呼び出しの戻り値に `DocumentDTO` を載せない**（AR-061）。表示は 8 の `document:changed` だけで届く。**戻り値とイベントの到着順は決まっていない**ため、フロントエンドはどちらが先でも成り立つように書く（IMP-261）。

**セルとコードブロックのソースの取得**（`GetCellSource` / `GetCodeSource`。IMP-316）

- 書き込みの 1〜3 と同じ手順を踏み（`ioMu` の内側）、`a.edit.CellSource(a.renderer, raw, ref)`（コードブロックと図は `a.edit.CodeSource(a.renderer, raw, current.Kind, ref)`）の結果を返す。`ErrChanged` なら `edit-conflict` として 4 と同じ処理をする。**編集欄を開く前に食い違いに気づかせる**ためである。`ErrStale` なら `Stale` を返す。**`ErrLocked` なら `code-locked` を返す**（編集欄を開かず、警告を出す。FR-145）。
- **3 の解決・読み込みの失敗は `Stale` を真で返し、通知しない。** `edit-failed` の文言は「Failed to save: <path>」であり、保存していない操作には合わない（IMP-310 の `GetCellSource` の回復したパニックと同じ扱い）。編集欄が開かないだけで済み、ファイルの削除や権限の変化は監視と次の書き込みが伝える。

**削除**

- 監視が `Removed` を送ったとき、**`ioMu` を取って**、IMP-192 の「イベントの照合」（`showing` と `session.SameFile(ev.Path, current.Path)`）を通ったときだけ `a.edit.Removed()` を呼び、`document:removed`（IMP-320）を送る（IMP-109）。編集モードは終わり、次に読み込めるまで開始できない。**編集欄を入力ごと残し、閉じた時点で編集モードの表示を解くのはフロントエンドである**（FR-140。IMP-264）。その間の確定はフロントエンドが Go へ送らずに拒む（`edit-removed`）。送っても、Go 側は編集モードでないため `Stale` で拒む（IMP-109 の 1）。**照合しないと、`ioMu` を待つ間に開いた別の文書 B に対して削除の印を立て、B で編集を始められなくなる。**

**状態を変える経路**

- `edit` を変えるのは、上の `SetEditMode`・書き込み・削除と、文書を開く処理（IMP-192 の `edit.Loaded` / `edit.Left`）だけとする。**いずれも `ioMu` の内側である。**

**編集欄が開いている間の操作**（FR-140 の表。4.75.0）

- **拒む・保留するの判断はフロントエンドが行う**（IMP-264）。編集欄はフロントエンドにしか無い。**拒む操作は Go を呼ばない**（文書の切り替え・再読み込み・他の書き込み・編集モードの終了・表の並べ替え）。
- **例外はドロップである。** Linux ではドロップが JavaScript を通らずに Go へ届く（IMP-313）ため、フロントエンドは止められない。**フロントエンドは編集欄を開いたら `SetEditBoxOpen(true)`、閉じたら `false` を呼び（IMP-310）、Go 側は `editBoxOpen`（IMP-190）が真の間に届いたドロップを開かずに拒む**（`error` イベントで `edit-busy` を送る。FR-140）。この値は書き込みの判断には使わない。
- **外部の更新による再描画の保留も、フロントエンドが行う。** Go 側は今どおり読み直して `document:changed` を送り、取り消し履歴を扱う（IMP-109 の `Loaded`）。フロントエンドは、届いた描画を画面へ当てずに控える（IMP-264, IMP-321）。保留の間の確定は、フロントエンドが Go へ送らずに拒む（`edit-held`）。送っても、鍵が新しくなっているため Go 側は `Stale` で拒む（FR-143）——**安全は Go 側の照合が保ち、フロントエンドの判断は知らせ方を決める。**

> [!IMPORTANT]
> **書き込みの判断をフロントエンドの状態に委ねない。** フロントエンドの `state.editMode`（IMP-210）は写しであり、ボタンの見た目のためにある。**Go 側が編集モードを持ち、書き込みのたびに確かめる**（IMP-109 の `Check`）。フロントエンドの不具合で編集モードの外から呼ばれても、ファイルは書き換わらない（FR-140）。

### IMP-196: ファイルツリーの操作 **MUST**

FR-030 / FR-032 / FR-033 / FR-036 / FR-037 / NFR-035 を実装する（4.75.0）。バインドメソッドの入口は `desktop/tree.go` に置く。**検査の判断は `filetree.Check`（IMP-135）が持ち、ここは錠を取り、検査を呼び、結果を起動・クリップボード・ツリールートの変更へ渡すだけにする**（IMP-012）。

| メソッド（IMP-310） | 検査（`filetree.Check` の `want`） | 通ったら | 戻り値 |
| --- | --- | --- | --- |
| `OpenFromTree(path)` | `WantOpenable` | `open(openFromTree)`（IMP-192） | `OpenResultDTO` |
| `ReadDir(path)` | `WantDir`。ツリールートそのものも可。**`path` が空文字ならツリールートを読む**（検査の前に `App.treeRoot` に置き換える。ツリールートが未確定なら空の一覧。IMP-310） | `filetree.ReadDir(path, show)` | `([]TreeNodeDTO, error)` |
| `ListEditorsForTree(path)` | `WantText` | `editorTarget` に控え、`ListEditors` と同じ一覧を返す | `EditorListDTO` |
| `OpenInFileManager(path)` | `WantDir`。ツリールートそのものも可 | `opener.OpenDir(path)`（IMP-173） | `TreeActionDTO` |
| `OpenContainingFolder(path)` | `WantOpenable` | `opener.OpenDir(filepath.Dir(path))` | `TreeActionDTO` |
| `SetTreeRoot(path)` | `WantDir`（ツリールートそのものは受け付けない） | ツリールートを変える（下記） | `TreeActionDTO` |
| `SetTreeRootUp()` | **パスを受け取らない。** いまのツリールートの親を Go 側で求める | 同上 | `TreeActionDTO` |
| `CanSetTreeRootUp()` | パスを受け取らない | いまのツリールートに親があるか（根でないか）を返す。メニューの `Set parent as tree root` の淡色に使う（UI-032。IMP-270） | `bool` |
| `CopyTreePath(path)` | `WantAny`。ツリールートそのものも可 | 絶対パスをクリップボードへ（`CopyToClipboard` と同じ経路。AR-062） | `TreeActionDTO` |

- **`show`（IMP-134）は `App.cfg` の `TreeShowDiagrams` / `TreeShowImages` から作る。** ツリールートは `App.treeRoot`。どちらも `mu` の内側で読み、検査・読み込み・起動は `mu` の外で行う。
- **検査の結果の扱い**: `ErrNotInTree` は何もしない（`OpenResultDTO` と `TreeActionDTO` は空、`ReadDir` は空の一覧とエラー、`ListEditorsForTree` は一覧を空で返し、フロントエンドはウィンドウを出さない）。**`ErrMissing` は `not-found`**（パスを添える。FR-036, FR-110）。**`ReadDir` は種別を返さない**（IMP-308）ため、どちらも Go の `error` として返し、`ErrMissing` の文言を `File not found: <path>` とする（フロントエンドはそのまま出す。4.76.0）。`ErrNotInTree` はツリーから操作する限り起きない。**`Check` がほかの失敗（権限が無い等）を返したら `permission`**（`Cannot access: <path>`。v1.1.0 の `OpenFromTree` と同じ文言。4.76.0）。
- **`OpenFromTree` も検査を通す**（4.75.0）。v1.1.0 は検査せず、`document.Load` が Markdown 以外を拒むことで足りていた。開けるファイルが 3 種類になり、**ツリーに出していない種類（フィルターで隠した画像など）をツリーの経路で開かない**ためである（FR-033）。
- **`ReadDir` も検査を通す。** 展開は読むだけで何も起動しないが、ツリーに出していないディレクトリ（`.git` など）の中身をフロントエンドへ返さない（FR-036。利用者の判断。実現性検討の表の 69）。
- **`OpenContainingFolder` は、ファイルの親ディレクトリを開く。** 親はツリールートそのものでもよい（FR-036）。ファイルを選んだ状態にはしない（IMP-173）。
- **ファイルマネージャの起動の失敗**（`opener.ErrNotFound` / `opener.ErrNotDir` / 起動できない）は `open-failed`（IMP-315）としてステータスに出す。
- **`CopyTreePath` の失敗は `clipboard`**（IMP-315）。成功は知らせない（UI-060 の「情報を出すのはエディタの起動だけ」）。パスの文字列はフロントエンドも持っているが、**ツリーからパスを受け取る操作はすべて同じ検査を通す**（FR-036。ファイルが消えていれば `not-found` を出す）。
- **ツリールートを変える**（`SetTreeRoot` / `SetTreeRootUp`。FR-030）:
  - `SetTreeRootUp` は `filepath.Dir(treeRoot)` を新しいルートとする。**ファイルシステムの根とドライブの根（`filepath.Dir` が自分自身を返す）では何もしない**（メニューの項目は淡色。UI-032）。親が存在しない・ディレクトリでないときも何もしない。
  - **表示中の文書・表示履歴・編集モード・監視・`target` は変えない**（文書の切り替えではない。FR-030）。ツリールートの比較は `session.SamePath`（IMP-191）で行い、同じなら何もしない。
  - 戻り値の `TreeActionDTO` に、新しいルートと、**画面の対象の表示用パス**（`session.DisplayPath(新しいルート, target)`。FR-052）を載せる。フロントエンドはツリーを読み直し、ステータス領域のパスを差し替える（IMP-270）。**パスを自分で相対化しない**（IMP-300 の 2）。
  - **`tree:root-changed` は送らない。** 呼び出したフロントエンドが戻り値で更新する（AR-061）。
  - ツリールートは保存しない（NFR-042）。
- **`ListEditorsForTree` は、`ListEditors`（IMP-310）と同じ一覧に、開くファイルの名前（`EditorListDTO.FileName`。IMP-309）を載せて返す。** `OpenInEditor` は `editorTarget` を開く（IMP-171）。**エディタで開くのは文字のファイルだけ**であり（`WantText`。FR-036）、画像はここで拒まれる。
- 各メソッドの入口で `recover` する（IMP-022）。`OpenFromTree` で回復したパニックは IMP-310 の文書を開くメソッドと同じに扱い、`ListEditorsForTree` はエディタのメソッドと同じく `editor-failed` とし、その他は通知しない（何もしなかったものとする）。

## 11.12 要求一覧

| ID | 概要 | 必須度 |
| --- | --- | --- |
| IMP-100 | document 型定義 | MUST |
| IMP-101 | サイズ閾値 | MUST |
| IMP-102 | 読み込み | MUST |
| IMP-103 | 文字コードの正規化 | MUST |
| IMP-104 | 行数の算出 | MUST |
| IMP-105 | 拡張子と種類の判定（`internal/mdfile`） | MUST |
| IMP-106 | 書き換え位置の対応 | MUST |
| IMP-107 | 置き換えによる書き込み | MUST |
| IMP-108 | 取り消し履歴 | MUST |
| IMP-109 | 編集モードの判断 | MUST |
| IMP-110 | renderer 型定義 | MUST |
| IMP-111 | goldmark の構成 | MUST |
| IMP-112 | GitHub Alerts 拡張 | MUST |
| IMP-113 | 数式の保護 | MUST |
| IMP-114 | シンタックスハイライト | MUST |
| IMP-115 | コードブロックのラッパと Mermaid | MUST |
| IMP-116 | サニタイズ | MUST |
| IMP-117 | 見出しアンカーの生成 | MUST |
| IMP-118 | 画像 URL の書き換え | MUST |
| IMP-119 | PlantUML ブロックの取り出し | MUST |
| IMP-120 | 編集・並べ替え・リンクの目印 | MUST |
| IMP-121 | 書き換え位置の特定 | MUST |
| IMP-122 | コードブロックと図の書き換え | MUST |
| IMP-123 | 見える文字の対応付け | MUST |
| IMP-124 | 修飾の書き込み | MUST |
| IMP-126 | PlantUML ファイルの変換 | MUST |
| IMP-127 | Mermaid ファイルの変換 | MUST |
| IMP-128 | 画像ファイルの変換 | MUST |
| IMP-130 | filetree 型定義 | MUST |
| IMP-131 | 読み込み | MUST |
| IMP-132 | フィルタ規則 | MUST |
| IMP-133 | 空ディレクトリの判定 | SHOULD |
| IMP-134 | 表示する種類 | MUST |
| IMP-135 | ツリーの操作の検査 | MUST |
| IMP-140 | watcher 型定義とライフサイクル | MUST |
| IMP-141 | 監視方式 | MUST |
| IMP-142 | デバウンス | MUST |
| IMP-150 | config 型定義 | MUST |
| IMP-151 | 読み書き | MUST |
| IMP-152 | 保存先の解決 | MUST |
| IMP-153 | 値の範囲 | MUST |
| IMP-160 | アセットサーバのハンドラ | MUST |
| IMP-161 | ローカル配信の検査順序 | MUST |
| IMP-162 | 応答ヘッダ | MUST |
| IMP-170 | 外部委譲 | MUST |
| IMP-171 | エディタの起動 | MUST |
| IMP-172 | プリセットの検出 | MUST |
| IMP-173 | ディレクトリをファイルマネージャで開く | MUST |
| IMP-175 | OS のテーマ設定の取得 | MUST |
| IMP-180 | バージョン情報 | MUST |
| IMP-181 | 同梱資産の情報 | MUST |
| IMP-190 | App が保持する状態 | MUST |
| IMP-191 | 表示履歴 | MUST |
| IMP-192 | 文書を開く共通処理 | MUST |
| IMP-193 | 起動シーケンス | MUST |
| IMP-194 | 終了処理 | MUST |
| IMP-195 | 編集モードの処理 | MUST |
| IMP-196 | ファイルツリーの操作 | MUST |
