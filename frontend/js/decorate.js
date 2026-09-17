// decorate.js — 本文の飾り付け（IMP-220 の手順 4〜6。IMP-225, IMP-226, IMP-227）。
//
// Go 側が出した HTML（サニタイズ済み。IMP-116）へ、フロントエンドが見た目のための要素を足す。
// **Alerts のアイコン**（インライン SVG はサニタイズで除去されるため。IMP-112）、**見出しのアンカー**、
// **読み込みに失敗した画像の置き換え**（描画をエンジン既定に委ねない。NFR-061, BUG-008）。
//
// viewer.js が 400 行の目安（IMP-011）を超えたため分けた。**呼び出しの順序は viewer.js の renderDocument が
// 決める**（IMP-220）。**このモジュールは viewer.js / overlay.js / main.js / navigate.js / docswitch.js を
// import しない。**
//
// **Go を経由しない文字列を innerHTML に渡さない**（IMP-220）。UI 文言と alt は textContent で入れる。

import { onImageBroken } from "./media.js";
import { S } from "./strings.js";
import { DOC_ID_PREFIX, icon } from "./util.js";

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
// **href は見出しの id から先頭の user-content- を除いたスラッグとする**（`#<スラッグ>`。IMP-227,
// MD-021, AR-053）。GitHub のアンカーのアイコンと同じ形であり、コピーして他の Markdown へ貼っても
// 動く。移動は findInDocument が接頭辞を補って探す。**id をそのまま使わない**——`#user-content-foo`
// は findInDocument が `user-content-user-content-foo`（`## user-content-foo` の見出し）を先に探すため、
// 両方の見出しがある文書で別の見出しへ移る。スラッグそのものは Go 側が生成したものであり、組み替えない。
export function decorateHeadings(root) {
  for (const heading of root.querySelectorAll("h1, h2, h3, h4, h5, h6")) {
    // Go 側は必ず ID を付ける（IMP-117）が、防御的に扱う。
    if (!heading.id) continue;

    const anchor = document.createElement("a");
    anchor.className = "heading-anchor";
    anchor.href = "#" + (heading.id.startsWith(DOC_ID_PREFIX) ? heading.id.slice(DOC_ID_PREFIX.length) : heading.id);
    // アイコンだけのリンクに読み上げ名を与える（IMP-295, IMP-290）。
    anchor.setAttribute("aria-label", S.headingAnchor);
    anchor.appendChild(icon("icon-link"));

    heading.insertBefore(anchor, heading.firstChild);
  }
}

// markBrokenImages は読み込みに失敗した画像を、代替テキストを持つ要素へ
// 置き換える（IMP-226, DSP-123, FR-022）。
//
// **CSS だけでは実装できない。** 読み込みに失敗した img を選ぶセレクタが
// どちらのエンジンにも存在しないため、この 1 か所だけ JavaScript が要る。
//
// **代替テキストは自前で描く。ブラウザ既定の alt 表示に任せてはならない**
// （NFR-061）。既定はエンジンごとに違い、**WebKitGTK は alt を描かずに自前の
// 壊れ画像アイコンを描く**。枠（こちらが CSS で描くもの）は両環境で出ていた
// のに、中身だけが Linux で空になっていた（BUG-008）。
//
// **枠と色は CSS が与える**（DSP-123）。ここで用意するのは要素とテキストだけ
// であり、体裁を JavaScript で組み立てない。
//
// ローカル画像とリモート画像（MD-071）を区別しない。FR-022 はどちらも
// 同じ扱いと定めている。
export function markBrokenImages(root) {
  for (const img of root.querySelectorAll("img")) {
    img.addEventListener("error", () => replaceBrokenImage(img));

    // **配線した時点ですでに失敗しているものを拾う。**
    //
    // innerHTML で挿入した直後に配線しても、キャッシュ済みの失敗では
    // error がすでに発火し終えている。これを落とすと、手元では再現せず
    // 実機でだけ枠が出ないという、最も追いにくい形の不具合になる。
    if (img.complete && img.naturalWidth === 0) {
      replaceBrokenImage(img);
    }
  }
}

// replaceBrokenImage は img を .img-broken の span へ置き換える（IMP-226）。
//
// **load で元へ戻す経路は持たない。** 同じ src に対して error の後で load が
// 発火することはなく、復帰は文書の再描画（F5。FR-015）で起こる。戻す経路を
// 残すと、置き換えた span から img を復元する処理が要り、得るものが無い。
//
// **title は付けない。** すでに見えている文字と同じものを重ねるだけであり、
// 読み上げにも寄与しない。
function replaceBrokenImage(img) {
  // **二重に置き換えない。** error と「配線時にすでに失敗」の 2 経路から
  // 呼ばれる。置き換えた後の img は親を失うため、それを目印にする。
  if (img.parentNode === null) return;

  const span = document.createElement("span");
  span.className = "img-broken";
  // **textContent で入れる。** alt は文書由来の文字列であり、
  // innerHTML に渡してはならない（IMP-220）。
  span.textContent = img.alt;

  // 手順 5a で振った番号を写し、拡大画面が開いていた画像の失敗を知らせる（IMP-226, IMP-228, IMP-253）。
  const index = img.getAttribute("data-media-index");
  if (index !== null) span.setAttribute("data-media-index", index);

  img.replaceWith(span);
  onImageBroken(span);
}
