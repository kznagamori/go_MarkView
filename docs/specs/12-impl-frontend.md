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
│   ├── main.js             起動処理とイベント配線
│   ├── state.js            フロントエンド側の状態
│   ├── api.js              Go バインディングの薄いラッパ（13 章）
│   ├── strings.js          UI 文言の一元定義（IMP-290）
│   ├── toolbar.js          ツールバー
│   ├── filetree.js         ファイルツリーペイン
│   ├── outline.js          アウトラインペイン
│   ├── viewer.js           本文の挿入と後処理
│   ├── copy.js             コードブロックのコピー
│   ├── search.js           文書内検索
│   ├── zoom.js             表示倍率
│   ├── theme.js            テーマ適用
│   ├── panes.js            ペインの開閉とリサイズ
│   ├── shortcuts.js        キーボードショートカット
│   ├── tooltip.js          ツールバーのツールチップ（IMP-247）
│   ├── dnd.js              ドラッグ＆ドロップ
│   ├── lazy.js             Mermaid / KaTeX の遅延ロード
│   ├── status.js           ステータス領域
│   ├── overlay.js          情報ダイアログと状態画面
│   └── util.js             共通ユーティリティ
├── icons/                  インライン SVG のソース（IMP-203。ファイル名はシンボル ID）
└── vendor/                 BR-042 が管理する資産
```

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
    <button id="btn-open"     class="tb-btn" type="button"></button>
    <button id="btn-reload"   class="tb-btn" type="button"></button>
    <button id="btn-theme"    class="tb-btn" type="button"></button>
    <button id="btn-outline"  class="tb-btn tb-toggle" type="button" aria-pressed="true"></button>
    <button id="btn-filetree" class="tb-btn tb-toggle" type="button" aria-pressed="false"></button>
    <span class="tb-spacer"></span>
    <button id="btn-about"    class="tb-btn" type="button"></button>
  </header>

  <main id="content" class="content">
    <nav id="pane-filetree" class="pane pane-filetree" hidden>
      <div class="pane-title"></div>
      <div class="pane-subtitle" id="tree-root-name"></div>
      <ul id="tree" class="tree" role="tree"></ul>
    </nav>
    <div id="resizer-filetree" class="resizer" hidden></div>

    <nav id="pane-outline" class="pane pane-outline">
      <div class="pane-title"></div>
      <ul id="outline" class="outline"></ul>
    </nav>
    <div id="resizer-outline" class="resizer"></div>

    <section id="viewer" class="viewer" tabindex="-1">
      <article id="markdown" class="markdown-body"></article>
      <div id="state-screen" class="state-screen" hidden></div>
      <div id="searchbar" class="searchbar" hidden></div>
    </section>
  </main>

  <footer id="status" class="status">
    <span id="status-path" class="status-path"></span>
    <span id="status-meta" class="status-meta"></span>
    <span id="status-message" class="status-message" hidden></span>
  </footer>

  <div id="overlay" class="overlay" hidden></div>
  <div id="dropzone" class="dropzone" hidden></div>
  <div id="tooltip" class="tooltip" hidden></div>
</div>
```

- ペインの表示・非表示は `hidden` 属性で切り替える。`style.display` を直接操作しない。
- **`index.html` に利用者向けの文言を書かない。** ペイン見出しの `Files` / `Outline` を含め、文言は `js/strings.js` から与える（IMP-290）。上の骨格でテキストが空の要素は、すべて実行時に埋める。
- テーマは `#app` の `data-theme` 属性で切り替える（DSP-011）。
- 本文は `.markdown-body` に挿入する。`github-markdown-css` が想定するクラス名に合わせる。
- `#viewer` に `tabindex="-1"` を与える。検索バーを閉じたときにフォーカスを本文へ戻す（UI-080）ために必要であり、負値のため `Tab` の巡回順には入らない。

### IMP-203: アイコン **MUST**

UI-022 を実装する。

- SVG は `index.html` の先頭に `<svg style="display:none">` のシンボル定義としてまとめ、各ボタンは `<svg class="icon"><use href="#icon-open"></use></svg>` で参照する。寸法（16 × 16）と色は `.icon` に対して CSS から与える（DSP-014）。
- `fill="currentColor"` とし、色は CSS から与える（DSP-014）。
- アイコンの一覧と対応は以下とする。同じ絵柄を複数箇所で使う場合、シンボルは 1 つだけ定義して共用する。`icon-dir` と `icon-open` のようにシンボル ID を分けたまま同じ絵柄を使う場合は、`<symbol id="icon-dir"><use href="#icon-open"/></symbol>` として参照で共用し、パスデータを二重に持たない。

