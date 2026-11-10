# 12. 実装仕様: フロントエンド側

> 索引: [README](README.md) | 実装仕様: [10](10-impl-overview.md) / [11](11-impl-backend.md) / **12** / [13](13-impl-interface.md)

本文書は `frontend/` 配下の実装仕様を定める。ここでは**構造と振る舞い**（DOM の構成、クラス名、モジュール分割、処理）を定め、**見た目**（配色・寸法・状態の表現）は表示仕様書（`20`〜`22`）で定める。両者はクラス名で接続する。

## 12.1 構成（IMP-200 系）

### IMP-200: ファイル構成 **MUST**

ビルド工程を持たないため、ここに置いたファイルがそのまま埋め込まれる（AR-050, IMP-030）。

```
frontend/
├── index.html              単一のページ。全ペインの骨格を含む
├── css/
│   ├── tokens.css          デザイントークン（DSP-010 系）
│   ├── base.css            リセットと全体レイアウト
│   ├── components.css      ツールバー・ペイン・検索バー・ダイアログ
│   ├── markdown.css        本文の描画（github-markdown-css 由来）
│   └── chroma.css          シンタックスハイライト配色（IMP-114）
├── js/
│   ├── main.js             起動処理・イベントの購読・ショートカットとダイアログの配線
│   ├── navigate.js         文書を開く経路と結果の反映（状態画面を出すのはここだけ。IMP-250）
│   ├── state.js            フロントエンド側の状態
│   ├── api.js              Go バインディングの薄いラッパ（13 章）
│   ├── strings.js          UI 文言の一元定義（IMP-290）
│   ├── toolbar.js          ツールバー（群と区切り・戻る / 進むの淡色・修飾の群の出し入れ。IMP-202）
│   ├── filetree.js         ファイルツリーペイン（読み込み・展開と折りたたみ・強調・クリックとキー操作）
│   ├── treenodes.js        ファイルツリーの項目の組み立てと、項目の中を読む関数（DOM だけを扱う）
│   ├── treemenu.js         ファイルツリーの右クリックメニューの項目と実行（IMP-270。4.75.0）
│   ├── treefilter.js       ファイルツリーのフィルターの枚（IMP-271。4.75.0）
│   ├── outline.js          アウトラインペイン
│   ├── viewer.js           本文の挿入と後処理の順序・リンク・スクロール・フォーカス・再描画で引き継ぐ状態
│   ├── decorate.js         本文の飾り付け（Alerts のアイコン・読み込みに失敗した画像・見出しのアンカー。IMP-225〜IMP-227）
│   ├── copy.js             コードブロックのコピー
│   ├── search.js           文書内検索
│   ├── zoom.js             表示倍率
│   ├── theme.js            テーマ適用
│   ├── panes.js            ペインの見出し・開閉・リサイズ
│   ├── shortcuts.js        キーボードショートカット
│   ├── tooltip.js          ツールチップ（ツールバー・本文中のボタン・拡大画面。IMP-247）
│   ├── dnd.js              ドラッグ＆ドロップ
│   ├── lazy.js             Mermaid / KaTeX / PlantUML の遅延ロード（呼び出し側が import する唯一の入口）
│   ├── puml.js             PlantUML の読み込みと描画（lazy.js が再 export する。IMP-233）
│   ├── drawing.js          図の器の連番・描画の世代・描き直しの間の高さ・資産の読み込み（lazy.js と puml.js が共有する。IMP-230, DSP-370）
│   ├── status.js           ステータス領域
│   ├── overlay.js          情報ダイアログ・エディタ選択ダイアログ・状態画面（#overlay の開閉と Tab、各 API）
│   ├── about.js            情報ダイアログの中身（IMP-251）
│   ├── editors.js          エディタ選択ダイアログの中身（IMP-252）
│   ├── editorflow.js       エディタで開く流れ（一覧を Go から得て出す・Browse・Open・閉じたときの控えの破棄。IMP-252, IMP-270。4.93.0）
│   ├── media.js            図と画像のボタン・原寸表示（IMP-228）
│   ├── mediabuttons.js     図と画像のボタンの組み立てと、小さい画像のボタンの置き場所（IMP-228。4.91.0）
│   ├── tablesort.js        表の表示上の並べ替え（IMP-229）
│   ├── contextmenu.js      右クリックメニュー（IMP-249）
│   ├── menuactions.js      右クリックメニューの項目の実行（IMP-249。4.70.0）
│   ├── menuparts.js        右クリックメニューの項目の組み立てと置き場所（IMP-249。4.93.0）
│   ├── expand.js           拡大画面（IMP-253）
│   ├── expandbar.js        拡大画面の操作バーとキーの表（IMP-253。4.91.0）
│   ├── editmode.js         編集モード・チェックボックス・取り消し（IMP-260, IMP-261, IMP-263）
│   ├── blockedit.js        編集欄の共通の部品（開いている間の拒む・保留・確定の待ち・開き直し。IMP-264。4.75.0）
│   ├── celledit.js         セルの編集欄（IMP-262。4.75.0 で editmode.js から分けた）
│   ├── codeedit.js         コードブロックと図の編集欄（IMP-264。4.75.0）
│   ├── format.js           選んだ文字の修飾（取り消し線と文字色。IMP-265。4.75.0）
│   ├── visible.js          修飾するブロックの見える文字の数え方（IMP-266。4.75.0）
│   ├── refs.js             目印の鍵の照合（IMP-260。state.js 以外を import しない）
│   ├── docswitch.js        文書の切り替えと状態画面への移行の後始末（IMP-250 の leaveDocument）
│   └── util.js             共通ユーティリティ
├── icons/                  インライン SVG のソース（IMP-203。ファイル名はシンボル ID）
└── vendor/                 BR-042 が管理する資産
```

- **`about.js` と `editors.js` は `overlay.js` を import しない。** `overlay.js` が 400 行の目安を超えていた（v1.0.0 で 509 行）ため、エディタ選択ダイアログ（v1.0.0）に続いて情報ダイアログの中身も分けた。閉じる処理とリンクの処理は引数（`handlers`）で受け取る（IMP-251, IMP-252）。
- **`treenodes.js` は `filetree.js` を import せず、Go を呼ばず、`state` も読まない。** `filetree.js` が 400 行の目安を超えた（v1.1.0 の BUG-012 の対策で 412 行）ため、項目の要素の組み立て（`fill`）と、項目の中を読む関数（`childByName` / `childGroup` / `depthOf` / `setDirIcons`）を分けた。
- **v1.2.0 で、セルの編集欄（IMP-262）を `celledit.js` へ分け、セルとコードブロックの編集欄に共通する部品を `blockedit.js` に置いた**（4.75.0。v1.1.0 の `editmode.js` は 358 行で、コードブロックの編集欄を足すと 400 行の目安（IMP-011）を超える）。分けても、鍵を読むのは `refs.js` だけとする（IMP-260）。
- **`blockedit.js` は `celledit.js` / `codeedit.js` / `editmode.js` / `viewer.js` / `navigate.js` / `main.js` を import しない**（いずれもこのモジュールを呼ぶ。IMP-264）。
- **`visible.js` は自前の他のモジュールを import しない**（描画スモーク（BR-054）が直接読み、Go 側の数え方と突き合わせる。IMP-266）。
- **`mediabuttons.js` と `expandbar.js` は、自前のモジュールのうち `strings.js` と `util.js` だけを import する**（4.91.0）。`media.js` と `expand.js` はどちらも 400 行で、画像ファイルの表示（FR-027）を足すと 400 行の目安を超えるため、状態を持たない組み立ての部分を分けた。**状態は元のモジュールが持ち、押されたときの処理を渡す**（IMP-228, IMP-253）。元のモジュールの import の制約（IMP-228 / IMP-253）を崩さない。
- **`menuparts.js` は、自前のモジュールのうち `refs.js`・`strings.js`・`util.js` だけを import し、状態を持たない**（4.93.0）。`contextmenu.js` は v1.2.0 の修飾の項目・サブメニュー・文字色の一覧（IMP-249, IMP-265）を足すと 469 行になったため、場所ごとの項目（`buildItems`）・印（`checkable`）・Text color の項目・色の一覧（`colorItems`）・置き場所（`placeAt` / `placeBeside` / `placeBelow`）・リンク先と選択範囲とキーボードから開いたときの位置の求め方（`linkTargetOf` / `selectionIn` / `keyboardPoint`）を分けた。**開閉・フォーカス・場所の判定は `contextmenu.js` に残す。**
- **`editorflow.js` は `main.js` を import しない**（4.93.0）。`main.js` はツリーのメニューの配線（IMP-270）を足すと 400 行の目安を超えるため、エディタで開く流れ（`showEditorDialog` / `openEditorsForTree` / `browseEditor` / `openInEditor` / `closeEditorDialog`）を分けた。**`overlay.js` / `editors.js` は描くだけ**（IMP-201, IMP-252）という分担は変えない。
- **`navigate.js` は `main.js` を import しない**（`main.js` が配線のために `navigate.js` を import する）。`main.js` が 400 行の目安を超えたため、文書を開く経路（ダイアログ・ツリー・リンク・履歴・再読み込み・確認画面の `Open anyway`）と、その結果（`OpenResultDTO` / `LinkResultDTO` / イベントの `DocumentDTO` / `ErrorDTO`）の反映を分けた。
- **`decorate.js` は `viewer.js` / `overlay.js` / `main.js` / `navigate.js` / `docswitch.js` を import しない**（`viewer.js` が `renderDocument` の手順 4〜6 で呼ぶ）。`viewer.js` が 400 行の目安を超えたため、手順の順序を持たない飾り付け（IMP-225 / IMP-226 / IMP-227）を分けた。**`viewer.js` は `markBrokenImages` を再 export する**——描画スモーク（BR-054）は `viewer.js` を動的に読んでこの関数を呼び、`viewer.js` の連結の失敗を 1 件の失敗として捉える（UT-811 のケース 8）。`decorate.js` を直接読むと、その失敗が見えなくなる。
- **`lazy.js` は `puml.js` の `ensurePlantUML` / `drawPlantUML` / `redrawPlantUML` と `drawing.js` の `startDrawing` を再 export する。** 呼び出し側（`viewer.js` / `theme.js` は `drawDiagrams` / `redrawDiagrams`、描画スモークの `harness.js` は `drawMermaid` / `drawPlantUML`）は `lazy.js` だけを import する（IMP-230 の export の置き場所は変えない）。**`drawing.js` は自前の他のモジュールを import しない**——図の器の連番（IMP-233 の 2 で Mermaid と PlantUML が共有する）と描画の世代を片方に置くと循環する。

### IMP-201: モジュール方式 **MUST**

- `<script type="module">` による ES モジュールとし、`import` / `export` で依存を明示する。バンドラを使わない（AR-050）。
- グローバル変数を追加しない。`window` への代入は行わない。
- 各モジュールは「初期化関数を 1 つ export する」形を基本とし、`main.js` が順に呼ぶ。

```js
// 例: js/toolbar.js
export function initToolbar(deps) { /* … */ }
```

### IMP-202: DOM の骨格 **MUST**

`index.html` は以下の構造を持つ。クラス名・ID は表示仕様書（21 章）が参照するため、変更する場合は両方を更新する。

```html
<div id="app" data-theme="light">
  <header id="toolbar" class="toolbar">
    <button id="btn-back"     class="tb-btn" type="button" disabled></button>
    <button id="btn-forward"  class="tb-btn" type="button" disabled></button>
    <span class="tb-sep" aria-hidden="true"></span>
    <button id="btn-open"     class="tb-btn" type="button"></button>
    <button id="btn-reload"   class="tb-btn" type="button" disabled></button>
    <span class="tb-sep" aria-hidden="true"></span>
    <button id="btn-filetree" class="tb-btn tb-toggle" type="button" aria-pressed="false"></button>
    <button id="btn-outline"  class="tb-btn tb-toggle" type="button" aria-pressed="true"></button>
    <button id="btn-theme"    class="tb-btn" type="button"></button>
    <span class="tb-sep" aria-hidden="true"></span>
    <button id="btn-edit"     class="tb-btn" type="button" disabled></button>
    <button id="btn-editmode" class="tb-btn tb-toggle" type="button" aria-pressed="false" disabled></button>
    <span id="tb-format" class="tb-group" hidden>
      <span class="tb-sep" aria-hidden="true"></span>
      <button id="btn-strike" class="tb-btn" type="button" disabled></button>
      <button id="btn-color"  class="tb-btn" type="button" aria-haspopup="menu" aria-expanded="false" disabled></button>
    </span>
    <span class="tb-spacer"></span>
    <button id="btn-about"    class="tb-btn" type="button"></button>
  </header>

  <main id="content" class="content">
    <nav id="pane-filetree" class="pane pane-filetree" hidden>
      <div class="pane-head">
        <div class="pane-title"></div>
        <div class="pane-actions">
          <button id="btn-treefilter" class="pane-btn" type="button" aria-haspopup="dialog" aria-expanded="false"></button>
        </div>
      </div>
      <div class="pane-subtitle" id="tree-root-name"></div>
      <div class="pane-body">
        <ul id="tree" class="tree" role="tree"></ul>
      </div>
    </nav>
    <div id="resizer-filetree" class="resizer" hidden></div>

    <nav id="pane-outline" class="pane pane-outline">
      <div class="pane-title"></div>
      <ul id="outline" class="outline"></ul>
    </nav>
    <div id="resizer-outline" class="resizer"></div>

    <div id="viewer-frame" class="viewer-frame">
      <section id="viewer" class="viewer" tabindex="-1">
        <div class="searchbar-anchor">
          <div id="searchbar" class="searchbar" hidden></div>
        </div>
        <article id="markdown" class="markdown-body"></article>
        <div id="state-screen" class="state-screen" hidden></div>
      </section>
      <div id="editmode-badge" class="editmode-badge" hidden></div>
    </div>
  </main>

  <footer id="statusbar" class="status">
    <span id="status-path" class="status-path"></span>
    <span id="status-meta" class="status-meta"></span>
    <span id="status-message" class="status-message" hidden></span>
  </footer>

  <div id="overlay" class="overlay" hidden></div>
  <div id="expand-view" class="expand-view" role="dialog" aria-modal="true" hidden></div>
  <div id="contextmenu" class="contextmenu" role="menu" hidden></div>
  <div id="contextsubmenu" class="contextmenu contextmenu-sub" role="menu" hidden></div>
  <div id="treefilter" class="treefilter" role="dialog" hidden></div>
  <div id="dropzone" class="dropzone" hidden></div>
  <div id="tooltip" class="tooltip" hidden></div>
</div>
```

- **ツールバーのボタンは、群（移動・ファイル・表示・編集・修飾）の順に並べ、群の間に区切り（`.tb-sep`）を置く**（UI-020。4.75.0）。**ボタンの id は変えない**——`panes.js` は `btn-${name}` で id を組み立て、`overlay.js` / `theme.js` も id を直接参照している。並びは `index.html` の順序だけで変わる。**`role="toolbar"` は付けない**（付けると矢印キーでボタン間を移る作法が求められ、`Tab` で移る今の操作（UI-021）と変わる）。
  - **修飾の群（`#tb-format`）は、手前の区切りと一緒に `hidden` で出し入れする**（編集モードの間だけ。UI-020）。**隠すとき、その中のボタンにフォーカスがあれば `#btn-editmode` へ移す**（`hidden` の要素にフォーカスが残ると `Tab` の起点が失われる）。**`#btn-editmode` が `disabled` なら（ファイルの削除・状態画面で終えたとき）、本文ペインへ移す**（`focusViewer()`。4.76.0）。
  - **v1.1.0 の `#btn-outline` の `margin-left` は消す**（4.75.0。群の間隔は区切りが受け持つ。DSP-032, DSP-103）。
  - **押せるかどうか**（UI-021）: `#btn-back` / `#btn-forward` は `state.history`（IMP-323）、`#btn-reload` は画面の対象があるか（`welcome` では押せない。v1.1.0 は淡色にしていなかった）、`#btn-edit` は画面の対象が文字のファイルか（`welcome` と画像ファイルでは押せない。FR-090。種類は `state.doc.kind`、状態画面の間は `state.target.targetKind`（`ErrorDTO.TargetKind`。IMP-307）で見る）、`#btn-editmode` は `state.editable`、`#btn-strike` / `#btn-color` は IMP-265 の `canFormat()`。**起動時は `#btn-about` 以外の押せるかが決まらないボタンを `disabled` で読み込む**（確定するまで押せない側に倒す）。
- **ファイルツリーペインは、見出しの行（`.pane-head`）とツリールートの名前の行（`#tree-root-name`）を、スクロールする器（`.pane-body`）の外に置く**（UI-030。4.75.0）。v1.1.0 は `.pane` そのものがスクロールする器で、見出しの行は `position: sticky; top: 0` で縦にしか留まらず、**横にスクロールすると右端に置いたボタンが画面の外へ流れた**（実現性検討の試作で、186px のスクロールで 209px → 23px へ流れた。付録 A.14）。器の外に出すと動かない。**アウトラインのペインは変えない**（ボタンを置かない。DSP-110）。
- **`#btn-treefilter` は `.pane-actions` の中に置き、今後ツリーの機能のボタンを足すときも、ここへ右から左へ並べる**（UI-030）。
- **`#treefilter` と `#contextsubmenu` はペインと本文の外に置く**（4.75.0）。フィルターの枚はペインより広く、アウトラインや本文の上へはみ出す（UI-033）——ペインの中に置くと、ペインの端で切れ、ツリーと一緒にスクロールして流れる（検索バーの受け皿で起きたのと同じ問題）。位置はボタンの位置から求める（IMP-271）。重なり順は DSP-015。
- **ツールバーの文字色の一覧は `#contextmenu` を使い回す**（右クリックメニューと同時に開かない。UI-085。IMP-249）。
- **`#viewer-frame` は v1.1.0 で足した受け皿である**（UI-055, DSP-126）。スクロールしない器として `#viewer` を包み、**編集モードの枠と右下のラベル（`#editmode-badge`）をスクロールに流されない位置に置く。** `#viewer` の中に置くと、`#searchbar` と同じく内容と一緒に流れる（下記）。**`#viewer` がスクロールする唯一の器であることは変わらない**（UI-051）。
  - `#viewer-frame` は `position: relative`（枠とラベルの位置の基準。DSP-126）とし、**これまで `.viewer` が `main` の中で持っていた伸縮（`flex: 1; min-width: 0`）を引き受ける。** 中の `#viewer` は `#viewer-frame` の幅と高さいっぱいに広げる。**本文ペインの寸法とスクロールの振る舞いを v1.0.0 から変えない。**
- **`#expand-view` と `#contextmenu` を `#overlay` の中に入れない。** `#overlay` は情報ダイアログとエディタ選択ダイアログの暗幕であり、全面を覆う（IMP-251）。拡大画面はステータス領域を覆わず（UI-104）、右クリックメニューは情報ダイアログの**上に**出る（ライセンス欄。FR-063）。重なり順は DSP-015 が定める。
- **新しい id（`viewer-frame` / `editmode-badge` / `expand-view` / `contextmenu`。4.75.0 で `btn-back` / `btn-forward` / `tb-format` / `btn-strike` / `btn-color` / `btn-treefilter` / `contextsubmenu` / `treefilter`）も、同梱資産が決め打ちする id と重ならないことを `scripts/domids` で確かめる**（BR-043, 下記の IMPORTANT）。
- **`scripts/domids` が読むのは `index.html` だけであり、JavaScript で作る id は検査されない。** v1.1.0 で足したモジュール（IMP-228, IMP-229, IMP-249, IMP-253, IMP-260〜IMP-263）と v1.2.0 で足すモジュール（IMP-264〜IMP-266, IMP-270, IMP-271）は JavaScript で `id` を作らず、要素はクラスで探す。
- **画面の id（`index.html` と JavaScript が作る要素）は `user-content-` で始めない**（AR-053）。この接頭辞は文書から生まれる id のためのものである（IMP-117）。**`$(id)`（`util.js`）で画面の要素を探してよいのは、この規則があるからである**——文書から生まれる id（Mermaid のラベルの id を含む。IMP-231）は接頭辞で始まり、画面の id と同じ値にならない（[BUG-011](../bugs/2026-09-14-bug-011-document-id-collision.md)）。**CSS の id セレクタ（`#app` など）も、同じ id を持つすべての要素に効く**ため、この規則が要る。
- **`#btn-edit` と `#btn-editmode` は `disabled` を持った状態で読み込む。** 押せるかどうかは起動後の状態から決める（UI-021。`#btn-edit` は `welcome` では押せず、`#btn-editmode` は起動時に `unavailable`。DSP-320）。**確定するまで押せない側に倒す。**
- **`#btn-edit`・ショートカットの id `edit`（IMP-244）・`toolbar.js` の `canEdit()` は「エディタで開く」を指す**（v1.0.0 からの名前）。編集モードは `editmode` / `editMode` の名前で持つ。**id と関数名を改名しない**——改名すると `scripts/domids` の検査対象と、それを参照する記述も動く。紛らわしかったのは文言であり、文言は `tipOpenInEditor` へ改めた（IMP-290）。

> [!IMPORTANT]
> **`id="status"` を使ってはならない。** 同梱している `plantuml.js`（@plantuml/core）は、
> 描画のたびに **`document.getElementById('status')` を決め打ちで探し、見つけた要素の
> `textContent` を自分のログで上書きする。** `textContent` への代入は子要素をすべて破棄するため、
> **ステータス領域の 3 要素が DOM から消え、以降のすべての通知（FR-110）が出なくなる**
> （[調査報告](../bugs/2026-09-05-bug-006-status-id-collision.md)）。
>
> **`statusbar` に改名したのはこの 1 点のためである。** 短く自然な名前ほど同梱資産と衝突しやすい。
> **ここを `status` に戻さない。** 資産を更新したときの検査は [BR-043](06-build-release.md) が定める。

- ペインの表示・非表示は `hidden` 属性で切り替える。`style.display` を直接操作しない。
- **`index.html` に利用者向けの文言を書かない。** ペイン見出しの `Files` / `Outline` を含め、文言は `js/strings.js` から与える（IMP-290）。上の骨格でテキストが空の要素は、すべて実行時に埋める。**埋めるモジュールは、その要素の機能の IMP に書く**（ペインの見出しは IMP-240 の `initPanes`）。**担当の書かれていない空の要素は、誰も埋めない**——ペインの見出しは v1.0.0 から空のまま出ていた（[BUG-028](../bugs/2026-11-09-bug-028-pane-title-empty.md)。4.90.0）。
- テーマは `#app` の `data-theme` 属性で切り替える（DSP-011）。
- 本文は `.markdown-body` に挿入する。`github-markdown-css` が想定するクラス名に合わせる。
- **`#viewer` に `tabindex="-1"` を与える。目的は 3 つある。** 負値のため `Tab` の巡回順には入らない。
  1. 検索バーを閉じたときにフォーカスを本文へ戻す（UI-080）
  2. **文書を表示したときにフォーカスを本文ペインへ移す**（UI-051, IMP-220）。**これが無いとキーボードでスクロールできない**（[調査報告](../bugs/2026-09-05-bug-007-viewer-focus-on-open.md)）
  3. **編集欄（セル・コードブロック）を閉じた後、拡大画面を閉じた後に、フォーカスを本文ペインへ戻す**（IMP-262, IMP-264, IMP-253, IMP-244。いずれも IMP-220 の `focusViewer()` を通す）
- **`#searchbar` は `.searchbar-anchor` の中へ入れ、`#viewer` の先頭に置く**（DSP-160）。
  受け皿は**高さ 0 の `position: sticky`** とする。`#viewer` は `overflow-y: auto` の
  スクロールする器であり、**その中の絶対配置は内容と一緒に流れて画面外へ出る。**

  ```css
  .searchbar-anchor { position: sticky; top: 0; height: 0; z-index: var(--z-search); }
  ```

  - **`sticky` なので本文をスクロールしても留まる**（DSP-160）。
  - **高さ 0 なので本文を押し下げない**（UI-080）。
  - **受け皿は `#viewer` の内容領域に置かれる。** その右端はスクロールバーの内側であり、
    `.searchbar` の `right` がそのまま DSP-160 の「スクロールバーの内側」の意味になる。
    **補正が要らない。**
  - **`#viewer` の外へ出してはならない。** 外の器に置くと `right` の基準がスクロールバーの
    **外側**へ移り、その幅だけ内側が詰まる（実測 31px → 16px）。
    **その幅を CSS だけで得る手段は無い**（2026-09-03 に実測。JS で測って CSS 変数へ渡すことになる）。
  - **`#viewer` の先頭に置く。** 後ろに置くと、スクロールし切るまで現れない。
  - **受け皿に `z-index: var(--z-search)` を与える**（DSP-015 の 20）。
    **`position: sticky` は、`z-index` の指定に関わらず重ね合わせ文脈を作る。**
    与えないと受け皿は `z-index: auto` の層に落ち、**中の `.searchbar` の `z-index: 20` は
    受け皿の中でしか効かない。** 本文のコードブロック（`position: relative`）や
    コピーボタン（`z-index: 10`）が検索バーの手前に来て、**クリックが届かなくなる**
    （2026-09-03 に実機で発生。当たり判定を `elementFromPoint` で確認した）。
- **`#state-screen` は `#viewer` の中に残す。** `inset: 0` で本文領域を覆う必要があり、
  状態画面を出すときは本文を空にする（IMP-250）ためスクロールが起きない。**同じ絶対配置でも
  条件が違う。**

### IMP-203: アイコン **MUST**

UI-022 を実装する。

- SVG は `index.html` の先頭に `<svg style="display:none">` のシンボル定義としてまとめ、各ボタンは `<svg class="icon"><use href="#icon-open"></use></svg>` で参照する。寸法（16 × 16）と色は `.icon` に対して CSS から与える（DSP-014）。
- `fill="currentColor"` とし、色は CSS から与える（DSP-014）。
- **出典は `@primer/octicons` 19.33.0（MIT）。** 原本を `frontend/icons/` に、シンボル定義を `index.html` に写している。**BR-042 の管理対象には加えない。** あちらは取得したファイルを改変せずに格納することが前提で、リリース CI が自動更新する（BR-043）。ここは名前を変えて置いており、実際に描画に使うのは `index.html` の `<symbol>` であるため、自動更新すると記録と実物が静かにずれる。版は `scripts/genlicenses` が記録し、ライセンス一覧へ載せる（BR-040, FR-101）。
- アイコンの一覧と対応は以下とする。同じ絵柄を複数箇所で使う場合、シンボルは 1 つだけ定義して共用する。`icon-dir` と `icon-open` のようにシンボル ID を分けたまま同じ絵柄を使う場合は、`<symbol id="icon-dir"><use href="#icon-open"/></symbol>` として参照で共用し、パスデータを二重に持たない。

| シンボル ID | 使用箇所 | 出典（Octicons） |
| --- | --- | --- |
| `icon-back` / `icon-forward` | ツールバー「戻る」「進む」（UI-020, FR-051。4.75.0） | `arrow-left` / `arrow-right` |
| `icon-open` | ツールバー「開く」、welcome 画面（DSP-181） | `file-directory` |
| `icon-reload` | ツールバー「再読み込み」 | `sync` |
| `icon-moon` / `icon-sun` | ツールバー「テーマ切り替え」（状態で入れ替え） | `moon` / `sun` |
| `icon-outline` | ツールバー「アウトライン」 | `list-unordered` |
| `icon-filetree` | ツールバー「ファイルツリー」、ツリーの展開済みディレクトリ | `file-directory-open-fill` |
| `icon-external` | ツールバー「エディタで開く」（UI-020, FR-090。**4.75.0 で鉛筆から変えた**） | `link-external` |
| `icon-pencil` | コードブロックと図の「編集」ボタン（UI-053, FR-145。**4.75.0 でツールバーの「エディタで開く」から譲った**） | `pencil` |
| `icon-editmode` | ツールバー「編集モード」（UI-020, FR-140） | `checkbox` |
| `icon-strike` | ツールバー「取り消し線」（UI-020, FR-146。4.75.0） | `strikethrough` |
| `icon-color` | ツールバー「文字色」（UI-020, FR-147。4.75.0） | `paintbrush` |
| `icon-filter` | ファイルツリーの見出しのフィルターボタン（UI-033。4.75.0） | `filter` |
| `icon-actual-size` | 図と画像の「原寸表示」ボタン（UI-053, DSP-124）。**拡大画面の `1:1` は文字のボタンであり、使わない**（DSP-173） | `arrow-both` |
| `icon-expand` | 図と画像の「拡大画面」ボタン（UI-053, DSP-124） | `screen-full` |
| `icon-sort` / `icon-sort-asc` / `icon-sort-desc` | 表の並べ替えボタン（並べ替えなし / 昇順 / 降順。UI-054, DSP-125） | `unfold` / `sort-asc` / `sort-desc` |
| `icon-zoom-in` / `icon-zoom-out` | 拡大画面の `+` / `-`（UI-104, DSP-173） | `zoom-in` / `zoom-out` |
| `icon-about` | ツールバー「アプリケーション情報」 | `question` |
| `icon-dir` | ツリーの折りたたみ状態のディレクトリ（DSP-112） | `file-directory` |
| `icon-file` | ツリーの Markdown のファイル（DSP-112） | `file` |
| `icon-diagram` | ツリーの図のファイル（PlantUML・Mermaid。UI-030, DSP-112。4.75.0） | `workflow` |
| `icon-image` | ツリーの画像ファイル（UI-030, DSP-112。4.75.0） | `file-media` |
| `icon-chevron-right` / `icon-chevron-down` | ツリーの展開矢印（DSP-112）。**`icon-chevron-down` は検索バー「次へ」（DSP-160）でも使う**。**`icon-chevron-right` は `Text color` のサブメニューの矢印（DSP-141）にも使う**（4.76.0） | `chevron-right` / `chevron-down` |
| `icon-search` | 検索バーの先頭（DSP-160） | `search` |
| `icon-chevron-up` | 検索バー「前へ」（DSP-160） | `chevron-up` |
| `icon-close` | 検索バー「閉じる」、情報ダイアログ「×」、エディタ選択ダイアログ「×」、拡大画面の閉じるボタン、ファイルツリーのフィルターの枚の閉じるボタン（DSP-160, DSP-170, DSP-172, DSP-173, DSP-115） | `x` |
| `icon-copy` / `icon-check` | コードブロックのコピーボタン（FR-061, DSP-252）。**`icon-check` は右クリックメニューと文字色の一覧の印（UI-085, DSP-141）にも使う** | `copy` / `check` |
| `icon-note` | Alerts: NOTE（DSP-261） | `info` |
| `icon-tip` | Alerts: TIP | `light-bulb` |
| `icon-important` | Alerts: IMPORTANT | `report` |
| `icon-warning` | Alerts: WARNING、確認画面（DSP-181） | `alert` |
| `icon-caution` | Alerts: CAUTION、エラー画面（DSP-181） | `stop` |
| `icon-link` | 見出しのアンカー（MD-020, IMP-227, DSP-023） | `link` |

- **v1.2.0 で足す 8 つのシンボル（`arrow-left` / `arrow-right` / `strikethrough` / `paintbrush` / `workflow` / `file-media` / `filter` / `link-external`）は、2026-11-07 に unpkg の 19.33.0 で実在を確かめた**（実現性検討の付録 A.4）。取り込むときに原本を `frontend/icons/` に置き、改めて原本を見る。
- **v1.1.0 で足した 8 つのシンボルは、取り込む前に `@primer/octicons` 19.33.0 に上の名前で実在することを確かめる。** 無ければ同じ版の中から意味の近いものを選び、この表を直す（版を上げて取りに行かない。上の「BR-042 の管理対象には加えない」と同じ理由）。
- **同じ鉛筆の絵柄を、2 つの機能に使わない**（UI-020 の NOTE。4.75.0）。v1.1.0 まで `icon-pencil` は「エディタで開く」だったが、本文の中の「その場で直す」（コードブロックと図の編集）に譲った。
- **`icon-sort` は「並べ替えなし」を表す。昇順・降順と同じ絵柄を淡色にして流用しない。** 色だけで状態を区別することになる（DSP-125, UI-054 の「昇順と降順を区別できる」）。

> [!IMPORTANT]
> ここに挙げたシンボルは、いずれも**単色の SVG** であり、アプリケーションアイコン（UI-025, IMP-032）とは別物である。アプリケーションアイコンはラスタ形式の固有画像で、`/appicon.png`（IMP-160）から取得する。シンボル定義に混ぜない。

## 12.2 状態と初期化（IMP-210 系）

### IMP-210: フロントエンドの状態 **MUST**

```js
// js/state.js
export const state = {
  doc: null,          // DocumentDTO（13 章）。未表示なら null
  target: null,       // 状態画面が対象にしているファイル { path, displayPath, outsideTree, targetKind }（ErrorDTO。IMP-307）。文書の表示中と welcome では null（IMP-250, DSP-302）
  treeRoot: '',       // 絶対パス
  theme: 'light',        // 実際に適用している値。Go 側が解決済みで渡す（IMP-303）
  themeExplicit: false,  // 利用者が自分でテーマを切り替えたか（FR-071）
  zoom: 100,
  outlineVisible: true,
  fileTreeVisible: false,
  outlineWidth: 240,
  fileTreeWidth: 260,
  search: { open: false, query: '', hits: [], index: -1 },
  lazy: { mermaid: false, katex: false, plantuml: false }, // 読み込み済みか
  editable: false,    // 編集モードを開始できるか。DocumentDTO.editable の写し（IMP-302）
  editMode: false,    // 編集モードか。DocumentDTO.editMode / EditModeDTO.on の写し（IMP-316）
  editSeq: 0,         // 写した編集モードの状態の版。DocumentDTO.editSeq / EditModeDTO.seq（IMP-302）
  history: { canBack: false, canForward: false, seq: 0 }, // HistoryDTO の写し（IMP-323。4.75.0）
  treeShowDiagrams: true,  // ツリーのフィルター（FR-037。ConfigDTO。4.75.0）
  treeShowImages: false,
};
```

- 状態の**正**は Go 側（IMP-190）に置く。フロントエンドの `state` は描画のための写しであり、永続化に関わる値（テーマ・ペイン幅・表示状態）を変更したときは Go 側へ通知する（IMP-310）。
- **`state.zoom` は例外で、フロントエンドだけが持つ。** 倍率は保存しないため（UI-111, UI-115）Go 側に対応するフィールドがなく、`configPatch` にも含めない。
- 表示中の文書パスをフロントエンドで `localStorage` 等に保存しない（NFR-042）。
- **保存しない一時的な状態は `state` に置かない。** 幅不足によるアウトラインの一時的な非表示（IMP-246）は `panes.js` のモジュール変数とする。`state` は「Go 側の状態の写し」であり、そこに保存しない値を混ぜると、`configPatch` が何を送るべきかが読めなくなる。**表の並べ替え（IMP-229）・原寸表示（IMP-228）・拡大画面（IMP-253）の状態も、それぞれのモジュール変数とする。**
- **`editable` / `editMode` / `editSeq` は Go 側の状態の写しであり、`state` に置く**（IMP-109 が正）。`configPatch` には含めない（保存しない。UI-111）。
- **`target` も Go 側の状態（画面が対象にしているファイル。IMP-190 の `target`）の写しであり、`state` に置く**（IMP-250）。`configPatch` には含めない（NFR-042）。
- **`history` も Go 側の履歴の写しであり、`state` に置く**（IMP-323）。`configPatch` には含めない（NFR-042）。**`seq` が手元より小さい `HistoryDTO` は写さない。**
- **`treeShowDiagrams` / `treeShowImages` は保存する設定であり、`configPatch` に含める**（UI-110, FR-037）。**起動時に `init.config` の値を写す**（IMP-211。4.76.0）——写さないと、枚のチェックボックスとツリーの中身（Go 側は保存値で絞る）が食い違い、最初の `saveConfig` が既定値を送って保存値を上書きする。

