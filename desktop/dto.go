package desktop

import (
	"path/filepath"

	"github.com/kznagamori/go_MarkView/internal/document"
	"github.com/kznagamori/go_MarkView/internal/renderer"
)

// 本ファイルは Go とフロントエンドの間でやり取りする型を定める（IMP-302〜307）。
//
// **ここが両者の唯一の接点であり、境界の規約は厳密に守る**（AR-061）。
// json タグで JavaScript 側のフィールド名を明示し、null を返しうるフィールドは
// ポインタ型とする（IMP-301）。
//
// app.go から分けているのは、型定義と合わせると 400 行を超えるためである（IMP-011）。
// 同じ理由で、外部エディタは dto_editor.go、編集モードは dto_editmode.go（IMP-316）、
// 情報ダイアログは dto_about.go、ファイルツリーと履歴は dto_tree.go（IMP-304, IMP-317, IMP-323）、
// 起動直後の状態と設定は dto_state.go（IMP-303）に分けている。

// documentEncoding は読み込んだ文書の文字コード（IMP-302）。
//
// 入力は常に UTF-8 として扱い、不正なバイト列は置換して読み込みを続ける
// （FR-021, IMP-103）。したがってこの値は常に "UTF-8" である。
// AppTitle は文書未表示時のウィンドウタイトル（UI-013）。
const AppTitle = "MarkView"

const documentEncoding = "UTF-8"

// DocumentDTO.Trigger の値（IMP-302, IMP-192）。
//
// フロントエンドは triggerEdit のときだけ、並べ替えを並べ直さずに行の並びを保つ（FR-130,
// IMP-229）。それ以外の用途に使わない。
const (
	triggerOpen = "open" // ダイアログ・ドロップ・引数・ツリー・リンク・履歴・確認画面の Open anyway
	// triggerReload は手動の再読み込み（FR-015）と、書き込み前の不一致による読み直し（IMP-195 の 4）と、
	// 編集モードを始めたときの修飾の目印付きの描き直し（IMP-195。自分の書き込みではない）。
	triggerReload = "reload"
	triggerWatch  = "watch" // ファイル更新の自動検知（FR-014）
	triggerEdit   = "edit"  // 編集モードの書き込み・取り消し・やり直しの直後の読み直し（IMP-195 の 8）
)

// ScrollDTO.Mode の値（IMP-302）。
//
// スクロールの扱いは「どの経路で開いたか」に依存するため Go 側が決める
// （IMP-192）。フロントエンドに経路を意識させない。
const (
	scrollTop     = "top"     // 文書の先頭へ
	scrollAnchor  = "anchor"  // Anchor の見出しがペイン上端付近に来る位置へ
	scrollRestore = "restore" // Top の値へ復元する（履歴移動。FR-051）
	scrollKeep    = "keep"    // フロントエンドが現在位置を保つ。Top は使わない
)

// InitialStateDTO.StateKind の値（IMP-303）。
//
// 空文字は「文書を表示している」ことを表す。
const (
	stateNone         = ""
	stateWelcome      = "welcome"
	stateConfirmLarge = "confirm-large"
	stateTooLarge     = "too-large"
	stateRenderError  = "render-error"
)

// LinkResultDTO.Kind の値（IMP-305）。
const (
	linkDocument = "document"
	linkExternal = "external"
	linkAnchor   = "anchor"
	linkError    = "error"
)