| シンボル ID | 使用箇所 | 出典（Octicons） |
| --- | --- | --- |
| `icon-open` | ツールバー「開く」、welcome 画面（DSP-181） | `file-directory` |
| `icon-reload` | ツールバー「再読み込み」 | `sync` |
| `icon-moon` / `icon-sun` | ツールバー「テーマ切り替え」（状態で入れ替え） | `moon` / `sun` |
| `icon-outline` | ツールバー「アウトライン」 | `list-unordered` |
| `icon-filetree` | ツールバー「ファイルツリー」、ツリーの展開済みディレクトリ | `file-directory-open-fill` |
| `icon-about` | ツールバー「アプリケーション情報」 | `question` |
| `icon-dir` | ツリーの折りたたみ状態のディレクトリ（DSP-112） | `file-directory` |
| `icon-file` | ツリーのファイル（DSP-112） | `file` |
| `icon-chevron-right` / `icon-chevron-down` | ツリーの展開矢印（DSP-112） | `chevron-right` / `chevron-down` |
| `icon-search` | 検索バーの先頭（DSP-160） | `search` |
| `icon-chevron-up` | 検索バー「前へ」（DSP-160） | `chevron-up` |
| `icon-close` | 検索バー「閉じる」、情報ダイアログ「×」（DSP-160, DSP-170） | `x` |
| `icon-copy` / `icon-check` | コードブロックのコピーボタン（FR-061, DSP-252） | `copy` / `check` |
| `icon-note` | Alerts: NOTE（DSP-261） | `info` |
| `icon-tip` | Alerts: TIP | `light-bulb` |
| `icon-important` | Alerts: IMPORTANT | `report` |
| `icon-warning` | Alerts: WARNING、確認画面（DSP-181） | `alert` |
| `icon-caution` | Alerts: CAUTION、エラー画面（DSP-181） | `stop` |

> [!IMPORTANT]
> ここに挙げたシンボルは、いずれも**単色の SVG** であり、アプリケーションアイコン（UI-025, IMP-032）とは別物である。アプリケーションアイコンはラスタ形式の固有画像で、`/appicon.png`（IMP-160）から取得する。シンボル定義に混ぜない。

## 12.2 状態と初期化（IMP-210 系）

### IMP-210: フロントエンドの状態 **MUST**

```js
// js/state.js
export const state = {
  doc: null,          // DocumentDTO（13 章）。未表示なら null
  treeRoot: '',       // 絶対パス
  theme: 'light',        // 実際に適用している値。Go 側が解決済みで渡す（IMP-303）
  themeExplicit: false,  // 利用者が自分でテーマを切り替えたか（FR-071）
  zoom: 100,
  outlineVisible: true,
  fileTreeVisible: false,
  outlineWidth: 240,
  fileTreeWidth: 260,
  search: { open: false, query: '', hits: [], index: -1 },
  lazy: { mermaid: false, katex: false }, // 読み込み済みか
};
```

- 状態の**正**は Go 側（IMP-190）に置く。フロントエンドの `state` は描画のための写しであり、永続化に関わる値（テーマ・倍率・ペイン幅・表示状態）を変更したときは Go 側へ通知する（IMP-310）。
- 表示中の文書パスをフロントエンドで `localStorage` 等に保存しない（NFR-042）。
- **保存しない一時的な状態は `state` に置かない。** 幅不足によるアウトラインの一時的な非表示（IMP-246）は `panes.js` のモジュール変数とする。`state` は「Go 側の状態の写し」であり、そこに保存しない値を混ぜると、`configPatch` が何を送るべきかが読めなくなる。

`state` を変更したあとの通知は、次の 1 関数を必ず経由する。

```js
// js/state.js
export function configPatch()   // ConfigDTO（IMP-303）を組み立てる
export function saveConfig()    // 現在の状態を Go 側へ通知する
```