`state` を変更したあとの通知は、次の 1 関数を必ず経由する。

```js
// js/state.js
export function configPatch()   // ConfigDTO（IMP-303）を組み立てる
export function saveConfig()    // 現在の状態を Go 側へ通知する
```

- **`api.updateConfig` を直接呼ばない。** バインドメソッドの呼び出しは Wails がメッセージごとに処理するため、立て続けに 2 つ投げると**到着順が入れ替わりうる**。実際に、ペインの開閉と別の設定変更を続けて行うと先に投げたほうが後に処理され、新しい値が古い値で上書きされた。`saveConfig` は前の応答を待ってから次を送ることで順序を保つ。
- 送信待ちが既にあるときは新たに積まない。`ConfigDTO` は差分ではなく状態の全体であり、待っている 1 つが送信時点の最新を読めば足りる。
- **`saveConfig()` は、呼んだ時点の状態を含む送信が終わったときに解決する `Promise` を返す**（4.75.0）。ツリーのフィルターを切り替えたら、これを待ってから `ReadDir` を呼ぶ（IMP-271）——Go 側の `ReadDir` は `UpdateConfig` で受け取った値で絞る（IMP-303）。
- **`configPatch` の `theme` は、利用者が自分で切り替えるまで空文字とする**（FR-071, IMP-303）。`state.theme` は Go 側が OS 設定まで解決した値であり、それをそのまま返すと「まだ選んでいない」状態が最初の保存で失われ、以後 OS 設定を変えても追従しなくなる。

### IMP-211: 起動順序 **MUST**

```js
// js/main.js
async function boot() {
  const init = await api.getInitialState(); // 13 章 InitialStateDTO
  applyTheme(init.config.theme);
  applyPanes(init.config);       // 倍率は復元しない。常に 100 %（UI-111, IMP-242）
  state.treeShowDiagrams = init.config.treeShowDiagrams; // ツリーのフィルター（FR-037。4.76.0）
  state.treeShowImages = init.config.treeShowImages;     // 写さないと、最初の saveConfig が既定値で上書きする
  state.treeRoot = init.treeRoot;
  initToolbar(); initTooltip(); initFileTree(); initOutline(); initViewer(); initPanes();
  initSearch(); initZoom(); initOverlay(); initDnd();
  initMedia(); initTableSort(); initExpand(); initContextMenu(); initEditMode();
  initBlockEdit(); initCellEdit(); initCodeEdit(); initFormat();      // 4.75.0（IMP-262, IMP-264, IMP-265）
  initTreeMenu(); initTreeFilter(init.extensions);                    // 4.75.0（IMP-270, IMP-271）
  applyHistory(init.history);                                         // toolbar.js。戻る・進むの淡色（IMP-323）
  initShortcuts();
  subscribe();                              // Go からのイベント購読（IMP-322）
  if (init.document) renderDocument(init.document);
  else {
    enterStateScreen(init.stateKind || 'welcome', init.error);       // 状態画面も同じ入口（IMP-250, IMP-303）
    if (init.error && !isStateScreenError(init.error)) showMessage(errorText(init.error), 'error'); // 開けなかった理由（FR-012, IMP-193）
  }
  if (init.treeRoot) await loadTreeRoot(init.treeRoot);
}
```

- テーマの適用を最優先で行い、既定色から切り替わるちらつきを防ぐ（UI-105）。
- **`initSearch` は最初の `renderDocument` より前に呼ぶ。** `renderDocument` は検索を閉じる処理を含む（IMP-220）ため、検索バーが未構築だと参照できない。
- `index.html` は `data-theme` を持たない状態で読み込まれるため、`<head>` 内のインラインスクリプトで暫定的に `prefers-color-scheme` を反映してもよい（**SHOULD**）。

## 12.3 本文の描画（IMP-220 系）

### IMP-220: 本文の挿入 **MUST**

```js
// js/viewer.js
export function renderDocument(doc) // doc: DocumentDTO
```

処理順序を固定する。**手順の番号は他の文書からも参照される**（「IMP-220 の手順 11」など）。**番号を振り直さない。** 後から足した手順は `5a` のように枝番で書く。枝番は Markdown の番号付きリストにならないため、箇条書きの先頭に太字で番号を置く。

- **0a.** **引き継ぐ状態を控える、または解除する**（DSP-352）。
  - **編集欄（セル・コードブロック）は、ここでは開いていない**（4.75.0）。開いている間は、文書を切り替える操作を拒み、再描画を控える（IMP-264）ため、`renderDocument` は編集欄を閉じた後にしか呼ばれない。v1.1.0 はここでセルの編集欄を取り消していた（FR-142 の改訂で、フォーカスが外れても入力を捨てなくなった）。**右クリックメニューもどちらの場合も閉じる**（IMP-249。本文を差し替えると、メニューが指していたリンクや選択範囲が無くなる。DSP-352）。**控えるより前に閉じる**——閉じるとフォーカスが開く前の要素（チェックボックスなど）へ戻り、その順番を控えられる。偽の場合は `leaveDocument()` も閉じるが、開いていなければ何もしない。
  - `doc.sameDocument` が**真**（同じ文書の再描画。1.7）なら、差し替える前の DOM から次を控える。**開いている `<details>` の順番**（`#markdown` の中の出現順）、**フォーカスのあるチェックボックスの順番**（**鍵の合うチェックボックス（`isOwnRef(input, "task")` が真のもの）を文書の順に並べたときの位置**。目印の値の番号 `<n>` を読まない——値を解くのは `refs.js` だけである（IMP-260）。Go 側は番号を文書の順に振る（IMP-120）ため、位置と番号は一致する。4.67.0。FR-014）、並べ替え（IMP-229 の `captureSort`）、原寸表示（IMP-228 の `captureMedia`）。拡大画面は閉じない（IMP-253）。
  - **偽**（文書の切り替え）なら何も控えず、`docswitch.js` の `leaveDocument()`（IMP-250）で後始末する。拡大画面を閉じ（IMP-253 の `onDocumentSwitched`）、並べ替えと原寸表示の状態を空にする。**状態画面へ移るときと同じ関数を通す。**
- **0b.** **`state.doc = doc` とし、`state.target` を `null` にする**（状態画面の対象を消す。IMP-250）。0a は差し替える前の DOM を**前の描画の鍵**で照合し、6b〜6d と 11 は新しい DOM を**新しい鍵**で照合する（`isOwnRef` は `state.doc.refKey` を読む。IMP-260）。**この位置から動かさない**——`F5` や更新検知では鍵が変わる（IMP-102）ため、前にずらすとチェックボックスの順番を控えられず、後ろにずらすと並べ替えと編集の印が付かない。
- **0.** 検索を閉じる（IMP-241）。包んだ `<mark>` を解いてから差し替える。ここを飛ばすと、検索状態が前の文書の `<mark>` を指したまま残る（FR-080 の「検索対象文書の切り替え・再描画時は検索状態をリセットする」）。
- **1.** `#markdown.innerHTML = doc.html` で**一度に**挿入する（AR-052）。分割挿入や逐次追加を行わない。
- **1a.** **文書の種類を本文の器に写す**（4.75.0）。`#markdown` に、画像ファイル（`doc.kind === 'image'`）なら `is-image-file` のクラスを付け、それ以外では外す（市松模様と中央への配置。DSP-129）。**図のファイルで `doc.noDiagrams` が真なら、本文の先頭に `<p class="no-diagrams">` を差し込み、`textContent` で `S.noDiagrams` を入れる**（FR-025, FR-026）。**判断は `doc.kind` と `doc.noDiagrams` で行い、HTML の `class` を見ない**（IMP-302）。
- **2.** `#state-screen` を隠す。
- **3.** コピーボタンを付与する（IMP-221）。
- **4.** GitHub Alerts のアイコンを付与する（IMP-225）。
- **5.** 見出しにアンカーを付与する（IMP-227）。
- **5a.** 画像に番号（`data-media-index`）を振る（IMP-228 の `numberImages`）。**6 より前に行う**——6 で読み込みに失敗した画像が置き換わると、順番が数えられなくなる（FR-120）。
- **6.** 画像の読み込み失敗を捉える配線を行う（IMP-226）。
- **6a.** 0a で控えた `<details>` を、同じ順番のものについて開き直す（FR-014）。**9 より前に行う**——開くと高さが変わり、先にスクロール位置を合わせると位置がずれる。
- **6b.** 画像に原寸表示・拡大画面のボタンを配線し、控えた原寸表示を当て直す（IMP-228）。**`restoreMedia` を `attachImageButtons` より前に呼ぶ**——読み込みの済んだ画像は `attachImageButtons` の中で包まれ、その時点で当て直すためである。
- **6c.** GFM の表に並べ替えのボタンを付け、控えた並べ替えを当て直す（IMP-229。`doc.trigger` を渡す）。
- **6d.** 編集モードの状態を画面へ写す（IMP-260 の `applyEditMode(doc)`）。コードブロックと図の編集のボタン（IMP-264）と修飾の操作（IMP-265）もここで付け直す。**修飾で残すと決めた選択（IMP-265）は、ここで当て直す。**
- **7.** スクロール連動の監視対象を作り直す（IMP-222）。
- **8.** `doc.needsMermaid` / `doc.needsKaTeX` / `doc.needsPlantUML` に応じて遅延ロードを起動する（IMP-230。図は `lazy.js` の `drawDiagrams`）。**図の描画が 1 つ終わるたびに（成功・失敗のどちらでも）、`media.js` の `onDiagramSettled` を呼ぶ**（IMP-228）。図のボタンの配線、原寸表示の当て直し、拡大画面の差し替え（IMP-253）はそこで行う。**描画の世代を 1 つ進める**——前の描画の図の知らせは、ここから先は届かない（IMP-230）。
- **9.** スクロール位置を設定する（13 章 `ScrollDTO` の `mode` に従う）。
- **10.** アウトライン（IMP-224）とステータス（DSP-150）を更新する。**画像ファイルでは、文字コードと行数の代わりに大きさ（`doc.fileSize`）を出し、寸法は画像の読み込みを待って足す**（IMP-228。UI-060）。
- **11.** **本文ペインへフォーカスを移す**（UI-051。`focusViewer()`）。**ただし 0a でチェックボックスの順番を控えていて、同じ順番のチェックボックスがあれば、そこへ移す**（FR-014, UI-051 の例外）。

3〜6d は DOM 走査を伴うため、`#markdown` を 1 回だけ走査してまとめて処理してよい（NFR-011）。**ただし、ここで順序を定めたもの（5a は 6 より前、6a は 9 より前）は保つ。**

**6 は 5a より後であれば順序を問わない**が、遅らせすぎてはならない。挿入から配線までの間に読み込みが終わった画像は `error` を受け取れないため、IMP-226 は配線時にすでに失敗しているものを別途拾う。

**手順 11 は 2 つの条件を満たす。** どちらを落としても別の要求が壊れる。

| # | 条件 | 落とすと |
| --- | --- | --- |
| 1 | **`focus({ preventScroll: true })` とする** | フォーカス移動に伴うスクロールが、直前の手順 9（`ScrollDTO`。DSP-350）を打ち消しうる。`F5` で位置が維持されず（FR-015）、`Alt+←` で復元されない（FR-051） |
| 2 | **ダイアログ（情報・エディタ選択）と拡大画面を表示している間は奪わない** | ダイアログと拡大画面は開いたままファイル更新の自動検知（FR-014）を受けうる。**背後の本文へフォーカスが移り、フォーカストラップ（IMP-251, IMP-252, IMP-253）が破れる** |

```js
// js/viewer.js
export function focusViewer()   // #viewer へ focus({ preventScroll: true })。ダイアログと拡大画面の表示中は何もしない
```

- **本文ペインへフォーカスを移す箇所は、すべてこの関数を通す**（手順 11、IMP-244 の `Esc` による編集の取り消し、IMP-253 の拡大画面を閉じたとき、IMP-260 / IMP-262 / IMP-264 の編集欄を閉じたとき）。上の 2 つの条件を 1 か所で満たすためである。
- **他のモジュールへは `main.js` が `deps.focusViewer` として渡す。** `editmode.js` / `expand.js` / `blockedit.js` / `celledit.js` / `codeedit.js` が `viewer.js` を import すると循環しうる（`viewer.js` は描画の後処理のために多くのモジュールを import する。IMP-201）。
- 拡大画面の判定（`isExpandOpen()`）があるため、**拡大画面を閉じる処理は、閉じた状態にしてからこの関数を呼ぶ**（IMP-253）。

> [!NOTE]
> **検索バーへの配慮は要らない。** 手順 0 が `closeSearch()` を呼んでおり（FR-080 の
> 「切り替え・再描画時は検索状態をリセットする」）、手順 11 に達した時点で検索は閉じている。
> **`closeSearch` は入力欄にフォーカスがあれば自分で本文へ戻す**（IMP-241）ため、
> ここで奪う相手がそもそも居ない。
>
> **状態画面についても要らない。** 状態画面は本関数ではなく `showStateScreen`（IMP-250）が出す。
> 逆に「大きなファイルの確認」から `Open anyway` で本文が出た場合は、**フォーカスを本文へ移すのが正しい。**

> [!IMPORTANT]
> **どの経路で開いてもフォーカスを移す**（ダイアログ・ドロップ・引数・ツリー・リンク・履歴）。
> 経路ごとに分けない。ツリーから開いた場合だけ移さないという案もあるが、**契機ごとの分岐は
> DSP-350 と同じ形の表をもう 1 つ増やす**。**ツリーのキーボード操作（UI-031）でも移すと決めた**（IMP-248 の IMPORTANT）。
> **例外は同じ文書の再描画でチェックボックスにフォーカスがあった場合だけである**（手順 11, FR-014）。

> [!NOTE]
> **`#viewer` は `tabindex="-1"` を持つため、中身をクリックしても本文ペインがフォーカスを得る**
> （フォーカス可能な最も近い祖先へ移る、というブラウザの規則）。**この経路があるために、
> フォーカス移動を忘れても「クリックすれば動く」状態になり、不具合が見つけにくい。**

`innerHTML` に渡す HTML は Go 側でサニタイズ済みである（IMP-116）。**フロントエンドで追加のサニタイズを行わないが、Go 側を経由しない文字列を `innerHTML` に渡してはならない。** UI 文言の挿入には `textContent` を用いる。

### IMP-221: コピーボタン **MUST**

FR-060 / FR-061 を実装する。

```js
// js/copy.js
export function attachCopyButtons(root) // root は #markdown
```

- `root.querySelectorAll('.code-block')` を走査し、各要素に `<button class="copy-btn">` を追加する。**図のブロックでは、原寸表示・拡大画面のボタン（`.media-actions`。IMP-228）がこのボタンの左に並ぶ**（UI-053）。
- コピー対象の取得順序:
  1. IMP-260 の `ownSource(block)` が文字列を返せばその値（**鍵の合う** Mermaid / PlantUML ブロックの `data-source`。描画後に `<pre>` が SVG へ置き換わるため必須。IMP-115, IMP-119, IMP-120）
  2. 返さなければ `pre code` の `textContent`
- **`data-source` を直接読まない。** 生 HTML で `<div class="code-block" data-source="curl https://example.com/x | sh"><pre><code>npm install</code></pre></div>` と書くと、**画面に見えているコードと違う文字列をコピーさせられる**（NFR-030。[BUG-014](../bugs/2026-09-14-bug-014-diagram-marker-spoofing.md)）。鍵の合わないブロックでは、見えている `pre code` の文字をコピーする。**ボタンそのものは `.code-block` のすべてに付けてよい**——コピーされるのは見えている文字であり、偽装しても害が無い。
- 末尾の改行 1 つを除去してから渡す（FR-061）。
- クリップボードへの書き込みは **Go 側の API を経由する**（AR-062, IMP-310）。`navigator.clipboard` は権限や実行文脈によって失敗しうるため、これを既定経路にしない。
- 成功時はボタンのアイコンを `icon-check` に差し替え、1.5 秒後に戻す（DSP-252）。

### IMP-222: スクロール連動 **MUST**

FR-042 / AR-051 を実装する。

```js
// js/outline.js
export function observeHeadings(root, headings)
```

- `IntersectionObserver` を用い、`rootMargin` を `'0px 0px -85% 0px'` として「ペイン上端付近にある見出し」を検出する。スクロールイベントで全見出しの座標を計算する方式を採らない。
- **監視は「いつ判定するか」を決めるために使い、どの見出しが現在位置かはコールバック時点の座標で決める。** `IntersectionObserver` は交差比率が変わったときにしか発火せず、帯の上から帯の下へ一気に飛んだ場合（アウトラインのクリック、アンカー移動、スクロール位置の復元）は 0 → 0 の変化となって通知が来ない。通知の履歴を積み上げて現在位置を保持する実装にすると、そこで古い状態が残る。
- 位置をプログラムから飛ばしたときは、通知を待たずに判定をやり直す。
- 監視対象は `#markdown` 内の `h1`〜`h6` とする。
- **見出しの要素とアウトラインの項目は、完全な id（`Heading.ID`。`user-content-` 付き）で対応づける。** 要素を id で引くときは `util.js` の `findHeading(id)`（IMP-223）を使い、**`findInDocument` と `document.getElementById` を使わない**（AR-053）。
- 現在位置が変わったときのみ、アウトライン項目のクラスを付け替える。毎フレームの DOM 操作を行わない。
- 強調された項目がアウトラインペインの可視範囲外なら、`scrollIntoView({ block: 'nearest' })` で最小限のスクロールを行う。

### IMP-223: リンククリックの捕捉 **MUST**

FR-050 / AR-060 を実装する。

```js
// js/viewer.js
document.getElementById('markdown').addEventListener('click', onLinkClick);
document.getElementById('expand-view').addEventListener('click', preventLinkDefault);     // 図の中のリンク（何もしない）
document.getElementById('markdown').addEventListener('auxclick', preventLinkDefault);     // 左ボタン以外（何もしない）
document.getElementById('expand-view').addEventListener('auxclick', preventLinkDefault);  // 同上

// js/util.js
export function linkHref(anchor)  // 書かれたリンク先。HTML の <a> は href、SVG の <a> は xlink:href。無ければ null
```

- `#markdown` に 1 つだけリスナを置き、イベント委譲で処理する。リンクごとにリスナを付けない。
- `event.target.closest('a')` で対象を求め、**リンク先が書かれていれば（`linkHref(anchor)`）常に `preventDefault()` を呼ぶ**。WebView 内でのページ遷移を一切発生させないため（AR-060）。
  - **`href` 属性だけを見ない。** SVG の `<a>` はリンク先を `xlink:href` に持つ（SVG 2 の `href` があればそちらを先に読む）。同梱の Mermaid は `click` で URL を指定したノードを `<a xlink:href>` で包むため、`href` だけを見ると既定の動作が残り、**WebView の中で遷移して戻れなくなった**（[BUG-015](../bugs/2026-09-17-bug-015-mermaid-click-link-navigation.md)）。
  - **図の中のリンク（MD-081）も同じ経路を通す**——`click` の URL のノードも、ラベルに書いた HTML の `<a href>` も、書かれたリンク先を Go へ渡す（FR-050）。
  - **拡大画面（`#expand-view`）にもリスナを置き、図の中のリンクの既定の動作だけを止めて何もしない**（UI-104）。図を本文の外へ移すため、`#markdown` のリスナは届かない（IMP-253）。**`expand.js` ではなく `viewer.js` に置く**——止める規則を 1 か所にまとめる。
- **左ボタン以外のクリックでは、リンクの既定の動作を止めるだけで何もしない**（AR-060, FR-050。[BUG-016](../bugs/2026-09-17-bug-016-link-middle-click.md)）。
  - **`auxclick` を `#markdown` と `#expand-view` の両方で受ける。** 中ボタンなど左ボタン以外のクリックは `click` ではなく `auxclick` で届き、**リンクをたどる処理はその既定の動作である**——Chromium（WebView2）も WebKit も、`click` と `auxclick` のうち右ボタン以外でリンクをたどる（WebKit は `MouseEvent::canTriggerActivationBehavior`）。`click` だけを受けていた v1.0.0 は、W1 では Go を通らずに既定のブラウザへ回り、L1 ではウィンドウの中が遷移して戻れなくなった。
  - **`click` でも `event.button` を見て、`0` でなければ `preventDefault()` の後に戻る。** **WebKit が `auxclick` を実装したのは 2024 年 7 月**（WebKitGTK 2.46）であり、それより前の版（Ubuntu 24.04 の初版の 2.44 など）は中ボタンでも `click` を出す。見ないと、そうした環境では中ボタンが左クリックと同じくリンクを開く。
  - リンクでない所の `auxclick` は止めない。キーボードで押したリンク（`Enter`）と `click()` は `button` が `0` であり、今までどおり開く。
- **次の場合は `preventDefault()` だけを行い、遷移の処理へ進まない。**
  - クリックが `.media-actions`（IMP-228）の中で起きた。画像をリンクで囲んだ記法でも、ボタンの操作ではリンク先へ移らない（UI-053）
  - **編集モードの間に、鍵の合う `data-ref` を持つセル（`th` / `td`）の中で起きた**（FR-050 の例外、FR-142）。ダブルクリックの 1 回目で文書が切り替わるのを防ぐ。判定は IMP-260 の `isEditableCell(element)` を使い、ここで鍵を読まない
  - **編集モードの間に、鍵の合う図のブロック（Mermaid・PlantUML）の中で起きた**（図の中のリンク。FR-050 の例外、FR-145。4.74.0）。判定は IMP-260 の `isEditableBlock(element)` を使う
- 同一文書内のアンカー（`#...`）のみフロントエンドで処理し、該当見出しへスクロールする。それ以外は `href` の生値を Go 側へ渡し、判断を委ねる（IMP-312）。**編集欄が開いている間は、Go へ渡すリンクを種類を問わず拒む**（IMP-264 の `refuseIfEditing()`。外部の URL と開けないファイルへのリンクも拒む——画面の側ではリンク先の種類を判定しない。同一文書内のアンカーは拒まない。FR-140。4.76.0。利用者の判断）。フロントエンドでスキームやパスの解釈を行わない。
- **移動先は `util.js` の `findInDocument(fragment)` で探す**（AR-053）。**`document.getElementById` をフラグメントにそのまま使わない**——本文に無い id（`#overlay` など）が画面の要素に当たる（[BUG-011](../bugs/2026-09-14-bug-011-document-id-collision.md)）。

  ```js
  // js/util.js
  export const DOC_ID_PREFIX = 'user-content-';
  export function findInDocument(fragment)  // 本文（#markdown）の中の要素。無ければ null
  ```

  | # | 手順 |
  | --- | --- |
  | 1 | フラグメントの先頭の `#` を除く。候補は、生のままの値と、百分率符号化を復号した値（復号できなければ生のままだけ）の順とする |
  | 2 | 各候補について、**まず `user-content-` を前に付けた値**で探し、見つからず候補が `user-content-` で始まっていれば、**候補そのもの**で探す |
  | 3 | **`#markdown` の中だけを探す**（`querySelector` に `CSS.escape` した値を渡す）。**見出しと重複した生 HTML の id は、文書の順で先の要素が見つかる**（AR-053）。どれでも見つからなければ `null` |

  - **手順 2 の順を逆にしない。** `## user-content-foo` の id は `user-content-user-content-foo` であり（IMP-117）、アンカーのアイコンの `href` は `#user-content-foo` になる（IMP-227）。候補そのものを先に探すと、別の見出しに当たるか見つからない。
  - **本文のリンクのクリックで見つからなければ、ステータス領域に `Link target not found: #<フラグメント>` を出す**（FR-050, IMP-315 の `link-not-found`。v1.0.0 から同じ）。**開いた直後のアンカーの復元（IMP-302 の `anchor`）で見つからない場合は通知せず、先頭を表示する**（v1.0.0 と同じ。リンク先の文書そのものは開けている）。
  - **リンクのフラグメントから本文の要素を探す箇所（本文のリンクと、開いた直後のアンカーの復元）は、すべてこの関数を通す。** 経路ごとに探し方を書かない。
  - **完全な id が分かっている見出し（アウトライン IMP-224、スクロール連動 IMP-222）は、`findHeading(id)` で `h1`〜`h6` の中だけを探す。** `findInDocument` に完全な id を渡すと、手順 2 が `user-content-` をもう 1 つ付けて先に探すため、`## foo` の項目が `## user-content-foo` の見出しへ移る。**生 HTML の `<div id="test">` が `## Test` の移動先を奪うこともない**（AR-053 の「重複したときは文書の順で先の要素」はリンクのフラグメントにだけ効く）。

  ```js
  // js/util.js
  export function findHeading(id)  // #markdown の中の h1〜h6 で id が一致する最初の要素。無ければ null
  ```

- **移動先が折りたたみ（`<details>`。[MD-026](04-markdown.md)）の中にある場合は、祖先の `<details>` を開いてからスクロールする**（[FR-050](02-functional.md)）。閉じたままの要素は `scrollIntoView` の対象にならず、**大きさを返すのにスクロールは起きない**。**IMP-241 と同じ関数を使う**（`util.js` に置く）。**文書を開いた直後のアンカー復元（IMP-302 の `anchor` モード）も同じ経路を通す。**
- `target="_blank"` を含むリンクも同じ経路で処理する。
- **Go 側の結果（`LinkResultDTO`。IMP-305）で状態画面を出すのは、`Kind` が状態画面の種別（`needs-confirm` / `too-large` / `render-error`。リンク先の Markdown が大きい・変換に失敗した）のときだけとする。** OS への委譲に失敗した `open-failed`（許可されていないスキーム・既定のアプリケーションが無い。IMP-315）は、ほかの種別と同じくステータス領域に出し、**本文を残す**（FR-050, FR-110）。**v1.0.0 は委譲の失敗が `render-error` に写され、状態画面が出て本文が消えていた**（[BUG-013](../bugs/2026-09-14-bug-013-link-open-failure-state-screen.md)）。

### IMP-224: アウトラインの構築 **MUST**

```js
// js/outline.js
export function renderOutline(headings)
```

- `DocumentDTO.headings`（Go 側が生成、IMP-117）をそのまま用いる。フロントエンドで DOM から見出しを抽出しない。抽出規則を 2 箇所に持たないため。
- 項目のクリックで移る先は、`Heading.ID`（`user-content-` 付き）を `util.js` の `findHeading(id)`（IMP-223）に渡して探す。**`findInDocument` と `document.getElementById` を使わない**（AR-053。理由は IMP-223）。
- 見出しが 0 件の場合、`strings.noHeadings` を表示する（FR-040）。
- インデントは**相対的な深さ**に応じた CSS カスタムプロパティで与える（DSP-113）。深さはレベルそのものではなく、`#` の次が `###` でも 1 段だけ下げる（FR-040 の「出現順を保ったまま相対的な深さで表示する」）。文字サイズはレベルで決める（DSP-113）ため、両者を別の属性で持つ。
- **項目のクリックで移動する際、見出しが折りたたみ（`<details>`。[MD-026](04-markdown.md)）の中にあれば、祖先の `<details>` を開いてからスクロールする**（[FR-041](02-functional.md)）。**アウトラインは本文の見出しをすべて挙げるため、折りたたみの中の見出しも項目として並ぶ。** 閉じたままの要素は `scrollIntoView` の対象にならず、**大きさを返すのにスクロールは起きない**。**IMP-241 / IMP-223 と同じ関数を使う**（`util.js` に置く）。

### IMP-225: GitHub Alerts のアイコン付与 **MUST**

MD-040 / DSP-260 を実装する。

```js
// js/decorate.js
export function decorateAlerts(root)
```

- `root.querySelectorAll('.markdown-alert')` を走査し、クラス名（`markdown-alert-warning` 等）から種別を判定する。
- 各 `.markdown-alert-title` の先頭に `<svg><use href="#icon-warning"></use></svg>` を挿入する。シンボル ID は IMP-203 の一覧に従う。
- **この処理をフロントエンドで行うのは、Go 側が出力したインライン SVG がサニタイズで除去されるためである**（IMP-112）。Go 側は種別をクラス名で伝え、フロントエンドが見た目を組み立てる。
- 種別が既知の 5 つに一致しない場合は何も挿入しない。Go 側が未知の種別を出力することはないが、防御的に扱う。

### IMP-226: 画像の読み込み失敗 **MUST**

FR-022 / DSP-123 を実装する。

```js
// js/decorate.js
export function markBrokenImages(root)   // viewer.js が再 export する（描画スモーク。IMP-200）
```

- `root.querySelectorAll('img')` を走査し、各要素に `error` を配線する。`error` で、**その `<img>` を `<span class="img-broken">` へ置き換える**（DSP-123）。
- **置き換えたら、`img` の `data-media-index`（IMP-220 の手順 5a で IMP-228 の `numberImages` が振った番号）を `span` へ写し、`media.js` の `onImageBroken(span)` を呼ぶ**（IMP-228）。拡大画面が開いていた画像の読み込みに失敗したことを、拡大画面へ知らせるためである（IMP-253）。
- **配線した時点ですでに失敗しているものを別途拾う。** `img.complete` で `img.decode()` が拒否されたら、その場で置き換える（4.75.0。v1.1.0 は `naturalWidth === 0` で判定していた）。**`naturalWidth` を失敗の判定に使わない**——固有の大きさを持たない SVG は、読み込みに成功しても `naturalWidth` が 0 になりうる（画像ファイルの表示と、URL の版で再描画のたびに同じ画像を使い回す場面で起きる。FR-027, IMP-118）。`naturalWidth` は寸法を出すかどうかの判断にだけ使う（IMP-228）。
- **本文から外れた画像（次の描画で差し替えられた前の本文の画像。`isConnected` が偽）では何もしない**（4.91.0）。`decode()` の拒否と `error` は描画の後に届く。外れた画像の番号を `onImageBroken` へ渡すと、**新しい本文の同じ番号の画像が対象から外れ、拡大画面が閉じる**（IMP-228 の図の `isConnected` と同じ守り。v1.1.0 からの潜在的な欠陥で、`decode()` が非同期なので起きやすくなる）。前の本文の画像は、外れても親（段落）を持ったままなので、親の有無では見分けられない。
- **代替テキストは自前で描く。** `span.textContent = img.alt` とする。**`alt` は文書由来の文字列であり、`innerHTML` に渡してはならない**（IMP-220）。
- **枠と色は CSS が与える**（DSP-123）。フロントエンドが用意するのは**要素とテキストだけ**であり、体裁を JavaScript で組み立てない。
- ローカル画像とリモート画像（MD-071）を区別しない。FR-022 はどちらも同じ扱いと定めている。
- **画像ファイル（FR-027）の画像も同じ経路を通る**（4.75.0）。代替テキストはファイル名（IMP-128）。原寸表示と拡大画面のボタンは付かず、ステータス領域の寸法も出さない（大きさだけ。IMP-228, DSP-150）。
- 文書全体の描画は止めない（FR-022, FR-111）。

> [!IMPORTANT]
> **ブラウザ既定の `alt` 表示に任せてはならない**（NFR-061, DSP-123）。
> **既定はエンジンごとに違い、WebKitGTK は `alt` を描かない**
> （[BUG-008](../bugs/2026-09-06-bug-008-broken-image-alt-webkitgtk.md)）。
> **`FR-022`（MUST）を満たすのはこちらの責任であり、エンジンに委ねられない。**

> [!IMPORTANT]
> **「配線時にすでに失敗しているもの」を拾う処理を省いてはならない。**
> `innerHTML` で挿入した直後に配線しても、キャッシュ済みの失敗や
> 同期的に解決される経路では `error` がすでに発火し終えている。
> これを落とすと、**手元では再現せず実機でだけ枠が出ない**という、
> 最も追いにくい形の不具合になる。
>
> CSS だけでは実装できない。読み込みに失敗した `img` を選ぶセレクタが
> **どちらのエンジンにも存在しない**ため、この 1 か所だけ JavaScript が要る。

> [!NOTE]
> **`load` で元に戻す経路は持たない。** 同じ `src` に対して `error` の後で `load` が発火することはなく、
> 復帰は文書の再描画（`F5`。FR-015）で起こる。**戻す経路を残すと、置き換えた `<span>` から
> `<img>` を復元する処理が要り、得るものが無い。**
>
> **`alt` の文字列が検索（FR-080）に当たるようになる。** 従来も画面には出ていた文字であり、
> **「見えているものが検索できる」という点ではむしろ整合する。**
>
> **`title` 属性は付けない。** すでに見えている文字と同じものを重ねるだけで、読み上げにも寄与しない。

### IMP-227: 見出しのアンカー **SHOULD**

MD-020 / MD-021 / DSP-023 を実装する。

```js
// js/decorate.js
export function decorateHeadings(root)
```

- `root.querySelectorAll('h1[id], h2[id], …, h6[id]')` を走査し、各見出しの**先頭の子**として次を挿入する。

```html
<a class="heading-anchor" href="#{id}" aria-label="…"><svg class="icon"><use href="#icon-link"></use></svg></a>
```

- `href` の値は、見出しの `id` から先頭の `user-content-` を除いたスラッグとする（`#<スラッグ>`。MD-021）。**GitHub のアンカーのアイコンと同じ形であり、コピーして他の Markdown へ貼っても動く。** 移動は IMP-223 の `findInDocument` が接頭辞を補って探す。スラッグそのものは Go 側が生成したものであり、組み替えない。
- `aria-label` は `strings.js` の `headingAnchor` を `setAttribute` で与える（IMP-290, IMP-295, UI-024）。
- **クリックの処理を書かない。** 本文中のリンクは IMP-223 が捕捉し、フラグメントは自前でスクロールに変える。ここで独自のハンドラを足すと経路が 2 つになる（AR-060）。
- `id` を持たない見出しには付けない。Go 側は必ず付与する（IMP-117）が、防御的に扱う。

> [!NOTE]
> **アウトラインと検索に影響しない。** アウトラインは `DocumentDTO.headings` を用いて DOM を読まない（IMP-224）。検索はテキストノードを走査するが、挿入するのは `<svg><use>` だけでテキストノードを持たないため、`textContent` も走査対象も変わらない。
>
> MD-020 は **SHOULD** であり、この機能自体は必須ではない。それでも実装するのは、MD-021 が GitHub 互換のスラッグを自前生成しているのに、**それを利用者へ見せる入口が他に無い**ためである。

### IMP-228: 図と画像のボタンと原寸表示 **MUST**

FR-120 / FR-121 / FR-027 / UI-053 / DSP-124 を実装する。

```js
// js/media.js
export function initMedia(deps)              // { onExpand, onTargetSettled, onImagesNumbered, onAllDiagramsSettled }。いずれも IMP-253 の同名（onExpand は openExpand）。main.js が配線する
export function numberImages(root)           // renderDocument の手順 5a。img に data-media-index を振り、deps.onImagesNumbered(数) を呼ぶ
export function onImageBroken(span)          // IMP-226 が img を span.img-broken へ置き換えた直後に呼ぶ
export function attachImageButtons(root)     // renderDocument の手順 6b
export function onDiagramSettled(block)      // 図の描画が 1 つ終わるたびに lazy.js が呼ぶ（手順 8）
export function onAllDiagramsSettled()       // その文書の図の知らせを出し終えたときに lazy.js が呼ぶ
export function captureMedia()               // 手順 0a。{ key → 原寸表示か } を返す
export function restoreMedia(snapshot)       // 手順 6b と onDiagramSettled で当て直す
export function clearMedia()                 // 状態を空にする。IMP-250 の leaveDocument が呼ぶ
export function naturalSize(target)          // 本来の大きさ { width, height }（下記）。IMP-253 も使う
```