// ErrorDTO.Kind の値（IMP-315）。
//
// フロントエンドはこの値で strings.js の文言を選ぶ。**値とキーは 1 対 1 で
// 対応させる**（IMP-290）。
//
// **Go 側が作る種別だけをここに置く。** edit-held / edit-removed / removed-editing /
// encoding-editing は編集欄の状態からフロントエンドだけが決める（IMP-315。編集欄は
// フロントエンドにしか無い）ため、Go に値を持たない。
const (
	errKindNotFound     = "not-found"
	errKindPermission   = "permission"
	errKindNotSupported = "not-supported" // 開けない種類のファイル（4.75.0。v1.1.0 は「Markdown でない」の種別だった）
	errKindNeedsConfirm = "needs-confirm"
	errKindTooLarge     = "too-large"
	errKindRenderError  = "render-error"
	errKindLinkNotFound = "link-not-found"
	errKindClipboard    = "clipboard"
	errKindRemoved      = "removed"
	errKindEditorFailed = "editor-failed"
	errKindEditorSelf   = "editor-self"
	errKindEncoding     = "encoding"

	// v1.1.0。どれもステータスに出す種別であり、状態画面にしない（IMP-315）。
	errKindOpenFailed   = "open-failed"   // リンク先・ファイルマネージャを OS へ委譲できない（FR-053, FR-036。BUG-013）
	errKindEditConflict = "edit-conflict" // 書き込む前にファイルが変更されていた（FR-143）
	errKindEditFailed   = "edit-failed"   // 書き込めない・不正なバイト列・位置の食い違い（FR-143）

	// v1.2.0（4.75.0）。どれもステータスに警告として出す（IMP-315, DSP-151）。
	errKindEditRejected   = "edit-rejected"   // 書き込む内容の確かめで拒んだ（document.ErrRejected。FR-143）
	errKindCodeLocked     = "code-locked"     // 前置きが行ごとに違うコードブロック（document.ErrLocked。FR-145）
	errKindFormatRejected = "format-rejected" // 修飾を分けられない・確かめで拒んだ（document.ErrCannotFormat。FR-146）
	errKindFormatWidened  = "format-widened"  // 手書きの取り消し線を全体で外した（EditResultDTO.Notice。FR-146）
	errKindEditBusy       = "edit-busy"       // 編集欄が開いている間のドロップを拒んだ（IMP-313。FR-140）
)

// ScrollDTO は描画後のスクロール指示（IMP-302）。
type ScrollDTO struct {
	Mode string `json:"mode"` // scrollTop | scrollAnchor | scrollRestore | scrollKeep
	// Anchor は Mode == scrollAnchor のときの、リンクのフラグメント（# を除き復号した値）。
	// **user-content- を Go 側で付けない**（IMP-302）。フロントエンドの findInDocument が
	// 生のままの値と接頭辞を補った値の順に、本文の中だけを探す（IMP-223, AR-053）。
	Anchor string `json:"anchor"`
	Top    int    `json:"top"` // Mode == scrollRestore のときの位置
}

// DocumentDTO は 1 つの文書の表示に必要な情報をまとめる（IMP-302）。
//
// **1 つの利用者操作に対する呼び出しは 1 回とし、必要な情報をまとめて返す**
// （IMP-300, AR-061）。
type DocumentDTO struct {
	Path        string `json:"path"`        // 絶対パス（IMP-025）
	DisplayPath string `json:"displayPath"` // ステータス表示用（UI-060）
	Name        string `json:"name"`        // ベース名（UI-013 のタイトル）
	OutsideTree bool   `json:"outsideTree"` // ツリー外の文書か（FR-052）

	HTML      string             `json:"html"`      // サニタイズ済み（IMP-116）
	Headings  []renderer.Heading `json:"headings"`  // アウトライン（FR-040）
	LineCount int                `json:"lineCount"` // 総行数（UI-060）
	Encoding  string             `json:"encoding"`  // 常に documentEncoding

	NeedsMermaid  bool `json:"needsMermaid"`  // Mermaid の遅延ロード判定（AR-021）
	NeedsKaTeX    bool `json:"needsKaTeX"`    // KaTeX の遅延ロード判定（AR-021）
	NeedsPlantUML bool `json:"needsPlantUML"` // PlantUML の遅延ロード判定（AR-021, MD-085）

	Scroll ScrollDTO `json:"scroll"` // 描画後のスクロール指示

	// Warnings は描画を継続する事象の Kind（IMP-315）。文言そのものではない。
	//
	// フロントエンドは Kind から strings.js の文言を選ぶ。文言の定義を
	// 1 箇所に集約するためであり、ErrorDTO.Kind と同じ扱いである（IMP-290）。
	Warnings []string `json:"warnings"`

	// v1.1.0（FR-120〜FR-144）。再描画のたびに状態をどう引き継ぐかを伝える（DSP-352）。
	// **判断はすべて Go 側で済ませて渡す**（IMP-300 の 2）。

	// SameDocument は直前の表示と同じファイルか（1.7。IMP-192 が session.SameFile で決める）。
	// **直前が状態画面なら同じファイルでも偽**。フロントエンドはパスを比べて自分で判断しない。
	SameDocument bool `json:"sameDocument"`
	// Trigger は開いた経路（triggerOpen など）。
	Trigger string `json:"trigger"`
	// RefKey は目印（data-ref / data-link。図のブロックを含む）の鍵（IMP-120, IMP-260）。
	RefKey string `json:"refKey"`
	// Editable は編集モードを開始できるか（FR-140 の表。EditSession.CanStart）。
	Editable bool `json:"editable"`
	// EditMode はいま編集モードか（Go 側が正。IMP-109）。
	EditMode bool `json:"editMode"`
	// EditSeq は編集モードの状態の版（EditSession.Seq）。フロントエンドは、それまでに受け取った
	// 版より小さければ Editable / EditMode を写さない（到着順が決まっていないため。IMP-260）。
	EditSeq uint64 `json:"editSeq"`

	// v1.2.0（FR-025〜FR-027, FR-145〜FR-147。4.75.0）。

	// Kind は文書の種類（"markdown" | "plantuml" | "mermaid" | "image"。IMP-105）。**フロントエンドは
	// 文書の種類による違いをこれだけで決める**——class="image-file" の有無で判断しない（生 HTML でも
	// 同じ class を書ける。IMP-128）。
	Kind string `json:"kind"`
	// FileSize はファイルの大きさ（バイト）。画像ファイルのステータス表示に使う（UI-060, DSP-150）。
	FileSize int64 `json:"fileSize"`
	// NoDiagrams は、図のファイルで図が 1 つも無いか（No diagrams found. を出す。IMP-126, IMP-127）。
	NoDiagrams bool `json:"noDiagrams"`
	// TextRefs は修飾の目印（text:<n>）を付けた描画か（IMP-120）。**真のときだけ修飾の操作を使える**
	// （IMP-265）。編集モードを始めた直後は、目印付きの描き直し（IMP-195）が届くまで偽である。
	TextRefs bool `json:"textRefs"`
}