- **`api.updateConfig` を直接呼ばない。** バインドメソッドの呼び出しは Wails がメッセージごとに処理するため、立て続けに 2 つ投げると**到着順が入れ替わりうる**。実際に、ペインの開閉と倍率の変更を続けて行うと先に投げたほうが後に処理され、新しい倍率が古い値で上書きされた。`saveConfig` は前の応答を待ってから次を送ることで順序を保つ。
- 送信待ちが既にあるときは新たに積まない。`ConfigDTO` は差分ではなく状態の全体であり、待っている 1 つが送信時点の最新を読めば足りる。
- **`configPatch` の `theme` は、利用者が自分で切り替えるまで空文字とする**（FR-071, IMP-303）。`state.theme` は Go 側が OS 設定まで解決した値であり、それをそのまま返すと「まだ選んでいない」状態が最初の保存で失われ、以後 OS 設定を変えても追従しなくなる。

### IMP-211: 起動順序 **MUST**

```js
// js/main.js
async function boot() {
  const init = await api.getInitialState(); // 13 章 InitialStateDTO
  applyTheme(init.config.theme);
  applyZoom(init.config.zoom);
  applyPanes(init.config);
  initToolbar(); initTooltip(); initFileTree(); initOutline(); initViewer(); initPanes();
  initSearch(); initZoom(); initOverlay(); initDnd(); initShortcuts();
  subscribe();                              // Go からのイベント購読（IMP-322）
  if (init.document) renderDocument(init.document);
  else showStateScreen('welcome');
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

処理順序を固定する。

0. 検索を閉じる（IMP-241）。包んだ `<mark>` を解いてから差し替える。ここを飛ばすと、検索状態が前の文書の `<mark>` を指したまま残る（FR-080 の「検索対象文書の切り替え・再描画時は検索状態をリセットする」）。
1. `#markdown.innerHTML = doc.html` で**一度に**挿入する（AR-052）。分割挿入や逐次追加を行わない。
2. `#state-screen` を隠す。
3. コピーボタンを付与する（IMP-221）。
4. GitHub Alerts のアイコンを付与する（IMP-225）。
5. スクロール連動の監視対象を作り直す（IMP-222）。
6. `doc.needsMermaid` / `doc.needsKaTeX` に応じて遅延ロードを起動する（IMP-230）。
7. スクロール位置を設定する（13 章 `ScrollDTO` の `mode` に従う）。
8. アウトライン（IMP-224）とステータス（DSP-150）を更新する。

3 と 4 は DOM 走査を伴うため、`#markdown` を 1 回だけ走査して両方を処理してよい（NFR-011）。

`innerHTML` に渡す HTML は Go 側でサニタイズ済みである（IMP-116）。**フロントエンドで追加のサニタイズを行わないが、Go 側を経由しない文字列を `innerHTML` に渡してはならない。** UI 文言の挿入には `textContent` を用いる。

### IMP-221: コピーボタン **MUST**

FR-060 / FR-061 を実装する。

```js
// js/copy.js
export function attachCopyButtons(root) // root は #markdown
```

- `root.querySelectorAll('.code-block')` を走査し、各要素に `<button class="copy-btn">` を追加する。
- コピー対象の取得順序:
  1. `data-source` 属性があればその値（Mermaid ブロック。描画後に `<pre>` が SVG へ置き換わるため必須。IMP-115）
  2. なければ `pre code` の `textContent`
- 末尾の改行 1 つを除去してから渡す（FR-061）。
- クリップボードへの書き込みは **Go 側の API を経由する**（AR-062, IMP-311）。`navigator.clipboard` は権限や実行文脈によって失敗しうるため、これを既定経路にしない。
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
- 現在位置が変わったときのみ、アウトライン項目のクラスを付け替える。毎フレームの DOM 操作を行わない。
- 強調された項目がアウトラインペインの可視範囲外なら、`scrollIntoView({ block: 'nearest' })` で最小限のスクロールを行う。

### IMP-223: リンククリックの捕捉 **MUST**

FR-050 / AR-060 を実装する。

```js
// js/viewer.js
document.getElementById('markdown').addEventListener('click', onLinkClick);
```

- `#markdown` に 1 つだけリスナを置き、イベント委譲で処理する。リンクごとにリスナを付けない。
- `event.target.closest('a')` で対象を求め、`href` が存在すれば **常に `preventDefault()` を呼ぶ**。WebView 内でのページ遷移を一切発生させないため（AR-060）。
- 同一文書内のアンカー（`#...`）のみフロントエンドで処理し、該当見出しへスクロールする。それ以外は `href` の生値を Go 側へ渡し、判断を委ねる（IMP-312）。フロントエンドでスキームやパスの解釈を行わない。
- `target="_blank"` を含むリンクも同じ経路で処理する。