**対象と、状態の鍵**（FR-120 の「同じ対象」）

| 種類 | 対象の要素 | 鍵 | 数え方 |
| --- | --- | --- | --- |
| Mermaid 図 | **鍵の合う**（IMP-260 の `isOwnRef(block, 'mermaid')`）`.code-block` の `.mermaid-rendered svg` | `mermaid:<n>` | **鍵の合う Mermaid のブロックを文書の順に数えた位置**（Go 側が `data-ref` に描画される順（DOM の順。脚注の中のものは末尾。BUG-027）で振る番号と一致する。**描画に失敗したブロックも数える**。IMP-120） |
| PlantUML 図 | **鍵の合う**（`isOwnRef(block, 'plantuml')`）`.code-block` の `.plantuml-rendered svg` | `plantuml:<n>` | **鍵の合う PlantUML のブロックを文書の順に数えた位置**（**描画しなかった `data-puml-error` のブロックも数える**。IMP-120） |
| 画像 | `#markdown img`（`span.img-broken` へ置き換わる前の順） | `image:<n>` | `innerHTML` の直後に `img` を走査した順。**読み込みに失敗したものも数える** |

- **番号は描画の成否を見る前に振る**（FR-120）。画像は `numberImages`（IMP-220 の手順 5a）が、手順 6（IMP-226）で置き換わる前に `data-media-index` 属性として振る。図は鍵の合う同じ種類のブロックを文書の順に数える。**目印の番号を読まない**——鍵を読むのは `refs.js` だけであり（IMP-260）、Go 側は図のブロックに文書の順で番号を振る（IMP-120）ため、両者は一致する（IMP-229 の表の番号、IMP-220 の手順 0a のチェックボックスと同じ求め方）。**1 つ壊れただけで後ろがすべて「別の対象」にならないようにする。**
- **鍵の合わない図のブロック（生 HTML で書いたもの）は対象にしない。** ボタンを付けず、拡大画面の対象にも数えない（NFR-030。[BUG-014](../bugs/2026-09-14-bug-014-diagram-marker-spoofing.md)）。そもそも描画されない（IMP-230）。
- **対象の準備が済んだら、成否とともに拡大画面へ知らせる**（`deps.onTargetSettled(key, target)`。IMP-253）。図は `onDiagramSettled` の中で、画像は包んだ時点（成功）と `onImageBroken(span)` が呼ばれた時点（失敗。`target` は `null`）で知らせる。
- **対象の数も拡大画面へ知らせる。** 画像は `numberImages` の中で `deps.onImagesNumbered(数)` を、図は `onAllDiagramsSettled` を受けたときに `deps.onAllDiagramsSettled()` を呼ぶ（同じ鍵の対象が文書に無いと分かる時点。IMP-253）。
- **`onDiagramSettled(block)` は、`block.isConnected` が偽なら何もしない**（前の描画の遅れた知らせ。IMP-230 の描画の世代と二重の防御）。
- **`media.js` は `lazy.js` / `puml.js` / `decorate.js` / `viewer.js` / `docswitch.js` / `main.js` を import しない**（いずれもこのモジュールを呼ぶ。IMP-250）。拡大画面へは `main.js` が渡す `deps` で知らせる。**描画スモーク（BR-054）は `initMedia` を呼ばない**——`lazy.js` から `onDiagramSettled` が呼ばれても、何もしない既定の `deps` で動く。
- **描き直し（テーマの切り替え）ではブロックが残るため、図のボタンを作り直さない**（フォーカスを失わない）。ボタンの器（`.media-actions`）に鍵を `data-media-key` で持ち、同じ鍵なら使い回す。

**ボタンの組み立て**

- 図は、`.code-block` の中に `<div class="media-actions">` を置き、`<button class="media-btn media-btn-actual" aria-pressed="false">` と `<button class="media-btn media-btn-expand">` を入れる。**コピーボタン（IMP-221）の左に並ぶ**（DSP-124）。
- 画像は、**読み込みに成功した時点で**次の形に包む。成功の判定は `load`、または配線時の `img.complete` で `img.decode()` が解決したときとする（IMP-226 と同じく、配線時に済んでいるものを拾う。4.75.0 で `naturalWidth > 0` から改めた）。**失敗した画像は包まない。**

  ```html
  <span class="media-image">                 <!-- 位置の基準。スクロールしない -->
    <span class="media-frame"><img …></span>  <!-- 原寸表示で横にスクロールする器 -->
    <span class="media-actions">…</span>      <!-- ボタン。器の外に置く -->
  </span>
  ```

- **固有の大きさを持たない画像（`width` / `height` の無い SVG）は、器（`.media-image`）に `is-fluid` を付けて本文の幅いっぱいに描く**（FR-022, DSP-124。4.91.0。[BUG-029](../bugs/2026-11-09-bug-029-svg-no-size-collapsed.md)）。`inline-block` の器は中身に合わせて縮み、大きさを持たない SVG はその中で幅 0 になる（包む前は本文の幅いっぱいに描かれる。v1.1.0 は見分けず、Markdown に埋め込んだ同じ SVG が見えなかった）。
  - **見分けるのは、`naturalWidth` が 0 か、包んだ後に器が描かれているのに画像の幅が 0 に縮んだもの**とする（包んだ直後に測る）。**`naturalWidth` だけでは見分けられない**——Chromium（WebView2）は 300 と返す（ヘッドレスの Edge 157 で測った）。
  - **器が描かれていない（`getClientRects()` が空。閉じた `<details>` の中をエンジンが配置しない場合）ときは決めずに残し**、判定のやり直しの契機（下の「縮小表示中の判定」）で決める。
  - **書き手が `width` / `height` を書いた画像（生 HTML。IMP-116 が数値だけ通す）は見分けず、その大きさで描く。**
- **画像が小さく、ボタンが画像に収まらないときは、ボタンの器に `is-outside` を付けて画像の右上の外側に出す**（DSP-124。4.81.0。画像ファイルの `.ico` など）。`mediabuttons.js` の `placeImageButtons` が、ボタンの表示を決めるたびに置き直す。
  - **収まるかは、ボタンを通常の位置に置いて測る**（ボタンの寸法と余白を JavaScript に書き写さない）。上端は画像の上端にそろえ、その位置を `--media-actions-top` に入れる（行の高さより小さい画像は、器の上端より下に描かれる）。
  - **外側に出す余地が無ければ（ボタンが画像を含む要素の右端を越える）内側のまま**とする（4.91.0）。本文の幅に縮めた、縦に短い横長の画像では本文の中でも収まらないが、外側に出すと本文ペインが横にはみ出す（UI-050）。親がインラインの要素（画像を囲むリンク）なら余地を測れないため、内側のまま。
- 図は `.mermaid-rendered` / `.plantuml-rendered` の要素そのものに `media-frame` のクラスを足す。ボタンはその外側の `.code-block` に置く（上記）。
- **`media-frame` は「原寸表示で横にスクロールする器」だけに付け、ボタンをその中に置かない。** スクロールする器の中の絶対配置は内容と一緒に流れる（IMP-202 の検索バーと同じ事情）。**ボタンは必ずスクロールしない親（図は `.code-block`、画像は `.media-image`）に置く**（DSP-124）。
- ボタンは `data-tip`（`S.tipActualSize` / `S.tipExpand`）と同じ文字列の `aria-label` を持つ（IMP-247, IMP-295）。**アイコンは `icon-actual-size` / `icon-expand`**（IMP-203）。
- **ボタンのクリックは `preventDefault()` と `stopPropagation()` を行う。** 画像がリンクに囲まれていても遷移させない（UI-053, IMP-223）。

**「縮小表示中」の判定**（FR-120）

```js
export function naturalSize(target)  // img は naturalWidth / naturalHeight（0 なら下記）。svg は下記
function isReduced(target)           // naturalSize(target).width > target.getBoundingClientRect().width + 0.5
```

- **SVG の本来の幅と高さ**は、`width` / `height` 属性が px の数値（単位の無い数値を含む）ならその値、そうでなければ `viewBox` の幅と高さとする。Mermaid は `width="100%"` と `style="max-width: <幅>px"` を出し、PlantUML は `width` を px で出す。**`getBBox` を使わない**——描画の内容の外接矩形であり、図の大きさではない。
- **img の `naturalWidth` が 0（固有の大きさを持たない SVG）なら、本文で描いている大きさ（`getBoundingClientRect`）を本来の大きさとみなす**（4.76.0）。0 のまま使うと、画像ファイル（拡大画面のボタンを常に出す。FR-027）を拡大画面で開いたとき、中身の大きさが 0 になる（DSP-173）。
- 判定は次の契機でやり直す（`requestAnimationFrame` で 1 フレームにまとめる。NFR-012）。**決めずに残した大きさを持たない画像の見分けと、小さい画像のボタンの置き場所も、同じ契機でやり直す**（4.91.0）。
  - `#markdown` の幅の変化（`ResizeObserver`。ウィンドウとペインの幅。FR-120）
  - 画像の `load`、図の描画の完了（`onDiagramSettled`）
  - `<details>` の `toggle`（閉じた中の要素は幅を持たないことがある）
- **拡大画面へ移している対象（`.media-frame` の中に無い対象）は判定し直さない**（IMP-253）。拡大画面での大きさで決めると、閉じたときにフォーカスを戻すボタンが隠れうる。
- ボタンの表示は `hidden` 属性で切り替える（IMP-202）。**原寸表示のボタン**は「縮小表示中 **または** 原寸表示中」に出し、**拡大画面のボタン**は図なら常に、画像なら原寸表示のボタンと同じ条件で出す（FR-120 の表）。**画像ファイル（`state.doc.kind === 'image'`）の画像は、読み込みに成功していれば拡大画面のボタンを常に出す**（FR-027。4.75.0）。**判断は `state.doc.kind` で行い、`.image-file` の有無で行わない**（IMP-128）。
- **画像ファイルの画像が読み込まれたら、`status.js` に本来の寸法（`naturalWidth` / `naturalHeight`）を渡し、ステータス領域の右端に出す**（UI-060, DSP-150。4.75.0。`status.js` の `setImageSize(width, height)`。**寸法はどの文書のものかと一緒に持ち**、文書を差し替えると残らない）。**固有の大きさを持たない SVG**（上の見分けで `is-fluid` にしたもの。**`naturalWidth` が 0 かどうかでは決めない**——Chromium は 300 と返し、W1 で `300 × 150 px` が出る。4.91.0。BUG-029）と、読み込みに失敗した画像（IMP-226）では寸法を出さない（大きさだけ）。
- **編集モードの間は、コードブロックと図に編集のボタン（`media-btn-edit`）も置く**（UI-053。IMP-264）。図では `.media-actions` の先頭（左端）に入れ、コードブロックでは `.media-actions` を作って編集のボタンだけを入れる。**付けるのも外すのも IMP-264 が行う**（このモジュールは場所を用意するだけにする）。**図の器は、IMP-264 が先に作っていればそれを使い**（同じ `data-media-key`。描画を待つ間と描けなかった図にも編集のボタンを置くため）、原寸表示・拡大画面のボタンを編集のボタンの右に足す（4.76.0）。

**原寸表示**（FR-121）

- `media-frame` に `is-actual` のクラスを付け、CSS カスタムプロパティ `--media-natural-width` に本来の幅（px）を入れる。**幅と横スクロールは CSS が与える**（DSP-124）。`style.width` を直接書かない。
- **Mermaid の SVG はインラインの `max-width` を持つ。** 原寸表示では CSS 側で `max-width: none` を効かせる必要があるため、原寸表示に入るときにインラインの `max-width` を控えて外し、縮小表示へ戻るときに戻す。**控えずに消すと、縮小表示へ戻したときに図が本文幅を超える。**
- ボタンの `aria-pressed` を状態に合わせる。
- **切り替えの前後で、押したボタンの画面上の位置を保つ**（DSP-350）。切り替えは要素の高さを変え、後ろの本文を押し下げる。切り替える前にボタンの `getBoundingClientRect().top` を控え、切り替えた後の差を `#viewer.scrollTop` に足す。
- **倍率（IMP-242）には触れない。** 本来の幅は論理ピクセルであり、`--zoom` の影響を受けない（DSP-021, FR-121）。

**状態の引き継ぎ**（FR-121, DSP-352）

- 状態は `Map<鍵, true>`（原寸表示中の鍵の集合）としてモジュール変数に持つ（IMP-210）。
- `captureMedia` はその集合と**当て直しを待つ鍵の集合（下記）を合わせた**写しを返す。図の描画を待つ間に次の再描画が来た（外部エディタで保存を繰り返した。UC-03）とき、まだ当て直していない原寸表示を落とさないためである。`restoreMedia(snapshot)` は、写しを**当て直しを待つ鍵の集合**としてモジュール変数に置き、原寸表示中の集合を空にする。以後、対象が準備できた時点（画像は包んだ時点、図は `onDiagramSettled`）で、鍵が待つ集合にあれば原寸表示にして、待つ集合から原寸表示中の集合へ移す。**`onDiagramSettled(block)` は控えを引数に取らない**ため、控えはこのモジュールの中に持つ。
- **待つ集合に残った鍵は捨てる。** 図の鍵は `onAllDiagramsSettled` を受けた時点で、画像の鍵は `numberImages` の数以上の番号のものを、読み込みに失敗した画像の鍵は `onImageBroken` の時点で捨てる。**手順 5a（`numberImages`）と 6（`onImageBroken`）は 6b（`restoreMedia`）より前に来る**ため、`restoreMedia` は置いた写しから、その時点の画像の数以上の番号と、既に読み込みに失敗した番号の鍵を捨てる。**残したままにすると、次の再描画で数が戻ったときに、利用者が戻していない原寸表示が当たる。**
- **文書の切り替え（`doc.sameDocument` が偽）と状態画面への移行では集合を空にする**（IMP-250 の `leaveDocument` が `clearMedia()` を呼ぶ。IMP-220 の手順 0a からも `leaveDocument` を通る）。
- **テーマの切り替え（IMP-243）で図が描き直されたときも `onDiagramSettled` を通り、原寸表示を当て直す**（FR-121, DSP-370）。

### IMP-229: 表の表示上の並べ替え **SHOULD**

> [!NOTE]
> **編集欄が開いている間は、並べ替えのボタンを押しても並べ替えない**（FR-140 の表の「拒む」。4.74.0）。押したら IMP-264 の `refuseIfEditing()` を呼び、警告を出して戻る。並べ替えは行を動かし、開いているセルの編集欄の位置が変わるためである。

FR-130 / UI-054 / DSP-125 を実装する。

```js
// js/tablesort.js
export function initTableSort(deps)             // { closeSearch }
export function attachSortButtons(root)        // renderDocument の手順 6c
export function captureSort()                   // 手順 0a
export function restoreSort(snapshot, trigger)  // 手順 6c。trigger は DocumentDTO.trigger
export function clearSort()                     // 状態を空にする。IMP-250 の leaveDocument が呼ぶ
export function compareCells(a, b)              // 比較。純粋な関数（下記）
export function parseNumber(text)               // 数値として解釈できれば数、できなければ null
```

**対象**

- **`table[data-ref]` のうち、IMP-260 の `isOwnRef(table, 'table')` が真のもの**だけとする（IMP-120）。**鍵をこのモジュールで読まない。****生 HTML の `<table>` は目印を持たないか、鍵が合わない**（FR-130）。
- `tbody` の行が 2 行以上ある表に限る。
- 各 `thead th` の末尾に `<button class="sort-btn" type="button">` を置く。`data-tip` と `aria-label` は `S.tipSort`（IMP-247, IMP-295）。**アイコンは状態に応じて `icon-sort` / `icon-sort-asc` / `icon-sort-desc`**（IMP-203）。
- 並べ替えている列の `th` に `aria-sort="ascending"` / `"descending"` を付け、それ以外からは外す（IMP-295）。
- **行の元の順は、`attachSortButtons` が並べ替える前の各 `tr` に振る番号（`data-sort-row`。0 起点）で持つ。** 描画した直後の DOM の並びはソースの順である。DOM の並びは並べ替えで変わるため、元の順を DOM から読み直さない。**先頭のセルの `data-ref` に頼らない**——縦棒だけの行はセルがすべて補われ、`data-ref` を持たない（IMP-120）。

**操作**（FR-130）

- ボタンを押すと、その列で **昇順 → 降順 → 元の順** を巡回する。別の列のボタンでは、その列の昇順から始める。
- 並べ替えは `tbody` の `tr` を並べ直すだけで行う（`appendChild` で付け替える）。**セルの中身を作り直さない**（ボタン・`<mark>`・図のボタンが消える）。
- **並べ替える前に `deps.closeSearch()` を呼ぶ**（FR-080。ヒットの順序が本文と食い違う）。

**比較**（FR-130）

- 比べる文字列はセルの `textContent` を前後の空白を除いたものとする。**自前で足した要素（`.sort-btn` / `.media-actions`）はテキストを持たない**ため、そのまま `textContent` を使ってよい。
- `parseNumber(text)`: 前後の空白を除き、`,` をすべて除き、末尾の `%` を 1 つ除いた結果が `^[+-]?(\d+(\.\d*)?|\.\d+)$` に一致すれば `Number(...)`、しなければ `null`。
- `compareCells(a, b)` は次の順で決める。**昇順の比較だけを定義し、降順はその符号を反転する。ただし空のセルは反転の対象にしない。**
  1. どちらかが空文字なら、空のほうを後ろにする（**昇順・降順とも**。FR-130）
  2. 両方が数なら数で比べる
  3. 片方だけが数なら、数を先にする
  4. どちらも数でなければ、`Intl.Collator(undefined, { numeric: true, sensitivity: 'base' })` で比べる（大文字小文字を区別せず、数字の並びを数値の大小で比べる。`item2` < `item10`）
- **向きは呼び出し側が扱う。** `compareCells(a, b)` は昇順の値を返す。降順では、**どちらのセルも空でないときだけ**符号を反転し、どちらかが空なら `compareCells` の値をそのまま使う（空のセルを末尾に置く。FR-130 の「逆順の例外」）。**引数に向きを足さない**——描画スモーク（UT-814）は昇順の値を見る。
- **安定な並べ替えにする**（`Array.prototype.sort` は安定であることが保証されている）。比較が等しい行は元の順を保つ。

**状態の引き継ぎ**（FR-130, DSP-352）

- 状態は表の番号（`table:<t>` の `t`）ごとに `{ col, dir, cols, order }` をモジュール変数に持つ。`cols` は列数、`order` は現在の行の並び（元の行番号の配列）。
- **表の番号は目印の番号を読まずに求める。** 鍵の合う表（`isOwnRef(table, 'table')`）を文書の順に並べたときの位置（0 起点）とする。Go 側は GFM の表に文書の順で番号を振る（IMP-120）ため、両者は一致する。**鍵を読むのは `refs.js` だけである**（IMP-260）。行が 2 行未満の表も番号には数える。
- `restoreSort(snapshot, trigger)` は、同じ番号の表があり、**列数が同じで、行が 2 行以上ある**ときだけ当て直す。そうでなければその表の状態を捨てる。

| `trigger` | 当て直し方 |
| --- | --- |
| `edit`（編集モードの書き込みの直後。IMP-195） | **`order` のとおりに行を並べる。並べ直さない。** `col` / `dir` とボタンの表示は保つ。**行の数が変わっていないことを確かめ、変わっていれば並べ直す** |
| それ以外（`watch` / `reload` / `open`） | `col` / `dir` で並べ直す |

- **文書の切り替え（`doc.sameDocument` が偽）と状態画面への移行では状態を空にする**（IMP-250 の `leaveDocument` が `clearSort()` を呼ぶ。IMP-220 の手順 0a からも `leaveDocument` を通る）。
- **`edit` で並べ直さないため、基準の列を直した行は並べ替えの順から外れうる**（FR-130）。次にボタンを押したときに巡回の次の段へ進み、その段の順で並べ直す。

## 12.4 遅延ロード（IMP-230 系）

### IMP-230: Mermaid・KaTeX・PlantUML **MUST**

AR-021 / MD-061 / MD-082 / MD-085 / NFR-013 を実装する。

```js
// js/lazy.js
export async function ensureMermaid()  // 未読込なら <script> を挿入して初期化
export async function ensureKaTeX()    // 未読込なら <script> と <link> を挿入
export async function ensurePlantUML() // 未読込なら viz-global.js → plantuml.js の順で読む
export async function drawDiagrams(root, needsMermaid, gen)  // IMP-220 の手順 8。Mermaid（needsMermaid のとき）と PlantUML を描き、両方を出し終えたら onAllDiagramsSettled
export async function redrawDiagrams(root, gen)              // IMP-243 のテーマの切り替え。知らせは drawDiagrams と同じ
```

- 読み込みは `frontend/vendor/` 配下への相対パスで行う。外部 URL を参照しない（AR-020）。
- 一度読み込んだら `state.lazy` に記録し、以降は再読み込みしない（AR-021）。
- **読み込み中の資産は、その読み込みを待つ。** `drawing.js` の `loadScript` / `loadStyle` は同じ `src` / `href` の約束を共有し、要素を 1 度だけ挿す。文書の描画とテーマの切り替えによる描き直し（IMP-231, IMP-233）が読み込みの途中で重なっても、**同じ資産を 2 度実行しない**（`viz-global.js` の Graphviz を含む。IMP-233）。**読み込めなかった（`error` になった）約束は忘れ、次の文書の描画（`F5` を含む）で読み直す**——`error` になった `<script>` は実行されていない（テーマの切り替えは、描けなかった図を描き直さない。IMP-231）。
- `doc.needsMermaid` / `doc.needsKaTeX` / `doc.needsPlantUML` が false の文書では**呼び出さない**。この条件分岐が NFR-013 の実体である。
- **`ensurePlantUML()` は読み込む順序を守る**。`viz-global.js` を先に、`plantuml.js` を後にする（AR-020, IMP-233）。**前者の読み込みに失敗したら、後者を読まないで false を返す。** Graphviz 不在のまま描こうとすると処理系ごと止まる（IMP-233 の 4）。
- 読み込みと描画は本文の表示をブロックしない。`renderDocument` の完了後に非同期で実行する（NFR-012）。
- **図の描画が 1 つ終わるたびに、成功・失敗・描画しなかったのいずれでも `media.js` の `onDiagramSettled(block)` を呼ぶ**（IMP-228）。テーマの切り替えによる描き直し（IMP-231, IMP-233）でも呼ぶ。**失敗のときにも呼ぶ**のは、拡大画面の差し替え（IMP-253）が「その図はもう来ない」ことを知って閉じる判断をするためである（FR-120 の「描画が終わってから判断する」）。
- **資産の読み込みに失敗して図を 1 つも描かない場合も、対象のブロックそれぞれについて呼ぶ。**
- **図を 1 つ描き終えたら、`search.js` の `syncHits(block)` も呼ぶ**（IMP-241。4.69.0）。PlantUML は**描き始めに原文を器へ置き換える**ため、そこでも呼ぶ（描き終えるまで 1 秒近くあり、その間も件数を画面に合わせる）。数式は `drawMath` が描き終えた要素ごとに呼ぶ（IMP-232）。
- **その文書の Mermaid と PlantUML の知らせをすべて出し終えたら、`media.js` の `onAllDiagramsSettled()` を 1 度だけ呼ぶ**（`drawDiagrams` / `redrawDiagrams` が両方の描画を待ってから、世代が今のときだけ呼ぶ。Mermaid と PlantUML は並行して描くため、片方の関数の中では「両方を出し終えた」が分からない）。 図を含まない文書でも、描画を起動しない代わりに呼ぶ（拡大画面が画像を開いていた場合の判断に要る。IMP-253）。
- **描画するのは鍵の合う図のブロックだけとする**（IMP-260 の `isOwnRef(block, 'mermaid')` / `isOwnRef(block, 'plantuml')`）。描く原文は `ownSource(block)` から取り、**`data-source` を直接読まない。** 生 HTML で書いた `<div class="code-block" data-plantuml data-source="…">` は、Go 側の取り込み指令の検査（IMP-119, MD-084）を通っていない。鍵を見ないと、**検査を通らない PlantUML が処理系へ渡り、PlantUML を含まない文書でも資産を読む**（NFR-013, NFR-030。[BUG-014](../bugs/2026-09-14-bug-014-diagram-marker-spoofing.md)）。**鍵の合わないブロックは HTML のまま残し、知らせも出さない。**
- **描画には世代の番号を付ける。** **世代は `drawing.js` が持ち、描画を始める側が `startDrawing()` を 1 回だけ呼んで番号を 1 増やす**——文書の描画（IMP-220 の手順 8。`viewer.js`）と、テーマの切り替えによる描き直し（`theme.js`）である。**返った番号を Mermaid と PlantUML の描く関数の両方に渡し、描く関数ごとに世代を進めない。** 手順 8 は両方を待たずに並べて起動するため、描く関数ごとに進めると、後から始めた側が先に始めた側の描画を止めてしまう（4.67.0。描く関数は、番号を省くと自分で世代を進める——描画スモークが単独で呼ぶため）。描く関数は、図を 1 つ描き終えるたびに、渡された番号が今の番号と同じかを確かめる。**違えば（その間に再描画か描き直しが始まった）、残りの図を描かず、`onDiagramSettled` も `onAllDiagramsSettled` も呼ばない。** PlantUML は 1 枚に数秒かかりうるため、外部エディタで保存を繰り返すと、前の描画の知らせが新しい描画の後に届く。区別しないと、**新しい描画の拡大画面を誤って閉じたり、DOM から外れた古い要素へ差し替えたりする**（FR-122, UC-03）。

> [!IMPORTANT]
> **同梱資産は、こちらが渡した要素の外にも書くことがある。** 描画対象の id を渡す形（IMP-233 の 2）は
> 「そこにしか書かない」ことを意味しない。**資産はページ全体を見ており、DOM の id はページと資産で
> 共有された名前空間である。**
>
> 実際に `plantuml.js` は `document.getElementById('status')` を決め打ちで書き換えており、
> **`index.html` の `<footer id="status">` を壊していた**（[調査報告](../bugs/2026-09-05-bug-006-status-id-collision.md)）。
> Mermaid にも同じ形の決め打ち（`cy`）がある。
>
> **自前の id は、同梱資産が決め打ちする id と重ならないようにする**（IMP-202）。
> 資産を更新したときの検査は [BR-043](06-build-release.md) が定める。
>
> **文書の id も同じ名前空間にある。** 見出し・脚注・生 HTML の `id` は `user-content-` で始まり（AR-053）、
> **資産の決め打ち（`status` / `cy`）とも画面の id とも重ならない。** 4.43.0 より前は見出しの id が
> スラッグそのものであり、**`# Status` という見出しが `plantuml.js` のログで書き換わっていた**
> （[BUG-011](../bugs/2026-09-14-bug-011-document-id-collision.md)）。

### IMP-231: Mermaid の初期化 **MUST**

```js
mermaid.initialize({
  startOnLoad: false,
  securityLevel: 'strict',   // MD-081
  theme: state.theme === 'dark' ? 'dark' : 'default',
  look: 'classic',           // MD-080, DSP-270。GitHub の描画に合わせる（BUG-021）
  dompurifyConfig: { SANITIZE_NAMED_PROPS: true },  // AR-053。ラベルの id / name に user-content- を付ける
});
```

- **`dompurifyConfig: { SANITIZE_NAMED_PROPS: true }` を落とさない**（AR-053）。**Mermaid の図のラベルには書き手が HTML を書け**（`A["<span id='tooltip'>x</span>"]`）、`securityLevel: 'strict'` でもその `id` は SVG の中に残る。残ると画面の要素（`#tooltip`）や同梱資産の決め打ち（`#status`）を乗っ取る（[BUG-011](../bugs/2026-09-14-bug-011-document-id-collision.md)）。この設定で DOMPurify が `id` / `name` を `user-content-` 付きへ書き換える（Mermaid 11.17.2 で実測）。**資産を更新したら、描画スモーク（BR-054）でこれが効いていることを確かめる。**
- **`look` を省かない**（MD-080, DSP-270, [BUG-021](../bugs/2026-09-28-bug-021-mermaid-neo-look.md)）。**Mermaid 12.0.0 から、図種別ごとの設定（`flowchart` / `sequence` / `class` / `state` / `er` ほか）の既定が `look: 'neo'` になり、全体の `look` を指定しないと `neo` で描かれる**（全体の既定は `classic` のまま。同梱の 12.0.0 で実測）。`neo` はノードに灰色（`rgba(185,185,185,1)`）の `drop-shadow` を付け、Dark では枠をグラデーションにする。**Dark の背景では光のにじみに見える。** **値は GitHub の描画に合わせる**（MD-002）。4.71.0 の時点は `classic` である。**GitHub が `neo` に切り替えたら追従し、描画スモークの期待値（`scripts/smoke` の `mermaidLook`。UT-816）と DSP-270 を一緒に変える。** PlantUML の `maxSvgSize`（IMP-233）と同じく、**同梱資産の更新だけで見た目が変わらないようにする**ための明示である。

- `startOnLoad: false` とし、描画対象を明示的に指定する。
- 描画対象は `.code-block[data-mermaid] pre.mermaid-source` のうち、**ブロックが鍵の合う目印を持つもの**（IMP-230）。`mermaid.render` に渡す id は `mermaid-svg-<n>` とする（IMP-233 の 2 と同じ連番。`user-content-` で始めない。AR-053）。
- 描画に失敗したブロックは、元のソースをコードブロックとして残し、エラー内容を併記する（FR-023）。1 つの失敗が他のブロックの描画を止めないよう、ブロック単位で例外を捕捉する。
- テーマ切り替え時は、`mermaid.initialize` をやり直したうえで、`ownSource(block)`（保存しておいた `data-source`）から再描画する（FR-070）。
- **`securityLevel: 'strict'` は、URL を指定した `click` のリンクを止めない**（11.17.2 で実測。ノードが SVG の `<a xlink:href>` で包まれる）。止まるのはコールバックだけである。図の中のリンクは、IMP-223 が本文のリンクと同じ経路で扱う（MD-081, [BUG-015](../bugs/2026-09-17-bug-015-mermaid-click-link-navigation.md)）。
  - **資産の読み込みが済んでいるかを条件にしない。** 初めての図の資産を読み込んでいる間に切り替えると、読み込みを待っていた文書の描画は世代が変わって描かずに戻る（IMP-230）。条件にすると、**図が原文のまま残る**（v1.0.0 には世代が無く、読み込みの後にそのまま描いていたため起きなかった）。読み込み中なら同じ読み込みを待ち（IMP-230）、鍵の合うブロックが無ければ資産を読まずに戻る（NFR-013）。
  - **描き直す図のブロックは、描き終えるまで描き直す前の高さに固定する**（DSP-370。`drawing.js` の `holdHeight` / `releaseHeight`。ブロックに `is-redrawing` を付け、高さを `--diagram-height` に入れる）。描画済みの SVG を原文の `<pre>` へ戻した時点で高さが変わり、図より下の本文が動くためである。**固定は `onDiagramSettled` を呼ぶ直前に解く**（成功・失敗のどちらでも）——原寸表示の判定と当て直しは図の本来の配置で測る（IMP-228）。世代が変わって描かずに戻ったブロックは解かない（次の描画が解くか、本文ごと差し替わる）。
  - **描けなかった図（理由を添えたブロック。`.mermaid-error` / `.plantuml-error`）は描き直さない**（DSP-370）。原文のコードブロックは CSS だけで新しい配色に追随する。描き直すと、PlantUML は描けないまま原文の `<pre>` を器に置き換えるため、理由が出るまで本文が動き、**原文の中の検索のハイライトが外れる**（v1.0.0 から。IMP-241）。読み込めなかった資産の読み直しは、次の文書の描画（`F5` を含む）で行う。

### IMP-232: KaTeX の初期化 **MUST**

Go 側が数式を `.math-inline` / `.math-block` の要素として出力している（IMP-113）ため、**要素単位で `katex.render` を呼ぶ**。

```js
document.querySelectorAll('#markdown .math-inline, #markdown .math-block')
  .forEach((el) => {
    const src = el.textContent;           // Go 側が入れた TeX ソース
    katex.render(src, el, {
      displayMode:  el.classList.contains('math-block'),
      throwOnError: false,                // FR-023 と同じ方針。エラーでも描画を継続する
      errorColor:   'var(--danger-fg)',   // 既定の #cc0000 を使わせない（下記）
      trust:        false,                // NFR-030
    });
  });
```

- **KaTeX の auto-render 拡張（`renderMathInElement`）を使わない。** Go 側の変換段階でデリミタ（`$` / `$$`）は既に除去されているため、デリミタ走査では一致しない。加えて auto-render は本文全体を走査するため、コードブロック内の `$` を数式と誤認する余地が生じ、MD-060 の「コードブロック内の `$` は数式として解釈しない」に反する。数式の範囲判定は Go 側の 1 箇所に集約する。
- 要素ごとに `katex.render` を呼ぶため、1 つの数式の失敗が他の数式へ波及しない。
- 失敗時は `throwOnError: false` により元のソースが赤字で出力される。DSP-271 の「元のソースを `--danger-fg` の等幅テキストで表示」と一致する。
- **`errorColor` を必ず渡す。** 省略すると KaTeX が既定の `#cc0000` を**インラインスタイル**として書き込み、CSS からは `!important` なしに上書きできない。テーマにも追従しなくなる。トークンを参照する式（`var(--danger-fg)`）をそのまま渡せば、解決は要素の位置で起きるため Light / Dark の双方に追従する。
- テーマ切り替えでの再描画は不要とする。KaTeX の出力は文字色を継承させる（DSP-271）ため、CSS の切り替えだけで追随する。この点が Mermaid（再描画が必要。IMP-231）と異なる。

### IMP-233: PlantUML の初期化と描画 **MUST**

FR-024 / MD-083 / MD-084 を実装する。**Mermaid とは API の形が違うため、同じやり方では書けない。**

```js
// js/lazy.js
// 順序を守る。viz-global.js がグローバルに Viz を置き、plantuml.js がそれを見る。
await load("vendor/plantuml/viz-global.js");   // 1
const puml = await import("vendor/plantuml/plantuml.js");  // 2

puml.render(lines /* string[] */, targetElementId, { dark: state.theme === "dark", maxSvgSize: 100000 });
```

**実装で押さえる点は 5 つある。**

| # | 処理系の振る舞い | 実装への帰結 |
| --- | --- | --- |
| 1 | **`render()` は `undefined` を返し、Promise も返さない。** SVG はあとから対象要素へ書き込まれる | **完了を DOM で見るしかない。** `MutationObserver` で対象要素を監視し、**タイムアウトを設ける**。`await` して終わりにはできない。**SVG 以外が書き込まれたら、タイムアウトを待たずに短い猶予で切り上げる**（下記） |
| 2 | **出力先を要素の id で指定する。** 要素そのものを渡せない | 図ごとに一意な id を振る。**文書を切り替えても衝突しない値にする。** **id は `plantuml-svg-<n>` とし、n は `lazy.js` がページを読み込んでから数える連番で、Mermaid の `mermaid-svg-<n>`（IMP-231）と共有する。** `user-content-` で始めない（AR-053）。描画スモーク（BR-054）はこの名前で図の器を見分ける |
| 3 | `renderToString` も export されているが、**どの引数の組でも `undefined` を返す** | 現状使えない。`render()` 経由でのみ取得する |
| 4 | **Graphviz を要する図を Graphviz 無しで描こうとすると、処理系ごと止まる。** その図だけでなく**以降のすべての描画が返ってこなくなる** | **`viz-global.js` の読み込みに失敗したら、PlantUML の描画を一切行わない。** 全ブロックをソースのまま残し、理由を表示する |
| 5 | **描画のたびに `document.getElementById('status')` を決め打ちで探し、見つけた要素の `textContent` を自分のログで上書きする。** 渡した要素とは無関係に、ページ全体を対象にする | **ページ側で `status` という id を使わない**（IMP-202 は `statusbar` とする）。**こちらから止める手立ては無い**——`plantuml.js` は改変できない（BR-042） |