// documentView は DocumentDTO のうち、文書そのものではなく「どう表示するか」を決める値
// （IMP-302, IMP-192）。開いた経路と編集モードの状態から、開く処理が決めて渡す。
type documentView struct {
	displayPath string    // session.DisplayPath（UI-060）
	outsideTree bool      // 同上（FR-052）
	scroll      ScrollDTO // 経路で決まる（IMP-192）

	sameDocument bool   // session.SameFile かつ直前が文書の表示（IMP-192）
	trigger      string // triggerOpen など
	editable     bool   // EditSession.CanStart
	editMode     bool   // EditSession.On
	editSeq      uint64 // EditSession.Seq
}

// OpenResultDTO は文書を開く操作の結果（IMP-308）。
//
// **失敗を Go の error ではなくこの構造体で返す。** Wails v2 は error を
// メッセージ文字列としてしかフロントエンドへ渡せず（NewErrorCallback）、
// ErrorDTO の Kind・Size・Limit が失われる。それでは大きなファイルの確認画面
// （FR-016, IMP-314）を出せない。
//
// Document と Error がどちらも nil の場合は「何も起きなかった」を表す。
// ダイアログの取り消し、履歴の端、表示中の文書がない状態での再読み込みが
// これにあたる。フロントエンドは表示を変えない。
type OpenResultDTO struct {
	Document *DocumentDTO `json:"document"` // 成功したとき
	Error    *ErrorDTO    `json:"error"`    // 失敗したとき
}

// newOpenResult は open の戻り値を OpenResultDTO へ写す（IMP-308, IMP-315）。
//
// path は ErrorDTO に載せる対象。エラー値からは取り出せないため呼び出し側が渡す。
func newOpenResult(path string, dto *DocumentDTO, err error) OpenResultDTO {
	if err != nil {
		return OpenResultDTO{Error: newErrorDTO(path, err)}
	}

	return OpenResultDTO{Document: dto}
}

// LinkResultDTO は本文中のリンクを踏んだ結果（IMP-305）。
type LinkResultDTO struct {
	Kind     string       `json:"kind"`     // linkDocument | linkExternal | linkAnchor | linkError
	Document *DocumentDTO `json:"document"` // Kind == linkDocument のとき
	Anchor   string       `json:"anchor"`   // Kind == linkAnchor のとき
	Error    *ErrorDTO    `json:"error"`    // Kind == linkError のとき
}