### IMP-224: アウトラインの構築 **MUST**

```js
// js/outline.js
export function renderOutline(headings)
```

- `DocumentDTO.headings`（Go 側が生成、IMP-117）をそのまま用いる。フロントエンドで DOM から見出しを抽出しない。抽出規則を 2 箇所に持たないため。
- 見出しが 0 件の場合、`strings.noHeadings` を表示する（FR-040）。
- インデントは**相対的な深さ**に応じた CSS カスタムプロパティで与える（DSP-113）。深さはレベルそのものではなく、`#` の次が `###` でも 1 段だけ下げる（FR-040 の「出現順を保ったまま相対的な深さで表示する」）。文字サイズはレベルで決める（DSP-113）ため、両者を別の属性で持つ。

### IMP-225: GitHub Alerts のアイコン付与 **MUST**

MD-040 / DSP-260 を実装する。

```js
// js/viewer.js
export function decorateAlerts(root)
```

- `root.querySelectorAll('.markdown-alert')` を走査し、クラス名（`markdown-alert-warning` 等）から種別を判定する。
- 各 `.markdown-alert-title` の先頭に `<svg><use href="#icon-warning"></use></svg>` を挿入する。シンボル ID は IMP-203 の一覧に従う。
- **この処理をフロントエンドで行うのは、Go 側が出力したインライン SVG がサニタイズで除去されるためである**（IMP-112）。Go 側は種別をクラス名で伝え、フロントエンドが見た目を組み立てる。
- 種別が既知の 5 つに一致しない場合は何も挿入しない。Go 側が未知の種別を出力することはないが、防御的に扱う。

## 12.4 遅延ロード（IMP-230 系）

### IMP-230: Mermaid と KaTeX **MUST**

AR-021 / MD-061 / MD-082 / NFR-013 を実装する。

```js
// js/lazy.js
export async function ensureMermaid()  // 未読込なら <script> を挿入して初期化
export async function ensureKaTeX()    // 未読込なら <script> と <link> を挿入
```

- 読み込みは `frontend/vendor/` 配下への相対パスで行う。外部 URL を参照しない（AR-020）。
- 一度読み込んだら `state.lazy` に記録し、以降は再読み込みしない（AR-021）。
- `doc.needsMermaid` / `doc.needsKaTeX` が false の文書では**呼び出さない**。この条件分岐が NFR-013 の実体である。
- 読み込みと描画は本文の表示をブロックしない。`renderDocument` の完了後に非同期で実行する（NFR-012）。

### IMP-231: Mermaid の初期化 **MUST**

```js
mermaid.initialize({
  startOnLoad: false,
  securityLevel: 'strict',   // MD-081
  theme: state.theme === 'dark' ? 'dark' : 'default',
});
```

- `startOnLoad: false` とし、描画対象を明示的に指定する。
- 描画対象は `.code-block[data-mermaid] pre.mermaid-source`。
- 描画に失敗したブロックは、元のソースをコードブロックとして残し、エラー内容を併記する（FR-023）。1 つの失敗が他のブロックの描画を止めないよう、ブロック単位で例外を捕捉する。
- テーマ切り替え時は、`mermaid.initialize` をやり直したうえで、保存しておいた `data-source` から再描画する（FR-070）。

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

## 12.5 操作（IMP-240 系）

### IMP-240: ペインの開閉とリサイズ **MUST**

FR-034 / FR-043 / UI-030 / UI-040 を実装する。

```js
// js/panes.js
export function togglePane(name)     // 'outline' | 'filetree'
export function setPaneWidth(name, px)
export function applyResponsive()    // ウィンドウ幅に応じた一時的な非表示（IMP-246）
```

