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
│   ├── lazy.js             Mermaid / KaTeX / PlantUML の遅延ロード
│   ├── status.js           ステータス領域
│   ├── overlay.js          情報ダイアログ・エディタ選択ダイアログ・状態画面
│   ├── editors.js          エディタ選択ダイアログの中身（IMP-252）
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
    <button id="btn-edit"     class="tb-btn" type="button"></button>
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
      <div class="searchbar-anchor">
        <div id="searchbar" class="searchbar" hidden></div>
      </div>
      <article id="markdown" class="markdown-body"></article>
      <div id="state-screen" class="state-screen" hidden></div>
    </section>
  </main>

  <footer id="statusbar" class="status">
    <span id="status-path" class="status-path"></span>
    <span id="status-meta" class="status-meta"></span>
    <span id="status-message" class="status-message" hidden></span>
  </footer>

  <div id="overlay" class="overlay" hidden></div>
  <div id="dropzone" class="dropzone" hidden></div>
  <div id="tooltip" class="tooltip" hidden></div>
</div>
```

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
- **`index.html` に利用者向けの文言を書かない。** ペイン見出しの `Files` / `Outline` を含め、文言は `js/strings.js` から与える（IMP-290）。上の骨格でテキストが空の要素は、すべて実行時に埋める。
- テーマは `#app` の `data-theme` 属性で切り替える（DSP-011）。
- 本文は `.markdown-body` に挿入する。`github-markdown-css` が想定するクラス名に合わせる。
- **`#viewer` に `tabindex="-1"` を与える。目的は 2 つある。** 負値のため `Tab` の巡回順には入らない。
  1. 検索バーを閉じたときにフォーカスを本文へ戻す（UI-080）
  2. **文書を表示したときにフォーカスを本文ペインへ移す**（UI-051, IMP-220）。**これが無いとキーボードでスクロールできない**（[調査報告](../bugs/2026-09-05-bug-007-viewer-focus-on-open.md)）
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
| `icon-open` | ツールバー「開く」、welcome 画面（DSP-181） | `file-directory` |
| `icon-reload` | ツールバー「再読み込み」 | `sync` |
| `icon-moon` / `icon-sun` | ツールバー「テーマ切り替え」（状態で入れ替え） | `moon` / `sun` |
| `icon-outline` | ツールバー「アウトライン」 | `list-unordered` |
| `icon-filetree` | ツールバー「ファイルツリー」、ツリーの展開済みディレクトリ | `file-directory-open-fill` |
| `icon-pencil` | ツールバー「エディタで開く」（UI-020, FR-090） | `pencil` |
| `icon-about` | ツールバー「アプリケーション情報」 | `question` |
| `icon-dir` | ツリーの折りたたみ状態のディレクトリ（DSP-112） | `file-directory` |
| `icon-file` | ツリーのファイル（DSP-112） | `file` |
| `icon-chevron-right` / `icon-chevron-down` | ツリーの展開矢印（DSP-112） | `chevron-right` / `chevron-down` |
| `icon-search` | 検索バーの先頭（DSP-160） | `search` |
| `icon-chevron-up` | 検索バー「前へ」（DSP-160） | `chevron-up` |
| `icon-close` | 検索バー「閉じる」、情報ダイアログ「×」、エディタ選択ダイアログ「×」（DSP-160, DSP-170, DSP-172） | `x` |
| `icon-copy` / `icon-check` | コードブロックのコピーボタン（FR-061, DSP-252） | `copy` / `check` |
| `icon-note` | Alerts: NOTE（DSP-261） | `info` |
| `icon-tip` | Alerts: TIP | `light-bulb` |
| `icon-important` | Alerts: IMPORTANT | `report` |
| `icon-warning` | Alerts: WARNING、確認画面（DSP-181） | `alert` |
| `icon-caution` | Alerts: CAUTION、エラー画面（DSP-181） | `stop` |
| `icon-link` | 見出しのアンカー（MD-020, IMP-227, DSP-023） | `link` |

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
  lazy: { mermaid: false, katex: false, plantuml: false }, // 読み込み済みか
};
```

- 状態の**正**は Go 側（IMP-190）に置く。フロントエンドの `state` は描画のための写しであり、永続化に関わる値（テーマ・ペイン幅・表示状態）を変更したときは Go 側へ通知する（IMP-310）。
- **`state.zoom` は例外で、フロントエンドだけが持つ。** 倍率は保存しないため（UI-111, UI-115）Go 側に対応するフィールドがなく、`configPatch` にも含めない。
- 表示中の文書パスをフロントエンドで `localStorage` 等に保存しない（NFR-042）。
- **保存しない一時的な状態は `state` に置かない。** 幅不足によるアウトラインの一時的な非表示（IMP-246）は `panes.js` のモジュール変数とする。`state` は「Go 側の状態の写し」であり、そこに保存しない値を混ぜると、`configPatch` が何を送るべきかが読めなくなる。

`state` を変更したあとの通知は、次の 1 関数を必ず経由する。

```js
// js/state.js
export function configPatch()   // ConfigDTO（IMP-303）を組み立てる
export function saveConfig()    // 現在の状態を Go 側へ通知する
```

- **`api.updateConfig` を直接呼ばない。** バインドメソッドの呼び出しは Wails がメッセージごとに処理するため、立て続けに 2 つ投げると**到着順が入れ替わりうる**。実際に、ペインの開閉と別の設定変更を続けて行うと先に投げたほうが後に処理され、新しい値が古い値で上書きされた。`saveConfig` は前の応答を待ってから次を送ることで順序を保つ。
- 送信待ちが既にあるときは新たに積まない。`ConfigDTO` は差分ではなく状態の全体であり、待っている 1 つが送信時点の最新を読めば足りる。
- **`configPatch` の `theme` は、利用者が自分で切り替えるまで空文字とする**（FR-071, IMP-303）。`state.theme` は Go 側が OS 設定まで解決した値であり、それをそのまま返すと「まだ選んでいない」状態が最初の保存で失われ、以後 OS 設定を変えても追従しなくなる。

### IMP-211: 起動順序 **MUST**

```js
// js/main.js
async function boot() {
  const init = await api.getInitialState(); // 13 章 InitialStateDTO
  applyTheme(init.config.theme);
  applyPanes(init.config);       // 倍率は復元しない。常に 100 %（UI-111, IMP-242）
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
5. 見出しにアンカーを付与する（IMP-227）。
6. 画像の読み込み失敗を捉える配線を行う（IMP-226）。
7. スクロール連動の監視対象を作り直す（IMP-222）。
8. `doc.needsMermaid` / `doc.needsKaTeX` / `doc.needsPlantUML` に応じて遅延ロードを起動する（IMP-230）。
9. スクロール位置を設定する（13 章 `ScrollDTO` の `mode` に従う）。
10. アウトライン（IMP-224）とステータス（DSP-150）を更新する。
11. **本文ペインへフォーカスを移す**（UI-051）。

3〜6 は DOM 走査を伴うため、`#markdown` を 1 回だけ走査してまとめて処理してよい（NFR-011）。

**6 は 1 より後であれば順序を問わない**が、遅らせすぎてはならない。挿入から配線までの間に読み込みが終わった画像は `error` を受け取れないため、IMP-226 は配線時にすでに失敗しているものを別途拾う。

**手順 11 は 2 つの条件を満たす。** どちらを落としても別の要求が壊れる。

| # | 条件 | 落とすと |
| --- | --- | --- |
| 1 | **`focus({ preventScroll: true })` とする** | フォーカス移動に伴うスクロールが、直前の手順 9（`ScrollDTO`。DSP-350）を打ち消しうる。`F5` で位置が維持されず（FR-015）、`Alt+←` で復元されない（FR-051） |
| 2 | **ダイアログ（情報・エディタ選択）を表示している間は奪わない** | ダイアログは開いたままファイル更新の自動検知（FR-014）を受けうる。**背後の本文へフォーカスが移り、フォーカストラップ（IMP-251, IMP-252）が破れる** |

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
> DSP-350 と同じ形の表をもう 1 つ増やす**。ツリーのキーボード操作（UI-030）を実装するときに
> 改めて判断する。

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

- `root.querySelectorAll('.code-block')` を走査し、各要素に `<button class="copy-btn">` を追加する。
- コピー対象の取得順序:
  1. `data-source` 属性があればその値（Mermaid / PlantUML ブロック。描画後に `<pre>` が SVG へ置き換わるため必須。IMP-115, IMP-119）
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
- **移動先が折りたたみ（`<details>`。[MD-026](04-markdown.md)）の中にある場合は、祖先の `<details>` を開いてからスクロールする**（[FR-050](02-functional.md)）。閉じたままの要素は `scrollIntoView` の対象にならず、**大きさを返すのにスクロールは起きない**。**IMP-241 と同じ関数を使う**（`util.js` に置く）。**文書を開いた直後のアンカー復元（IMP-302 の `anchor` モード）も同じ経路を通す。**
- `target="_blank"` を含むリンクも同じ経路で処理する。

### IMP-224: アウトラインの構築 **MUST**

```js
// js/outline.js
export function renderOutline(headings)
```

- `DocumentDTO.headings`（Go 側が生成、IMP-117）をそのまま用いる。フロントエンドで DOM から見出しを抽出しない。抽出規則を 2 箇所に持たないため。
- 見出しが 0 件の場合、`strings.noHeadings` を表示する（FR-040）。
- インデントは**相対的な深さ**に応じた CSS カスタムプロパティで与える（DSP-113）。深さはレベルそのものではなく、`#` の次が `###` でも 1 段だけ下げる（FR-040 の「出現順を保ったまま相対的な深さで表示する」）。文字サイズはレベルで決める（DSP-113）ため、両者を別の属性で持つ。
- **項目のクリックで移動する際、見出しが折りたたみ（`<details>`。[MD-026](04-markdown.md)）の中にあれば、祖先の `<details>` を開いてからスクロールする**（[FR-041](02-functional.md)）。**アウトラインは本文の見出しをすべて挙げるため、折りたたみの中の見出しも項目として並ぶ。** 閉じたままの要素は `scrollIntoView` の対象にならず、**大きさを返すのにスクロールは起きない**。**IMP-241 / IMP-223 と同じ関数を使う**（`util.js` に置く）。

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

### IMP-226: 画像の読み込み失敗 **MUST**

FR-022 / DSP-123 を実装する。

```js
// js/viewer.js
export function markBrokenImages(root)
```

- `root.querySelectorAll('img')` を走査し、各要素に `error` を配線する。`error` で、**その `<img>` を `<span class="img-broken">` へ置き換える**（DSP-123）。
- **配線した時点ですでに失敗しているものを別途拾う。** `img.complete && img.naturalWidth === 0` なら、その場で置き換える。
- **代替テキストは自前で描く。** `span.textContent = img.alt` とする。**`alt` は文書由来の文字列であり、`innerHTML` に渡してはならない**（IMP-220）。
- **枠と色は CSS が与える**（DSP-123）。フロントエンドが用意するのは**要素とテキストだけ**であり、体裁を JavaScript で組み立てない。
- ローカル画像とリモート画像（MD-071）を区別しない。FR-022 はどちらも同じ扱いと定めている。
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
// js/viewer.js
export function decorateHeadings(root)
```

- `root.querySelectorAll('h1[id], h2[id], …, h6[id]')` を走査し、各見出しの**先頭の子**として次を挿入する。

```html
<a class="heading-anchor" href="#{id}" aria-label="…"><svg class="icon"><use href="#icon-link"></use></svg></a>
```

- `href` の値は見出しの `id`（MD-021 のスラッグ）をそのまま用いる。**Go 側が生成した ID を組み替えない。**
- `aria-label` は `strings.js` の `headingAnchor` を `setAttribute` で与える（IMP-290, IMP-295, UI-024）。
- **クリックの処理を書かない。** 本文中のリンクは IMP-223 が捕捉し、フラグメントは自前でスクロールに変える。ここで独自のハンドラを足すと経路が 2 つになる（AR-060）。
- `id` を持たない見出しには付けない。Go 側は必ず付与する（IMP-117）が、防御的に扱う。

> [!NOTE]
> **アウトラインと検索に影響しない。** アウトラインは `DocumentDTO.headings` を用いて DOM を読まない（IMP-224）。検索はテキストノードを走査するが、挿入するのは `<svg><use>` だけでテキストノードを持たないため、`textContent` も走査対象も変わらない。
>
> MD-020 は **SHOULD** であり、この機能自体は必須ではない。それでも実装するのは、MD-021 が GitHub 互換のスラッグを自前生成しているのに、**それを利用者へ見せる入口が他に無い**ためである。

## 12.4 遅延ロード（IMP-230 系）

### IMP-230: Mermaid・KaTeX・PlantUML **MUST**

AR-021 / MD-061 / MD-082 / MD-085 / NFR-013 を実装する。

```js
// js/lazy.js
export async function ensureMermaid()  // 未読込なら <script> を挿入して初期化
export async function ensureKaTeX()    // 未読込なら <script> と <link> を挿入
export async function ensurePlantUML() // 未読込なら viz-global.js → plantuml.js の順で読む
```

- 読み込みは `frontend/vendor/` 配下への相対パスで行う。外部 URL を参照しない（AR-020）。
- 一度読み込んだら `state.lazy` に記録し、以降は再読み込みしない（AR-021）。
- `doc.needsMermaid` / `doc.needsKaTeX` / `doc.needsPlantUML` が false の文書では**呼び出さない**。この条件分岐が NFR-013 の実体である。
- **`ensurePlantUML()` は読み込む順序を守る**。`viz-global.js` を先に、`plantuml.js` を後にする（AR-020, IMP-233）。**前者の読み込みに失敗したら、後者を読まないで false を返す。** Graphviz 不在のまま描こうとすると処理系ごと止まる（IMP-233 の 4）。
- 読み込みと描画は本文の表示をブロックしない。`renderDocument` の完了後に非同期で実行する（NFR-012）。

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

### IMP-233: PlantUML の初期化と描画 **MUST**

FR-024 / MD-083 / MD-084 を実装する。**Mermaid とは API の形が違うため、同じやり方では書けない。**

```js
// js/lazy.js
// 順序を守る。viz-global.js がグローバルに Viz を置き、plantuml.js がそれを見る。
await load("vendor/plantuml/viz-global.js");   // 1
const puml = await import("vendor/plantuml/plantuml.js");  // 2

puml.render(lines /* string[] */, targetElementId, { dark: state.theme === "dark" });
```

**実装で押さえる点は 5 つある。**

| # | 処理系の振る舞い | 実装への帰結 |
| --- | --- | --- |
| 1 | **`render()` は `undefined` を返し、Promise も返さない。** SVG はあとから対象要素へ書き込まれる | **完了を DOM で見るしかない。** `MutationObserver` で対象要素を監視し、**タイムアウトを設ける**。`await` して終わりにはできない。**SVG 以外が書き込まれたら、タイムアウトを待たずに短い猶予で切り上げる**（下記） |
| 2 | **出力先を要素の id で指定する。** 要素そのものを渡せない | 図ごとに一意な id を振る。**文書を切り替えても衝突しない値にする** |
| 3 | `renderToString` も export されているが、**どの引数の組でも `undefined` を返す** | 現状使えない。`render()` 経由でのみ取得する |
| 4 | **Graphviz を要する図を Graphviz 無しで描こうとすると、処理系ごと止まる。** その図だけでなく**以降のすべての描画が返ってこなくなる** | **`viz-global.js` の読み込みに失敗したら、PlantUML の描画を一切行わない。** 全ブロックをソースのまま残し、理由を表示する |
| 5 | **描画のたびに `document.getElementById('status')` を決め打ちで探し、見つけた要素の `textContent` を自分のログで上書きする。** 渡した要素とは無関係に、ページ全体を対象にする | **ページ側で `status` という id を使わない**（IMP-202 は `statusbar` とする）。**こちらから止める手立ては無い**——`plantuml.js` は改変できない（BR-042） |

- 描画対象は `.code-block[data-plantuml] pre.plantuml-source`。**`data-puml-error` を持つブロックは描画しない**（IMP-119 が拒んだもの）。
- **描画結果は `.plantuml-rendered` の中へ入れ、描かなかった理由は `.plantuml-error` へ出す。** Mermaid の `.mermaid-rendered` / `.mermaid-error`（IMP-231）と同じ形にそろえる。**この 2 つの名前は描画スモークテストが見る**（BR-054, E2E-109）ため、変えるときは `scripts/smoke/harness.js` も同じ変更で直す。
- 描画に失敗したブロックは、元のソースをコードブロックとして残し、理由を併記する。**1 つの失敗が他のブロックの描画を止めない**（IMP-231 と同じ）。
- **取り込み指令で拒まれたブロックの理由は、資産を読まずに表示する**（IMP-119, DSP-272）。それらは `needsPlantUML` を立てないため、**`needsPlantUML` を条件に描画関数を呼ぶと理由が出ない。** 描画関数を「描くものが無ければ資産を読まずに戻る」形にし、**条件を付けずに呼ぶ**。NFR-013 は早期の戻りで保たれる。
- **構文エラーは失敗ではない。** PlantUML はエラーを描いた SVG を返すので、**そのまま出す**（FR-024）。行番号と該当行を含むため、こちらで書き直すより情報量が多い。
- **4096 px を超える図は SVG ではなく例外のテキストが返る**（`Diagram too large for browser rendering: <幅>x<高さ> (max 4096)`）。これを検知して FR-110 の表示に回す。テキストをそのまま本文へ出さない（UI 文言は `strings.js`。IMP-290）。**`render()` は例外を投げない。** 正常に戻ったうえで、**出力先の要素へ例外のテキストが書き込まれる。** したがってこれは `try` / `catch` ではなく、**上の 1 の完了検知が「SVG 以外が入った」と判定する経路**で拾う（実測。[BUG-010](../bugs/2026-09-06-bug-010-plantuml-4096-testdata.md)）。**表示はどちらの経路でも `pumlUnsupported` であり、利用者から見た違いは無い**（DSP-272）。
- テーマ切り替え時は、保存しておいた `data-source` から `{ dark: ... }` を変えて**描き直す**（FR-070, IMP-243）。
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
> **描画は逐次行う。** Graphviz を要する図は 1 枚 400〜700 ms かかる（NFR-011）。全部を一気に投げても処理系はコルーチンで細切れに実行するため UI は固まらないが（メインスレッドの最大停止は 4 ms）、**完了検知の監視対象が図の数だけ同時に存在する状態を作らない**。

## 12.5 操作（IMP-240 系）

### IMP-240: ペインの開閉とリサイズ **MUST**

FR-034 / FR-035 / FR-043 / UI-030 / UI-040 を実装する。

```js
// js/panes.js
export function togglePane(name)     // 'outline' | 'filetree'。**表示になったかを返す**
export function setPaneWidth(name, px)
export function applyResponsive()    // ウィンドウ幅に応じた一時的な非表示（IMP-246）
```

- 開閉は `hidden` 属性の切り替えと、対応するリサイザの表示切り替えで行う。
- **ファイルツリーが非表示から表示になったら、ツリーを読み直す**（[FR-035](02-functional.md) の 1 番目の契機）。`filetree.js` の `loadTreeRoot` を呼ぶ。
  - **`panes.js` から `filetree.js` を呼ばない。** `togglePane` は「表示になったか」を返すだけとし、**契機の判断は `main.js` に置く**（IMP-201 の依存の明示）。ここで呼ぶと、ペインの開閉というひとつの関心にツリーの読み込みが混ざる。
  - **ツールバーのボタンとショートカットの両方が同じ関数を通るようにする。** 片方だけに足すと、経路によって挙動が変わる。
  - **表示になったときだけ呼ぶ。** 閉じる操作や、既に開いている状態では呼ばない。`ReadDir` は毎回ディスクを読む（IMP-310）ため、大きなディレクトリで引っかかる（NFR-020）。
  - `loadTreeRoot` はツリーを作り直すため、**利用者が開いていたディレクトリの展開状態は失われ、表示中の文書までの経路だけが開き直される**（`revealCurrent`。DSP-331）。FR-035 は展開状態の保持を求めていない。
- **再読み込み操作（[FR-015](02-functional.md)）でもツリーを読み直す**（FR-035 の 2 番目の契機）。`Reload()`（IMP-310）は表示中の文書を開き直すだけでツリーに触れないため、**フロントエンドが続けて `loadTreeRoot` を呼ぶ。**

> [!IMPORTANT]
> **FR-035 は再読み込みの契機を 3 つ定めている。担当を分けて書く。**
>
> | # | 契機 | 担当 |
> | --- | --- | --- |
> | 1 | ツリーペインを非表示から表示に切り替えたとき | **本 ID**（`togglePane` の呼び出し側） |
> | 2 | 再読み込み操作（FR-015）を行ったとき | **本 ID**（`reloadCurrent` の後段） |
> | 3 | ディレクトリノードを折りたたんでから再度展開したとき | [DSP-330](22-display-states.md)（展開のたびに `ReadDir` を呼ぶ） |
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
```

- **本文を表示している状態でのみ開く。** 状態画面を表示中（`welcome` / `confirm-large` / `too-large` / `render-error`）は `Ctrl+F` を無視する（DSP-300）。
- 走査対象は `#markdown` のテキストノードのみ。`<script>` や属性値は対象外。
- **フロントエンドが後から描いた領域を走査から外す。** 対象は `svg`（**Mermaid と PlantUML の描画結果**。HTML の `<mark>` を差し込むと図が壊れる）と `.katex`（KaTeX の描画結果。MathML と HTML に同じ文字が二重に入っており、包むと数式が崩れるうえ件数も倍になる）。いずれも Go が出力した本文ではなく、原文は `data-source` と TeX ソースとして別に残っている。**PlantUML の図は文字を多く含む**ため、除外を忘れると図の中の語が大量にヒットする。
- **折りたたみ（`<details>`。[MD-026](04-markdown.md)）の中は除外しない。** 上の 2 つは「原文が別に残っている描画の副産物」だが、**折りたたみの中身は原文そのもの**であり、除外の理由が当てはまらない。除外すると、件数が利用者の開閉で変わるうえ、**「文書にあるのに見つからない」**状態になる。折りたたみの中は「まだ読んでいない箇所」であり、検索で見つけたい場所そのものである（FR-080, UI-080）。
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
> **3 か所で同じ関数を使う**（`util.js` に置く。`search.js` に置くと `outline.js` との循環参照になる）。
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

- `#app` の `data-theme` 属性を書き換えるだけで全体に反映する。CSS 変数の切り替えで完結させ、要素の再生成や本文の再変換を行わない（UI-105）。これにより DSP-370 が求める維持（スクロール位置・検索状態・ツリー・アウトライン・倍率）は、何もしなくても成り立つ。
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

  - **コールバックは空でよい。** 結果はイベント（`document:opened` / `tree:root-changed` / `error`）で受け取る（IMP-320, IMP-322）。ここに処理を書くと経路が 2 つになる。
  - 第 2 引数（`useDropTarget`）は `true` とする。ただし**これが検査するのは JS 側のコールバックだけ**であり、Go 側のコールバックは検査を通らずに呼ばれる。
- **受け口となる要素は CSS で宣言する。** ドロップ地点の要素の計算済みスタイルに `--wails-drop-target: drop`（既定のプロパティ名と値）があるかで、Wails の**JS 側のコールバック**が呼ばれるかが決まる。カスタムプロパティは継承するため、`#app` に 1 度だけ置けばウィンドウ全体が対象になる（UI-070, FR-011）。**この宣言は Go 側のコールバックの条件ではない**（上記のとおり検査を通らない）。宣言を残すのは UI-070 の意図を DOM 上に残すためである。
- **`#dropzone` は `pointer-events: none` とする。** Wails は `document.elementFromPoint` でドロップ地点の要素を求めるため、全面を覆うオーバーレイがマウスイベントを受け取ると、その下の要素を検出できずドロップが無効になる。
- `dragenter` / `dragover` / `dragleave` は、オーバーレイ（`#dropzone`）の表示制御にのみ使う。
- `dragover` と `drop` では自分でも `preventDefault()` を呼ぶ。Wails のランタイムも同じことを行うが、**WebView 内でページ遷移を起こさないという規約（AR-060）を外部のランタイムの実装に委ねない。**
- OS からのファイルのドラッグかを `dataTransfer.types` に `Files` が含まれるかで判定する。本文中のテキストを選択して動かした場合など、ウィンドウ内で完結するドラッグではオーバーレイを出さない。
- `dragleave` はウィンドウ内の要素間移動でも発生するため、カウンタ方式で入れ子の出入りを数え、0 になったときだけオーバーレイを隠す。
- 受け取ったパスの判定（Markdown か、ディレクトリか）は Go 側で行う（IMP-313）。

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
- 対象は**ツールバーのボタンだけ**とする。ツリー・アウトライン・ステータスの「全文をツールチップで示す」（DSP-113, DSP-151）は隠れた文字を読ませるためのものであり、`title` のままでよい。
- 文言は `data-tip` 属性に置く（`toolbar.js` の `setTip`）。同じ文字列を `aria-label` にも与える（IMP-295）。
- 描画する要素は `#tooltip` の 1 つだけを使い回す。ボタンごとに持たせない。
- 配線はツールバーへの委譲で行う。ボタンごとにリスナを置くと、文言が変わるボタン（テーマ。IMP-243）で付け替えが要る。
- 位置はボタンの下・水平中央。**ウィンドウの外へ出る場合は左右に寄せて収める。** 左端の `Open` と右端の `?` は、中央に置くと枠外へ出る。
- `pointer-events: none` を与える。ツールチップがポインタを受け取ると `pointerout` が発生して点滅する。
- 押下時・ウィンドウのフォーカス喪失時・キー操作時に消す。トグルの状態が変わると文言が古くなるため（IMP-243）。

## 12.6 状態画面とダイアログ（IMP-250 系）

### IMP-248: ツリーのキーボード操作 **SHOULD**

UI-031 を実装する。

```js
// js/filetree.js
// initFileTree が #tree へ keydown を配線する（クリックと同じ 1 か所）
```

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

- **修飾キー（`Alt` / `Ctrl` / `Meta` / `Shift`）が押されているときは何もしない。** `Alt+←` / `Alt+→` は履歴の移動である（UI-090, IMP-244）。**ここで奪うと戻れなくなる。**
- **上の表の 4 行すべてで `preventDefault()` する**（キーとしては 5 つ）。`↑` / `↓` は既定でページをスクロールする。**それ以外のキーは素通りさせる。**
- 移動したら `scrollIntoView({ block: "nearest" })` で可視にする。**`block` を省かない**——既定値では本文ペインまで動きうる。
- フォーカスリングは **`.tree-item:focus-visible > .tree-row`** に出す（DSP-330, DSP-016）。**`li` に出すと、展開中は配下の木全体が囲まれる。** `base.css` の `:focus-visible` が全要素にリングを引くため、**`li` の側は打ち消す**——**同じ体裁を行へ移すだけであり、DSP-016 の「フォーカスリングを消さない」に反しない。**

> [!IMPORTANT]
> **`Enter` でファイルを開いたら、フォーカスは本文ペインへ移る**（UI-051, IMP-220 の手順 11）。
> **IMP-220 が「ツリーのキーボード操作を実装するときに改めて判断する」と書いていた点であり、ここで決める。**
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
export function showEditors(list)   // list: EditorListDTO（IMP-309）
export function hideEditors()       // 閉じたら true、開いていなければ false
export function isEditorsOpen()
```

**API はこの 3 つだけとし、`overlay.js` に置く。** ダイアログの中身（行の組み立てと選択の状態）は `editors.js` が持つ（IMP-011。`overlay.js` が 400 行の目安を大きく超えるため）。`editors.js` は `overlay.js` を `import` しない。開閉・フォーカスの復帰・`Tab` の制御は `overlay.js` の側にあり、`Browse` / `Open` / `Cancel` の処理は引数として渡す。**循環参照を作らない。**

- **情報ダイアログ（IMP-251）と同じ `#overlay` を使い、同じ規則に従う。** フォーカストラップ、`Esc` だけを通すキー制御、暗幕そのもののクリックで閉じる判定のいずれも共通とする。**2 つを同時に開かない。**
- **Go を呼ぶのは `main.js` の役目とし、`overlay.js` は受け取った `EditorListDTO` を描くだけにする**（IMP-201）。`Browse` と `Open` の処理は `deps` として受け取る。
- 一覧は `<input type="radio" name="editor">` のリストとする。`Available` が偽の行は `disabled` とし、`(not installed)` を添える（UI-103）。**行を消さない。**
- 描画順は `EditorListDTO.editors` の順そのままとする。**フロントエンドで並べ替えない**（IMP-309）。
- 開いた時点のフォーカスは、`Selected` の行があれば `Open` ボタン、無ければ一覧の先頭の選択可能な行に置く（UI-103）。
- `Open` は、選択が無い間 `disabled` とする。**`custom` の行そのものは常に選べる**（UI-103）が、`Browse` で実行ファイルが選ばれるまでは `Open` を `disabled` のままとする。行まで選べなくすると `Browse` へ辿り着けない。
- `Open` の活性は「選ばれていて、かつ `Available` が真」という**1 つの式だけで決める。** 行の種類ごとに条件を書くと `custom` が別扱いになり、いつか食い違う。フロントエンドは実行ファイルのパスを見ない（NFR-035 の 3）。
- `Browse` は `deps.onBrowse()` を呼び、返ってきた `EditorListDTO` で**一覧全体を描き直す**。差分更新しない。行数も選択状態も Go 側が決める（IMP-309）。
- 閉じたら**フォーカスを `#btn-edit` へ戻す**（IMP-251 と同じ理由）。
- `EditorListDTO.Error` があるときは**ウィンドウを出さない。** 選べるものが 1 つも無いウィンドウを出しても意味がなく、理由はステータス領域へ出す（IMP-315）。`Browse` の戻りが同じ状態だった場合は**描き直さず、いま出ている一覧を保つ。** 描き直すと、出ていた一覧が消えて何も選べないウィンドウが残る。
- 表示・非表示は `hidden` 属性で切り替える。`style.display` を直接触らない（IMP-202）。
- **文言は `strings.js` から採る**（IMP-290）。エディタ名は `EditorDTO.name` をそのまま `textContent` で入れる。Go 側が組み立てた文字列であり、`innerHTML` へは渡さない（IMP-220）。

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
  tipEdit:      'Edit / 編集',
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

  // 見出しのアンカー（IMP-227, DSP-023）。アイコンだけのリンクに読み上げ名を与える
  headingAnchor: 'Link to this section',

  // ステータス領域（DSP-150）
  statusLines: (n) => `${n} lines`,
  statusZoom:  (z) => `${z}%`,
  statusEditor:(name) => `Opened in ${name}`,   // FR-090, DSP-151

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

  // PlantUML の図を描かなかった理由（FR-024, DSP-272）。図の代わりに本文中へ併記する
  pumlInclude:     'Include directives are not supported.',
  pumlUnsupported: 'This diagram could not be rendered.',
  pumlFailed:      'Failed to render this diagram.',

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
  errNotMarkdown: (p) => `Not a Markdown file: ${p}`,
  errLinkNotFound:(h) => `Link target not found: ${h}`,
  errClipboard:   'Failed to copy.',
  errRemoved:     (p) => `File was deleted: ${p}`,
  errEditorFailed:'Failed to start the editor.',
  errEditorSelf:  'MarkView cannot be used as an editor.',
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
- ツリーは `role="tree"` / `role="treeitem"` を用い、`aria-expanded` を更新する。**キーボードで操作できるようにする規定は IMP-248 が持つ**（UI-031）。
- フォーカスリングを消さない。`outline: none` を無条件に指定しない（DSP-016）。
- 状態画面・ダイアログの表示時に、フォーカスを内部の操作要素へ移す。
- **文書の表示時に、フォーカスを本文ペインへ移す**（UI-051）。規定は IMP-220 が持つ。**キーボードだけで操作する利用者が、開いた文書を読み進められるようにするためである。**

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
| IMP-226 | 画像の読み込み失敗 | MUST |
| IMP-227 | 見出しのアンカー | SHOULD |
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
| IMP-250 | 状態画面 | MUST |
| IMP-251 | 情報ダイアログ | MUST |
| IMP-252 | エディタ選択ダイアログ | MUST |
| IMP-290 | UI 文言の一元定義 | MUST |
| IMP-295 | アクセシビリティの最低限の対応 | SHOULD |