// ErrorDTO は異常をフロントエンドへ伝える（IMP-307, IMP-315）。
//
// **文言の組み立てはフロントエンドで行う。** Go 側は Kind と要素（パス・
// サイズ）を渡し、フロントエンドが strings.js の文言を選ぶ（IMP-290）。
// Message には Go 側が組み立てた英語も入れる。未知の Kind を受け取った場合の
// フォールバックとして用いる。
type ErrorDTO struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
	Path    string `json:"path"`  // 対象がある場合
	Size    int64  `json:"size"`  // サイズ関連のときのみ
	Limit   int64  `json:"limit"` // サイズ関連のときのみ

	// DisplayPath / OutsideTree は、**状態画面を出す種別（needs-confirm / too-large /
	// render-error）でだけ**設定する（IMP-307）。値は DocumentDTO の同名のフィールドと同じ規則
	// （session.DisplayPath）で、画面の対象（target）について求める。
	//
	// 状態画面では DocumentDTO が届かないため、ステータス領域の左に出すパスをここから得る
	// （FR-016, DSP-302, IMP-250）。**v1.0.0 はこの値を持たず、状態画面の間も前の文書の
	// パスが残っていた**（BUG-012）。ステータスに出す種別では空のまま（表示を変えない失敗）。
	DisplayPath string `json:"displayPath"`
	OutsideTree bool   `json:"outsideTree"`

	// TargetKind は、状態画面の対象の種類（mdfile.KindOf の文字列。IMP-105, IMP-307。4.75.0）。
	// DisplayPath と同じく**状態画面を出す種別でだけ**設定する。状態画面では DocumentDTO.Kind が
	// 届かず、フロントエンドは拡張子から種類を判断しない（IMP-300 の 2）。大きすぎる画像の状態画面で
	// 「エディタで開く」を淡色にするためにある（UI-021, IMP-202）。
	TargetKind string `json:"targetKind"`
}

// newDocumentDTO は Document を DTO へ写す（IMP-302）。
//
// 表示用パス・スクロール指示・経路・編集モードの状態は、開いた経路と Go 側の状態で
// 決まるため、呼び出し側から受け取る（IMP-192）。
func newDocumentDTO(doc *document.Document, v documentView) *DocumentDTO {
	return &DocumentDTO{
		Path:        doc.Path,
		DisplayPath: v.displayPath,
		Name:        filepath.Base(doc.Path),
		OutsideTree: v.outsideTree,

		HTML:      doc.HTML,
		Headings:  headingsOrEmpty(doc.Headings),
		LineCount: doc.LineCount,
		Encoding:  documentEncoding,

		NeedsMermaid:  doc.NeedsMermaid,
		NeedsKaTeX:    doc.NeedsKaTeX,
		NeedsPlantUML: doc.NeedsPlantUML,

		Scroll:   v.scroll,
		Warnings: warningKinds(doc.Warnings),

		SameDocument: v.sameDocument,
		Trigger:      v.trigger,
		RefKey:       doc.RefKey,
		Editable:     v.editable,
		EditMode:     v.editMode,
		EditSeq:      v.editSeq,

		Kind:       string(doc.Kind),
		FileSize:   doc.Size,
		NoDiagrams: doc.NoDiagrams,
		TextRefs:   doc.TextRefs,
	}
}

// headingsOrEmpty は nil を空スライスへ均す。
//
// JSON の null をフロントエンドへ渡さないためである。null が来ると
// outline.js の走査が落ち、アウトラインだけでなく描画全体が止まる。
func headingsOrEmpty(headings []renderer.Heading) []renderer.Heading {
	if headings == nil {
		return []renderer.Heading{}
	}
	return headings
}

// warningKinds は警告を Kind の並びへ写す（IMP-315）。
//
// 戻り値は常に非 nil とする（headingsOrEmpty と同じ理由）。未知の種別は
// 落とす。フロントエンドが解釈できない値を渡しても表示できないためである。
func warningKinds(warnings []document.Warning) []string {
	kinds := make([]string, 0, len(warnings))

	for _, w := range warnings {
		if w.Kind == document.WarnInvalidEncoding {
			kinds = append(kinds, errKindEncoding)
		}
	}

	return kinds
}
