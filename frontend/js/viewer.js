// viewer.js — 本文の挿入と後処理（IMP-220, IMP-223, IMP-225）。
//
// **Go を経由しない文字列を innerHTML に渡さない**（IMP-220）。UI 文言の挿入は
// textContent を使う。doc.html は Go 側でサニタイズ済みであり（IMP-116）、
// フロントエンドで追加のサニタイズは行わない。

import { state } from "./state.js";
import { S, warningText } from "./strings.js";
import { hideStateScreen } from "./overlay.js";
import { renderOutline, observeHeadings, syncActive } from "./outline.js";
import { attachCopyButtons } from "./copy.js";
import { drawMermaid, drawMath } from "./lazy.js";
import { closeSearch } from "./search.js";
import { updateStatus, showMessage } from "./status.js";
import { $, icon } from "./util.js";

let onFollow = null;

// initViewer は本文のイベントを配線する（IMP-211, IMP-223）。
//
// **#markdown に 1 つだけリスナを置く。** リンクごとに付けない。
export function initViewer(deps) {
  onFollow = deps.onFollow;
  $("markdown").addEventListener("click", onLinkClick);
}

// onLinkClick は本文中のリンクを捕まえる（IMP-223, FR-050, AR-060）。
//
// **href があれば必ず preventDefault する。** WebView 内でのページ遷移を
// 一切発生させない。判断は Go 側に置き、ここではスキームもパスも解釈しない
// （IMP-300, IMP-312）。
function onLinkClick(event) {
  const anchor = event.target.closest("a");
  if (!anchor) return; // 埋め込まれた画像のクリックでは何も起きない（FR-053）

  const href = anchor.getAttribute("href");
  if (!href) return;

  event.preventDefault();

  // 同一文書内のアンカーだけはフロントエンドで処理する（IMP-223）。
  if (href.startsWith("#")) {
    scrollToAnchor(href.slice(1));
    return;
  }

  // それ以外は生値のまま Go へ渡す（IMP-312）。target="_blank" も同じ経路。
  if (onFollow) onFollow(href);
}

// scrollToAnchor は同一文書内の見出しへ移動する（FR-050）。
//
// **スムーススクロールを使わない**（FR-041）。
export function scrollToAnchor(fragment) {
  const target = findAnchor(fragment);
  if (!target) {
    showMessage(S.errLinkNotFound(`#${fragment}`), "error");
    return;
  }

  target.scrollIntoView();
  syncActive();
}

// findAnchor は見出し ID で要素を探す。
//
// リンクの生値は百分率符号化されていることがある（`#%E6%97%A5%E6%9C%AC%E8%AA%9E`）。
// 見出し ID は素の文字列で出力されるため（IMP-117）、復号したものでも探す。
function findAnchor(fragment) {
  const direct = document.getElementById(fragment);
  if (direct) return direct;

  try {
    return document.getElementById(decodeURIComponent(fragment));
  } catch {
    return null; // 復号できない値は見つからないものとして扱う
  }
}

// renderDocument は DocumentDTO を画面へ反映する（IMP-220）。
//
// **処理順序を固定する。** 番号は IMP-220 の手順に対応する。
export function renderDocument(doc) {
  const viewer = $("viewer");
  const markdown = $("markdown");

  // scroll.mode === "keep" のために、差し替える前の位置を控える（IMP-321）。
  const previousTop = viewer.scrollTop;

  state.doc = doc;

  // 0. 検索を閉じる（FR-080, IMP-241）。包んだ <mark> を解いてから
  //    差し替える。ここを飛ばすと、検索状態が前の文書の <mark> を
  //    指したままになる。
  closeSearch();

  // 1. 一度に挿入する。分割挿入や逐次追加を行わない（AR-052）。
  markdown.innerHTML = doc.html;

  // 2. 状態画面を隠す。
  hideStateScreen();

  // 3. コピーボタンを付与する（IMP-221）。
  attachCopyButtons(markdown);

  // 4. GitHub Alerts のアイコンを付与する（IMP-225）。
  decorateAlerts(markdown);

  // 5. 見出しにアンカーを付与する（IMP-227, MD-020）。
  decorateHeadings(markdown);

  // 6. 画像の読み込み失敗を捉える配線を行う（IMP-226, DSP-123）。
  //    **遅らせすぎない。** 配線までに読み込みが終わった画像は error を
  //    受け取れないため、markBrokenImages が既に失敗しているものを別途拾う。
  markBrokenImages(markdown);

  // 7. スクロール連動の監視対象を作り直す（IMP-222）。
  observeHeadings(markdown, doc.headings);

  // 8. needsMermaid / needsKaTeX に応じて遅延ロードする（IMP-230, NFR-013）。
  //    **await しない。** 読み込みと描画で本文の表示をブロックしない（NFR-012）。
  if (doc.needsMermaid) drawMermaid(markdown);
  if (doc.needsKaTeX) drawMath(markdown);

  // 9. スクロール位置を設定する（13 章 ScrollDTO）。
  applyScroll(viewer, doc.scroll, previousTop);

  // 10. アウトラインとステータスを更新する。
  renderOutline(doc.headings);
  // 9 で位置を飛ばしているため、監視の通知を待たずにここで合わせる（IMP-222）。
  syncActive();
  updateStatus();

  // 変換時の警告はステータスの一時メッセージで伝える（IMP-302, FR-110）。
  // 未知の Kind は無視される（IMP-290）。
  const warning = doc.warnings.map(warningText).find(Boolean);
  showMessage(warning, "warning");
}