- 開閉は `hidden` 属性の切り替えと、対応するリサイザの表示切り替えで行う。
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
```

- **本文を表示している状態でのみ開く。** 状態画面を表示中（`welcome` / `confirm-large` / `too-large` / `render-error`）は `Ctrl+F` を無視する（DSP-300）。
- 走査対象は `#markdown` のテキストノードのみ。`<script>` や属性値は対象外。
- **フロントエンドが後から描いた領域を走査から外す。** 対象は `svg`（Mermaid の描画結果。HTML の `<mark>` を差し込むと図が壊れる）と `.katex`（KaTeX の描画結果。MathML と HTML に同じ文字が二重に入っており、包むと数式が崩れるうえ件数も倍になる）。どちらも Go が出力した本文ではなく、原文は `data-source` と TeX ソースとして別に残っている。
- **テキストノードは先にすべて集めてから包む。** 包む処理はテキストノードを分割するため、走査しながら変更すると同じ箇所を二重に処理する。
- 解除時は、`<mark>` を外したあとに親要素へ `normalize()` を呼び、分割したテキストノードを 1 つへ結合し直す。これを省くと、次の検索で分割の境界をまたぐ語が見つからなくなる。
- ハイライトは `<mark class="search-hit">` で包む方式とし、原文の DOM 構造を壊さないよう、テキストノードの分割のみで実現する。要素の入れ子構造を変更しない。
- 現在位置のヒットには `search-hit` に加えて `search-hit-current` を付与する。配色は DSP-161 で定める。クラス名を 2 種に分けることで、移動時は付け替えだけで済み、DOM の作り直しが起きない。
- 検索終了時（`Esc`・文書切り替え・再描画）は、包んだ `<mark>` を必ず解除して元のテキストノードへ戻す。解除処理を持たないハイライト実装を採らない。
- 大文字小文字を区別しない比較には `toLowerCase()` を用いる。正規表現でユーザ入力を直接使わない（メタ文字の混入を避けるため）。
- 200 件を超えるヒットがある場合もハイライトは全件に付ける。件数表示は実数を出す。
- `find` の直後は、**本文ペインの上端以降にある最初のヒット**を現在位置とする。常に先頭へ戻すと、入力を 1 文字足すたびに文書の冒頭へ引き戻される。
- `jump` は端で反対側へ回り込む。ヒットが 1 件でも操作が空振りしない。
- **検索を開いていないときの `jump` は何もせず、`false` を返す**（IMP-244）。`Enter` は検索が閉じていてもこの経路へ来るため、`preventDefault` してしまうとフォーカス中のボタンを `Enter` で実行できなくなる（UI-021）。

### IMP-242: 表示倍率 **MUST**

FR-081 を実装する。

```js
// js/zoom.js
export function applyZoom(percent) // 画面へ反映するだけ。保存しない
export function setZoom(percent)   // 50..300、10 刻みに丸めて反映し、保存する
export function stepZoom(delta)    // +1 / -1（1 段 = 10 %）
export function initZoom()         // Ctrl + ホイールを配線する
```

- 適用は `#markdown` の CSS カスタムプロパティ `--zoom` を更新することで行い、`font-size` を `calc(16px * var(--zoom) / 100)` として与える（DSP-021）。
- ツールバー・サイドペイン・ステータスには適用しない（FR-081）。
- `Ctrl` + ホイールは `wheel` イベントで `ctrlKey` を見て処理し、`preventDefault()` でブラウザ既定のズームを抑止する。リスナは `window` に置く。本文の外（ペインやツールバーの上）でも WebView 既定の拡大が起きてはならない（AR-060）。
- **起動時の適用と操作による変更を別の関数に分ける。** `applyZoom` は反映のみを行い、保存は `setZoom` から `saveConfig`（IMP-210）を呼ぶ。起動時に保存すると、利用者が変更していない値を書き戻すことになる。
- 値が変わらないときは何もしない。上限・下限に張り付いた状態でキーを押し続けたときに、同じ値の保存を送り続けない。
- 100 % 以外のときだけステータス領域へ倍率を出す（FR-081, DSP-150）。

### IMP-243: テーマ **MUST**

FR-070 / UI-105 を実装する。

```js
// js/theme.js
export function applyTheme(theme)  // 'light' | 'dark'。反映のみ。保存しない
export function toggleTheme()      // 切り替えて反映し、保存する
```