- 描画対象は `.code-block[data-plantuml] pre.plantuml-source` のうち、**ブロックが鍵の合う目印を持つもの**（IMP-230）。**`data-puml-error` を持つブロックは描画しない**（IMP-119 が拒んだもの）。
- **描画結果は `.plantuml-rendered` の中へ入れ、描かなかった理由は `.plantuml-error` へ出す。** Mermaid の `.mermaid-rendered` / `.mermaid-error`（IMP-231）と同じ形にそろえる。**この 2 つの名前は描画スモークテストが見る**（BR-054, E2E-109）ため、変えるときは `scripts/smoke/collect.js`（ページ側で結果を集める関数。4.66.0 で `harness.js` から分けた）も同じ変更で直す。
- 描画に失敗したブロックは、元のソースをコードブロックとして残し、理由を併記する。**1 つの失敗が他のブロックの描画を止めない**（IMP-231 と同じ）。
- **取り込み指令で拒まれたブロックの理由は、資産を読まずに表示する**（IMP-119, DSP-272）。それらは `needsPlantUML` を立てないため、**`needsPlantUML` を条件に描画関数を呼ぶと理由が出ない。** 描画関数を「描くものが無ければ資産を読まずに戻る」形にし、**条件を付けずに呼ぶ**。NFR-013 は早期の戻りで保たれる。**「描くもの」は鍵の合うブロックで数える**——生 HTML の偽のブロックで資産を読まない（IMP-230, BUG-014）。
- **構文エラーは失敗ではない。** PlantUML はエラーを描いた SVG を返すので、**そのまま出す**（FR-024）。行番号と該当行を含むため、こちらで書き直すより情報量が多い。
- **図の大きさの上限は `maxSvgSize` で必ず明示する（省かない）**（MD-083。4.64.0）。**値は 100000**（4.96.0。v1.2.0-rc.1 までは 4096。[BUG-030](../bugs/2026-11-11-bug-030-plantuml-size-limit.md)）。1.2026.8 から処理系の既定が 8192 px になり、省くと同梱資産の更新だけで描ける大きさが変わる。**1.2026.7 以前はこの指定を無視する**（4096 px の固定）。値は `puml.js` の定数に置き（UI 文言ではない）、理由の文言（`pumlUnsupported`）へも渡す（IMP-290）。
- **処理系は配置を計算し終えてから大きさを確かめる**（実測。BUG-030）。上限を超える図も、描ける図と同じだけ計算に時間がかかる（JSON 26,983 px で、拒むまで 5,532 ms・描くまで 5,595 ms）。**上限は描画の待ち時間を縛らず、計算した図を画面に出すかだけを決める**——値は画面に出す手間で決めた（MD-083）。**計算の間はメインスレッドが止まる**（Graphviz を使わない図は全体が 1 つのタスク。NFR-012 の例外）。
- **描き終えるまで原文を出しておく**（DSP-272。4.96.0。BUG-030）。器（`.plantuml-rendered`）は `hidden` にして原文の `<pre>` の後ろに置き、処理系にはそこへ描かせる。**SVG が入ったら原文と入れ替える**（器の `hidden` は原文の `hidden` を引き継ぐ。編集欄を開いている間は隠したまま。IMP-264）。**描けなければ器を外して理由を添える**（原文はそのまま。中の検索のヒットも残る。IMP-241）。描き直し（IMP-243）が描きかけのブロック（原文がまだある）に当たったら、器だけを外して原文から描き直す。待っている間に世代が変わったら、器を外して何も書かない。**隠した器へ描かせても、得られる図は変わらない**（8 種類の図で大きさと時間が同じだった。実測）。v1.2.0-rc.1 までは描き始めに原文を器へ置き換えており、処理系が細切れに動く間（Graphviz の図）は図の場所が空になっていた（線 1,000 本の class 図で 3 フレーム。直した後は 0）。Mermaid（IMP-231）は前から描き終えるまで原文を残している。
- **上限を超える図は SVG ではなく例外のテキストが返る**（1.2026.7 は `Diagram too large for browser rendering: <幅>x<高さ> (max 4096)`。1.2026.8 からは `(max <上限>; override via the maxSvgSize option, or set it to 0 to disable this check)`。**文言で判定しない**）。これを検知して FR-110 の表示に回す。テキストをそのまま本文へ出さない（UI 文言は `strings.js`。IMP-290）。**`render()` は例外を投げない。** 正常に戻ったうえで、**出力先の要素へ例外のテキストが書き込まれる。** したがってこれは `try` / `catch` ではなく、**上の 1 の完了検知が「SVG 以外が入った」と判定する経路**で拾う（実測。[BUG-010](../bugs/2026-09-06-bug-010-plantuml-4096-testdata.md)）。**表示はどちらの経路でも `pumlUnsupported` であり、利用者から見た違いは無い**（DSP-272）。
- テーマ切り替え時は、`ownSource(block)`（保存しておいた `data-source`）から `{ dark: ... }` を変えて**描き直す**（FR-070, IMP-243）。**資産の読み込みが済んでいるかを条件にせず、描き直す図のブロックの高さを描き終えるまで固定し、描けなかった図は描き直さない**（IMP-231 と同じ）。
- **フロントエンドが図のソースから独自に HTML を組み立てて挿入しない**（IMP-220, MD-084）。DOM へ書くのは処理系であり、こちらは対象要素を用意して id を渡すだけにする。

> [!IMPORTANT]
> **SVG 以外が書き込まれたら、タイムアウトを待たずに切り上げる。** 処理系は「描けない」と
> 答えるときも対象要素へ何かを書くが、それは SVG にならない。**描画は逐次であるため**（下記）、
> 待ち続けると描けない図 1 枚が後続の図をタイムアウトいっぱい待たせる。
>
> 実測では `@startditaa` が内容を即座に返しながら SVG にならず、**猶予を設けない実装では
> 1 枚で 30 秒を空費した**（2026-09-03。猶予を入れて文書全体が 31.4 秒 → 1.9 秒）。
>
> 猶予を 0 にしない。処理系が入れ物を先に置いてから SVG を入れる場合に早合点するため。

> [!IMPORTANT]
> **描画は逐次行う。** Graphviz を要する図は 1 枚 400〜700 ms かかる（NFR-011）。**完了検知の監視対象が図の数だけ同時に存在する状態を作らない**。逐次であれば、図と図の間では操作が効く（処理系は図 1 枚の計算の間メインスレッドを止める。NFR-012 の例外。4.95.0 までは「コルーチンで細切れに実行するため UI は固まらない（最大停止 4 ms）」としていた。BUG-030）。

## 12.5 操作（IMP-240 系）

### IMP-240: ペインの開閉とリサイズ **MUST**

FR-034 / FR-035 / FR-043 / UI-030 / UI-040 を実装する。

```js
// js/panes.js
export function initPanes()          // 起動時に 1 度（IMP-211）。ペインの見出しの文字を入れ、リサイザを配線する
export function togglePane(name)     // 'outline' | 'filetree'。**表示になったかを返す**
export function setPaneWidth(name, px)
export function applyResponsive()    // ウィンドウ幅に応じた一時的な非表示（IMP-246）
```

- **ペインの見出しの文字（`Files` / `Outline`。UI-030, UI-040, DSP-111）は、`initPanes` が `strings.js` の `paneFiles` / `paneOutline` から入れる**（IMP-202, IMP-290）。文字は変わらないため、起動時の 1 度でよい。**v1.0.0〜v1.1.0 は、どのモジュールも入れておらず、見出しの行が空のまま出ていた**（4.90.0 で担当を書いた。[BUG-028](../bugs/2026-11-09-bug-028-pane-title-empty.md)）。
- 開閉は `hidden` 属性の切り替えと、対応するリサイザの表示切り替えで行う。
- **ファイルツリーが非表示から表示になったら、ツリーを読み直す**（[FR-035](02-functional.md) の 1 番目の契機）。`filetree.js` の `loadTreeRoot` を呼ぶ。
  - **`panes.js` から `filetree.js` を呼ばない。** `togglePane` は「表示になったか」を返すだけとし、**契機の判断は `main.js` に置く**（IMP-201 の依存の明示）。ここで呼ぶと、ペインの開閉というひとつの関心にツリーの読み込みが混ざる。
  - **ツールバーのボタンとショートカットの両方が同じ関数を通るようにする。** 片方だけに足すと、経路によって挙動が変わる。
  - **表示になったときだけ呼ぶ。** 閉じる操作や、既に開いている状態では呼ばない。`ReadDir` は毎回ディスクを読む（IMP-310）ため、大きなディレクトリで引っかかる（NFR-020）。
  - `loadTreeRoot` はツリーを作り直すため、**利用者が開いていたディレクトリの展開状態は失われ、表示中の文書までの経路だけが開き直される**（`revealCurrent`。DSP-331）。FR-035 は展開状態の保持を求めていない。
- **再読み込み操作（[FR-015](02-functional.md)）でもツリーを読み直す**（FR-035 の 2 番目の契機）。`Reload()`（IMP-310）は表示中の文書を開き直すだけでツリーに触れないため、**フロントエンドが続けて `loadTreeRoot` を呼ぶ。**

> [!IMPORTANT]
> **FR-035 は再読み込みの契機を 5 つ定めている（4.73.0 で 3 つから 5 つになった）。担当を分けて書く。**
>
> | # | 契機 | 担当 |
> | --- | --- | --- |
> | 1 | ツリーペインを非表示から表示に切り替えたとき | **本 ID**（`togglePane` の呼び出し側） |
> | 2 | 再読み込み操作（FR-015）を行ったとき | **本 ID**（`reloadCurrent` の後段） |
> | 3 | ディレクトリノードを折りたたんでから再度展開したとき | [DSP-330](22-display-states.md)（展開のたびに `ReadDir` を呼ぶ） |
> | 4 | ツリーの右クリックメニューでツリールートを変えたとき（4.73.0） | IMP-270（`SetTreeRoot` / `SetTreeRootUp` の戻り値で `loadTreeRoot`） |
> | 5 | ツリーのフィルターを切り替えたとき（4.73.0） | IMP-271（展開の状態・強調・スクロール位置を保って読み直す `reloadTreeKeepingState`） |
>
> **1 と 2 の担当がどの ID にも無かったため、実装されないまま通過した**（[調査報告](../bugs/2026-09-04-bug-002-filetree-reload-on-show.md)）。**箇条書きが複数ある要求では、90 章の対応表の 1 行が「全部見た」に見える。** 項目ごとに担当があるかを確かめる。
- 幅の下限は 160 px、上限はウィンドウ幅の 40 %（IMP-153 の補足）。ドラッグ中に毎回クランプする。
- ドラッグ中は Go 側へ通知せず、`pointerup` の時点で 1 回だけ通知する（UI-114）。
- ドラッグ中は `pointermove` を `requestAnimationFrame` でまとめ、レイアウト計算の頻度を抑える（NFR-012）。
- **開閉・リサイズの前後で本文のスクロール位置を維持する**（DSP-311）。幅が変わると折り返し位置が変わるため、絶対座標ではなく「文書全体に対する相対位置」（`scrollTop / scrollHeight`）を保存し、レイアウト確定後に復元する。
- 維持の対象は**ペインの操作**（開閉・ドラッグによる幅変更・幅不足による一時的な非表示）とする。**ウィンドウ自体のリサイズは対象外**である。`resize` イベントが届く時点で本文は既に新しい幅で組み直されており、変更前の相対位置を読む手段がない。DSP-350 の表も「ペインの開閉・リサイズ」を対象としている。


### IMP-241: 検索 **MUST**

FR-080 / UI-080 を実装する。

```js
// js/search.js
export function openSearch()
export function closeSearch()
export function find(query)      // インクリメンタル
export function jump(delta)      // +1 / -1
export function isSearchOpen()   // 開いているか（Esc の振り分けに使う）
export function syncHits(root)   // 図・数式を描き終えたときに件数とハイライトを合わせる（FR-080）
```

- **本文を表示している状態でのみ開く。** 状態画面を表示中（`welcome` / `confirm-large` / `too-large` / `render-error`）は `Ctrl+F` を無視する（DSP-300）。
- 走査対象は `#markdown` のテキストノードのみ。`<script>` や属性値は対象外。
- **フロントエンドが後から描いた領域を走査から外す。** 対象は `svg`（**Mermaid と PlantUML の描画結果**。HTML の `<mark>` を差し込むと図が壊れる）と `.katex`（KaTeX の描画結果。MathML と HTML に同じ文字が二重に入っており、包むと数式が崩れるうえ件数も倍になる）。いずれも Go が出力した本文ではなく、原文は `data-source` と TeX ソースとして別に残っている。**PlantUML の図は文字を多く含む**ため、除外を忘れると図の中の語が大量にヒットする。
- **折りたたみ（`<details>`。[MD-026](04-markdown.md)）の中は除外しない。** 上の 2 つは「原文が別に残っている描画の副産物」だが、**折りたたみの中身は原文そのもの**であり、除外の理由が当てはまらない。除外すると、件数が利用者の開閉で変わるうえ、**「文書にあるのに見つからない」**状態になる。折りたたみの中は「まだ読んでいない箇所」であり、検索で見つけたい場所そのものである（FR-080, UI-080）。
- **テキストノードは先にすべて集めてから包む。** 包む処理はテキストノードを分割するため、走査しながら変更すると同じ箇所を二重に処理する。
- 解除時は、`<mark>` を外したあとに親要素へ `normalize()` を呼び、分割したテキストノードを 1 つへ結合し直す。これを省くと、次の検索で分割の境界をまたぐ語が見つからなくなる。
- ハイライトは `<mark class="search-hit">` で包む方式とし、原文の DOM 構造を壊さないよう、テキストノードの分割のみで実現する。要素の入れ子構造を変更しない。
- 現在位置のヒットには `search-hit` に加えて `search-hit-current` を付与する。配色は DSP-161 で定める。クラス名を 2 種に分けることで、移動時は付け替えだけで済み、DOM の作り直しが起きない。
- 検索終了時（`Esc`・文書切り替え・再描画・**表の並べ替え**。FR-080, IMP-229）は、包んだ `<mark>` を必ず解除して元のテキストノードへ戻す。解除処理を持たないハイライト実装を採らない。
- **自前で足したボタン（`.copy-btn` / `.media-actions` / `.sort-btn`）は `svg` しか持たず、上の `svg` の除外で走査から外れる。** セルの編集欄（`input.cell-editor`。IMP-262）の値はテキストノードではなく、走査に入らない。
- **文字を持つ自前の要素は、走査から外す**（4.75.0）: 編集欄の `Save` / `Cancel`（`.edit-actions`。IMP-262, IMP-264）と、図のファイルの `No diagrams found.`（`p.no-diagrams`。IMP-220）、**描けなかった図に併記した理由とエラー内容（`.plantuml-error` / `.mermaid-error`。IMP-231, IMP-233。4.81.0 で書き足した）**。UI の文言であり、文書の文字ではない。コードブロックの編集欄（`textarea.code-editor`）の値も走査に入らない。
- **コードブロックの編集欄を開いている間は、隠した元の中身（`.is-code-editing` の中の `[hidden]`）を走査から外す**（4.76.0）。数えると、移っても見えないヒットが件数に残る（BUG-004 と同じ形）。**そのブロックで `syncHits` を呼ぶのは、中身が変わるとき**——コードブロックの編集欄を開いたとき・確定の直後に `pre.code-pending` を置いたとき・失敗で開き直したとき・元の中身を戻したとき、セルの確定の直後に `.cell-pending` を置いたとき・失敗で退避した中身を戻したとき（セルの編集欄の開閉では中身が変わらない。4.93.0）。DOM から外れた `<mark>` を落とし、戻った中身を探し直す（BUG-017 と同じ形）。
  - **`syncHits` は、DOM に残っていても走査から外れたヒットを落とす**（4.93.0）。隠した元の中身は DOM から外れないため、外れたものだけを落とすと件数に残る。**現在位置のヒットが走査から外れたときも「外れた」とみなし**、後ろに残っている最初のヒットへ移す（`0 / N` にしない）。**件数が 0 になっても戻らない**——そのブロックにしか無かった語は、編集欄を開くと 0 件になり、閉じたときに探し直す。
  - **探し直す前に、そのブロックに残る古い `<mark>` を解く**（4.93.0）。隠した中身の中のものと、確定で退避して失敗で戻した中身の中のもの（件数からは落ちているが要素は残っている）である。解かずに探すと `<mark>` の中を二重に包む。
- 大文字小文字を区別しない比較には `toLowerCase()` を用いる。正規表現でユーザ入力を直接使わない（メタ文字の混入を避けるため）。
- 200 件を超えるヒットがある場合もハイライトは全件に付ける。件数表示は実数を出す。
- **`syncHits(root)` は、図か数式を 1 つ描き終えたときに呼ぶ**（IMP-230, IMP-232。4.69.0。[BUG-017](../bugs/2026-09-18-bug-017-search-hits-before-rendering.md)）。検索を開いていなければ何もしない。
  - **DOM から外れた `<mark>` を件数から落とす。** 描画は原文の `<pre>`（Mermaid / PlantUML）と数式の要素の中身を置き換えるため、そこに包んだ `<mark>` は外れる。**外れたものは `state.search.hits` にも残る**——長さをそのまま件数に出すため、落とさないと画面に無いものを数える
  - **`root`（描き終えたブロック）の中にヒットが 1 つも残っていなければ、その中を探し直して包み直す。** 描けなかった PlantUML は**新しい `<pre>`** に原文が戻るためである（IMP-233 の `restorePlantUML`）。**残っていれば触らない**——描けなかった Mermaid の原文はそのまま残っており、二重に包むことになる
  - **包み直したものは文書の順（`compareDocumentPosition`）で並べ直す。** 件数と移動の順序を本文の順序に保つ
  - **スクロールしない**（`select` を呼ばない）。**現在位置が外れていたら、元の並びでその後ろに残っている最初のヒットへ移す**（末尾なら先頭）。現在位置のクラス（`search-hit-current`）は付け替える
- `find` の直後は、**本文ペインの上端以降にある最初のヒット**を現在位置とする。常に先頭へ戻すと、入力を 1 文字足すたびに文書の冒頭へ引き戻される。
- `jump` は端で反対側へ回り込む。ヒットが 1 件でも操作が空振りしない。
- **検索を開いていないときの `jump` は何もせず、`false` を返す**（IMP-244）。`Enter` は検索が閉じていてもこの経路へ来るため、`preventDefault` してしまうとフォーカス中のボタンを `Enter` で実行できなくなる（UI-021）。
- **移動の前に、移動先が見える状態を作る**（FR-080, MD-026）。**閉じた `<details>` の中身は `scrollIntoView` の対象にならない。** `getBoundingClientRect` は大きさも位置も返す（実測で 723×35、`top=1277`）のに、**スクロールは起きず（`scrollTop` は 0 のまま）、`<details>` が自動で開くこともない。** 例外も警告も出ない。**開いてから呼べば動く**（同じ条件で `scrollTop` が 1161 になり、器の中に入る）。
  **「大きさを持っているか」で判定しない**——持っている。判定できるのは「祖先に閉じた
  `<details>` があるか」だけである（2026-09-05 に Edge 152 で実測）。
  現在位置を移す処理の中で、**祖先の `<details>` を根までたどって開く**（入れ子に対応する）。
  **開いたものを閉じ直さない。** 検索を閉じた時点で閉じると、利用者が中身を読んでいる最中に畳んでしまう。
- **移動は「前へ」「次へ」のボタンでも起こる**（FR-080）。`Enter` / `Shift+Enter` と
  **同じ経路を通す。** 片方だけに処理を足すと、もう片方だけが壊れたときに気づきにくい。
- **移動のあとにフォーカスを操作する場合、スクロールを巻き戻してはならない。**
  `HTMLElement.focus()` は既定で対象を可視域へスクロールするため、`jump` が行った移動を
  打ち消しうる。フォーカスを戻すなら `focus({ preventScroll: true })` とする。
  **UI-080 が求めているのは「表示と同時に」入力欄へフォーカスを移すことであり、
  移動のたびに戻すことではない。**

> [!IMPORTANT]
> **この 2 つは対で意味を持つ。** 検索バーが本文と一緒に流れる状態（DSP-160 の違反）では、
> `focus()` が「見せるために」スクロールを先頭側へ巻き戻し、**ボタンからの移動だけが効かなくなる。**
> `Enter` は同じ `jump` を呼びながらフォーカスを操作しないため動いてしまい、**症状が片側にしか
> 出ないので原因を取り違えやすい。** 実際に起きた（[調査報告](../bugs/2026-09-03-search-jump-buttons.md)）。

> [!IMPORTANT]
> **「ハイライトが付いた」ことを「移動できた」の証拠にしない。** 閉じた `<details>` の中でも、
> テキストノードの分割・`<mark>` の生成・クラスの付け替え・件数の更新は**すべて成功する。**
> 失敗するのは `scrollIntoView` だけであり、しかも**黙って戻る。** 実際に通り抜けた
> （[調査報告](../bugs/2026-09-04-bug-004-search-collapsed-details.md)）。
>
> **`find` の直後の現在位置決定（本文ペインの上端以降にある最初のヒット）には手を入れなくてよい。**
> 折りたたみの中の `<mark>` は座標がすべて 0 を返すため、候補から自然に外れる。開くのは
> 現在位置を移す時点でよい。
>
> **同じ構造の問題が、アンカー移動（IMP-223）とアウトラインからの移動（IMP-224）にもある。**
> **3 つの経路（検索・アンカー・アウトライン）の 4 か所で同じ関数を使う**（アンカーは、本文のリンクのクリックと、開いた直後のアンカーの復元の 2 か所。IMP-223。`util.js` に置く。`search.js` に置くと `outline.js` との循環参照になる）。
>
> ```js
> // js/util.js
> export function openAncestorDetails(element)   // 祖先の <details> を根まで開く
> ```
> 片方だけ直すと「検索では行けるがリンクでは行けない」という、説明の付かない差が残る。

### IMP-242: 表示倍率 **MUST**

FR-081 を実装する。

```js
// js/zoom.js
function applyZoom(percent)        // 画面へ反映する。**公開しない**
export function setZoom(percent)   // 50..300、10 刻みに丸めて反映する
export function stepZoom(delta)    // +1 / -1（1 段 = 10 %）
export function initZoom()         // Ctrl + ホイールを配線する
```

**倍率は設定に保存しない**（UI-111, UI-115）。`state.zoom` はセッション内でのみ保持し、`saveConfig`（IMP-210）を呼ばない。`ConfigDTO` にも含めない（IMP-303）。起動時は常に 100 % から始まる。

- 適用は `#markdown` の CSS カスタムプロパティ `--zoom` を更新することで行い、`font-size` を `calc(16px * var(--zoom) / 100)` として与える（DSP-021）。
- ツールバー・サイドペイン・ステータスには適用しない（FR-081）。
- `Ctrl` + ホイールは `wheel` イベントで `ctrlKey` を見て処理し、`preventDefault()` でブラウザ既定のズームを抑止する。リスナは `window` に置く。本文の外（ペインやツールバーの上）でも WebView 既定の拡大が起きてはならない（AR-060）。
- **拡大画面（IMP-253）を開いている間は、本文の倍率を変えない。** `preventDefault()` だけを行って戻る。`Ctrl` + ホイールと `Ctrl` + `+` / `-` / `0` は拡大画面の倍率を変える（UI-104）。判定は `expand.js` の `isExpandOpen()` を使う。
- **範囲（50〜300）と刻み（10）はこのモジュールが持つ。** 倍率は保存されないため、`config` パッケージ側に対応する定数を置かない（IMP-153）。
- **反映と丸めを別の関数に分ける。** `applyZoom` は渡された値をそのまま反映し、`setZoom` は 10 の倍数へ丸めて範囲へ収めてから `applyZoom` を呼ぶ。**`applyZoom` は公開しない。** 操作の入口を `setZoom` / `stepZoom` に限ることを、モジュールの外から呼べないことで保証する。倍率を復元しなくなった（UI-111）ため、丸めを経ない反映を外部から行う理由がなくなった。
- 値が変わらないときは何もしない。上限・下限に張り付いた状態でキーを押し続けたときに、同じ値の反映とステータス更新を繰り返さない。
- 100 % 以外のときだけステータス領域へ倍率を出す（FR-081, DSP-150）。

### IMP-243: テーマ **MUST**

FR-070 / UI-105 を実装する。

```js
// js/theme.js
export function applyTheme(theme)  // 'light' | 'dark'。反映のみ。保存しない
export function toggleTheme()      // 切り替えて反映し、保存する
```

- `#app` の `data-theme` 属性を書き換えるだけで全体に反映する。CSS 変数の切り替えで完結させ、要素の再生成や本文の再変換を行わない（UI-105）。これにより DSP-370 が求める維持（スクロール位置・検索状態・ツリー・アウトライン・倍率・**並べ替え・編集モード**）は、何もしなくても成り立つ。**原寸表示だけは、描き直した図へ当て直す必要がある**（IMP-228 の `onDiagramSettled`）。
- Mermaid（IMP-231）と PlantUML（IMP-233）は再描画が必要。**どちらも待たない。** 図の描画で画面全体の切り替えを遅らせない。PlantUML は Graphviz を要する図で 1 枚 1 秒近くかかる（NFR-011）。
- **起動時の適用と切り替えを別の関数に分ける。** `applyTheme` は反映のみを行う。起動時にここが保存すると、利用者が選んでいないテーマが記録され、OS 設定への追従（FR-071）が失われる。
- `toggleTheme` は `state.themeExplicit` を立ててから `saveConfig`（IMP-210）を呼ぶ。**この印が立つまで設定にテーマを書かない。**
- **属性を書き換える前後だけトランジションを止める。** ツールバーのボタンは背景色を 80ms でフェードさせるため（DSP-050）、そのままではテーマ切り替え時に ON のトグルの背景だけが遅れて追いつく。DSP-011 の「即時に完了させる」を満たすため、`#app` に一時的なクラスを付けて `transition: none` を効かせ、レイアウトを 1 度確定させてから外す。
- アイコンとツールチップは**切り替え先**を示す（Light 表示中は月と `Dark theme / ダークテーマ`）。

### IMP-244: ショートカット **MUST**

UI-090 を実装する。

```js
// js/shortcuts.js
export function initShortcuts(handlers)  // id をキーとする処理の表
```

- `window` の `keydown` に 1 つだけリスナを置き、テーブル駆動で処理する。個々の要素にキーハンドラを分散させない。
- 入力欄（検索バーの入力欄、セルの編集欄）にフォーカスがある間は、テキスト編集に関わるキーを素通しする（UI-090）。**素通しさせるのは編集に関わるものだけとする。** UI-090 は「テキスト編集に関わるキーを優先する」と定めているのであって、入力中はすべてのショートカットを止める、とは定めていない。`Ctrl+Shift+T` などは入力中も有効とする。
  - **入力欄の判定は `input.search-input, input.cell-editor, textarea.code-editor`（文字の入力欄。コードブロックの編集欄は 4.75.0）に限る。** チェックボックス（`input[type=checkbox]`）とラジオボタンは入力欄に数えない。v1.0.0 の `isEditing` は `input` 要素をすべて入力欄とみなしている。**v1.1.0 では再描画の後にフォーカスがチェックボックスへ戻る**（IMP-220 の手順 11）ため、そのままでは**チェックボックスを切り替えた直後の `Ctrl+Z` / `Ctrl+Y` が素通しされ、既定の動作にも何も無いため黙って何も起きない**（FR-144）。
- **`preventDefault()` はリスナ側で一括して呼ぶ。** 個々のハンドラに書き漏らす余地を残さない。ただし**ハンドラが `false` を返したときは「何もしなかった」とみなし、既定の動作を止めない**。検索が閉じているときの `Enter` が、フォーカス中のボタンの実行（UI-021）を妨げないようにするためである。
- **1 文字のキーは小文字へ揃えて表を引く。** `Shift` と `CapsLock` で `KeyboardEvent.key` の大小が変わるためであり、`Shift` の有無は修飾子側で区別する。CapsLock が入っているだけで `Ctrl+O` が効かない、という事態を避ける。
- 修飾子の順序は `Ctrl` → `Alt` → `Shift` に固定し、表の表記もこれに揃える。
- **倍率のキーには別名を並べる。** US 配列では `+` が `Shift`+`=`、JIS 配列では `Shift`+`;` であり、テンキーの `+` には `Shift` が付かない。どの経路でも同じ動作になるよう、実際に届く組み合わせをすべて表に書く。表以外の場所で例外を作らない。
- IME の変換中（`event.isComposing`）は何も割り当てない。変換確定の `Enter` を検索の移動として拾わないため。
- ショートカットの定義は 1 箇所の配列にまとめ、ツールチップ（IMP-290）とキー割り当てが同じ定義を参照する。**キーの表記が 2 箇所に分かれることを避ける。**
- `Ctrl+C`（FR-062）はハンドラを結び付けず、WebView の既定に任せる。`Alt+F4` と閉じるボタンは OS とウィンドウマネージャが処理する。`Ctrl+Q` のみアプリケーション側で受け、`api.quit()`（IMP-310）を呼ぶ。

**v1.1.0 で足す割り当て**（UI-090）

| id | キー | 処理 | 返り値 |
| --- | --- | --- | --- |
| `editMode` | `Ctrl+Shift+M` | IMP-260 の `toggleEditMode()`。**ツールバーのボタンと同じ入口を通す** | 開始できない状態なら `false`（何もしない） |
| `undo` | `Ctrl+Z` | IMP-263 の `undoEdit()` | 編集モードでなければ `false`（既定の動作を止めない） |
| `redo` | `Ctrl+Y` / `Ctrl+Shift+Z` | IMP-263 の `redoEdit()` | 同上 |

- **`undo` / `redo` はテキスト編集に関わるキーとして扱う。** 入力欄（検索バーの入力欄、セル・コードブロックの編集欄）にフォーカスがある間は素通しし、入力欄の中の取り消しに任せる（FR-144, UI-090）。

**v1.2.0 で足す割り当て**（UI-090。4.75.0）

| id | キー | 処理 | 返り値 |
| --- | --- | --- | --- |
| `back` / `forward` | `Alt+←` / `Alt+→` | `navigate.js` の `goBack()` / `goForward()`。**ツールバーのボタン・マウスのサイドボタンと同じ入口を通す**（FR-051） | なし（**履歴の端でも既定の動作を止める**——WebView 自身のページの履歴を戻らせない。AR-060。サイドボタンと同じ。v1.1.0 も止めていた。4.76.0） |
| `strike` | `Ctrl+Shift+X` | IMP-265 の `toggleStrike()`。**入力欄（検索バー・編集欄）の中では働かない**（FR-146）——テキスト編集に関わるキーとして素通しせず、何もしない | 使えないとき `false` |

- **`Ctrl+Enter` と `Tab` / `Shift+Tab` は表に入れない。** コードブロックの編集欄の `keydown` が受ける（IMP-264。IMP-262 の `Enter` と同じく、特定の要素で受ける例外）。
- **マウスのサイドボタン**（FR-051 の SHOULD。UI-090）: `window` の `mouseup`（キャプチャ）で `event.button` が `3`（戻る）/ `4`（進む）なら、`preventDefault()` してから `goBack()` / `goForward()` を呼ぶ。**`mousedown` と `auxclick` でも `button` が `3` / `4` なら `preventDefault()` する**——WebView 自身のページの履歴を戻らせない（AR-060）。**`Alt+←` / `Alt+→` と同じ場面（拡大画面・ダイアログ・右クリックメニュー・文字色の一覧の表示中）では何もしない**（既定の動作は止める）。**L1 でサイドボタンが DOM へ届くかは実機で確かめる**——届かなくても機能が欠けるだけで、安全上の問題にはならない（実現性検討の 3.5）。
- **編集欄が開いている間は、`open` / `reload` / `back` / `forward` / `editMode`（終える側）/ `undo` / `redo` / `strike`（`undo` 以下の 3 つは、編集欄の外にフォーカスがあるとき。中にあれば上の入力欄の扱い）を拒む**（FR-140 の表。UI-090）。各ハンドラの入口で IMP-264 の `refuseIfEditing()` を呼び、真なら何もせずに戻る（警告とフォーカスの戻しは `refuseIfEditing` が行う）。**`theme` / `outline` / `fileTree` / `search` / 倍率 / `about` は拒まない**（表示中の文書を差し替えない）。
- **`Shift+F10` とアプリケーションキーは割り当てない。** WebView はこれらのキーで `contextmenu` イベントを発火させるため、右クリックと同じ経路（IMP-249）で受ける。
- **`Esc` の振り分けは UI-090 の順序に固定し、この 1 か所に書く。** (1) `isContextMenuOpen()` なら、サブメニューが開いていれば `closeSubmenu()`（サブメニューだけ）、そうでなければ `closeContextMenu()`（ツールバーの文字色の一覧も同じ要素。IMP-249。4.93.0 で `closeSubmenu` を分けた——`closeContextMenu()` は文書の切り替え・スクロール・`blur` からも呼ばれ、そこではすべて閉じる）、(2) `isExpandOpen()` / `isAboutOpen()` / `isEditorsOpen()` のうち開いているものを閉じる、(3) **編集欄（セル・コードブロック）にフォーカスがあれば**（`Save` / `Cancel` のボタンを含む。IMP-264 の `isEditBoxFocused()`）、`cancelEditBox()` を呼び、`focusViewer()` で本文ペインへフォーカスを戻す（IMP-262, IMP-264, UI-055）、(4) フィルターの枚（IMP-271）と検索バーのうち、**フォーカスのある方**を閉じる。どちらにもフォーカスが無ければ、**フィルターの枚を先に**閉じる（4.73.0）。**1 回の押下で 1 つだけに働かせ、最初に当てはまったところで戻る。**
  - **開いたままの編集欄は、フォーカスが無ければ `Esc` で取り消さない**（UI-090。4.73.0）。検索バーを閉じるつもりの `Esc` で入力を捨てないためである。v1.1.0 の (3) は「編集欄が開いていれば」だった（フォーカスが外れたら取り消していたため、同じことだった）。
  - **他のモジュールは `Esc` を扱わない。** 編集欄の `keydown`（IMP-262, IMP-264）・`handleExpandKey`（IMP-253）・フィルターの枚（IMP-271）は `Esc` を受け持たない。**2 か所に書くと、`stopPropagation` の有無で順序が入れ替わる。**
  - IME の変換中の `Esc` は変換の取り消しであり、ここへ来ない（上記の `isComposing`）。
- **右クリックメニューを開いている間（`isContextMenuOpen()`）は、`Esc` 以外の割り当てを働かせず、既定の動作も止めない。** **ただし `Alt+←` / `Alt+→` は、働かせずに既定の動作だけを止める**（AR-060。サイドボタンと同じ。4.76.0）。 メニューの中の `↑` / `↓` / `Enter` / `Space` / `Tab` は `#contextmenu` の `keydown` が受ける（IMP-249。IMP-248 のツリー、IMP-262 の編集欄と同じく、特定の要素で受ける例外）。**止めないと、検索バーを開いたまま入力欄でメニューを開き、`Paste` へ移って `Enter` を押したとき、`Enter` が検索の移動（`searchNext`）へ回って貼り付けが起きない。**
- **編集欄（セル・コードブロック）にフォーカスがある間は、`Enter` / `Shift+Enter` を検索の移動へ回さない**（FR-142, FR-145, UI-090）。セルの編集欄の `keydown` が確定を受け持ち（IMP-262）、コードブロックの編集欄では改行である（IMP-264）。
- **拡大画面を開いている間は、`Esc` を先に上の振り分けで扱い、UI-104 の表の残りのキーを `expand.js` の `handleExpandKey(event)` へ渡し、それ以外のショートカットを止める**（`Ctrl+Q` を除く）。止めるときは既定の動作も抑止する（IMP-251 と同じ理由。`Ctrl` + `+` が WebView 自身の拡大になるため）。**ただし `Enter` / `Shift+Enter`（`searchNext` / `searchPrev`）は既定の動作を残す**——フォーカスしている操作バーのボタンの実行であり（IMP-253）、ダイアログの表示中と同じ扱いである（IMP-251）。**割り当てを持たないキー（`Tab` / `Space` など）には何もしない**（`Tab` は `#expand-view` の `keydown` が操作バーの中で巡らせる）。
- **ツールチップのキー表記**（IMP-290）は、`editMode` の代表キーを `Ctrl+Shift+M` とする。