// ALERT_ICONS は Alerts のクラス名とシンボル ID の対応（IMP-203, DSP-261）。
const ALERT_ICONS = {
  "markdown-alert-note": "icon-note",
  "markdown-alert-tip": "icon-tip",
  "markdown-alert-important": "icon-important",
  "markdown-alert-warning": "icon-warning",
  "markdown-alert-caution": "icon-caution",
};

// decorateAlerts は Alerts のラベルにアイコンを挿入する（IMP-225, MD-040）。
//
// **これをフロントエンドで行うのは、Go 側が出力したインライン SVG が
// サニタイズで除去されるためである**（IMP-112, IMP-116）。Go 側は種別を
// クラス名で伝え、フロントエンドが見た目を組み立てる。
export function decorateAlerts(root) {
  for (const alert of root.querySelectorAll(".markdown-alert")) {
    const symbol = symbolFor(alert);
    if (!symbol) continue; // 未知の種別には何も挿入しない

    const title = alert.querySelector(".markdown-alert-title");
    if (!title) continue;

    title.insertBefore(icon(symbol), title.firstChild);
  }
}

function symbolFor(alert) {
  for (const name of alert.classList) {
    if (ALERT_ICONS[name]) return ALERT_ICONS[name];
  }

  return "";
}

// decorateHeadings は見出しにアンカーを付与する（IMP-227, MD-020, DSP-023）。
//
// **クリックの処理は書かない。** 本文中のリンクは onLinkClick が捕捉し、
// フラグメントは scrollToAnchor がスクロールに変える（IMP-223, AR-060）。
// ここで独自のハンドラを足すと経路が 2 つになる。
//
// href に使う値は Go 側が生成した見出し ID（MD-021 のスラッグ）であり、
// ここで組み替えない。
export function decorateHeadings(root) {
  for (const heading of root.querySelectorAll("h1, h2, h3, h4, h5, h6")) {
    // Go 側は必ず ID を付ける（IMP-117）が、防御的に扱う。
    if (!heading.id) continue;

    const anchor = document.createElement("a");
    anchor.className = "heading-anchor";
    anchor.href = "#" + heading.id;
    // アイコンだけのリンクに読み上げ名を与える（IMP-295, IMP-290）。
    anchor.setAttribute("aria-label", S.headingAnchor);
    anchor.appendChild(icon("icon-link"));

    heading.insertBefore(anchor, heading.firstChild);
  }
}

// markBrokenImages は読み込みに失敗した画像へ is-broken を付ける
// （IMP-226, DSP-123, FR-022）。
//
// **CSS だけでは実装できない。** 読み込みに失敗した img を選ぶセレクタが
// Chromium に存在しないため、この 1 か所だけ JavaScript が要る。
//
// **クラスを付けるだけで DOM を組み立てない。** 代替テキストの描画は
// ブラウザ既定の alt 表示に任せ、枠と色は CSS が与える（DSP-123）。alt は
// 文書由来の文字列であり、innerHTML に渡してはならない（IMP-220）。
//
// ローカル画像とリモート画像（MD-071）を区別しない。FR-022 はどちらも
// 同じ扱いと定めている。
export function markBrokenImages(root) {
  for (const img of root.querySelectorAll("img")) {
    img.addEventListener("error", () => img.classList.add("is-broken"));
    // 再読み込みで復帰しうる。成功したら外す。
    img.addEventListener("load", () => img.classList.remove("is-broken"));

    // **配線した時点ですでに失敗しているものを拾う。**
    //
    // innerHTML で挿入した直後に配線しても、キャッシュ済みの失敗では
    // error がすでに発火し終えている。これを落とすと、手元では再現せず
    // 実機でだけ枠が出ないという、最も追いにくい形の不具合になる。
    if (img.complete && img.naturalWidth === 0) {
      img.classList.add("is-broken");
    }
  }
}

// applyScroll は ScrollDTO の mode に従って位置を決める（IMP-302）。
//
// **スムーススクロールを使わない。** アンカーへは即座に移動する（FR-041）。
function applyScroll(viewer, scroll, previousTop) {
  switch (scroll.mode) {
    case "anchor": {
      const target = scroll.anchor ? document.getElementById(scroll.anchor) : null;
      if (target) {
        target.scrollIntoView();
        return;
      }
      // 見出しが見つからない場合は先頭へ。位置を動かさないと、リンクを
      // 踏んだのに何も起きていないように見える。
      viewer.scrollTop = 0;
      return;
    }

    case "restore":
      viewer.scrollTop = scroll.top;
      return;

    case "keep":
      viewer.scrollTop = previousTop;
      return;

    default:
      viewer.scrollTop = 0;
  }
}