- `#app` の `data-theme` 属性を書き換えるだけで全体に反映する。CSS 変数の切り替えで完結させ、要素の再生成や本文の再変換を行わない（UI-105）。これにより DSP-370 が求める維持（スクロール位置・検索状態・ツリー・アウトライン・倍率）は、何もしなくても成り立つ。
- Mermaid のみ再描画が必要（IMP-231）。再描画は待たない。図の描画で画面全体の切り替えを遅らせない。
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
- 入力欄（検索バー）にフォーカスがある間は、テキスト編集に関わるキーを素通しする（UI-090）。**素通しさせるのは編集に関わるものだけとする。** UI-090 は「テキスト編集に関わるキーを優先する」と定めているのであって、入力中はすべてのショートカットを止める、とは定めていない。`Ctrl+Shift+T` などは入力中も有効とする。
- **`preventDefault()` はリスナ側で一括して呼ぶ。** 個々のハンドラに書き漏らす余地を残さない。ただし**ハンドラが `false` を返したときは「何もしなかった」とみなし、既定の動作を止めない**。検索が閉じているときの `Enter` が、フォーカス中のボタンの実行（UI-021）を妨げないようにするためである。
- **1 文字のキーは小文字へ揃えて表を引く。** `Shift` と `CapsLock` で `KeyboardEvent.key` の大小が変わるためであり、`Shift` の有無は修飾子側で区別する。CapsLock が入っているだけで `Ctrl+O` が効かない、という事態を避ける。
- 修飾子の順序は `Ctrl` → `Alt` → `Shift` に固定し、表の表記もこれに揃える。
- **倍率のキーには別名を並べる。** US 配列では `+` が `Shift`+`=`、JIS 配列では `Shift`+`;` であり、テンキーの `+` には `Shift` が付かない。どの経路でも同じ動作になるよう、実際に届く組み合わせをすべて表に書く。表以外の場所で例外を作らない。
- IME の変換中（`event.isComposing`）は何も割り当てない。変換確定の `Enter` を検索の移動として拾わないため。
- ショートカットの定義は 1 箇所の配列にまとめ、ツールチップ（IMP-290）とキー割り当てが同じ定義を参照する。**キーの表記が 2 箇所に分かれることを避ける。**
- `Ctrl+C`（FR-062）はハンドラを結び付けず、WebView の既定に任せる。`Alt+F4` と閉じるボタンは OS とウィンドウマネージャが処理する。`Ctrl+Q` のみアプリケーション側で受け、`api.quit()`（IMP-310）を呼ぶ。

### IMP-245: ドラッグ＆ドロップ **MUST**

FR-011 / UI-070 を実装する。

```js
// js/dnd.js
export function initDnd()   // ドラッグ中の表示だけを配線する
```

- Wails のファイルドロップ機能（`OnFileDrop`）で**絶対パス**を受け取る。HTML5 の `DataTransfer` からパスは得られないため、そちらに依存しない。
- **`OnFileDrop` は既定では呼ばれない。** Wails の起動オプションに `DragAndDrop: &options.DragAndDrop{EnableFileDrop: true}` を渡す必要がある。これを忘れると、コールバックを登録してもドロップが一切届かない。
- **受け口となる要素は CSS で宣言する。** Wails はドロップ地点の要素の計算済みスタイルに `--wails-drop-target: drop`（既定のプロパティ名と値）があるかで受け付けるかを決める。カスタムプロパティは継承するため、`#app` に 1 度だけ置けばウィンドウ全体が対象になる（UI-070, FR-011）。
- **`#dropzone` は `pointer-events: none` とする。** Wails は `document.elementFromPoint` でドロップ地点の要素を求めるため、全面を覆うオーバーレイがマウスイベントを受け取ると、その下の要素を検出できずドロップが無効になる。
- `dragenter` / `dragover` / `dragleave` は、オーバーレイ（`#dropzone`）の表示制御にのみ使う。
- `dragover` と `drop` では自分でも `preventDefault()` を呼ぶ。Wails のランタイムも同じことを行うが、**WebView 内でページ遷移を起こさないという規約（AR-060）を外部のランタイムの実装に委ねない。**
- OS からのファイルのドラッグかを `dataTransfer.types` に `Files` が含まれるかで判定する。本文中のテキストを選択して動かした場合など、ウィンドウ内で完結するドラッグではオーバーレイを出さない。
- `dragleave` はウィンドウ内の要素間移動でも発生するため、カウンタ方式で入れ子の出入りを数え、0 になったときだけオーバーレイを隠す。
- 受け取ったパスの判定（Markdown か、ディレクトリか）は Go 側で行う（IMP-313）。

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
- ファイルツリーは抑制の対象としない（UI-026）。
- 抑制中に利用者がトグルボタンを押した場合は、`outlineVisible` を通常どおり切り替える。抑制が解けたときにその値が反映される。

### IMP-247: ツールチップ **MUST**

UI-024 / DSP-102 を実装する。