### IMP-245: ドラッグ＆ドロップ **MUST**

FR-011 / UI-070 を実装する。

```js
// js/dnd.js
export function initDnd()   // Wails の drop リスナの取り付けと、ドラッグ中の表示
```

- Wails のファイルドロップ機能（`OnFileDrop`）で**絶対パス**を受け取る。HTML5 の `DataTransfer` からパスは得られないため、そちらに依存しない。
- **`OnFileDrop` は既定では呼ばれない。** Wails の起動オプションに `DragAndDrop: &options.DragAndDrop{EnableFileDrop: true}` を渡す必要がある。これを忘れると、コールバックを登録してもドロップが一切届かない。
- **`initDnd` は `window.runtime.OnFileDrop()` を呼ぶ**（`frontend/wailsjs/runtime/runtime.js` 経由）。**これが Wails のランタイム側の `drop` リスナを取り付ける唯一の手段である。**

  ```js
  import { OnFileDrop } from "../wailsjs/runtime/runtime.js";

  // パスの処理は Go 側で行う（IMP-313）。ここで受け取るものは無い。
  OnFileDrop(() => {}, true);
  ```

  - **コールバックは空でよい。** 結果はイベント（`document:opened` / `tree:root-changed` / `history:changed` / `error`）で受け取る（IMP-320, IMP-322, IMP-323）。ここに処理を書くと経路が 2 つになる。
  - 第 2 引数（`useDropTarget`）は `true` とする。ただし**これが検査するのは JS 側のコールバックだけ**であり、Go 側のコールバックは検査を通らずに呼ばれる。
- **受け口となる要素は CSS で宣言する。** ドロップ地点の要素の計算済みスタイルに `--wails-drop-target: drop`（既定のプロパティ名と値）があるかで、Wails の**JS 側のコールバック**が呼ばれるかが決まる。カスタムプロパティは継承するため、`#app` に 1 度だけ置けばウィンドウ全体が対象になる（UI-070, FR-011）。**この宣言は Go 側のコールバックの条件ではない**（上記のとおり検査を通らない）。宣言を残すのは UI-070 の意図を DOM 上に残すためである。
- **`#dropzone` は `pointer-events: none` とする。** Wails は `document.elementFromPoint` でドロップ地点の要素を求めるため、全面を覆うオーバーレイがマウスイベントを受け取ると、その下の要素を検出できずドロップが無効になる。
- `dragenter` / `dragover` / `dragleave` は、オーバーレイ（`#dropzone`）の表示制御と、下のスクロール位置の記録にだけ使う。
- **OS からのファイルのドラッグがウィンドウに入ったとき（オーバーレイを出すとき）に、表示履歴へスクロール位置を記録する**（`navigate.js` の `recordScroll()` を公開して呼ぶ。`navigate.js` は `dnd.js` を import しないため循環しない。IMP-311, FR-051。4.81.0。利用者の判断）。**ドロップは Go が直接受け取る**（IMP-313）ため、フロントエンドは文書を離れる直前を知らない。ドロップの前には必ずドラッグが入るので、ここで記録する。ドラッグを取り消しても、いまの項目の位置が新しくなるだけで害は無い。
- **`dragover` と `drop` では、条件を付けずに自分でも `preventDefault()` を呼ぶ**（4.70.0。[BUG-020](../bugs/2026-09-21-bug-020-file-drop-webview-navigation.md)）。Wails のランタイムも同じことを行うが、**WebView 内でページ遷移を起こさないという規約（AR-060）を外部のランタイムの実装に委ねない。**
  - **下のファイルの見分けに掛けない。** 見分けはオーバーレイを出すためのものであり、**遷移を止める条件ではない。**掛けると、見分けられないエンジンで**画面がファイルの中身に置き換わり、戻す手段が無くなる**（Wails の Linux の実装は `onDragDrop` が `FALSE` を返し、WebKitGTK の既定の処理を止めない）。ウィンドウ内のドラッグでも、リンクを落とせば遷移しうる
  - **`DisableWebViewDrop` は使わない。** `gtk_drag_dest_unset` を呼ぶだけであり、真にすると**ドロップ自体が届かなくなる**（FR-011）
  - **入力欄（検索欄・セルの編集欄）へのドロップも受け付けない**（4.70.0。利用者の判断）。条件を付けずに止める以上、文字列のドロップも入らなくなる。**セルの編集欄ではむしろ正しい**——ドロップは IMP-262 の改行の正規化を通らず、表のセルに生の改行が入る（FR-142）
- **OS からのファイルのドラッグかは、`dataTransfer.types` と、ウィンドウ内で始まったかどうかで判定する**（4.70.0。BUG-020）。**オーバーレイの出し分けにだけ使う。**
  - **`Files` か `text/uri-list` のどちらかがあればファイルとみなす。** **WebKitGTK は `Files` を入れず `text/uri-list` だけを渡す**——`Files` だけを見ると L1 で案内が出ない（UI-070）
  - **`dragstart` が出ていたら（ウィンドウ内で始まったドラッグなら）出さない。** `dragend` と `drop` で下ろす。**本文のリンクをドラッグすると `text/uri-list` が付く**ため、型だけでは本文中のドラッグと区別できない
  - **既定の動作を止められた `dragstart` では旗を立てない**（`internal = !event.defaultPrevented`）。**中止された `dragstart` には `dragend` が続かない**ため（拡大画面の台紙が止めている。IMP-253）、立てたままにすると**旗が固定され、以後ドロップの案内が二度と出なくなる。** 台紙のリスナは対象の段階で動き、`window`（泡立ちの段階）へ来たときには既に止まっている
- `dragleave` はウィンドウ内の要素間移動でも発生するため、カウンタ方式で入れ子の出入りを数え、0 になったときだけオーバーレイを隠す。
- 受け取ったパスの判定（開けるファイルか、ディレクトリか）は Go 側で行う（IMP-313）。
- **編集欄が開いている間のドロップは、Go 側が拒む**（FR-140。4.75.0）。フロントエンドは編集欄を開いたら `api.setEditBoxOpen(true)`、閉じたら `false` を呼ぶ（IMP-264, IMP-310）。**Linux ではドロップが JavaScript を通らずに Go へ届く**（下の IMPORTANT）ため、フロントエンドだけでは止められない。拒んだら Go が `error` イベント（`edit-busy`）を送り、フロントエンドは警告を出す（ダイアログ・拡大画面の表示中はフォーカスを動かさない）。**編集欄を出す前に `await api.setEditBoxOpen(true)` を呼び**、応答の後で、対象の要素がまだ DOM にあり、その間に文書が届いていないことを確かめてから出す（届いていたら `setEditBoxOpen(false)` を呼んで出さない。IMP-264）。それでも、Go へ知らせる前にドロップが処理され、その結果のイベントが後から届いた場合は、外部の更新と同じく控え、閉じた時点で当てる（**入力は失われない。FR-140 の「切り替えずに拒む」から外れるのは、この競合の窓だけであり、許容する**）。
- **拡大画面・右クリックメニュー・ダイアログの表示中もドロップを受け付ける**（UI-090）。ドロップで文書が切り替わると `document:opened` が届き、`renderDocument` の手順 0a（IMP-220）が拡大画面と右クリックメニューを閉じる。**ダイアログは閉じない。** このモジュールで個別に閉じない（経路を 1 つに保つ）。

> [!IMPORTANT]
> **オーバーレイの表示と、パスの受け取りは別の配線である。** 前者は本モジュールの `dragenter` / `dragover` / `dragleave` だけで完結し、後者は `OnFileDrop()` の呼び出しを要する。**後者だけを欠くと、ドラッグ中の案内は正しく出るのにドロップが無反応になる。** 見た目が動いている分、原因を取り違えやすい（[調査報告](../bugs/2026-09-04-bug-001-file-drop-windows.md)）。
>
> **Windows と Linux で仕組みが違う。**
>
> | OS | パスが Go へ届く経路 | JS 側の登録 |
> | --- | --- | --- |
> | **Windows**（WebView2） | Wails の JS が `drop` を捕まえ、File オブジェクトを `postMessageWithAdditionalObjects("file:drop:x:y", files)` で Go へ渡す。Go が `ICoreWebView2File` から絶対パスを取り出して `wails:file-drop` を発火する | **必須** |
> | **Linux**（WebKitGTK） | GTK の `drag-data-received` / `drag-drop` シグナルが直接 Go へ届く | 不要（あっても害はない） |
>
> **Go の `runtime.OnFileDrop`（IMP-313）は `wails:file-drop` を購読するだけである。** 「Go でコールバックを登録し、CSS で受け口を宣言すれば届く」という理解は Linux でしか成り立たない。

### IMP-246: ウィンドウ幅に応じた一時的な非表示 **MUST**

UI-026 / DSP-380 を実装する。**利用者の設定と、幅による一時的な非表示を別の変数で持つ**。この分離が UI-026 の要点である。

```js
// state.outlineVisible … 利用者の意思（設定に保存する。UI-110）
// outlineSuppressed    … 幅不足による一時的な非表示（保存しない。
//                         panes.js のモジュール変数。state に置かない。IMP-210）
// 実際の表示 = state.outlineVisible && !outlineSuppressed
```

- `resize` イベントで本文ペインの幅を算出し、240 px を下回るなら `outlineSuppressed = true`、回復したら `false` に戻す。
- **`state.outlineVisible` と設定値、ツールバーの `aria-pressed` は変更しない。** ここを変えてしまうと、ウィンドウを広げても戻らなくなり、設定にも誤った値が保存される。
- `resize` は連続して発火するため、`requestAnimationFrame` で 1 フレームにまとめる（NFR-012）。
- 本文の幅は「**アウトラインを出したとしたら何 px になるか**」で測る。隠れている間も同じ式で測ることで、抑制と復帰を行ったり来たりしない。
- 判定は `window.innerWidth - clamp(アウトライン幅) - clamp(ツリー幅) < 240` とする。**幅は利用者が決めた値**（UI-110）であり、上限（ウィンドウ幅の 40 %）で丸めたあとの実効値を使う。

> [!NOTE]
> **この判定は幅の組み合わせによっては一度も真にならない。** 両ペインが下限（160）にあるとき、最小ウィンドウ幅でも本文は 320 px あり、閾値 240 を上回る（[UI-026](03-ui.md) の IMPORTANT、[DSP-380](22-display-states.md) の表）。**それは不具合ではない。** 閾値やペイン幅の下限を動かして発火させることは行わない。
- ファイルツリーは抑制の対象としない（UI-026）。
- 抑制中に利用者がトグルボタンを押した場合は、`outlineVisible` を通常どおり切り替える。抑制が解けたときにその値が反映される。

### IMP-247: ツールチップ **MUST**

UI-024 / DSP-102 を実装する。

- **ブラウザ既定の `title` 属性を使わない。** DSP-102 はボタンの下 6px・遅延 400ms・反転色・折り返しなしという具体値を定めているが、`title` は見た目も遅延も表示位置も OS が決めており、いずれも満たせない。英日併記（`Open / 開く (Ctrl+O)`）で横に長くなるため、折り返さないことが特に効く。
- 対象は**ツールバーのボタン**と、**v1.1.0 で足した本文中・拡大画面のボタン**（図と画像のボタン UI-053、並べ替えのボタン UI-054、拡大画面の操作バー UI-104）と、**v1.2.0 で足したファイルツリーペインの見出しのボタン（フィルターボタン）とフィルターの選択肢**（UI-024, UI-033）、**コードブロックと図の編集のボタン**（UI-053）とする。ツリー・アウトライン・ステータスの「全文をツールチップで示す」（DSP-113, DSP-151）は隠れた文字を読ませるためのものであり、`title` のままでよい。**コードブロックのコピーボタン（FR-060）には出さない**（UI-053）。
- **英日併記はツールバーのボタンと、ファイルツリーペインの見出しのボタン・フィルターの選択肢だけ**とし（4.73.0）、本文中と拡大画面のボタンは英語だけの文言を置く（UI-024）。書式の違いは文言（IMP-290）の側で表し、このモジュールは `data-tip` の文字列をそのまま出す。
- 文言は `data-tip` 属性に置く（`toolbar.js` の `setTip`）。同じ文字列を `aria-label` にも与える（IMP-295）。
- 描画する要素は `#tooltip` の 1 つだけを使い回す。ボタンごとに持たせない。
- 配線は `#app` への委譲で行い、`closest('[data-tip]')` で対象を求める。ボタンごとにリスナを置くと、文言が変わるボタン（テーマ。IMP-243）や、描画のたびに作り直すボタン（IMP-228, IMP-229）で付け替えが要る。
- **位置はボタンの下を基本とし、ウィンドウの下端から出る場合はボタンの上に出す**（DSP-102）。本文の最下部の図や、拡大画面の操作バーの近くでも読めるようにする。
- 位置はボタンの下・水平中央。**ウィンドウの外へ出る場合は左右に寄せて収める。** 左端の戻るボタンと右端の `?` は、中央に置くと枠外へ出る（4.81.0 で、v1.2.0 の並び（UI-020）に合わせて `Open` から改めた）。
- `pointer-events: none` を与える。ツールチップがポインタを受け取ると `pointerout` が発生して点滅する。
- 押下時・ウィンドウのフォーカス喪失時・キー操作時に消す。トグルの状態が変わると文言が古くなるため（IMP-243）。
- **フィルターボタンの文言は、表示している種類から組み立てる**（`S.tipFilter(['Markdown', 'Diagrams'])`。UI-024, UI-033）。切り替えたら `data-tip` と `aria-label` を書き換える（テーマのボタンと同じ。IMP-243）。

### IMP-248: ツリーのキーボード操作 **SHOULD**

UI-031 を実装する。

```js
// js/filetree.js
// initFileTree が #tree へ keydown を配線する（クリックと同じ 1 か所）
```

- **キー操作は `filetree.js` に置く**（展開・折りたたみ・開く処理と同じモジュール）。項目の要素の組み立てと、項目の中を読む関数は `treenodes.js` にある（IMP-200）。

- **キーは `#tree` の 1 か所で受ける。** ノードごとに購読すると、展開のたびに登録と解除が要る（クリックと同じ考え方）。
- **roving tabindex とする。** 各 `li.tree-item` が `tabindex` を持ち、**同時に `0` を持つのは 1 つだけ**。残りは `-1`。
- `0` を持つのは、**直前までフォーカスしていた項目**。無ければ**選択中の項目**（表示中の文書）、それも無ければ**先頭の項目**。ツリーを組み直したとき（`loadTreeRoot` / `expand` / `collapse`）に付け直す。
- **付け替えは `focusin` の 1 か所で行う。** クリック・`Tab`・キーでの移動のどれで来ても同じ規則になる。
- **`tabindex` は `li.tree-item` に置く**（`role="treeitem"` を持つ要素）。`.tree-row` に置かない——**フォーカスを受ける要素と役割を持つ要素を分けない**（IMP-295）。
- 切り詰めの行（`li.tree-more`。`role="none"`）は**フォーカスを受けない**。移動のときに飛ばす。

| キー | 動作 |
| --- | --- |
| `↑` / `↓` | **見えている項目**の間を移動する。折りたたまれた中は飛ばす |
| `→` | ディレクトリが折りたたまれていれば展開する。**それ以外では何もしない** |
| `←` | ディレクトリが展開されていれば折りたたむ。**それ以外では何もしない** |
| `Enter` | **クリックと同じ扱い**。ファイルなら開き、ディレクトリなら展開・折りたたみを切り替える |

- **`Shift+F10` とアプリケーションキーは、ここで扱わない。** WebView が `contextmenu` を発火させ、IMP-249 がツリーのメニュー（IMP-270）を開く（UI-031。4.73.0）。
- **修飾キー（`Alt` / `Ctrl` / `Meta` / `Shift`）が押されているときは何もしない。** `Alt+←` / `Alt+→` は履歴の移動である（UI-090, IMP-244）。**ここで奪うと戻れなくなる。**
- **上の表の 4 行すべてで `preventDefault()` する**（キーとしては 5 つ）。`↑` / `↓` は既定でページをスクロールする。**それ以外のキーは素通りさせる。**
- 移動したら `scrollIntoView({ block: "nearest" })` で可視にする。**`block` を省かない**——既定値では本文ペインまで動きうる。
- フォーカスリングは **`.tree-item:focus-visible > .tree-row`** に出す（DSP-330, DSP-016）。**`li` に出すと、展開中は配下の木全体が囲まれる。** `base.css` の `:focus-visible` が全要素にリングを引くため、**`li` の側は打ち消す**——**同じ体裁を行へ移すだけであり、DSP-016 の「フォーカスリングを消さない」に反しない。**

> [!IMPORTANT]
> **`Enter` でファイルを開いたら、フォーカスは本文ペインへ移る**（UI-051, IMP-220 の手順 11）。
> **ツリーのキーボード操作を足したとき（4.38.0）に決めた点である。** それまでは IMP-220 が判断を先送りしていた。
>
> **経路ごとに分けない。** ツリーから開いたときだけツリーに残す案もあるが、
> **契機ごとの分岐は DSP-350 と同じ形の表をもう 1 つ増やす。**
> さらにこの製品は閲覧に特化しており、**開いた直後にしたいことは読むことである。**
> フォーカスがツリーに残ると `PageUp` / `PageDown` が効かず、
> **利用者から見れば [BUG-007](../bugs/2026-09-05-bug-007-viewer-focus-on-open.md) と同じ症状になる。**
>
> **ツリーへは `Shift+Tab` で戻れる。** 新しいショートカットを増やさない（UI-090）。

> [!NOTE]
> **`←` で親へ移らない。** WAI-ARIA の木のパターンは「閉じた項目で `←` を押すと親へ移る」と
> 定めているが、**UI-031 は「`←` で折りたたみ」としか書いていない。要求を超えて実装しない。**
> 広げるなら、まず UI-031 を改訂する。
>
> **`Home` / `End` も足さない。** 同じ理由である（UI-090 の一覧にも無い）。

### IMP-249: 右クリックメニュー **MUST**

FR-063 / UI-085 / AR-060 / AR-062 / DSP-140 / DSP-141 を実装する。**ファイルツリーのメニュー（FR-036, UI-032）と、ツールバーの文字色の一覧（UI-020）も、このモジュールの器と開閉を使う**（4.75.0。項目と実行は IMP-270 と IMP-265）。

```js
// js/contextmenu.js — 場所の判定・メニューの組み立て・開閉
export function initContextMenu(deps)  // { copyText(text), readClipboard(), notify(error), format, treeMenu }
export function isContextMenuOpen()    // 右クリックメニュー・ツリーのメニュー・文字色の一覧のどれかが開いているか
export function closeContextMenu()     // フォーカスを戻して閉じ（サブメニューも）、true を返す。開いていなければ何もせず false
export function closeSubmenu()         // Text color のサブメニューだけを閉じて Text color へフォーカスを戻し、true。開いていなければ false（4.93.0）
export function openColorList(button)  // ツールバーの文字色のボタンの下に色の一覧を出す（UI-020, UI-085。4.75.0）

// js/menuactions.js — 項目の実行（4.70.0。IMP-011 の 400 行の目安で分けた）
export function hasAction(action)              // 実行できる項目か
export function runAction(action, current, deps)  // 項目を実行する
export function restoreRange(current)          // 控えた選択範囲を当て直す（BUG-019）

// js/menuparts.js — 項目の組み立てと置き場所（4.93.0。IMP-011 の 400 行の目安で分けた。状態を持たない）
export function buildItems(current, deps)      // 場所ごとの項目（下の表）。current.formatColor にサブメニューの印の色を控える
export function colorItems(colors, checked)    // 色の一覧（7 色 → 区切り → Default）
export function placeAt(root, point) / placeBeside(sub, item, main) / placeBelow(root, button)  // 位置（DSP-140, DSP-141）
export function linkTargetOf(anchor) / selectionIn(place) / keyboardPoint(current)            // リンク先・選択範囲・キーボードから開いたときの位置
```

- **`menuactions.js` は「押されたときに何をするか」だけを持つ**（4.70.0）。場所の判定・メニューの組み立て・開閉・フォーカスの扱いは `contextmenu.js` が持つ。**Go は直接呼ばない**——クリップボードへの格納は `deps`（`contextmenu.js` が受け取ったもの）を引数で渡す。

- `deps.notify(error)` の `error` は `ErrorDTO`（IMP-307）の形とする。Go を経由しない失敗は `{ kind: 'clipboard' }` のように `kind` だけを持つ値で渡す。文言は `strings.js` の `errorText` が `kind` から選ぶ（IMP-290, IMP-315）。**IMP-261〜IMP-265 と IMP-270 の `notify` も同じ形とする**（フロントエンドだけで決める `edit-busy` / `edit-held` / `edit-removed` / `code-locked` / `encoding-editing` も、`kind` だけの値で渡す。**`removed-editing` は、控えた `document:removed` の `ErrorDTO` の `kind` を `removed-editing` に替えて渡す**——文言にパスが要る。4.76.0）。
- `deps.format` は IMP-265 の修飾の操作（状態の読み取りと実行）、`deps.treeMenu` は IMP-270 のツリーのメニューの項目と実行（`main.js` が配線する。`contextmenu.js` は `format.js` / `treemenu.js` / `editmode.js` を import しない）。**v1.1.0 の `deps.cancelCellEdit` は外した**——メニューの外のクリックで閉じても、編集欄を取り消さなくなった（UI-085, FR-142。4.73.0）。
- `deps.copyText` は `api.js` の `copyToClipboard`（成功なら `null`、失敗なら `ErrorDTO`）、`deps.readClipboard` は `api.js` の `readClipboard`（`{ text, error }`。失敗の `error` は `kind` が `paste` の `ErrorDTO`）とする（IMP-310）。
- **`contextmenu.js` は `viewer.js` / `overlay.js` / `main.js` / `navigate.js` / `docswitch.js` / `editmode.js` を import しない**（IMP-250）。拡大画面の表示中かどうかは `expand.js` の `isExpandOpen` を、書かれたとおりのリンク先は `refs.js` の `ownLinkTarget` を直接 import して聞く。`viewer.js`（IMP-220 の手順 0a）・`docswitch.js`（IMP-250）・`shortcuts.js`（IMP-244）・`main.js` がこのモジュールを import する。

**`contextmenu` を常に止める**（AR-060）

- `document` に `contextmenu` のリスナを 1 つだけ置き、**どの場所でも `preventDefault()` する。** 開発ビルドでは Wails が標準のメニューを出すため（IMP-193）、止めないとリリースビルドと振る舞いが変わる。
- そのうえで、場所が次の表に当たるときだけメニューを出す。**判定は上から順に行い、最初に当たったものを採る**（入力欄は本文ペインの中にもあるため。FR-063）。

| # | 判定（`event.target.closest(...)`） | 場所 | 項目 |
| --- | --- | --- | --- |
| 1 | `input.search-input, input.cell-editor, textarea.code-editor` | 入力欄（コードブロックの編集欄は 4.75.0） | `Cut` / `Copy` / `Paste` / `Select all` |
| 2 | `.about-licenses` | ライセンス欄 | `Copy` / `Select all` |
| 3 | `#searchbar, #viewer button, #treefilter` | 検索バーの入力欄以外の部分、本文中のボタン（コピー・図と画像・並べ替え・編集・編集欄の `Save` / `Cancel`）、フィルターの枚 | 出さない |
| 4 | `#viewer` | 本文ペイン | （リンクの上なら `Copy link address` / 区切り）/ **編集モードなら修飾の項目（`Strikethrough` / `Text color ▸`。使えなければ淡色）/ 区切り** / `Copy` / `Select all`（区切りは UI-085, DSP-141。4.76.0） |
| 5 | `#tree .tree-item, #tree-root-name` | ファイルツリーの項目とツリールートの名前の行（4.75.0。FR-036） | IMP-270 の項目（ファイル・ディレクトリ・ツリールートの名前の行で違う） |
| — | 上のいずれでもない、または拡大画面の中 | — | 出さない |

- **拡大画面の表示中は、どこでも出さない**（UI-104）。
- **場所の判定に `event.defaultPrevented` を使わない。** 本番ビルドの Wails のランタイムは、編集できない要素の上などで先に `preventDefault()` を呼ぶことがある（Wails v2.15.0 の `runtime/desktop/main.js` の `processDefaultContextMenu`）。

**項目の可否**（FR-063）

| 項目 | 使える条件 |
| --- | --- |
| `Copy`（本文・ライセンス欄） | `getSelection().toString()` が空でなく、選択範囲（`getRangeAt(0).commonAncestorContainer`）がその場所の中にある。**その場所の外にかかる選択範囲では使えない** |
| `Copy` / `Cut`（入力欄） | `selectionStart !== selectionEnd` |
| `Paste` | **クリップボードにテキストがあるとき**（FR-063）。判定は下記の「`Paste` の可否」 |
| `Select all` | 常に選べる |
| `Strikethrough` / `Text color`（4.75.0） | **行ごと出すかどうか**は、編集モード（`state.editMode`）であること（FR-063。図のファイルでも出す）。**使えるか**は IMP-265 の `canFormat()`（修飾の目印付きの描画で、選択が空でなく、修飾できる 1 つのブロックに収まり、入力欄の外）。印（全体が取り消し線・全体が同じ色）は IMP-265 の `selectionState()` |
| `Copy link address` | 行ごと出すかどうかの判定。`closest('a')` があり、下の「リンク先の求め方」で文字列が求まるとき。**鍵の合う `data-link` も `href` も持たない `<a>`（`<a name>` など）はリンクではないため出さない** |

**`Paste` の可否**（FR-063）

- **入力欄で開くときだけ**、`Paste` を使えない状態で出してから `await deps.readClipboard()`（Go 側の `ReadClipboard`。IMP-310）を呼ぶ。本文ペインとライセンス欄では呼ばない（`Paste` が無い）。
- `text` が空でない文字列なら、`Paste` を使える状態にし、**その文字列を控える。** 空文字なら使えないままとする（テキスト以外しか入っていないことは失敗ではない）。読み取りに失敗したら（`error` がある）使えないままとし、`deps.notify(error)` を呼ぶ（`kind` は `paste`。IMP-315）。
- **応答が届く前にメニューが閉じていたら何もしない。**
- **`Paste` を選んだら、控えた文字列を入れる。もう一度読まない。** メニューはウィンドウがフォーカスを失うと閉じる（FR-063）ため、開いている間に他のアプリケーションでクリップボードが変わることはない。
- 使える状態へ変えるときにフォーカスを動かさない（利用者が既に `↓` で移っている場合がある）。

**リンク先の求め方**（FR-063）

- IMP-260 の `ownLinkTarget(a)` が文字列を返せば、それを使う（`data-link` の鍵が合うとき、鍵の後ろの文字列。IMP-120）。**鍵をこのモジュールで読まない。**
- `null` なら（生 HTML の `<a>`、鍵の合わない目印、図の中のリンク）`util.js` の `linkHref(anchor)` の値（HTML の `<a>` は `href`、SVG の `<a>` は `xlink:href`。IMP-223）を使う。**`a.href`（解決済みの URL）を使わない。**
- 格納は `deps.copyText(text)`（Go 側の `CopyToClipboard`。IMP-310）で行う。

**選択範囲を保つ**（FR-063, UI-085）

- **メニューの項目は `pointerdown` で `preventDefault()` する。** 項目を押した瞬間に本文の選択範囲が外れるのを防ぐ。**`mousedown` でも止める**——`pointerdown` を止めたときに `mousedown` の既定の動作（フォーカスの移動と選択の開始）まで止まるかをエンジンに委ねない（NFR-061）。`click` はどちらを止めても届く。
- 開くときに、開く前のフォーカス要素と、入力欄なら `selectionStart` / `selectionEnd` を控える。
- **開く前に本文の選択範囲（`Range`）を控え、開いたらメニューの最初の使える項目へ `focus({ preventScroll: true })` し、その直後に控えた `Range` を当て直す**（4.70.0。[BUG-019](../bugs/2026-09-21-bug-019-context-menu-copy-webkitgtk.md)）。**「選択範囲はフォーカスの移動では消えない」と考えてはならない**——**WebKit は要素へフォーカスを移すと文書の選択範囲を解除する。**当て直さないと、右クリックした時点で選択が画面から消え、`Copy` が空のままになる（NFR-061）。
  - 控えるのは**項目の可否を決めるより前**とし、`Copy` の可否もその控えで判断する
  - **控えた位置が DOM から外れていたら当て直さない**（再描画・文書の切り替え）
  - 入力欄の `start` / `end` を控えているのと同じ扱いである。**片方だけ用心しない**
  - **当て直しは、フォーカスを移すすべての場所で行う**（4.70.0）。**`closeContextMenu()` の中（フォーカスを戻した直後）に置く**——`Esc`・スクロール・`blur`・`resize`・項目の実行・開き直しの**6 つの経路がここを通る**。呼び出し側ごとに書くと、1 か所でも忘れたときに選択が消える。**項目の間の移動（`↑` / `↓`）でも当て直す**
  - **位置を決めるときも控えた `Range` を使う**（`keyboardPoint`。UI-085）。選択を引き直すと、開き直しの経路では既に解除されていることがある
  - **当て直せないことは失敗ではない。** 要素が残っていても文字が短くなっていれば例外を投げるため、握りつぶして当て直さないだけにする（`open()` の途中で投げると、メニューを出したまま残りの処理が飛ぶ）

**実行**

| 項目 | 処理 |
| --- | --- |
| `Copy`（本文・ライセンス欄） | **`document.execCommand('copy')`** を呼ぶ。WebView 自身のコピー処理であり、`Ctrl+C` と同じ内容（書式付き）が入る（FR-063, AR-062） |
| `Copy` / `Cut`（入力欄） | 控えた入力欄へフォーカスを戻し、控えた選択範囲を `setSelectionRange` で当て直してから `execCommand('copy')` / `execCommand('cut')` |
| `Paste` | 上の「`Paste` の可否」で控えた文字列を使う。入力欄へフォーカスを戻し、控えた範囲を `setRangeText(text, start, end, 'end')` で置き換え、`input` イベントを発火する（検索はインクリメンタルなため。IMP-241）。**セルの編集欄では改行を半角空白へ置き換えてから入れる**（FR-142）。**コードブロックの編集欄では改行をそのまま入れる**（FR-145） |
| `Strikethrough` / `Text color` の色 | **控えた選択範囲を当て直してから**、IMP-265 の `toggleStrike()` / `applyColor(name)` を呼ぶ（BUG-019 と同じ理由） |
| `Select all`（本文） | 本文ペインが文書を表示していれば `#markdown`、状態画面なら `#state-screen` を `getSelection().selectAllChildren` で選ぶ |
| `Select all`（ライセンス欄） | `.about-licenses` を `selectAllChildren` で選ぶ |
| `Select all`（入力欄） | 控えた入力欄へフォーカスを戻して `select()` |
| `Copy link address` | 上で求めた文字列を `deps.copyText` |

- **項目を実行するときは、先に `closeContextMenu()` でメニューを閉じてフォーカスを開く前の位置へ戻し、それから上の表の処理を行う。****本文の `Copy` は、その直前にもう一度、控えた `Range` を当て直してからコピーする**（4.70.0。BUG-019）——**フォーカスを戻す処理そのものが、選択範囲を解除するエンジンがある。**
- **`execCommand('copy')` が `false` を返した場合は、選択範囲の `toString()`（入力欄では控えた範囲の文字列）を `deps.copyText` で格納する。** 書式は失われるが、コピーそのものが失敗するよりよい。**入力欄の `Cut` で `execCommand('cut')` が `false` を返した場合も、同じく格納し、格納できたら控えた範囲を取り除いて `input` イベントを発火する**（`Paste` と同じ置き換え）。**これは本来の経路ではない**——WebView2 と WebKitGTK の両方で `true` が返ることを実機で確かめる（AR-062, NFR-061）。**格納する文字列が空のときは `deps.copyText` を呼ばない**（4.70.0。BUG-019）——**空文字を渡すと利用者のクリップボードの中身を消す。****選択が空なら `execCommand` も呼ばない**（当て直せなかったとき）——**空のまま実行すると、エンジンによっては成功を返し、同じようにクリップボードを消す。**
- **格納に失敗したら `deps.notify({ kind: 'clipboard' })` を、読み取りに失敗したら `deps.notify({ kind: 'paste' })` を呼ぶ**（IMP-315）。`deps.copyText` が `ErrorDTO` を返したときは、それをそのまま渡す。
- **`Copy` を Go 側の `CopyToClipboard` で実装しない。** Go 側のクリップボード機能はプレーンテキストしか扱わず、`Ctrl+C`（書式付き）と結果が変わる（FR-063, AR-062）。

**表示と閉じる契機**（UI-085）

- `#contextmenu` の中に `<button class="contextmenu-item" role="menuitem">` を並べ、区切りは `<div class="contextmenu-separator" role="separator">` とする。使えない項目は `disabled` と `aria-disabled="true"` を与える。文言は `strings.js`（IMP-290）。
- 位置は `event.clientX` / `clientY`。**キーボードから開いた場合**（`Shift+F10` とアプリケーションキー。WebView は座標を 0 にすることがある）は、選択範囲の矩形、無ければフォーカス要素の矩形の左下に置く。**ウィンドウの内側に収まるよう、右端と下端で折り返す**（DSP-140）。
  - **キーボードから開いたかは、座標ではなく、直前の右ボタンの `pointerdown` の有無で見分ける。** `window` の `pointerdown`（キャプチャ）で `button === 2` なら印を付け、`contextmenu` と `keydown` で外す。印が無いまま届いた `contextmenu` をキーボードからとみなす。座標は 0 になるとは限らない（エンジンが選択範囲の近くの座標を入れることがある）。
  - 入力欄では文書の選択範囲を見ず、入力欄の矩形を使う（入力欄の外に残る選択範囲の近くへ出てしまう）。選択範囲がその場所の中に無いときも、フォーカス要素（`body` なら場所の要素）の矩形を使う。
  - **幅は前の位置のまま測ってよい。** 項目は折り返さない（DSP-140 の `white-space: nowrap`）ため、最小の内容幅が文言の幅になり、ウィンドウの右端の近くに置いてあっても幅は縮まない。
- `↑` / `↓` は使える項目の間を巡回し、`Enter` / `Space` で実行する。`Tab` / `Shift+Tab` は `preventDefault()` して何もしない（メニューの外へ出さない）。**これらは閉じる契機ではない**（UI-085）。**これらのキーは `#contextmenu` の `keydown` で受ける**（IMP-244 はメニューの表示中、`Esc` 以外を扱わない）。**扱ったキーは `preventDefault()` に加えて `stopPropagation()` する**——`Enter` で項目を実行するとメニューが閉じるため、`window` の `keydown`（IMP-244）から見ると「メニューは開いていない」になり、`Enter` が検索の移動（`searchNext`）へ回る。
- 閉じる契機は、項目の実行、`Esc`（IMP-244 の振り分け）、メニューの外での `pointerdown`、`#viewer` と `.about-licenses` の `scroll`（キャプチャで拾う）、`window` の `blur` と `resize` とする（FR-063）。**ただしマウスのサイドボタン（`button` が `3` / `4`）の `pointerdown` では閉じない**（4.79.0）。メニューの表示中、サイドボタンは何もしない操作である（FR-051, UI-090）。ここで閉じると、続く `mouseup` の時点ではメニューが閉じているため、IMP-244 がそのまま戻る・進むを行ってしまう（E2E-266 の確認内容 7）。
- **メニューの外の `pointerdown` 以外の契機（項目の実行・`Esc`・`scroll`・`blur`・`resize`）で閉じたときは、控えたフォーカス要素へ `focus({ preventScroll: true })` で戻す。** **戻してから項目を取り除く**（フォーカスのある項目を先に取り除くと、フォーカスが一度 `body` へ落ちる）。開く前のフォーカスが `body` だったときは戻さない。
- **メニューの上の `contextmenu`**（メニューにフォーカスがあるときの `Shift+F10` など）は `preventDefault()` だけを行い、閉じも開き直しもしない。**メニューを開いたまま別の場所で右クリックした場合**は、先に届く `pointerdown` で閉じ（戻さない）、続く `contextmenu` で開き直す。 `blur` で閉じた場合もその場で戻しておく（ウィンドウへ戻ったとき、そこにフォーカスがある）。
- **メニューの外の `pointerdown` で閉じたときは戻さない**（フォーカスは押した先へ移る。UI-085）。**`preventDefault()` しない**——押した操作はそのまま働く（ツールバーの戻る・進むのボタンなど。FR-051）。**編集欄から開いていても、編集欄は開いたまま残す**（FR-142, FR-145。4.73.0。v1.1.0 はセルの編集欄を取り消していた）。
- **編集欄から開いていたときは、本文のスクロールとウィンドウのフォーカス喪失で閉じたときも、編集欄へフォーカスを戻す**（ウィンドウの場合は戻ったとき。FR-142, UI-085）。
- **情報ダイアログのフォーカストラップ（IMP-251）は、`#contextmenu` へのフォーカスを外に出たとみなさない。** ライセンス欄でメニューを開くと、ダイアログの外の要素へフォーカスが移るためである。
**サブメニューと文字色の一覧**（UI-085, DSP-141。4.75.0）

- **`Text color` はサブメニュー（`#contextsubmenu`）を持つ。** 項目の右に矢印（`icon-chevron-right`）を出し、`→`・`Enter`・マウスを乗せることで、`Text color` の項目の右に 7 色と `Default` を出す（ウィンドウの右端を越えるなら左に出す）。開いたらフォーカスをサブメニューの最初の項目へ移す。`←` か `Esc`（IMP-244 の (1)）でサブメニューだけを閉じて `Text color` の項目へフォーカスを戻す。マウスが本メニューの別の項目へ移ったら閉じる。
- 各色の項目には、色の見本（`span.color-swatch.md-color-<名前>`。いまのテーマの色。DSP-141）と名前（`S.colorRed` など）を出す。**選択の全体が同じ色なら、その項目に印（`icon-check`、`aria-checked="true"`。`role="menuitemradio"`）を付ける**。`Strikethrough` は全体が取り消し線なら印を付ける（`role="menuitemcheckbox"`）。
- **ツールバーの文字色のボタン（`#btn-color`）は `openColorList(button)` を呼び、同じ一覧を `#contextmenu` に出す**（ボタンの下。右クリックメニューのサブメニューと同じ見た目と操作。UI-085）。**開いている一覧は、そのボタンをもう一度押すと閉じる**（4.93.0）——ボタンの `pointerdown` はメニューの外の `pointerdown` として一覧を閉じるため、続く `click` を 1 回だけ見送る。見送らないと、閉じた直後に開き直し、ボタンを押しても一覧が閉じない。**開く前に本文の選択範囲を控え**（BUG-019）、フォーカスを一覧へ移し、`↑` / `↓` / `Enter` で選ぶ。閉じる契機は本メニューと同じ。閉じたらフォーカスを開く前の位置（本文の選択を保つため、ボタンではなく開く前のフォーカス要素）へ戻す。**右クリックメニューと同時には開かない**（同じ要素を使う）。`#btn-color` の `aria-expanded` を開閉に合わせる。**開いている間は `#contextmenu` に `is-color-list` を付け、閉じたら外す**（幅の最小値が右クリックメニューと違う。DSP-141。4.76.0）。
- **色の名前と値は IMP-265 の一覧から採る**（FR-147。文言は `strings.js`）。

## 12.6 状態画面とダイアログ（IMP-250 系）

### IMP-250: 状態画面 **MUST**

UI-052 を実装する。

```js
// js/overlay.js
export function showStateScreen(kind, params)
// kind: 'welcome' | 'confirm-large' | 'too-large' | 'render-error'
export function hideStateScreen()
```

```js
// js/docswitch.js
export function leaveDocument()   // 文書の切り替えと状態画面への移行の後始末（下記）
```

- `#state-screen` に描画し、`#markdown` は空にする。
- **状態画面へ移ることは文書の切り替えである**（1.7, DSP-352）。`#markdown` を空にする**前に**、`renderDocument` の手順 0a の「偽」の場合と同じ後始末を行う。**2 か所に書かず、`docswitch.js` の 1 つの関数（`leaveDocument()`）にまとめ、`viewer.js`（手順 0a）と `navigate.js`（状態画面を出す前）から呼ぶ。** `leaveDocument()` は次を行う。
  - 編集欄は開いていない（開いている間は文書の切り替えを拒み、状態画面への移行も控える。IMP-264。4.75.0）。右クリックメニューを閉じる（`closeContextMenu`。IMP-249）。検索を閉じる（IMP-241）。
  - 拡大画面を閉じる（`onDocumentSwitched`。IMP-253）。
  - 並べ替えの状態を空にする（`tablesort.js` の `clearSort`。IMP-229）。原寸表示の状態を空にする（IMP-228）。修飾の控え（当て直す選択と `Range`）を捨てる（`format.js` の `clearFormatState`。IMP-265, DSP-352。4.76.0）。
  - `state.editable` と `state.editMode` を偽にして画面へ写す（IMP-260 の `leaveEditMode()`。`state.editSeq` は変えない）。
- **`leaveDocument()` を `viewer.js` にも `overlay.js` にも置かない。** `viewer.js` は `overlay.js` を import しており（`hideStateScreen` ほか）、`overlay.js` から `viewer.js` の関数を呼ぶと循環参照になる（IMP-201）。**`docswitch.js` が import するモジュール（`editmode.js` / `contextmenu.js` / `search.js` / `expand.js` / `tablesort.js` / `media.js` / `format.js`）は、`viewer.js` / `overlay.js` / `main.js` / `navigate.js` / `docswitch.js` を import しない。**
- **v1.0.0 の `main.js` の `leaveDocument`（文書を離れる直前にスクロール位置を記録する。IMP-311）は `recordScroll` と呼び分ける**（`navigate.js` に置く）。同じ名前の関数が 2 つあると、呼ぶべきほうを取り違える。
- **状態画面を出すのは `navigate.js` の `enterStateScreen` だけとする**（`OpenResultDTO` / `LinkResultDTO` / イベントの `ErrorDTO` と、起動時。起動時は `main.js` がこれを呼ぶ）。順序は、`leaveDocument()` → `state.doc = null` と `state.target` の設定 → `showStateScreen(kind, params)` → `updateStatus()` とする。
- **状態画面の間も、ステータス領域の左に対象のファイルのパスを出す**（FR-016, UI-060, DSP-302）。`state.target` に `ErrorDTO` の `path` / `displayPath` / `outsideTree` / `targetKind`（IMP-307。`targetKind` は 4.76.0）を入れ、`status.js` の `updateStatus()` は **`state.doc ?? state.target`** のパスを出す（ツリー外なら絶対パスと `(outside tree)`。DSP-150）。**右の文字コード・行数は出さない**（描画していない）。`welcome` では `state.target` を `null` にする。**ツリーの強調（`filetree.js` の選択状態と、対象までの経路を開く処理。DSP-330）も同じく `state.doc ?? state.target` から決める**（DSP-302 の「対象ファイル」。ツリー外なら強調しない）。**v1.0.0 の `updateStatus()` とツリーの `markSelected` は `state.doc` しか見ておらず、状態画面の間も前の文書のパスと強調が残っていた**（[BUG-012](../bugs/2026-09-14-bug-012-state-screen-previous-document.md)）。
- `confirm-large` の `Open anyway` の押下で `api.openConfirmed(path)` を呼ぶ（IMP-310, IMP-314）。**呼ぶ前に、ボタンを `disabled` にして文言を `S.rendering`（`Rendering...`）に変える**（処理中であることを示す。二度押しも防ぐ。FR-016, UI-052, DSP-181。4.81.0。利用者の判断）。本文が出れば状態画面ごと消え、別の状態画面になれば描き直される。**応答で何も変わらなかったとき（`OpenResultDTO` の `document` も `error` も空。確認待ちでなかった等。IMP-314）は、元の文言に戻して押せるようにする。**
- **`welcome` の文言は、開くファイルの種類を名指ししない**（`S.welcomeTitle` / `S.welcomeHintDrop`。UI-052。4.75.0）。
- 文言は `strings.js` から取得する。

### IMP-251: 情報ダイアログ **MUST**

UI-100 を実装する。

```js
// js/overlay.js
export function initOverlay(deps)   // { onLink }。暗幕クリックと Tab の制御を配線する
export function showAbout(about)    // about: AboutDTO（IMP-306）
export function hideAbout()         // 閉じたら true、開いていなければ false
export function isAboutOpen()
```

- `#overlay` に描画する。**独立したウィンドウを開かない**（AR-060, UI-100）。
- **`main.js` は `api.getAbout()` の応答を受け取って `showAbout` を呼ぶ直前に、拡大画面が開いていれば（`isExpandOpen()`）出さない。** 応答を待つ間に拡大画面を開けるためである。両者は同じ層（DSP-015 の 60）にあり、DOM では `#expand-view` が `#overlay` より後ろにあるため、後から出したダイアログが拡大画面の下に隠れる。
- **Go を呼ぶのは `main.js` の役目とし、`overlay.js` は受け取った `AboutDTO` を描くだけにする**（IMP-201）。リンクの処理も `onLink` として受け取り、本文中のリンクとまったく同じ経路（IMP-312）へ渡す。**ダイアログの中身は `about.js` の `buildAboutDialog(about, handlers)` が組み立てる**（IMP-011。IMP-252 の `editors.js` と同じ形）。API はこの 4 つだけとし、`overlay.js` に置く。
- 表示中は背後をフォーカストラップし、`Tab` がダイアログ外へ出ないようにする。端に来たときだけ折り返し、途中では既定の移動に任せる。**ただし右クリックメニュー（`#contextmenu`。IMP-249）へのフォーカスは外に出たとみなさない。**
- **キーボードのショートカットも止める**（UI-100 の「背後のメインウィンドウの操作を受け付けない」）。暗幕はマウスしか塞がない。`Esc` だけは通す。止めるときも既定の動作は抑止する（`false` を返さない。IMP-244）。そうしないと `Ctrl` + `+` が WebView 自身のページ拡大として処理される。
- **ただし `Enter` / `Shift+Enter` は既定の動作を残す**（`false` を返す。IMP-244）。`Enter` の既定の動作は「フォーカスしている操作要素を実行する」であり、これは背後ではなく**ダイアログ自身の操作**である。抑止すると、`Open`（UI-103）や `Close` にフォーカスがあっても `Enter` で実行できない。**割り当て自体（検索の次候補へ移動）は止まる**ので、背後を受け付けないことと両立する。
- `Esc`、閉じるボタン、またはオーバーレイ**そのもの**のクリックで閉じる。中身のクリックで閉じないよう、対象が `#overlay` 自身であることを確かめる。
- 閉じたら**フォーカスを `#btn-about` へ戻す。** 開く前に別の操作要素へフォーカスがあった場合だけ、そこへ戻す。`F1` で開いたときの直前のフォーカスは `<body>` であることが多く、そのまま戻すとどこにもフォーカスがない状態になる。
- リポジトリの URL には `href` を与えず、クリックと `Enter` / `Space` で `onLink` を呼ぶ（UI-102）。**遷移し得ない形にしておく。**
- ライセンス欄だけがダイアログの残りの高さを受け持つ。基準は 240px とし、**伸びはせず、収まらないときだけ縮む**（DSP-170 の「ウィンドウが小さい場合は内側に収まるよう縮小」）。
- ライセンス全文は `<textarea readonly>` ではなく `<pre>` + `overflow:auto` で表示し、選択・コピーを可能にする（UI-101）。
- 見出しの横にアプリケーションアイコンを `<img src="/appicon.png" alt="">` として表示する（UI-025, DSP-171）。装飾目的のため `alt` は空とし、読み上げの対象にしない。パスは IMP-160 が配信するもので、外部 URL を参照しない。

### IMP-252: エディタ選択ダイアログ **MUST**

[UI-103](03-ui.md) を実装する。

```js
// js/overlay.js
export function showEditors(list, opener)   // list: EditorListDTO（IMP-309）。opener は閉じたときにフォーカスを戻す要素（4.75.0）
export function hideEditors()       // 閉じたら true、開いていなければ false
export function isEditorsOpen()
```

**API はこの 3 つだけとし、`overlay.js` に置く。** ダイアログの中身（行の組み立てと選択の状態）は `editors.js` が持つ（IMP-011。`overlay.js` が 400 行の目安を大きく超えるため）。`editors.js` は `overlay.js` を `import` しない。開閉・フォーカスの復帰・`Tab` の制御は `overlay.js` の側にあり、`Browse` / `Open` / `Cancel` の処理は引数として渡す。**循環参照を作らない。**

- **情報ダイアログ（IMP-251）と同じ `#overlay` を使い、同じ規則に従う。** フォーカストラップ、`Esc` だけを通すキー制御、暗幕そのもののクリックで閉じる判定のいずれも共通とする。**2 つを同時に開かない。**
- **Go を呼ぶのは `editorflow.js` の役目とし（4.93.0 で `main.js` から分けた。IMP-200）、`overlay.js` は受け取った `EditorListDTO` を描くだけにする**（IMP-201）。`Browse` と `Open` の処理、**`Open` 以外で閉じたときの処理（`onCloseEditors`。下の `api.closeEditor()`。4.93.0）**は `deps` として受け取る（`main.js` の `initOverlay` が配線する）。**`editorflow.js` は応答を受け取って `showEditors` を呼ぶ直前に `isExpandOpen()` を確かめ、真なら出さない**（IMP-251 と同じ理由）。
- **見出しの下に、開くファイルの名前（`EditorListDTO.fileName`）を `textContent` で出す**（UI-103, DSP-172。4.75.0）。**ウィンドウを開いた時点で Go が控えた対象の名前**であり、開いている間に文書が切り替わっても書き換えない（FR-091）。
- **ツールバー・`Ctrl+E` からは `api.listEditors()`、ファイルツリーの `Open in editor...` からは `api.listEditorsForTree(path)` で一覧を得る**（IMP-310, IMP-270）。どちらも同じ `showEditors` で描く。**Go を呼ぶのはどちらも `editorflow.js` である**（ツリーのメニューは `deps.openEditorsForTree(path, opener)` を呼ぶ。IMP-270。4.93.0）。`editors` が空なら出さない（ツリーに出していないもの（IMP-196）と、ツールバーから開いたときに対象が文字のファイルでないもの（IMP-310））。
- **`Cancel`・`Esc`・暗幕のクリックで閉じたら、`api.closeEditor()` を呼ぶ**（Go 側の控えを捨てる。IMP-310）。`Open` で閉じた場合は呼ばない（`OpenInEditor` が捨てる）。
- 一覧は `<input type="radio" name="editor">` のリストとする。`available` が偽の行は `disabled` とし、`(not installed)` を添える（UI-103）。**行を消さない。**
- 描画順は `EditorListDTO.editors` の順そのままとする。**フロントエンドで並べ替えない**（IMP-309）。
- 開いた時点のフォーカスは、`selected` の行があれば `Open` ボタン、無ければ一覧の先頭の選択可能な行に置く（UI-103）。
- `Open` は、選択が無い間 `disabled` とする。**`custom` の行そのものは常に選べる**（UI-103）が、`Browse` で実行ファイルが選ばれるまでは `Open` を `disabled` のままとする。行まで選べなくすると `Browse` へ辿り着けない。
- `Open` の活性は「選ばれていて、かつ `available` が真」という**1 つの式だけで決める。** 行の種類ごとに条件を書くと `custom` が別扱いになり、いつか食い違う。フロントエンドは実行ファイルのパスを見ない（NFR-035 の 3）。
- `Browse` は `deps.onBrowse()` を呼び、返ってきた `EditorListDTO` で**一覧全体を描き直す**。差分更新しない。行数も選択状態も Go 側が決める（IMP-309）。
- 閉じたら**フォーカスを開いたボタンへ戻す**（ツールバーなら `#btn-edit`、ツリーのメニューから開いたならメニューを開く前のツリーの項目。IMP-251 と同じ理由。UI-032）。
- `EditorListDTO.error` があるときは**ウィンドウを出さない。** 選べるものが 1 つも無いウィンドウを出しても意味がなく、理由はステータス領域へ出す（IMP-315）。`Browse` の戻りが同じ状態だった場合は**描き直さず、いま出ている一覧を保つ。** 描き直すと、出ていた一覧が消えて何も選べないウィンドウが残る。
- 表示・非表示は `hidden` 属性で切り替える。`style.display` を直接触らない（IMP-202）。
- **文言は `strings.js` から採る**（IMP-290）。エディタ名は `EditorDTO.name` をそのまま `textContent` で入れる。Go 側が組み立てた文字列であり、`innerHTML` へは渡さない（IMP-220）。

### IMP-253: 拡大画面 **MUST**

FR-122 / UI-104 / DSP-173 を実装する。

```js
// js/expand.js
export function initExpand(deps)                 // { focusViewer }
export function openExpand(target, key, opener)  // target: svg / img、key: IMP-228 の鍵、opener: 押したボタン
export function closeExpand()                    // 閉じたら true、開いていなければ false
export function isExpandOpen()
export function handleExpandKey(event)           // IMP-244 から。扱ったら true
export function onTargetSettled(key, target)     // IMP-228 から。対象の準備が済んだ（target が null なら失敗）。差し替えに使う
export function onImagesNumbered(count)          // IMP-228 の numberImages から。開いている画像の番号が count 以上なら閉じる
export function onAllDiagramsSettled()           // IMP-228 から。開いている図の知らせが届いていなければ閉じる
export function onDocumentSwitched()             // IMP-220 の手順 0a と IMP-250。閉じる
```

**器**

- `#expand-view` に描画する（IMP-202）。**`#overlay` を使わない**——`#overlay` は全面を覆い、拡大画面はステータス領域を覆わない（UI-104）。表示・非表示は `hidden` 属性で切り替える。
- **図の中のリンク（MD-081）は押しても何もしない**（UI-104）。既定の動作（WebView の中での遷移）は `viewer.js` が `#expand-view` に置くリスナが止める（IMP-223）。**`expand.js` にリンクの処理を置かない。** ドラッグの `setPointerCapture` により、押して離したクリックは舞台へ届くことが多いが、エンジンの振る舞いに頼らない（NFR-061）。
- 中身は、操作バー（`.expand-bar`）と、図を置く舞台（`.expand-stage`）と、その中の移動する器（`.expand-content`）とする。**画像ファイル（`state.doc.kind === 'image'`）の画像を開いたときは、`#expand-view` に `is-image-file` のクラスを付け、画像の範囲に市松模様を敷く**（FR-027, DSP-129, DSP-173。4.75.0）。
- **編集欄が開いたままでも開ける**（FR-140 の表の「保ったまま」）。開いている間は背後を操作できないため、閉じてから編集を続ける（FR-145）。操作バーのボタンは `.expand-fit` / `.expand-actual` / `.expand-zoom-out` / `.expand-zoom-in` / `.expand-close`、倍率の表示は `.expand-zoom-value` とする。
- **`expand.js` は `viewer.js` / `main.js` / `docswitch.js` / `shortcuts.js` / `zoom.js` を import しない**（いずれもこのモジュールを呼ぶ。`shortcuts.js` は `isExpandOpen` / `handleExpandKey` を、`zoom.js` は `isExpandOpen` を直接 import する）。本来の大きさのために `media.js` の `naturalSize` を import する（`media.js` は `expand.js` を import しない。拡大画面へは `main.js` が渡す `deps` で知らせる）。
- 操作バーのボタンは `data-tip` と同じ文字列の `aria-label` を持つ（IMP-247, IMP-295）。**キーの表記は操作バーのキーの表（`expandbar.js`。4.91.0 で `expand.js` から分けた）から組み立て、`strings.js` に書かない**（IMP-290 の考え方）。**キーの表には閉じるボタンの表記のために `Esc` も載せるが、`handleExpandKey` は `Esc` を処理しない**（下記）。

**対象を移す**

- **複製せず、本文の要素そのものを `.expand-content` へ移す。** 本文の元の位置には、移す前の大きさを持つ代わりの要素（`.expand-placeholder`）を置く。閉じたら要素を代わりの要素の位置へ戻す。
- **複製しない理由は 2 つある。** (1) Mermaid の SVG は `id` を持ち、中の `<style>` と矢印（`url(#…)`）がその `id` を参照する。複製すると `id` が 2 つになり、参照の解決先が定まらない。(2) 画像を複製すると、FR-120 の「再取得しない」を WebView のキャッシュの振る舞いに委ねることになる（NFR-061）。同じ URL の画像はページの中で使い回されることが多いが（[BUG-026](../bugs/2026-11-08-bug-026-stale-embedded-image.md)。`no-store` でも使い回す）、それを当てにしない（4.81.0 で理由を改めた。4.80.0 までは「`no-store` なので取り直しが起きうる」としていた——BUG-026 で覆った前提である。IMP-162）。
- 代わりの要素の大きさは、移す前の `getBoundingClientRect()` の幅と高さを CSS カスタムプロパティ（`--placeholder-width` / `--placeholder-height`）で与える。**本文のレイアウトを動かさない**（FR-122 の「本文のスクロール位置を保つ」）。
- **Mermaid の SVG のインラインの `max-width` は、移すときに控えて外し、戻すときに戻す**（IMP-228 と同じ扱い）。

**倍率と位置**

- 状態は `{ scale, x, y, fit }` をモジュール変数に持つ。`scale` は 0.1〜10（FR-122 の 10 %〜1000 %）。
- **倍率は CSS の `transform: scale()` で掛けない。`.expand-content` の幅と高さを「本来の大きさ × 倍率」にする**（CSS カスタムプロパティ `--expand-width` / `--expand-height`）。中の `svg` / `img` はその器いっぱいに描く（DSP-173）。**`transform` による拡大は、エンジンが描画済みのビットマップを引き伸ばすことがあり、SVG の輪郭がぼやけうる**（FR-122 の MUST）。**位置の移動にだけ `transform: translate()` を使う**（CSS カスタムプロパティ `--expand-x` / `--expand-y`。px）。 `will-change` を付けない（ビットマップとして固定されうる）。**両エンジンで輪郭がぼやけないことを実機で確かめる**（NFR-061）。
- 本来の大きさは IMP-228 の `naturalSize`（幅と、同じ規則の高さ。`height` 属性か `viewBox` の高さ、`naturalHeight`）とする。**本文から外す前に測る**——`naturalWidth` が 0 の画像は本文で描いている大きさを使うため、外した後では 0 になる（4.91.0。v1.1.0 は外した後に測り、0 なら外す前の大きさを使う処理を別に持っていた。規則を `naturalSize` の 1 か所にした）。
- **開いた時点の倍率（`Fit`）**は `min(舞台の幅 / 本来の幅, 舞台の高さ / 本来の高さ)` を 0.1〜10 に丸めたものとし、中央に置く（FR-122）。
- `+` / `-` とボタンは 1.25 倍ずつ、**画面の中心を基準に**変える。範囲の端では丸める（UI-104）。
- **範囲の端では、`-`（10 %）または `+`（1000 %）のボタンに `aria-disabled="true"` を与え、`disabled` 属性は付けない。** `disabled` にするとフォーカス中のボタンからフォーカスが外れ、操作バーの中を巡るフォーカス（下記）が壊れる。`aria-disabled` のボタンを押しても何もしない。見た目は DSP-173。
- **ホイールは、カーソルの下の点が動かないように倍率と位置を同時に変える**（FR-122）。1 回の `wheel` を 1 段とし、`deltaY` の符号で向きを決める。**タッチパッドの細かいイベントは、50 ms の間に届いたものを 1 段にまとめる。** `Ctrl` の有無を問わない。`preventDefault()` する（IMP-242 の本文の倍率を動かさない）。
- ドラッグは `pointerdown`（主ボタン）で `setPointerCapture` し、`pointermove` を `requestAnimationFrame` でまとめて位置を変える（NFR-012 の 60 fps）。**選択を始めない**（`user-select: none`。DSP-173）。
- 方向キーは 1 回 40px 移動する。**向きはスクロールと同じ**とする（`→` で右側が見える。中身は左へ動く）。ドラッグ（中身がポインタと一緒に動く）とは逆向きになる。
- **位置は、対象の少なくとも 48px が舞台の中に残る範囲に丸める**（UI-104 の「画面の外へ完全に出ない」）。
- `0` は倍率 1、`F` は開いた時点の規則で決め直す。`fit` は `F` と `Fit` で真、それ以外の倍率の操作で偽にする。
- `resize` のとき、`fit` が真なら決め直し、偽なら舞台の中心に見えている点を保つ（UI-104）。
- 倍率は `.expand-zoom-value` に整数の百分率で出す（`S.expandZoom`）。

**開閉とフォーカス**

- 開いたら、操作バーの最初のボタンへ `focus({ preventScroll: true })` する。`Tab` / `Shift+Tab` は操作バーの中だけを巡る（IMP-251 のフォーカストラップと同じ形）。
- `handleExpandKey` は UI-104 の表のキーを扱う。**`Esc` は扱わない**（閉じるのは IMP-244 の振り分けの (2)。IMP-244 は `Esc` を先に振り分けてから、残りのキーをここへ渡す）。**`Enter` / `Space` はフォーカスしているボタンの既定の実行に任せる**（`false` を返す。IMP-251 と同じ理由）。
- 閉じたら、要素を戻してから、**`opener` がまだ DOM にあればそこへ、無ければ同じ鍵のボタン（IMP-228）へ、それも無ければ `deps.focusViewer()` で本文ペインへ**フォーカスを戻す（FR-122）。
  - **`opener` と同じ鍵のボタンへは `focus({ preventScroll: true })` で戻す。** 再描画の後のボタンは画面の外にありうるため、`focus()` だけでは本文がスクロールして、FR-122 の「本文のスクロール位置を開く前のまま保つ」と DSP-350 に反する。
  - **閉じた状態にしてから戻す**（`focusViewer` は拡大画面の表示中は何もしない。IMP-220）。
  - **`onDocumentSwitched` で閉じるときはフォーカスを戻さない**（新しい文書の IMP-220 の手順 11 が移す）。
- **開いている間は背後の操作を受け付けない**（UI-100 と同じ）。`#expand-view` がツールバーと本文を覆うため、マウスは届かない。キーは IMP-244 が止める。
- **ステータス領域は覆わないが、操作も受け付けない**（UI-104）。開いている間は `#statusbar` に `inert` 属性を与え、閉じたら外す。**表示の更新（通知）は止まらない。**

**再描画と文書の切り替え**（FR-122, DSP-352）

- **同じ文書の再描画**（IMP-220 の手順 0a で `sameDocument` が真）では閉じない。本文が差し替わると、移していた要素の元の位置（代わりの要素）が消える。**移していた要素は拡大画面に残したまま、同じ鍵の対象を待つ。**
- `onTargetSettled(key, target)` で**開いている鍵と同じ鍵**が届いたら、`target` が図か読み込めた画像なら新しい要素と差し替え（古い要素は捨てる）、`scale` / `x` / `y` を保つ。**描画に失敗した・読み込めなかった知らせなら閉じる**（FR-120 の「描画が終わってから判断する」）。
- **「開いている鍵の知らせがまだ届いていない」は、`onImagesNumbered` を受けた時点から数える。** 同じ文書の再描画は必ず手順 5a（`numberImages`）を通るため、これを再描画が始まった合図とし、以後に同じ鍵の `onTargetSettled` が来たかを控える。開く前の描画の遅れた `onAllDiagramsSettled`（開いた時点で他の図がまだ描画中だった）では閉じない。
- **同じ鍵の対象が文書に無い**（再描画後の図や画像の数が減った）ことは、図なら `onAllDiagramsSettled`（IMP-228, IMP-230）を受けた時点で、開いている鍵の知らせがまだ届いていないことから分かる。画像なら `onImagesNumbered(count)`（IMP-228 の `numberImages`。IMP-220 の手順 5a）で、開いている番号が `count` 以上であることから分かる。そこで閉じる。**画像はあっても読み込みを待つ**（成否は `onTargetSettled` で届く）。
- 差し替えた後に閉じた場合、新しい要素を新しい代わりの要素の位置へ戻す。**差し替えの前に閉じた場合は、古い要素を戻さず捨てる**（元の位置はもう無い）。
- **文書の切り替え**（`onDocumentSwitched`）では閉じる。移していた要素は捨てる。

## 12.7 編集モード（IMP-260 系）

FR-140〜FR-147 / UI-055 のフロントエンド側。**判断と書き込みは Go 側が持つ**（IMP-195）。ここにあるのは、指示を送ることと、見た目を先に変えて結果で整えることだけである。

### IMP-260: 編集モードの切り替えと目印の照合 **MUST**

```js
// js/editmode.js
export function initEditMode(deps)       // { api, notify, focusViewer }
export function toggleEditMode()         // ボタンと Ctrl+Shift+M の共通の入口。何もしなければ false
export function applyEditMode(doc)       // renderDocument の手順 6d
export function leaveEditMode()          // editable と editMode を偽にして画面へ写す（leaveDocument と document:removed）
export function isEditableCell(element)  // 編集モード中で、element が鍵の合うセルの中にあるか（IMP-223）
export function isEditableBlock(element) // 編集モード中で、element が鍵の合う図のブロックの中にあるか（IMP-223。4.75.0）

// js/refs.js — 目印の照合（IMP-120）。state.js 以外を import しない
export function isOwnRef(element, kind)  // data-ref の鍵が state.doc.refKey と合い、種類が kind か（task / table / cell / mermaid / plantuml / code / text。4.75.0 で code と text を足した）
export function ownLinkTarget(anchor)    // data-link の鍵が合えば書かれたとおりのリンク先、合わなければ null（IMP-249）
export function ownSource(block)         // 鍵の合う図のブロック（mermaid / plantuml）なら data-source、それ以外は null（IMP-221, IMP-230）
export function ownRef(element, kind)    // isOwnRef が真なら data-ref の値そのもの、偽なら null（IMP-261, IMP-262, IMP-264, IMP-265 が Go へ渡す。IMP-316）
```

- **目印の照合は `refs.js` の `isOwnRef` / `ownLinkTarget` / `ownSource` / `ownRef` の 4 関数に閉じ込め、鍵を読むのはこのモジュールだけとする。** **`data-ref` の値を Go へ渡す箇所（チェックボックス・セル・コードブロックと図・修飾するブロック）も、`getAttribute('data-ref')` を自分で読まずに `ownRef` を通す**——照合を通ったものだけが渡り、`data-ref` を読む箇所が `refs.js` だけに保たれる（`grep` で確かめられる）。 並べ替え（IMP-229）、右クリックメニュー（IMP-249）、リンク捕捉（IMP-223 の `isEditableCell` 経由）、チェックボックスとセル（IMP-261, IMP-262）、コードブロックと図の編集欄（IMP-264）、修飾（IMP-265）、図の描画（IMP-230）、コピー（IMP-221）、図と画像のボタン（IMP-228）もこれを使う。**鍵を読む箇所を増やさない**——1 か所でも照合を忘れると、生 HTML の要素が編集・並べ替え・描画・コピーの対象になる（FR-141, FR-130, NFR-030, [BUG-014](../bugs/2026-09-14-bug-014-diagram-marker-spoofing.md)）。
- **照合を `editmode.js` ではなく葉のモジュール（`refs.js`）に置くのは、`lazy.js` / `copy.js` / `media.js` からも使うためである。** これらが `editmode.js` を import すると、`editmode.js` の依存を通じて循環しうる（`viewer.js` → `lazy.js` → `editmode.js` → …）。**`refs.js` は `state.js` 以外を import しない。**
- `data-ref` の値は `<鍵>:<種類>:<番号…>` とし、種類と番号の形は IMP-120 の表のとおりとする。**鍵と種類の両方が合うときだけ真とする。** `ownSource` は `data-mermaid` / `data-plantuml` の有無ではなく、鍵の合う `mermaid` / `plantuml` の目印で判断する。
- `toggleEditMode()` は、`state.editable` が偽なら何もせず `false` を返す（UI-021 のボタンは淡色）。**編集モードを終える側で、編集欄が開いていれば IMP-264 の `refuseIfEditing()` で拒む**（FR-140）。真なら `api.setEditMode(!state.editMode)` を呼び、返った `EditModeDTO.on` を `state.editMode` へ入れて画面へ写す。**呼んだら応答を待たずに `true` を返す**（IMP-244 は戻り値が `false` でなければ既定の動作を止める。`Promise` を返すと、開始できない状態の `false` と区別できない）。**呼ぶ前に見た目を変えない**——開始できなかった場合に戻す処理が要り、得るものが無い。
- `applyEditMode(doc)` は `doc.editable` / `doc.editMode` を `state` へ写し、次を行う。**Go 側の値を正とし、フロントエンドが持っていた値で上書きしない**（IMP-302）。**ただし `doc.editSeq` が `state.editSeq` より小さければ写さない**（到着順が入れ替わった古い値。IMP-302）。写すときは `state.editSeq` も更新する。**版が古くて値を写さなかったときも、画面への反映（下の箇条）は行う**——本文は差し替わっており、チェックボックスの有効化とセルの印を新しい本文に付け直す必要がある。`toggleEditMode` の結果（`EditModeDTO.seq`）も同じ規則で写す。**さらに、`state.editable` が偽のときは、`on` が真の `EditModeDTO` を写さない。** 状態画面への移行と `document:removed` は版番号を運ばない（`EditSeq` は `DocumentDTO` と `EditModeDTO` にしか無い。IMP-302, IMP-316）ため、その後に遅れて届いた `SetEditMode(true)` の結果（版は手元より大きい）を写すと、**状態画面や削除の後に枠とラベルとボタンの ON が出る**（FR-140, DSP-320）。次に編集できる文書が届けば、`DocumentDTO` の版で揃う。
  - `#btn-editmode` の `aria-pressed` と、`state.editable` が偽のときの `disabled`（UI-021）
  - `#viewer-frame` の `is-editing` クラスと、`#editmode-badge` の `hidden`（UI-055, DSP-126）。**ラベルをスクロールバーの内側に置くため、`#viewer` の `offsetWidth - clientWidth` を `--viewer-scrollbar-width` として `#viewer-frame` に与える**（ウィンドウ幅の変化でも測り直す。DSP-126）
  - チェックボックスの有効化（IMP-261）と、セルの印（IMP-262）、**コードブロックと図の編集のボタン（IMP-264）、ツールバーの修飾の群と修飾の操作（IMP-265）**（4.75.0）
  - **ボタン・枠・ラベル・チェックボックス・セルの印は 1 つの関数（`render`）で同時に切り替え、`applyEditMode` / `leaveEditMode` / `EditModeDTO` を写すときのすべてがそこを通る**（DSP-320）。
  - **編集欄が開いている間は、編集モードを終える表示へ移らない**（FR-140。4.75.0）。利用者の操作で終える（ボタン・`Ctrl+Shift+M`）ことは拒み（IMP-264）、自動で終える知らせ（`document:removed`、不正なバイト列を含んでいた再描画）は IMP-264 が控えて、**編集欄を閉じた時点で当てる。** v1.1.0 はここでセルの編集欄を取り消していた（FR-142 の改訂で、入力を捨てなくなった）。**閉じた時点で編集欄にフォーカスがあれば `deps.focusViewer()` で本文ペインへ移す**（`body` に落とすと `PageUp` / `PageDown` が効かなくなる。BUG-007 と同じ症状）。