- **ブラウザ既定の `title` 属性を使わない。** DSP-102 はボタンの下 6px・遅延 400ms・反転色・折り返しなしという具体値を定めているが、`title` は見た目も遅延も表示位置も OS が決めており、いずれも満たせない。英日併記（`Open / 開く (Ctrl+O)`）で横に長くなるため、折り返さないことが特に効く。
- 対象は**ツールバーのボタンだけ**とする。ツリー・アウトライン・ステータスの「全文をツールチップで示す」（DSP-113, DSP-151）は隠れた文字を読ませるためのものであり、`title` のままでよい。
- 文言は `data-tip` 属性に置く（`toolbar.js` の `setTip`）。同じ文字列を `aria-label` にも与える（IMP-295）。
- 描画する要素は `#tooltip` の 1 つだけを使い回す。ボタンごとに持たせない。
- 配線はツールバーへの委譲で行う。ボタンごとにリスナを置くと、文言が変わるボタン（テーマ。IMP-243）で付け替えが要る。
- 位置はボタンの下・水平中央。**ウィンドウの外へ出る場合は左右に寄せて収める。** 左端の `Open` と右端の `?` は、中央に置くと枠外へ出る。
- `pointer-events: none` を与える。ツールチップがポインタを受け取ると `pointerout` が発生して点滅する。
- 押下時・ウィンドウのフォーカス喪失時・キー操作時に消す。トグルの状態が変わると文言が古くなるため（IMP-243）。

## 12.6 状態画面とダイアログ（IMP-250 系）

### IMP-250: 状態画面 **MUST**

UI-052 を実装する。

```js
// js/overlay.js
export function showStateScreen(kind, params)
// kind: 'welcome' | 'confirm-large' | 'too-large' | 'render-error'
export function hideStateScreen()
```

- `#state-screen` に描画し、`#markdown` は空にする。
- `confirm-large` の `Open anyway` ボタンは、押下で `api.openConfirmed(path)` を呼ぶ（IMP-310, IMP-314）。
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
- **Go を呼ぶのは `main.js` の役目とし、`overlay.js` は受け取った `AboutDTO` を描くだけにする**（IMP-201）。リンクの処理も `onLink` として受け取り、本文中のリンクとまったく同じ経路（IMP-312）へ渡す。
- 表示中は背後をフォーカストラップし、`Tab` がダイアログ外へ出ないようにする。端に来たときだけ折り返し、途中では既定の移動に任せる。
- **キーボードのショートカットも止める**（UI-100 の「背後のメインウィンドウの操作を受け付けない」）。暗幕はマウスしか塞がない。`Esc` だけは通す。止めるときも既定の動作は抑止する（`false` を返さない。IMP-244）。そうしないと `Ctrl` + `+` が WebView 自身のページ拡大として処理される。
- `Esc`、閉じるボタン、またはオーバーレイ**そのもの**のクリックで閉じる。中身のクリックで閉じないよう、対象が `#overlay` 自身であることを確かめる。
- 閉じたら**フォーカスを `#btn-about` へ戻す。** 開く前に別の操作要素へフォーカスがあった場合だけ、そこへ戻す。`F1` で開いたときの直前のフォーカスは `<body>` であることが多く、そのまま戻すとどこにもフォーカスがない状態になる。
- リポジトリの URL には `href` を与えず、クリックと `Enter` / `Space` で `onLink` を呼ぶ（UI-102）。**遷移し得ない形にしておく。**
- ライセンス欄だけがダイアログの残りの高さを受け持つ。基準は 240px とし、**伸びはせず、収まらないときだけ縮む**（DSP-170 の「ウィンドウが小さい場合は内側に収まるよう縮小」）。
- ライセンス全文は `<textarea readonly>` ではなく `<pre>` + `overflow:auto` で表示し、選択・コピーを可能にする（UI-101）。
- 見出しの横にアプリケーションアイコンを `<img src="/appicon.png" alt="">` として表示する（UI-025, DSP-171）。装飾目的のため `alt` は空とし、読み上げの対象にしない。パスは IMP-160 が配信するもので、外部 URL を参照しない。

## 12.7 UI 文言（IMP-290 系）

### IMP-290: 文言の一元定義 **MUST**

UI-024 / AR-050 を実装する。