- `document:removed`（IMP-320）を受けたら `state.editMode` と `state.editable` を偽にして画面へ写す（`main.js` の購読が `leaveEditMode()` を呼ぶ。Go 側は既に終えており、次に読み込めるまで開始させない。IMP-195, FR-140 の表）。**編集欄が開いていれば、IMP-264 が控え、閉じた時点で呼ぶ**（入力を残す。FR-140）。
- **編集モードを始めたら、Go 側が修飾の目印付きで描き直した `document:changed` が届く**（`trigger` が `reload`、`textRefs` が真。Markdown のときだけ。IMP-195）。それまでは `state.doc.textRefs` が偽であり、修飾の操作は使えない（IMP-265）。**終えたときは描き直さない**——残った目印は、編集モードでない間は使われない。
- **状態画面（`showStateScreen`。IMP-250）を出すときも、`state.editable` と `state.editMode` を偽にして写す**（FR-140 の表。`leaveDocument()` が `leaveEditMode()` を呼ぶ）。

### IMP-261: チェックボックスの切り替え **MUST**

FR-141 / DSP-126 / DSP-321 を実装する。

- 編集モードの間、`isOwnRef(input, 'task')` が真の `input[type=checkbox]` から `disabled` を外し、`is-editable` のクラスを付ける。**編集モードでない間と、鍵の合わない `input`（生 HTML）には何もしない**——`disabled` のまま、CSS の `pointer-events: none`（DSP-121）も残る。
- `#markdown` に `change` のリスナを 1 つ置く（イベント委譲）。対象が上の条件を満たすときだけ処理する。
- **編集欄が開いていれば、`checked` を元へ戻し、IMP-264 の `refuseIfEditing()` を呼んで終える**（FR-140, FR-141。4.75.0）。Go を呼ばない。
- **見た目はブラウザが既に反転している**（`change` の時点）。これが FR-141 の「即座に反転」（NFR-012）にあたる。その値を `checked` として `api.setTask(ref, checked)` を呼ぶ。`ref` は `data-ref` の値そのものを渡す（IMP-316）。**値は `refs.js` の `ownRef(input, 'task')` で得る**（IMP-260）。**呼び出しそのものが失敗した（`Promise` が拒否された）ときは `stale` と同じく戻し、通知しない**（Go 側は回復したパニックも DTO で返す。IMP-310）。
- 結果に応じて整える。

| 結果 | 見た目 | 通知 |
| --- | --- | --- |
| `changed` | そのまま（再描画で同じ状態が届く） | なし |
| `changed` が偽で `stale` も `error` も無い（既にその状態だった） | そのまま | なし |
| `stale` | **戻す**（`checked` を反転） | なし（FR-143） |
| `error`（`edit-conflict` / `edit-failed`） | **戻す** | `notify(error)`（IMP-315） |

- **戻すのは、その `input` がまだ DOM にある場合だけとする。** 再描画（`document:changed`）が先に届いていれば、`input` は既に差し替わっており、新しい DOM がファイルの状態を示している。**戻り値とイベントのどちらが先でも同じ結果になる**（IMP-316）。
- `Space` で切り替えた場合も `change` が発火し、同じ経路を通る。**再描画の後もフォーカスは同じ順番のチェックボックスに戻る**（IMP-220 の手順 11）ため、続けて `↓` や `Tab` で移って `Space` を押せる（FR-014）。

### IMP-262: セルの編集欄 **MUST**

FR-142 / UI-055 / DSP-127 / DSP-321 を実装する。配置は `js/celledit.js`（4.75.0 で `editmode.js` から分けた。IMP-200）。**開いている間の扱い（拒む・保留・確定の待ち・開き直し）は、コードブロックの編集欄と共通の `blockedit.js`（IMP-264）が持つ。**

```js
// js/celledit.js
export function initCellEdit(deps)   // { api, notify, focusViewer }
export function markEditableCells()  // applyEditMode（IMP-260）から。is-editable を付け外しする
```

**編集できるセルの印**

- 編集モードの間、`isOwnRef(cell, 'cell')` が真の `th` / `td` に `is-editable` のクラスを付け、編集モードを終えたら外す（`applyEditMode` から呼ぶ。IMP-260）。**鍵の合わないセル（生 HTML の表、補われたセル）には付けない**——見分けられることが UI-055 の要件であり、CSS は鍵を読めないためクラスで伝える（DSP-126）。

**開く**

- `#markdown` に `dblclick` のリスナを 1 つ置く。`state.editMode` が真で、`closest('th, td')` が `isOwnRef(cell, 'cell')` を満たすときだけ処理する。
- **セルの中のボタンの上でのダブルクリックは無視する**（FR-142）。対象は `.sort-btn`（IMP-229）と `.media-actions`（IMP-228）であり、判定は `closest('.sort-btn, .media-actions, .cell-editor-box')` で行う。**開いている編集欄（`.cell-editor-box`）の中のダブルクリックも無視する**——語を選ぼうとしてダブルクリックすると、同じセルで編集欄が作り直され、入力中の文字が失われる。無視するときは下の `removeAllRanges()` も呼ばない（編集欄の中の語の選択を残す）。**セルの中のリンクの上では編集を始めてよい**（1 回目のクリックの遷移は IMP-223 が止めている）。
- **別の編集欄（セル・コードブロック）が開いていれば、開かずに IMP-264 の `refuseIfEditing()` を呼ぶ**（FR-140 の「別の編集欄」。4.75.0）。v1.1.0 は開いていた編集欄を取り消してから開いていた。
- 既定のダブルクリックによる単語の選択は、`dblclick` で `getSelection().removeAllRanges()` を呼んで消す。
- `api.getCellSource(ref)` を呼ぶ（`ref` は `ownRef(cell, 'cell')`）。`stale` なら何もしない。`error` なら `notify` して開かない（`edit-conflict` のときは Go 側が読み直しを送っている。IMP-195）。
- **応答を待つ間にセルが DOM から消えていたら（再描画が先に届いた）、または `state.editMode` が偽になっていたら、開かない。** **後から別のセルをダブルクリックしていたときも開かない**——ダブルクリックのたびに番号を進め、応答が届いた時点で最新の番号でなければ捨てる。
- **IMP-264 の `await openBox(cell)` が `true` を返したら**（4.76.0。偽なら何もしない）、**セルの中**に `<div class="cell-editor-box">` を置き、`<input type="text" class="cell-editor">`（`value` に `text`）と、`Save` / `Cancel` のボタンの行（`<div class="edit-actions">` に、**`button.edit-cancel`、`button.edit-save` の順**。見た目の並び（右寄せで `Cancel` `Save`）と `Tab` の順をそろえる。DSP-172 の `Cancel` / `Open` と同じ。文言は `S.cancel` / `S.save`）を入れる（4.75.0。UI-055）。`th` / `td` に `is-cell-editing` のクラスを付ける（配置と寸法は DSP-127）。**置いた直後に `.edit-actions` の左端が表（`table`）の左端より左へ出ていれば、`.cell-editor-box` に `is-align-left` を付ける**（左端をセルの左端にそろえる。DSP-127。表は横スクロールの器であり、左へはみ出した分は見えない。4.76.0）。`focus({ preventScroll: true })` し、`setSelectionRange` でカーソルを末尾へ置く。**`value` の設定でもカーソルは末尾へ移る（HTML の規則）が、フォーカスの移動で全体を選ぶエンジンがありうるため、明示して置く**（NFR-061）。

**キー**（`input.cell-editor` の `keydown`）

- **`event.isComposing` が真なら何もしない**（FR-142 の IME）。**`keyCode` が 229 のときも何もしない**——変換を確定する `Enter` を、`isComposing` が偽の `keydown` として届けるエンジンがある（NFR-061。実機で見る）。
- `Enter`: 確定する（`Save` と同じ）。`preventDefault()` と `stopPropagation()` を行う（IMP-244 の検索の移動へ届かせない）。
- `Esc` はここで扱わない。**IMP-244 の振り分けの (3) が、編集欄にフォーカスがあるときに `cancelEditBox()` を呼ぶ**（右クリックメニューが開いていれば (1) が先に閉じる）。
- `Ctrl+Z` / `Ctrl+Y` は素通しする（入力欄の中の取り消し。FR-144, IMP-244）。
- `Tab` は既定の動作に任せる（`Save` / `Cancel` へ移る。編集欄は開いたまま）。

**貼り付け**

- 編集欄の `paste` で `preventDefault()` し、`clipboardData.getData('text/plain')` の改行（`\r\n` / `\r` / `\n`）を半角空白へ置き換えて `setRangeText(text, start, end, 'end')` で入れる（FR-142）。**`input type="text"` は既定で改行を黙って取り除き、語が連結される**ため、既定に任せない。
- 右クリックメニューの `Paste` は IMP-249 が同じ置き換えを行う。

**フォーカスが外れたとき**

- **何もしない**（取り消さない。書き込まない。FR-142。4.73.0）。**v1.1.0 は `blur` で取り消していた。** フォーカスを奪い返さない（UI-055）——利用者が移した先（ツールバーのボタンや検索バー）から奪わない。
- ウィンドウが `focus` を取り戻したとき、編集欄がまだ開いていて、フォーカスがどこにも無ければ（`document.activeElement` が `body`）、編集欄へフォーカスを戻す。
- 右クリックメニューを閉じたときのフォーカスは IMP-249 が扱う（メニューの外のクリックでは押した先へ、それ以外の契機では編集欄へ。FR-142, UI-085）。

**確定**（`Enter` か `Save`）

0. IMP-264 の `canSubmit()` が偽なら何もしない（外部の更新を控えている・削除された。警告は `canSubmit` が出す。編集欄は開いたまま）。
1. `value` を控え、編集欄（`.cell-editor-box`）を DOM から外して控える（開き直しに使う）。
2. **見た目を先に変える**（NFR-012, DSP-321）。**セルの直接の子のうち `.sort-btn` 以外**を 1 つの `DocumentFragment` へ退避し（画像の `.media-actions` は `.media-image` の中にあり、画像ごと退避する。IMP-228）、代わりに `<span class="cell-pending">` に `textContent` で控えた文字列を入れて**セルの先頭に**置く（並べ替えのボタンは末尾に残る）。**`innerHTML` を使わない**（IMP-220）。**本文ペインへのフォーカスの移動（`deps.focusViewer()`）は、この直後、`setCell` の応答を待つ前に行う。**
3. `api.setCell(ref, value)` を呼ぶ。**応答を待つ間も、IMP-264 は編集欄が開いているものとみなす**（再描画を控える）。
4. 結果に応じて整える。

| 結果 | 見た目 | 編集欄 | 通知 |
| --- | --- | --- | --- |
| `changed` | そのまま（再描画を待つ） | 閉じる（`is-cell-editing` を外して `closeBox`） | なし |
| `changed` が偽で `stale` も `error` も無い（内容が変わらない） | 退避した子で戻す | 閉じる（同上） | なし |
| `stale` | 退避した子で戻す | **入力ごと開き直し、フォーカスを編集欄へ戻す**（4.74.0。FR-142） | なし（控えた外部の更新があれば `edit-held`） |
| `error`（`edit-conflict` / `edit-failed` / `edit-rejected`） | 退避した子で戻す | **入力ごと開き直し、フォーカスを編集欄へ戻す** | `notify(error)`（IMP-315） |

- **文字列の整形（改行・前後の空白・`|` のエスケープ）はフロントエンドで行わない。** Go 側の `PlanCell`（IMP-106）が行う。**2 か所で整形すると、見た目と書き込まれる内容が食い違う**——`.cell-pending` に出すのは入力された文字列そのままでよい（FR-142 の「文字としてそのまま出したものでよい」）。
- **v1.1.0 は失敗のときに入力を捨てていた**（見た目を戻して通知するだけ）。**4.74.0 で、入力を捨てない**と決めた（FR-142）。

**取り消す**（`Esc`（編集欄にフォーカスがあるとき）か `Cancel`）

- 編集欄を取り除き、セルの `is-cell-editing` を外し、IMP-264 の `closeBox()` を呼ぶ（控えた描画があれば当てる）。`deps.focusViewer()` で本文ペインへフォーカスを戻す（UI-055）。Go は呼ばない。

### IMP-263: 取り消し・やり直し **MUST**

FR-144 を実装する。

```js
export function undoEdit()   // 編集モードでなければ false
export function redoEdit()
```

- `state.editMode` が偽なら `false` を返す（IMP-244。既定の動作を止めない）。**真なら呼んで、応答を待たずに `true` を返す**（`toggleEditMode` と同じ理由）。
- `api.undoEdit()` / `api.redoEdit()` を呼ぶ。**見た目を先に変えない。** どこが変わるかをフロントエンドは知らない（`Patch` は Go 側にある。IMP-108）。表示は `document:changed` で届く。
- `error` なら `notify` する。`changed` が偽（取り消すものが無い）なら何もしない（FR-144 の「通知しない」）。**`stale` なら何もしない**（通知しない。取り消しは鍵を持たないため、編集モードでなくなっていた・状態画面へ移っていた場合に当たる。FR-143）。
- **入力欄にフォーカスがあるときは、ここへ来ない**（IMP-244 が素通しする）。
- **編集欄が開いていて、フォーカスが外にあるときは、IMP-264 の `refuseIfEditing()` で拒む**（FR-144, FR-140。4.75.0）。

### IMP-264: 編集欄（共通の部品とコードブロック） **MUST**

FR-140 / FR-142 / FR-145 / UI-053 / UI-055 / UI-090 / DSP-128 を実装する（4.75.0）。

```js
// js/blockedit.js — 編集欄の共通の部品（セル・コードブロック）
export function initBlockEdit(deps)   // { api, notify, focusViewer, isModalOpen, applyHeld(item) }
export async function openBox(target, handlers) // api.setEditBoxOpen(true) を待ち、出してよければ開いたと登録して true（同時に 1 つだけ。下記）
                                      // handlers: { cancel(), focus(), root() }（取り消す・フォーカスを戻す・中にフォーカスがあるかを見る要素。4.93.0）
export function closeBox()            // 閉じたと登録し、控えたものを当てる。api.setEditBoxOpen(false)
export function markSubmitting(on)    // 確定の応答を待つ間か（警告の抑止。下記の 4.81.0。4.93.0）
export function isEditBoxOpen()       // 開いているか（確定の応答を待つ間を含む）
export function isEditBoxFocused()    // 編集欄（入力欄と Save / Cancel）にフォーカスがあるか
export function cancelEditBox()       // フォーカスがあれば取り消して true（IMP-244 の Esc の (3)）
export function refuseIfEditing()     // 開いていれば edit-busy の警告を出し、編集欄へフォーカスを戻して true
export function holdIfEditing(item)   // 開いていれば、画面を差し替えるものを控えて true
export function canSubmit()           // 確定してよいか。控えた外部の更新・削除があれば警告を出して false

// js/codeedit.js — コードブロックと図の編集欄
export function initCodeEdit(deps)    // { api, notify, focusViewer }
export function markEditableBlocks()  // applyEditMode（IMP-260）から。編集のボタンを付け外しする
```

**開いている間の操作**（FR-140 の表。**判断はこのモジュールだけに置く**）

| 扱い | 操作 | 入口 |
| --- | --- | --- |
| **拒む**（`refuseIfEditing()`） | 文書を開く（ダイアログ・ツリー・ツリーのメニューの `Open`・リンク（**同一文書内のアンカーを除き、外部の URL と開けないファイルへのリンクも拒む**。4.76.0。利用者の判断。IMP-223）・戻る / 進む・同じファイルの開き直し）、再読み込み | `navigate.js` の各経路の入口（ボタン・キー・サイドボタン・クリックのどれもここを通る） |
| 〃 | チェックボックス | IMP-261（ブラウザが反転した見た目を戻す） |
| 〃 | 別の編集欄を開く（セル・コードブロック。ダブルクリックと編集のボタン） | IMP-262 と本 ID |
| 〃 | 修飾（ツールバー・メニュー・`Ctrl+Shift+X`） | IMP-265 |
| 〃 | 編集欄の外にフォーカスがあるときの `Ctrl+Z` / `Ctrl+Y` / `Ctrl+Shift+Z` | IMP-263 |
| 〃 | 編集モードの終了（ボタン・`Ctrl+Shift+M`） | IMP-260 の `toggleEditMode` |
| 〃 | 表の並べ替え | IMP-229 |
| 〃 | ドロップ | **Go 側**（`SetEditBoxOpen`。IMP-245, IMP-313）。拒んだ知らせ（`error` イベントの `edit-busy`）を受けて、ここと同じく警告を出す |
| **保留する**（`holdIfEditing()`） | `document:changed`（外部の更新・自分の書き込みの直後）、`document:opened`（知らせが Go に届く前のドロップ）、状態画面の種別の `error`、`document:removed` | `main.js` の購読と `navigate.js` |
| **保ったまま行う** | 同一文書内のアンカーへの移動・テーマ・検索・ツリーの展開とフィルター・ペインの開閉・倍率・拡大画面を開く・情報ウィンドウ・エディタ選択ウィンドウ・ツリーのメニュー（`Open` 以外） | （何もしない） |

- **拒んだら、何もせずに `edit-busy` の警告を出し（IMP-315）、フォーカスを編集欄へ戻す。** **ただし情報ウィンドウ・エディタ選択ウィンドウ・拡大画面の表示中（`deps.isModalOpen()`）は、フォーカスを動かさない**（FR-140）——フォーカストラップを破らない。
- **編集モードを始めたときの描き直し**（IMP-195）は、編集欄が開いている間には届かない（始めるのは編集モードでない間、編集欄を開くのは編集モードの間だけで、終了は拒む）。表に入れない（4.76.0）。
- **控えるのは最新の 1 つだけ**とし、後から届いたもので置き換える。**置き換える前に控えていたものが文書の切り替え（`sameDocument` が偽・状態画面）なら、後のものも切り替えとして当てる**（`sameDocument` を偽として扱う。並べ替え・原寸表示・拡大画面の解除を落とさない。4.76.0）。**ただし削除の印（`document:removed`）は、その後の再描画で消さず、閉じた時点まで残す**（FR-140 の「閉じた時点で終える」）。警告は次の順に 1 つだけ選ぶ（DSP-151 は前のメッセージを置き換える。4.76.0）: **削除を控えたら `removed-editing`**、**読み直した内容が不正なバイト列を含んでいた（`editable` が偽の再描画）なら `encoding-editing`**（FR-110。その旨と、編集欄を閉じると編集モードを終える旨。4.75.0）、それ以外の外部の更新（`trigger` が `watch` / `reload`）・切り替え・状態画面なら `edit-held`。**自分の書き込みの直後（`trigger` が `edit`）では出さない。**
- **`blockedit.js` は呼び出し側を import しないため、取り消し・フォーカスの戻し・中にフォーカスがあるかの判定は `openBox` の `handlers` で受ける**（4.93.0。`cancelEditBox` / `refuseIfEditing` / `isEditBoxFocused` が使う）。**確定の応答を待つ間かは `markSubmitting(on)` で知らせる**（呼び出し側が確定の前後で呼ぶ。下の警告の抑止に使う）。
- **閉じた時点で、控えたものを当てる**（`deps.applyHeld(item)`。`main.js` が配線し、届いたときと同じ処理——`renderDocument`・`enterStateScreen`・`leaveEditMode`——へ渡す）。削除の印があり、**その後に再描画を控えていなければ**、その後に `leaveEditMode()` を呼ぶ。**削除の後に再描画が届いていれば呼ばない**——ファイルが戻った後の Go 側の状態（編集モードは終わり、開始できる）を運んでおり、それに従う（4.76.0）。
- **確定の応答を待つ間に控えた再描画では、警告（`edit-held`）を出さない。知らせは確定の応答が決める**（4.81.0。利用者の判断。**削除（`document:removed`）は再描画ではないため、待つ間も `removed-editing` を出す**。4.93.0）——ファイルが変わっていれば `edit-conflict`（エラー）、成功なら何も出さない。Go は衝突のとき読み直した再描画（`trigger: reload`）も送り、応答との届く順は決まっていない（IMP-195）。ここで出すと、最後に残る知らせが到着順で変わる（DSP-151 は前のメッセージを置き換える）。控えた再描画は、開き直した編集欄の次の確定で `canSubmit()` が `edit-held` を出す。
- **確定の応答を待つ間も「開いている」とみなす**（4.75.0）。待つ間に届いた再描画も控え、失敗で開き直すときに、書き込む先の要素が DOM に残っているようにする。応答で閉じるときに、控えたものを当てる。**待つ間も `is-cell-editing` / `is-code-editing` は付けたままにし、閉じるときに外す**（表の下の余白（DSP-127）と右上のボタンの隠し（DSP-128）を保ち、テーマの描き直しもそのブロックを飛ばす。4.76.0）。**失敗で開き直すときは `openBox` を呼び直さない**——登録は開いたまま続いており、Go 側の `editBoxOpen` も真のままである。
- **確定の呼び出しそのものが失敗した（`setCell` / `setCode` / `applyFormat` の `Promise` が拒否された）ときは、`stale` と同じに扱う**（IMP-261 のチェックボックスと同じ。編集欄は入力ごと開き直し、修飾は包みを外す。通知しない。4.76.0）。扱わないと「開いている」とみなしたまま固まり、文書の切り替えもドロップも拒まれ続ける。
- **確定してよいか**（`canSubmit()`）: 外部の更新・切り替え・状態画面を控えていれば `edit-held`、削除の印があれば `edit-removed` の警告を出し直して `false` を返す（FR-145, FR-140）。**編集欄は閉じず、Go へ送らない。** 送っても Go 側は鍵の照合か編集モードの確かめで `Stale` にする（IMP-195）——**安全は Go 側が保ち、ここは知らせ方を決める。** 自分の書き込みの直後の再描画だけを控えているときは確定してよい（鍵は引き継がれている。IMP-102）。
- **`openBox(target, handlers)` は `Promise<boolean>` を返す**（4.76.0。順序は IMP-245）。`api.setEditBoxOpen(true)` を待ち、応答の後で `target`（セルかブロック）がまだ DOM にあれば（`isConnected`）、開いたと登録して `true` を返す。**待つ間に文書が描かれたかは `isConnected` だけで見る**（4.93.0）——本文の描画は手順 1 で `#markdown` の中身を差し替えるため、描かれれば `target` は必ず外れる（4.92.0 までは「描画の世代が同じ」も求めていたが、同じことを 2 度見るだけだった）。そうでなければ `api.setEditBoxOpen(false)` を呼んで `false` を返す。**呼び出し側（IMP-262 と下のコードブロック）は、`true` が返ってから編集欄を DOM に置く**——待つ間、編集欄は見えない。**待つ間は「開きかけ」とし、`refuseIfEditing()` は拒むが、`holdIfEditing()` は控えない**（届いた描画はそのまま当て、上の `isConnected` の確かめで検知する）。`closeBox` は `false` を呼ぶ（IMP-310）。**`setEditBoxOpen` は、前の呼び出しの応答を待ってから送る**——立て続けの `false` と `true` の到着順が入れ替わらないようにする（IMP-210 の `saveConfig` と同じ事情）。Go 側はドロップを拒むためにだけ使う（IMP-313）。
- **開いている編集欄の入力を、MarkView が黙って捨てる場面を作らない**（FR-140）。閉じるのは、確定の成功・内容が変わらない確定・`Cancel`・フォーカスのある `Esc` だけである（アプリケーションの終了を除く）。

**コードブロックと図の編集欄**（`codeedit.js`。FR-145）

- **対象**: `isOwnRef(block, 'code' | 'mermaid' | 'plantuml')` が真の `.code-block`（PlantUML ファイルの図と Mermaid ファイルを含む。IMP-126, IMP-127）。鍵の合わないブロック（生 HTML）は対象にしない（NFR-030）。
- **編集のボタン**: 編集モードの間、対象に `<button class="media-btn media-btn-edit">`（`icon-pencil`。`data-tip` と `aria-label` は `S.tipEditBlock`）を置く。図では `.media-actions` の先頭（**無ければ作る**——図の描画は手順 8 で非同期に終わり、手順 6d の時点では器が無い。描けなかった図と描かなかった PlantUML も編集できるようにする。`data-media-key` は IMP-228 と同じ図の鍵とし、後から描き終えた図は同じ器にボタンを足す。4.76.0）、コードブロックでは `.media-actions` を作って入れる（IMP-228, UI-053）。**`data-locked="1"` を持つブロックでは `disabled` とする**（前置きが行ごとに違う。FR-145）。編集モードを終えたら取り除く（`markEditableBlocks`）。ボタンのクリックは `preventDefault()` と `stopPropagation()` を行う。
- **開き方**: 編集のボタンの押下か、`#markdown` の `dblclick`（`closest('.code-block')` が対象で、`state.editMode` が真）。**右上のボタン（`.copy-btn` / `.media-actions`）と、開いている編集欄の上のダブルクリックは無視する**（FR-145）。**`data-locked` のブロックをダブルクリックしたら、`code-locked` の警告を出して開かない**（FR-145, IMP-315）。既定のダブルクリックによる語の選択は `removeAllRanges()` で消す。**既に編集欄が開いていれば `refuseIfEditing()`。**
- `api.getCodeSource(ref)`（`ref` は `ownRef(block, kind)`）を呼ぶ。`stale` なら何もしない。`error` なら `notify` して開かない（`code-locked` は警告、`edit-conflict` は Go 側が読み直しを送っている）。応答を待つ間にブロックが DOM から消えた・編集モードが終わった・後から別のブロックを押していたら開かない（IMP-262 と同じ）。
- **編集欄**: **`await openBox(block)` が `true` を返したら**（4.76.0。偽なら何もしない）、`.code-block` の中に `<div class="code-editor-box">` を置き、`<textarea class="code-editor" spellcheck="false">`（`value` に `text`）と、`Save` / `Cancel` のボタンの行（`.edit-actions`。IMP-262 と同じ）を入れる。**ブロックの元の中身（`pre` や描いた図）は `hidden` にして残し**（取り消したら戻す）、`.code-block` に `is-code-editing` のクラスを付ける（閉じたら外す。右上のボタンを隠す。DSP-128）。**高さ**（DSP-128。4.76.0）: `#viewer` の `clientHeight` の 60 % を `--code-editor-max`（px）として編集欄に入れる（開いたときと、`#viewer` の大きさが変わったとき。IMP-246 の `ResizeObserver`）。`input` のたびに `scrollHeight` を `--code-editor-height` に入れ直す（最小 3 行と上限は CSS の `min-height` / `max-height`）。**利用者が手で高さを変えたら（`resize` が `style.height` を付ける）、以後は入れ直さない**（上限は `max-height` なので、手で変えたときにも効く）。`focus({ preventScroll: true })` し、カーソルを先頭に置く。
- **キー**（`textarea` の `keydown`。IME の変換中（`isComposing` か `keyCode` 229）は何もしない）:
  - `Ctrl+Enter`: 確定する（`preventDefault()` と `stopPropagation()`）。
  - `Enter`: 改行（既定の動作）。**検索の移動へ回さない**（`stopPropagation()`。IMP-244）。
  - `Tab` / `Shift+Tab`: 字下げを入れる / 外す（`preventDefault()`）。選択が無ければカーソル位置にタブ文字を入れる（`Shift+Tab` は行の先頭のタブ文字を 1 つ除く）。**選択が複数行にかかれば、選択にかかる各行の先頭にタブ文字を足す / 先頭のタブ文字を 1 つ除く。** 変更は `setRangeText` で行い（`textarea` の中の取り消しが効く形）、`input` イベントを発火する。**`Tab` で編集欄の外へ出る手段は持たない**（確定か取り消しで閉じる。FR-145）。
  - `Esc` はここで扱わない（IMP-244 の (3)）。`Ctrl+Z` / `Ctrl+Y` は素通しする（`textarea` の中の取り消し）。
- **貼り付け**は既定の動作に任せる（改行をそのまま入れる。FR-145）。右クリックメニューの `Paste` も改行を残す（IMP-249）。
- **フォーカスが外れても何もしない**（開いたまま。FR-145）。ウィンドウがフォーカスを取り戻したときの扱いは IMP-262 と同じ。
- **確定**（`Ctrl+Enter` か `Save`）: `canSubmit()` が偽なら何もしない。真なら、入力を控えて編集欄を外し、**ブロックの元の中身の代わりに `<pre class="code-pending">` を置いて、`textContent` で入力をそのまま出す**（ハイライトと図は再描画で戻る。NFR-012, DSP-321）。`focusViewer()` で本文ペインへフォーカスを移し、`api.setCode(ref, text)` を呼ぶ。**文字列の整形（改行コード・前置き・囲いの延長）はフロントエンドで行わない**（Go 側の `PlanCode`。IMP-122）。
- **結果**: IMP-262 の確定の表と同じに扱う。`changed` なら再描画を待って閉じる。`changed` が偽なら元の中身へ戻して閉じる。**`stale` と `error` では、元の中身へ戻し、編集欄を入力ごと開き直して、フォーカスを編集欄へ戻す**（FR-145。4.74.0）。`error` は `notify`（`edit-rejected` / `code-locked` は警告。IMP-315）。
- **取り消し**（`Esc`（編集欄にフォーカスがあるとき）か `Cancel`）: 編集欄を取り除き、元の中身を戻し、`closeBox()`（控えたものを当てる）、`focusViewer()`。Go は呼ばない。
- **テーマの切り替えの図の描き直し（IMP-231, IMP-233）は、編集欄を開いているブロック（`.is-code-editing`）を飛ばす**（FR-140 の表の「保ったまま」）。飛ばしたブロックは、編集欄を閉じた時点の扱い（取り消しなら元の図を、いまのテーマで描き直す。確定なら再描画を待つ）で整える。編集欄の色だけがテーマに追随する（DSP-370）。
- **拡大画面へ移している図**（IMP-253）の編集のボタンは、閉じるまで本文に無い。拡大画面を開いたまま編集欄を開くことはできない（背後を操作できない）。

### IMP-265: 選んだ文字の修飾 **MUST**

FR-146 / FR-147 / UI-020 / UI-021 / UI-085 / UI-090 / DSP-141 / DSP-321 を実装する（4.75.0）。配置は `js/format.js`。**画面の選択をソースの位置に対応付けるのは Go 側である**（IMP-123, IMP-124）。ここは、選択を「ブロックの目印・見える文字の上の位置・見える文字の全体」に変えて送り、見た目を先に変えて結果で整えることだけを行う。

```js
// js/format.js
export const COLORS                     // [{ name: 'red', label: S.colorRed }, …]。FR-147 の順
export function initFormat(deps)        // { api, notify }
export function canFormat()             // 使えるか（下記）
export function selectionState()        // { strike: bool, color: 名前 | null }（メニューと色の一覧の印）
export function toggleStrike()          // Ctrl+Shift+X・ツールバー・メニュー。使えなければ false
export function applyColor(name)        // 色の一覧・メニューのサブメニュー。"red" … "gray" / "default"
export function updateFormatButtons()   // selectionchange と applyEditMode で、ツールバーのボタンの淡色を切り替える
export function restoreSelection(trigger) // renderDocument の手順 6d。doc.trigger が edit なら控えた選択を当て直す（4.93.0）
export function clearFormatState()      // leaveDocument（IMP-250）から。控えた選択と Range を捨てる（4.76.0）
```

**使える条件**（`canFormat()`。FR-146）。次のすべてを満たすとき。

1. `state.editMode` と `state.doc.textRefs` が真（修飾の目印付きの描画。IMP-302）
2. 選択（`getSelection()` の最初の `Range`）が空でない。**`#btn-strike` / `#btn-color` にフォーカスがある間は、下の控えた `Range` で判定する**（L1 ではボタンへフォーカスが移った時点で選択が解除される。4.76.0）
3. 選択の両端が、**同じ 1 つの修飾できるブロック**の中にある。ブロックは、端の要素から祖先へたどり、`isOwnRef(el, 'text')` か `isOwnRef(el, 'cell')` が真の最も近い要素とする（IMP-260）。**実装は、始まりの端からブロックを決め、終わりの端をそのブロックの見える文字の中の位置として求める**——別のブロックやリストの項目の数えない部分にあれば `offsetOf` が -1 を返し、使えない（4.93.0）
4. 選択の端が入力欄（検索バー・編集欄）の中に無く、フォーカスも入力欄に無い（FR-146。フォーカスは 4.93.0。Chromium では入力欄にフォーカスがあると選択の端は入力欄の位置へ移り、どちらか一方で足りるが、選択の表し方をエンジンに委ねない。NFR-061）

- 満たさないとき、ツールバーのボタンとメニューの項目は淡色、`Ctrl+Shift+X` は何もしない（`false` を返す）。**編集欄が開いていれば、押したときに IMP-264 の `refuseIfEditing()` で拒む。**
- **値の正規化（両端の空白・部品の全体への広げ）はフロントエンドで行わない**（Go 側の IMP-124 の 3）。2 か所で整えると、見た目と書き込まれる内容が食い違う。

**送るもの**（`api.applyFormat`。IMP-310, IMP-316）

- `ref`: そのブロックの `ownRef(el, 'text' | 'cell')`
- `start` / `end`: 選択の両端を、**そのブロックの見える文字（IMP-266）の中の Unicode の符号位置**へ変えたもの（JavaScript の文字列の位置は UTF-16 なので変換する。`visible.js` の `offsetOf`。**始まりは `edge` を `start`、終わりは `end` で求める**——部品の中の端は、どちらでも部品を含む側へ寄る。4.92.0）
- `visible`: そのブロックの見える文字の全体（`visible.js` の `visibleText`）
- `style`: `{ strike: true }` か `{ color: name }`

**L1 の選択範囲**（[BUG-019](../bugs/2026-09-21-bug-019-context-menu-copy-webkitgtk.md)。UI-021）

- **ツールバーの `#btn-strike` / `#btn-color` は、`pointerdown` と `mousedown` で `preventDefault()` し、フォーカスを移さない。** WebKit は要素へフォーカスを移すと文書の選択範囲を解除する。**ここを外すと、L1 でだけ「押しても何も起きない」になる。**
- **`Tab` でボタンへ移ってから `Enter` を押す場合に備え、`selectionchange` のたびに、修飾できる選択なら `Range` を控え**、ボタンの実行では控えた `Range` を当て直してから使う。**修飾できない選択になっても控えは捨てない**（4.93.0）——捨てると、ボタンへ移った時点の解除（WebKit）でボタンが押せなくなる。修飾できない選択の間はボタンが淡色でフォーカスを受けないため、古い控えが使われることはない。控えた位置が DOM から外れていたら当て直さない。**ボタンが `focus` を受けたときも、控えた `Range` を当て直す**（IMP-249 の `restoreRange` と同じ形。淡色の切り替えが選択の解除で外れないようにする。4.76.0）。
- 右クリックメニューと色の一覧（IMP-249）は、開く前に `Range` を控え、実行の直前に当て直す（BUG-019 と同じ）。

**見た目を先に変える**（NFR-012, DSP-321）

- **取り消し線を外す操作では、仮の見た目を出さない**（祖先の `<del>` が引いた線は子孫の要素から消せない。再描画で外れて見える。FR-146, NFR-012。4.81.0。利用者の判断）。選択の中の文字（空白を除く）がすべて `del` / `s` / `strike` の中にあれば外す操作とみなし、包まずに送る（外すかどうかの正は Go 側。IMP-124 の 5。ここは見た目だけを決める）。
- 送る前に、選んだ範囲のテキストを仮の `span`（取り消し線は `fmt-pending-strike`、色は `fmt-pending-color md-color-<名前>`、`default` は `fmt-pending-default`）で包む。**テキストノードごとに包み**（要素の境界をまたいで 1 つの `span` で包まない）、包んだ `span` を控える。包みは見える文字を変えない（IMP-266 は包みそのものを数えない）。
- 結果: `changed` なら再描画を待つ。`changed` が偽・`stale`・`error` なら、包みを外して元に戻す。`error` は `notify`（`format-rejected` は警告。IMP-315）。**`notice` が `format-widened` なら警告を出す**（FR-146。IMP-316）。

**再描画の後も選択を残す**（FR-146）

- 送るときに `{ ref, start, end }` を控え、**自分の修飾の書き込みの直後の再描画（`doc.trigger` が `edit`）**（IMP-220 の手順 6d の `restoreSelection(doc.trigger)`）で、**同じ目印のブロックの見える文字の同じ位置**へ選択を当て直す（`visible.js` の `rangeAt`）。**同じ目印のブロックは、同じ種類（`text` / `cell`）の鍵の合うブロックを文書の順に数えた位置で引く**（4.93.0）——再描画で鍵が変わるため `data-ref` の値では引けず、番号を読むと鍵を読むのが `refs.js` だけという規約（IMP-260）に触れる（IMP-228 の図の番号と同じ考え方）。修飾は見える文字を変えない（IMP-124 の 9）ため、同じ位置が同じ文字を指す。**取り消し線の直後に続けて色を付けられる。** ブロックが見つからない・見える文字の長さが足りない・結果が失敗だった・`changed` が偽だった・**それ以外の再描画が先に届いた**（DSP-352。4.76.0）ときは当て直さず、控えを捨てる。

**ツールバーの修飾の群**（UI-020）

- `applyEditMode` で、編集モードの間だけ `#tb-format` を出す（IMP-202）。**`updateFormatButtons()` は `document` の `selectionchange` を `requestAnimationFrame` でまとめて呼び、`canFormat()` で `disabled` を切り替える**（UI-021）。
- `#btn-strike` は `toggleStrike()`、`#btn-color` は IMP-249 の `openColorList(button)` を呼ぶ。
- **色の一覧の名前と順は FR-147 の表と同じにする**（`COLORS`。Go 側は IMP-123 の `TextColors`、CSS は DSP-010 のトークン）。**値（`#d1242f` など）はフロントエンドに書かない**——ファイルへ書く値は Go 側が決め（IMP-124）、画面の色は CSS が決める。

### IMP-266: 見える文字の数え方 **MUST**

FR-146 / BR-054 を実装する（4.75.0）。配置は `js/visible.js`（自前の他のモジュールを import しない。IMP-200）。**Go 側の IMP-123 と同じ数え方にする。** どちらかを変えたら、もう一方と描画スモーク（BR-054）を同じ変更で直す。

```js
// js/visible.js
export function visibleText(block)            // ブロックの見える文字（文字列）
export function offsetOf(block, node, offset, edge) // DOM の位置 → 見える文字の中の符号位置。edge は "start" | "end"（選択の始まりの端か終わりの端か。4.92.0）。範囲の外なら -1
export function rangeAt(block, start, end)    // 見える文字の符号位置 → DOM の Range（再描画の後の当て直し）
```

| 本文の中のもの | 数え方 |
| --- | --- |
| テキストノード | 文字（符号位置で数える。サロゲートペアは 1 つ） |
| `br` | **0**（変換器は `<br>` の後ろに改行を書き、そのテキストノードの `\n` が改行の 1 文字になる。IMP-123 の「改行」。4.76.0） |
| インラインコード（`code`） | 中の文字 |
| 数式（`.math-inline`。KaTeX が描いた中身を含む） | **要素全体で 1 文字（U+FFFC）** |
| 画像（`img`）と、読み込みに失敗した画像（`.img-broken`。IMP-226） | **1 文字（U+FFFC）**（`.img-broken` の中の代替テキストを数えない） |
| 脚注の参照（`a.footnote-ref` を含む `sup`） | 1 文字（U+FFFC）（中の番号を数えない） |
| 脚注の戻りリンク（`a.footnote-backref`） | 中の文字（`↩`。U+21A9）。直前の `&#160;`（U+00A0）も文字として数える（合わせて 2 文字。IMP-123） |
| タスクのチェックボックス（`input`） | 0。**直後のテキストノードの先頭の半角空白 1 つも数えない**（変換器が書く。構文木には無い。IMP-120。4.76.0） |
| **UI が足した要素**（見出しのアンカー、並べ替えのボタン、`.media-actions`、`.copy-btn`） | **要素ごと数えない** |
| **包むだけの要素**（検索の `<mark>`、修飾の仮の `span.fmt-pending-*`、`.cell-pending`） | 要素そのものは数えず、中の文字を数える |
| リストの項目（`li`） | **最初のブロック要素（`p` / `ul` / `ol` / `pre` / `div` / `blockquote` / `table` / `details` など）の手前まで**。**その手前の空白だけのテキストと、文字の後ろの改行 1 つは数えない**（変換器が書く。IMP-123 の「入れ子のブロックを持つリストの項目」。4.76.0） |

- 対象のブロックは IMP-265 が決める（`text` / `cell` の目印を持つ要素）。
- **1 文字と数える部品（数式・画像・壊れた画像・脚注の参照）の中の位置は、`edge` が `start` なら部品の手前、`end` なら部品の後ろとする**——どちらの端でも部品を選択に含める（4.92.0。利用者の判断）。部品には中の位置に当たる符号位置が無い。**IMP-124 の 3 の「部品の一部にかかっていたら全体へ広げる」と同じ向き**であり、選択の終わりが数式の描画の中にあっても、その数式は選んだ範囲に入る。部品の直前・直後（要素の位置）はどちらの端でも同じ値になる。判定は `visible.js` の 1 か所に置く（呼び出し側で部品を見分けない）。4.91.0 までは端の向きが無く、実装（T27-2）は「部品の手前」としていたため、終わりの端が部品の中にあると部品が範囲から外れた。
- **生 HTML で、上の表の対象と同じ形を作ったもの**（行の中の `<input type="checkbox">`、`a.footnote-ref` を含む `sup` など）も表のとおりに数える。**`visible.js` は鍵を照合しない**ため、Go 側（IMP-123）と食い違い、そのブロックが修飾できないだけで済む（設計どおり。4.78.0）。
- **脚注の戻りリンクにかかった選択は、そのまま Go へ送る。** 選択から外すのは Go 側である（IMP-124 の 3。4.78.0。利用者の判断）。
- **1 文字と数える部品は U+FFFC で表す**（4.77.0。利用者の判断。IMP-123 と同じ文字）。見える文字は Go 側と文字列として照合する（IMP-124 の 2、BR-054）ため、**数だけでなく文字もそろえる。** 4.76.0 までは「1 文字」とだけ書いており、どの文字かが決まっていなかった。
- **4.75.0 の表は、`br` を 1 つと数え、チェックボックスの後ろの空白と `li` の入れ子の手前の改行を数えていた**（4.76.0 で直した）。Go 側（IMP-123）はどれも数えず、タスクの項目・ハード改行を含む段落・入れ子を持つリストの項目で、修飾が黙って効かなくなる（IMP-124 の 2 の `Stale`）。実現性検討の試作はこの 3 つを扱っていた。
- **描画スモーク（BR-054）は、全ブロックについて `visibleText` と Go 側の `VisibleTexts`（IMP-123）を突き合わせる**（Chromium だけ）。WebKitGTK との差は手動テストで見る（NFR-061）。
- **数え方を合わせるために、Go 側の結果を画面へ送らない**（送ると、照合の意味が無くなる。FR-146 は両側が独立に数えて照合することを求めている）。

## 12.8 ファイルツリーの操作（IMP-270 系）

FR-036 / FR-037 / UI-032 / UI-033 のフロントエンド側（4.75.0）。**パスの検査は Go 側が行う**（IMP-135, IMP-196）。ここは、メニューと枚を出し、押された操作を Go へ渡し、結果を画面へ写すことだけを行う。

### IMP-270: ファイルツリーの右クリックメニュー **MUST**

FR-036 / UI-032 / DSP-140 を実装する。配置は `js/treemenu.js`。**器・開閉・キー・フォーカス・閉じる契機は IMP-249 の右クリックメニューを使う**（`#contextmenu`。場所の表の 5）。

```js
// js/treemenu.js — contextmenu.js / filetree.js / navigate.js を import しない（main.js が配線する）
export function initTreeMenu(deps)     // { api, notify, openFromTree(path), openEditorsForTree(path, opener), applyTreeAction(dto) }
export function treeMenuItems(place)   // 右クリックした所（ファイル・ディレクトリ・ツリールートの名前の行）の項目
export function runTreeAction(action, place)
```

| 右クリックした所 | 判定 | 項目（`strings.js`） | 実行 |
| --- | --- | --- | --- |
| ファイル | `li.tree-item` で、ディレクトリでない | `Open` / `Open in editor...` / `Open containing folder` / 区切り / `Copy path` | `deps.openFromTree(path)`（ツリーのクリックと同じ `navigate.js` の経路。編集欄が開いていれば拒む）/ `deps.openEditorsForTree(path, opener)`（`editorflow.js` が `api.listEditorsForTree(path)` を呼び、IMP-252 の確かめ（拡大画面の表示中は出さない）を経て `showEditors` で描く。4.76.0。4.81.0 で、IMP-252 に無い「フィルターの枚」を確かめの中から外した。**`opener` はメニューを開いた項目**で、閉じたときのフォーカスの戻し先（IMP-252）。右クリックだけではフォーカスを動かさない（FR-036）ため、渡さないと戻す先が分からない。4.93.0）/ `api.openContainingFolder(path)` / `api.copyTreePath(path)` |
| ディレクトリ | `li.tree-item` で、ディレクトリ | `Open in file manager` / `Set as tree root` / 区切り / `Copy path` | `api.openInFileManager(path)` / `api.setTreeRoot(path)` / `api.copyTreePath(path)` |
| ツリールートの名前の行 | `#tree-root-name` | `Open in file manager` / `Set parent as tree root` / 区切り / `Copy path` | `api.openInFileManager(root)` / `api.setTreeRootUp()` / `api.copyTreePath(root)` |

- **パスは `li` の `data-path`（Go が一覧を返したときの値）とツリールート（`state.treeRoot`）を渡す。** 検査は Go 側が行う（IMP-135, IMP-196）。**フロントエンドでパスの形や種類を判断しない**（IMP-300 の 2）。
- **使えない項目**（淡色。UI-032）:
  - 画像ファイル（`TreeNodeDTO.kind` が `image`。`li` に `data-kind` として写す。IMP-304）の `Open in editor...`
  - **ツリールートの名前の行の `Set parent as tree root` は、使えない状態で出してから `api.canSetTreeRootUp()` を呼び、真なら使える状態にする**（ファイルシステムの根とドライブの根では偽。IMP-310）。**右クリックメニューの `Paste` の可否と同じ形**（IMP-249）とし、応答が届く前にメニューが閉じていたら何もしない。根かどうかをフロントエンドでパスから判断しない。
- **右クリックだけでは項目を選ばず、開かず、ツリーの強調とフォーカスも動かさない**（FR-036）。**`#tree` と `#tree-root-name` で、右ボタンの `pointerdown` / `mousedown` の既定の動作を止める**（4.93.0）——Chromium は右ボタンでもフォーカスできる要素（`tabindex` を持つ項目）へフォーカスを移し、roving tabindex も付け替わる。`contextmenu` は止めても届く。メニューを閉じたら、フォーカスは開く前の位置（ツリーの項目）へ戻る（IMP-249）。**`Open` で文書を開いたら、フォーカスは本文ペインへ移る**（IMP-220 の手順 11）。`Open in editor...` ではエディタ選択ウィンドウへ移り、閉じたらツリーの項目へ戻る（IMP-252）。
- **キーボード**（UI-031, UI-032）: ツリーの項目にフォーカスがあるときの `Shift+F10` / アプリケーションキーは、IMP-249 がフォーカスのある項目の下に開く（`li` の矩形ではなく `.tree-row` の矩形を使う。展開中の `li` は配下の木全体を含む）。
- **結果の反映**:
  - `TreeActionDTO.error` があれば `notify`（`not-found` / `permission` / `open-failed` / `clipboard`。IMP-315。`permission` は 4.81.0 で書き足した。IMP-196）。
  - **`root` が空でなければ、`deps.applyTreeAction(dto)` を呼ぶ**（`main.js` が配線する）。`state.treeRoot` を新しいルートにし、ツリーを読み直し（`loadTreeRoot`。FR-035 の 4 番目の契機。IMP-240）、ステータス領域のパスを `displayPath` / `outsideTree` で差し替える（`state.doc` か `state.target` の表示用パスを書き換えて `updateStatus()`。FR-052）。**表示中の文書・履歴・編集モードは変えない**（FR-030）。**メニューを開いた項目が組み直しで消えたら（フォーカスが `body` に落ちる）、組み直したツリーの `tabindex="0"` の項目（IMP-248）へ `focus({ preventScroll: true })` で移す**（4.76.0。DSP-330）。
  - `Copy path` の成功は知らせない（UI-060）。
- **閉じる契機は IMP-249 と同じ**とし、**ツリーのスクロール（`.pane-body` の `scroll`）でも閉じる**（UI-032）。IMP-249 の `scroll` の購読（キャプチャ）に `.pane-body` を含める。
- **ツリーの項目とツリールートの名前の行の外（ペインの余白・見出しの行・フィルターの枚）では出さない**（UI-032, IMP-249 の場所の表）。

### IMP-271: ファイルツリーのフィルター **MUST**

FR-037 / UI-033 / UI-024 / DSP-115 を実装する。配置は `js/treefilter.js`。

```js
// js/treefilter.js
export function initTreeFilter(extensions, deps) // ExtensionsDTO（IMP-303）。文言の拡張子（このモジュールの変数に持つ）。チェックボックスの初期値は state から（IMP-211）。deps: { focusViewer }（4.93.0）
export function toggleTreeFilter()            // フィルターボタン（#btn-treefilter）
export function isTreeFilterOpen()
export function isTreeFilterFocused()         // IMP-244 の Esc の (4)
export function closeTreeFilter(restoreFocus) // Esc・閉じるボタン・ボタンでは true、自動で閉じるときは false
```

- **枚の中身**: 見出しの行（`.treefilter-head`）に見出し `Show in tree`（`.treefilter-title`。`S.filterTitle`）と閉じるボタン（`button.treefilter-close`。`icon-close`。`aria-label` は `S.close`）、その下に 3 つの選択肢（`label.treefilter-option` に `input[type=checkbox]` と文言の `span`）。文言は `S.filterMarkdown(exts)` / `S.filterDiagrams(exts)` / `S.filterImages(exts)`（`exts` は `ExtensionsDTO` の並びを `/` でつないだもの）。**拡張子をこのモジュールにも `strings.js` にも書かない**（FR-037 の「2 か所に書かない」）。`Markdown` は `checked` と `disabled`（淡色だが文言は読める濃さ。DSP-115）。各選択肢の `label` に `data-tip`（`S.tipFilterMarkdown` など。英日併記。UI-024）を置く。
- **位置**（UI-033, DSP-115）: 開くとき、`#btn-treefilter` の `getBoundingClientRect()` から求める。**枚の右端をボタンの右端にそろえ、ボタンの下に置く。** 左端がウィンドウの左端から 8px より左へ出るなら、左端を 8px に寄せる。**`window` の `resize` とファイルツリーペインの幅の変化（`ResizeObserver`）で位置を求め直す。**
- **開閉**: `#btn-treefilter` で開閉する（`aria-expanded`。押された見た目は DSP-115 がこれで選ぶ）。**利用者が閉じるのは `Esc`（IMP-244 の (4)）・閉じるボタン・フィルターボタンだけとし、外のクリックでは閉じない**（UI-033）。**自動で閉じる**: ファイルツリーペインを隠したとき（IMP-240）、拡大画面・情報ウィンドウ・エディタ選択ウィンドウを開いたとき（それぞれを開く入口で `closeTreeFilter(false)` を呼ぶ）。検索バー・右クリックメニューとは同時に開ける。
- **フォーカス**: 開いたら `Diagrams` のチェックボックスへ移し、利用者が閉じたら `#btn-treefilter` へ戻す（UI-033）。**自動で閉じたときは、枚の中にフォーカスがあれば本文ペインへ移す**（`deps.focusViewer()`。4.93.0）。ダイアログ・拡大画面を開く入口では枚を閉じてから開くため、移ったフォーカスはすぐ開いたものへ移り、結果は「開いたものへ移る」と同じになる。ファイルツリーペインを隠して閉じたときは、開いたものが無いため本文ペインに残る（4.76.0）。`treefilter.js` は `viewer.js` を import しない（循環しうる。IMP-220 の `deps.focusViewer` と同じ渡し方）。**`Tab` は枚の中に閉じ込めない**（モーダルではない。FR-110）。
- **切り替え**（FR-037）: チェックボックスの `change` で `state.treeShowDiagrams` / `state.treeShowImages` を書き換え、**`await saveConfig()`（IMP-210）の後で、ツリーを展開の状態・強調・スクロール位置を保って読み直す**（`filetree.js` の `reloadTreeKeepingState()`。FR-035 の 5 番目の契機）。ボタンのツールチップ（`S.tipFilter`）も書き換える（IMP-247）。
  - `reloadTreeKeepingState()` は、展開している `li` の `data-path` の集合と `.pane-body` の `scrollTop` を控えて `loadTreeRoot` で組み直し、控えたディレクトリを上から順に展開し直してから、スクロール位置を戻す。**隠した種類だけを含むディレクトリは一覧から無くなり、展開し直さない。** 強調は `state.doc ?? state.target` から付け直す（IMP-250）。**表示中のファイルの種類を隠したら、強調する行は無くなる**（FR-031, FR-052。ステータス領域のパスは変えない）。
- **編集欄が開いていても使える**（FR-140 の表の「保ったまま」）。ツリーのペインが非表示の間は開けない（ボタンが見えない）。
- **多重起動**: 切り替えは自分のウィンドウだけに効く（設定は後勝ち。UI-115）。

## 12.9 UI 文言（IMP-290 系）

### IMP-290: 文言の一元定義 **MUST**

UI-024 / AR-050 を実装する。

```js
// js/strings.js
export const S = {
  // ツールチップのみ英日併記（UI-024）。
  // キー表記は含めない。shortcuts.js の定義から組み立てる（IMP-244）
  tipBack:      'Back / 戻る',                       // UI-024（4.75.0）
  tipForward:   'Forward / 進む',
  tipOpen:      'Open / 開く',
  tipReload:    'Reload / 再読み込み',
  tipThemeDark: 'Dark theme / ダークテーマ',
  tipThemeLight:'Light theme / ライトテーマ',
  tipOutline:   'Outline / アウトライン',
  tipFileTree:  'File tree / ファイルツリー',
  tipOpenInEditor: 'Open in editor / エディタで開く',   // UI-024（4.39.0 で改名）
  tipEditMode:     'Edit mode / 編集モード',            // UI-024, FR-140
  tipStrike:       'Strikethrough / 取り消し線',         // UI-024, FR-146（4.75.0）
  tipColor:        'Text color / 文字色',                // UI-024, FR-147（ショートカットを持たない）
  tipAbout:     'About / アプリケーション情報',

  // ファイルツリーペインの見出しのボタンとフィルターの選択肢も英日併記（UI-024。4.75.0）
  tipFilter:         (kinds) => `Filter / フィルター (${kinds.join(', ')})`,  // kinds は下の kind* の並び
  tipFilterMarkdown: 'Markdown / マークダウン (always shown)',
  tipFilterDiagrams: 'Diagrams / 図 (PlantUML, Mermaid)',
  tipFilterImages:   'Images / 画像',

  // それ以外はすべて英語（UI-024）
  paneFiles:   'Files',
  paneOutline: 'Outline',
  noHeadings:  'No headings',
  searchPlaceholder: 'Find in document',
  searchNoResults:   'No results',
  searchCount:    (i, n) => `${i} / ${n}`,        // DSP-160
  searchPrevious: 'Previous match',               // アイコンボタンの読み上げ名
  searchNext:     'Next match',
  searchClose:    'Close search',
  dropHint:    'Drop a file to open',            // UI-070（4.75.0。種類を名指ししない）
  outsideTree: '(outside tree)',
  treeMore:    (n) => `… and ${n} more`,          // DSP-112

  // ファイルツリーのフィルター（UI-033。4.75.0）。拡張子は ExtensionsDTO から渡す（FR-037）
  filterTitle:    'Show in tree',
  filterMarkdown: (exts) => `Markdown (${exts})`,
  filterDiagrams: (exts) => `Diagrams (${exts})`,
  filterImages:   (exts) => `Images (${exts})`,
  kindMarkdown:   'Markdown',
  kindDiagrams:   'Diagrams',
  kindImages:     'Images',

  // ファイルツリーの右クリックメニュー（UI-032。4.75.0）
  menuOpen:              'Open',
  menuOpenInEditor:      'Open in editor...',
  menuOpenContaining:    'Open containing folder',
  menuOpenInFileManager: 'Open in file manager',
  menuSetTreeRoot:       'Set as tree root',
  menuSetParentRoot:     'Set parent as tree root',
  menuCopyPath:          'Copy path',

  // 図のファイル（FR-025, FR-026。4.75.0）
  noDiagrams:  'No diagrams found.',

  // コードブロック（DSP-251）。アイコンだけのボタンに読み上げ名を与える（IMP-295）
  copy:        'Copy',

  // 図と画像・表のボタン（UI-053, UI-054）。ツールチップは英語だけ（UI-024）
  tipActualSize: 'Actual size',
  tipExpand:     'Expand',
  tipSort:       'Sort',
  tipEditBlock:  'Edit',              // コードブロックと図の編集のボタン（UI-053。4.75.0）

  // 拡大画面（UI-104, DSP-173）。キーの表記は expand.js のキーの表から足す（IMP-253）
  expandTitle:   'Expanded view',   // #expand-view の aria-label
  expandFit:     'Fit',
  expandActual:  '1:1',
  tipExpandFit:    'Fit',           // ツールチップは `Fit (F)` の形に組み立てる
  tipExpandActual: 'Actual size',
  tipZoomOut:      'Zoom out',
  tipZoomIn:       'Zoom in',
  tipExpandClose:  'Close',
  expandZoom:    (z) => `${z}%`,

  // 右クリックメニュー（FR-063, UI-085）
  menuCut:       'Cut',
  menuCopy:      'Copy',
  menuPaste:     'Paste',
  menuSelectAll: 'Select all',
  menuCopyLink:  'Copy link address',
  menuStrikethrough: 'Strikethrough',   // FR-146（4.75.0）
  menuTextColor:     'Text color',      // FR-147。サブメニューを持つ

  // 文字色の一覧（FR-147, DSP-141。4.75.0）。名前と順は FR-147 の表
  colorRed: 'Red', colorOrange: 'Orange', colorYellow: 'Yellow', colorGreen: 'Green',
  colorBlue: 'Blue', colorPurple: 'Purple', colorGray: 'Gray', colorDefault: 'Default',

  // 編集モード（UI-055, DSP-126）
  editModeBadge: 'Edit mode',
  save:          'Save',                // 編集欄（UI-055。4.75.0）。Cancel は下の cancel を使う

  // 見出しのアンカー（IMP-227, DSP-023）。アイコンだけのリンクに読み上げ名を与える
  headingAnchor: 'Link to this section',

  // ステータス領域（DSP-150）
  statusLines: (n) => `${n} lines`,
  statusZoom:  (z) => `${z}%`,
  statusEditor:(name) => `Opened in ${name}`,   // FR-090, DSP-151
  statusImageSize: (w, h) => `${w} × ${h} px`,  // 画像ファイル（UI-060, DSP-150。4.75.0）
  statusFileSize:  (bytes) => formatSize(bytes),      // util.js。1024 未満は B、1 MB 未満は整数の KB、それ以上は確認画面と同じ MB（DSP-150, DSP-181）

  // 状態画面（DSP-181）
  welcomeTitle:   'Open a file',                 // UI-052（4.75.0。種類を名指ししない）
  welcomeHintOpen:'Press Ctrl+O to choose a file',
  welcomeHintDrop:'Or drop a file onto this window',
  welcomeHintTree:'Use the file tree to browse documents',
  openAnyway:     'Open anyway',
  rendering:      'Rendering...',                // Open anyway を押した後（FR-016, UI-052。4.81.0）
  largeTitle:     'This file is large.',
  largeHint:      'Rendering may take a while.',
  tooLarge:       (limit) => `Maximum size is ${limit}.`,
  renderError:    'Failed to render this document.',

  // PlantUML の図を描かなかった理由（FR-024, DSP-272）。図の代わりに本文中へ併記する
  pumlInclude:     'Include directives are not supported.',
  pumlUnsupported: (maxPx) => `PlantUML could not render this diagram. It may be unsupported or larger than ${maxPx.toLocaleString("en-US")} px.`, // 上限は puml.js の定数（4.96.0。BUG-030）
  pumlFailed:      'PlantUML rendering did not complete.',

  // 情報ダイアログ（DSP-171）
  appName:      'MarkView',                    // 見出し。固有名だが画面に出る
  aboutVersion: (v, c) => (c ? `Version ${v} (${c})` : `Version ${v}`),  // コミットが空なら括弧を省く
  aboutVendor:  (name, version) => `${name} ${version}`,  // Bundled 行の 1 項目
  aboutAuthor:     'Author',
  aboutRepository: 'Repository',
  aboutLicense:    'License',
  aboutEnvironment:'Environment',
  aboutBundled:    'Bundled',
  aboutLicenses:   'Third-party licenses',
  close:           'Close',

  // エディタ選択ダイアログ（UI-103, DSP-172）
  editorTitle:  'Choose an editor',
  editorOther:  'Other...',
  editorMissing:'(not installed)',
  editorNone:   '(no file chosen)',
  editorBrowse: 'Browse',
  editorOpen:   'Open',
  cancel:       'Cancel',

  // エラー（IMP-315 の Kind に対応）
  errNotFound:    (p) => `File not found: ${p}`,
  errPermission:  (p) => `Cannot access: ${p}`,
  errNotSupported:(p) => `Not a supported file: ${p}`,   // not-supported（4.75.0。v1.1.0 の errNotMarkdown）
  errLinkNotFound:(h) => `Link target not found: ${h}`,
  errClipboard:   'Failed to copy.',
  errRemoved:     (p) => `File was deleted: ${p}`,
  errEditorFailed:'Failed to start the editor.',
  errEditorSelf:  'MarkView cannot be used as an editor.',
  errEditConflict:'The file changed on disk and was not saved.',   // IMP-315 の edit-conflict
  errEditFailed:  (p) => `Failed to save: ${p}`,                    // edit-failed
  errPaste:       'Failed to paste.',                               // paste
  errOpenFailed:  (h) => `Cannot open: ${h}`,                       // open-failed（FR-050, FR-036。BUG-013）
  // 4.75.0。いずれも警告（UI-060, DSP-151）
  errEditRejected:  'This change would alter the document structure and was not saved.', // edit-rejected
  errCodeLocked:    'This code block cannot be edited in MarkView.',                      // code-locked
  errFormatRejected:'This formatting cannot be applied here.',                            // format-rejected
  warnFormatWidened:'The whole strikethrough was removed.',                               // format-widened
  warnEditBusy:     'Save or cancel the open edit first.',                                // edit-busy
  warnEditHeld:     'The file changed on disk. Cancel the edit to see the changes.',      // edit-held
  warnEditRemoved:  'The file was deleted. The edit cannot be saved.',                    // edit-removed
  warnRemovedEditing: (p) => `File was deleted: ${p}. Edit mode ends when you close the edit.`, // removed-editing（FR-110）
  warnEncodingEditing: 'Some characters were replaced. Edit mode ends when you close the edit.', // encoding-editing（FR-110）
  warnEncoding:   'Some characters were replaced.',
};
```

- **文言は、配布物（`frontend/js/strings.js`）・利用者向け文書（`docs/troubleshooting.md`）・描画スモーク（`scripts/smoke`）と一致させる。** PlantUML の 2 つの理由（`pumlUnsupported` / `pumlFailed`）と `aboutVersion` は、v1.0.0 から実物とこの節が食い違っていたため、4.44.0 で実物に合わせた。**この節の文言を変えるときは、実物・文書・`scripts/smoke` を同じ変更で直す。**

- **`errNotMarkdown` を残さない**（4.75.0）。種別の名前ごと `not-supported` / `errNotSupported` へ改めた（IMP-315）。古いキーを参照する箇所が残っていれば、読み込みの時点で気づける。
- **`tipEdit` を残さない。** v1.0.0 の `Edit / 編集` は、編集モード（`tipEditMode`）と並ぶと区別がつかない（UI-024 の NOTE）。キーの名前ごと `tipOpenInEditor` へ改め、古いキーを参照する箇所が残っていれば読み込みの時点で気づけるようにする。

- 引数を取る文言は関数として定義し、呼び出し側で文字列を組み立てない。表示文言の全体像がこのファイルだけで読める状態を保つ。
- `IMP-315` の `Kind` と、ここのキーを 1 対 1 で対応させる。未知の `Kind` を受け取った場合は `ErrorDTO.message` をそのまま表示する。
- `DocumentDTO.warnings`（IMP-302）も同じ `Kind` の並びであり、同じ対応表で文言を選ぶ。こちらにはフォールバックの `Message` がないため、未知の `Kind` は無視する。
- **ツールチップのキー表記だけは例外とし、この定義に含めない。** ショートカットの割り当ては `shortcuts.js` の 1 箇所で管理しており（IMP-244）、キー表記を文言側にも書くと二重管理になる。ツールバーは `${S.tipOpen} (${keyLabel('open')})` の形で組み立て、UI-024 が定める `Open / 開く (Ctrl+O)` という表示結果を得る。

```js
// js/shortcuts.js — キー割り当ての唯一の定義
export const SHORTCUTS = [
  { id: 'open',     keys: ['Ctrl+O'],           label: 'Ctrl+O' },
  { id: 'reload',   keys: ['F5', 'Ctrl+R'],     label: 'F5' },      // label は代表キー
  { id: 'theme',    keys: ['Ctrl+Shift+T'],     label: 'Ctrl+Shift+T' },
  // …
];
export function keyLabel(id)  // ツールチップに載せる代表キーを返す
```

`reload` のようにキーが複数ある場合、ツールチップには `label`（代表キー）のみを載せる。すべてのキーは UI-090 の一覧で示す。

- **すべての利用者向け文言をこのファイルに集約する。** 他のモジュールに文字列リテラルを直接書かない。
- ロケール判定・言語切り替えの仕組みを設けない（NFR-062）。
- Go 側が返すエラーメッセージも英語だが、UI に出す最終的な文言はこのファイルの定義を用いる（IMP-315）。

## 12.10 アクセシビリティ（IMP-295 系）

### IMP-295: 最低限の対応 **SHOULD**

- ツールバーの各ボタンに `aria-label` を与える。値はツールチップと同じ文字列とする。
- トグルボタンは `aria-pressed` を状態に応じて更新する（UI-021）。
- ツリーは `role="tree"` / `role="treeitem"` を用い、`aria-expanded` を更新する。**キーボードで操作できるようにする規定は IMP-248 が持つ**（UI-031）。
- フォーカスリングを消さない。`outline: none` を無条件に指定しない（DSP-016）。
- 状態画面・ダイアログの表示時に、フォーカスを内部の操作要素へ移す。
- **文書の表示時に、フォーカスを本文ペインへ移す**（UI-051）。規定は IMP-220 が持つ。**キーボードだけで操作する利用者が、開いた文書を読み進められるようにするためである。**
- **v1.1.0 で足した部品**（規定はそれぞれの ID が持つ）:
  - 原寸表示のボタンは `aria-pressed` を状態に応じて更新する（IMP-228）
  - 並べ替えている列の `th` に `aria-sort` を与える（IMP-229）
  - 右クリックメニューは `role="menu"` / `role="menuitem"`、使えない項目に `aria-disabled="true"`（IMP-249）
  - 拡大画面は `role="dialog"` / `aria-modal="true"` と `aria-label`（`S.expandTitle`）（IMP-253）
  - アイコンだけのボタン（図と画像・並べ替え・拡大画面の操作バー）は、ツールチップと同じ文字列の `aria-label` を持つ（IMP-247）

## 12.11 要求一覧

| ID | 概要 | 必須度 |
| --- | --- | --- |
| IMP-200 | ファイル構成 | MUST |
| IMP-201 | モジュール方式 | MUST |
| IMP-202 | DOM の骨格 | MUST |
| IMP-203 | アイコン | MUST |
| IMP-210 | フロントエンドの状態 | MUST |
| IMP-211 | 起動順序 | MUST |
| IMP-220 | 本文の挿入 | MUST |
| IMP-221 | コピーボタン | MUST |
| IMP-222 | スクロール連動 | MUST |
| IMP-223 | リンククリックの捕捉 | MUST |
| IMP-224 | アウトラインの構築 | MUST |
| IMP-225 | GitHub Alerts のアイコン付与 | MUST |
| IMP-226 | 画像の読み込み失敗 | MUST |
| IMP-227 | 見出しのアンカー | SHOULD |
| IMP-228 | 図と画像のボタンと原寸表示 | MUST |
| IMP-229 | 表の表示上の並べ替え | SHOULD |
| IMP-230 | Mermaid・KaTeX・PlantUML の遅延ロード | MUST |
| IMP-231 | Mermaid の初期化 | MUST |
| IMP-232 | KaTeX の初期化 | MUST |
| IMP-233 | PlantUML の初期化と描画 | MUST |
| IMP-240 | ペインの開閉とリサイズ | MUST |
| IMP-241 | 検索 | MUST |
| IMP-242 | 表示倍率 | MUST |
| IMP-243 | テーマ | MUST |
| IMP-244 | ショートカット | MUST |
| IMP-245 | ドラッグ＆ドロップ | MUST |
| IMP-246 | ウィンドウ幅に応じた一時的な非表示 | MUST |
| IMP-247 | ツールチップ | MUST |
| IMP-248 | ツリーのキーボード操作 | SHOULD |
| IMP-249 | 右クリックメニュー | MUST |
| IMP-250 | 状態画面 | MUST |
| IMP-251 | 情報ダイアログ | MUST |
| IMP-252 | エディタ選択ダイアログ | MUST |
| IMP-253 | 拡大画面 | MUST |
| IMP-260 | 編集モードの切り替えと目印の照合 | MUST |
| IMP-261 | チェックボックスの切り替え | MUST |
| IMP-262 | セルの編集欄 | MUST |
| IMP-263 | 取り消し・やり直し | MUST |
| IMP-264 | 編集欄（共通の部品とコードブロック） | MUST |
| IMP-265 | 選んだ文字の修飾 | MUST |
| IMP-266 | 見える文字の数え方 | MUST |
| IMP-270 | ファイルツリーの右クリックメニュー | MUST |
| IMP-271 | ファイルツリーのフィルター | MUST |
| IMP-290 | UI 文言の一元定義 | MUST |
| IMP-295 | アクセシビリティの最低限の対応 | SHOULD |