```js
// js/strings.js
export const S = {
  // ツールチップのみ英日併記（UI-024）。
  // キー表記は含めない。shortcuts.js の定義から組み立てる（IMP-244）
  tipOpen:      'Open / 開く',
  tipReload:    'Reload / 再読み込み',
  tipThemeDark: 'Dark theme / ダークテーマ',
  tipThemeLight:'Light theme / ライトテーマ',
  tipOutline:   'Outline / アウトライン',
  tipFileTree:  'File tree / ファイルツリー',
  tipAbout:     'About / アプリケーション情報',

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
  dropHint:    'Drop a Markdown file to open',
  outsideTree: '(outside tree)',
  treeMore:    (n) => `… and ${n} more`,          // DSP-112

  // コードブロック（DSP-251）。アイコンだけのボタンに読み上げ名を与える（IMP-295）
  copy:        'Copy',

  // ステータス領域（DSP-150）
  statusLines: (n) => `${n} lines`,
  statusZoom:  (z) => `${z}%`,

  // 状態画面（DSP-181）
  welcomeTitle:   'Open a Markdown file',
  welcomeHintOpen:'Press Ctrl+O to choose a file',
  welcomeHintDrop:'Or drop a Markdown file onto this window',
  welcomeHintTree:'Use the file tree to browse documents',
  openAnyway:     'Open anyway',
  largeTitle:     'This file is large.',
  largeHint:      'Rendering may take a while.',
  tooLarge:       (limit) => `Maximum size is ${limit}.`,
  renderError:    'Failed to render this document.',

  // 情報ダイアログ（DSP-171）
  appName:      'MarkView',                    // 見出し。固有名だが画面に出る
  aboutVersion: (v, c) => `Version ${v} (${c})`,
  aboutVendor:  (name, version) => `${name} ${version}`,  // Bundled 行の 1 項目
  aboutAuthor:     'Author',
  aboutRepository: 'Repository',
  aboutLicense:    'License',
  aboutEnvironment:'Environment',
  aboutBundled:    'Bundled',
  aboutLicenses:   'Third-party licenses',
  close:           'Close',

  // エラー（IMP-315 の Kind に対応）
  errNotFound:    (p) => `File not found: ${p}`,
  errPermission:  (p) => `Cannot access: ${p}`,
  errNotMarkdown: (p) => `Not a Markdown file: ${p}`,
  errLinkNotFound:(h) => `Link target not found: ${h}`,
  errClipboard:   'Failed to copy.',
  errRemoved:     (p) => `File was deleted: ${p}`,
  warnEncoding:   'Some characters were replaced.',
};
```

- 引数を取る文言は関数として定義し、呼び出し側で文字列を組み立てない。表示文言の全体像がこのファイルだけで読める状態を保つ。
- `IMP-315` の `Kind` と、ここのキーを 1 対 1 で対応させる。未知の `Kind` を受け取った場合は `ErrorDTO.Message` をそのまま表示する。
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

## 12.8 アクセシビリティ（IMP-295 系）

### IMP-295: 最低限の対応 **SHOULD**

- ツールバーの各ボタンに `aria-label` を与える。値はツールチップと同じ文字列とする。
- トグルボタンは `aria-pressed` を状態に応じて更新する（UI-021）。
- ツリーは `role="tree"` / `role="treeitem"` を用い、`aria-expanded` を更新する。
- フォーカスリングを消さない。`outline: none` を無条件に指定しない（DSP-016）。
- 状態画面・ダイアログの表示時に、フォーカスを内部の操作要素へ移す。

## 12.9 要求一覧

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
| IMP-230 | Mermaid と KaTeX の遅延ロード | MUST |
| IMP-231 | Mermaid の初期化 | MUST |
| IMP-232 | KaTeX の初期化 | MUST |
| IMP-240 | ペインの開閉とリサイズ | MUST |
| IMP-241 | 検索 | MUST |
| IMP-242 | 表示倍率 | MUST |
| IMP-243 | テーマ | MUST |
| IMP-244 | ショートカット | MUST |
| IMP-245 | ドラッグ＆ドロップ | MUST |
| IMP-246 | ウィンドウ幅に応じた一時的な非表示 | MUST |
| IMP-247 | ツールチップ | MUST |
| IMP-250 | 状態画面 | MUST |
| IMP-251 | 情報ダイアログ | MUST |
| IMP-290 | UI 文言の一元定義 | MUST |
| IMP-295 | アクセシビリティの最低限の対応 | SHOULD |
