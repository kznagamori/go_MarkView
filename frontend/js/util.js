// util.js — 共通ユーティリティ。
//
// 利用者に見える文言をここに書かない（IMP-290）。

const SVG_NS = "http://www.w3.org/2000/svg";
const XLINK_NS = "http://www.w3.org/1999/xlink";

// $ は ID で**画面の**要素を取る。
//
// **本文（文書）の要素を探すのに使わない。** 文書の書き手は id を自由に決められ、
// document.getElementById は文書の順で最初の要素を返す——本文に無い id が画面の要素に
// 当たり、画面の id が本文の要素に奪われる（AR-053, BUG-011）。本文は findInDocument /
// findHeading で #markdown の中だけを探す。
export function $(id) {
  return document.getElementById(id);
}

// linkHref はリンクの要素に書かれたリンク先を返す（IMP-223, IMP-249）。無ければ null。
//
// **HTML の <a> は href 属性、SVG の <a> は xlink:href 属性**（SVG 2 の href 属性があればそちらを先に読む）。
// 同梱の Mermaid は `click A "URL"` のノードを SVG の <a xlink:href> で包む（securityLevel: strict でも）。
// href 属性だけを読むとリンクに見えず、既定の動作（WebView の中での遷移）が残る（BUG-015）。
// **a.href（解決済みの URL）を使わない**——書かれた値のまま Go へ渡す（IMP-312）。
export function linkHref(anchor) {
  return anchor.getAttribute("href") ?? anchor.getAttributeNS(XLINK_NS, "href");
}

// DOC_ID_PREFIX は文書から生まれる id の接頭辞（AR-053。GitHub と同じ）。
//
// Go 側（renderer.DocumentIDPrefix。IMP-117）が見出し・脚注・生 HTML の id に付ける。
// **画面の id はこの接頭辞で始めない**（scripts/domids が検査する。BR-052）。
export const DOC_ID_PREFIX = "user-content-";

// HEADING_TAGS は見出しの要素名（findHeading）。
const HEADING_TAGS = ["h1", "h2", "h3", "h4", "h5", "h6"];

// findInDocument はリンクのフラグメントが指す本文の要素を返す。無ければ null（IMP-223, AR-053）。
//
// **リンクのフラグメントから本文の要素を探す箇所（本文のリンクと、開いた直後のアンカーの復元）は、
// すべてこの関数を通す。** 手順は IMP-223 の表のとおり。
//
//  1. 先頭の # を除く。候補は生のままの値、百分率符号化を復号した値（復号できなければ生のままだけ）の順
//  2. 各候補について、まず user-content- を前に付けた値で探し、見つからず候補が user-content- で
//     始まっていれば候補そのもので探す
//  3. #markdown の中だけを探す。重複した id は文書の順で先の要素（AR-053）
//
// **手順 2 の順を逆にしない。** `## user-content-foo` の id は `user-content-user-content-foo` であり
// （IMP-117）、アンカーのアイコンの href は `#user-content-foo` になる（IMP-227）。候補そのものを
// 先に探すと、別の見出しに当たるか見つからない。
//
// **完全な id が分かっている見出しには使わない**（findHeading）。ここは接頭辞を補って先に探すため、
// `## foo` の完全な id を渡すと `## user-content-foo` の見出しに当たる。
export function findInDocument(fragment) {
  const markdown = document.getElementById("markdown");
  if (!markdown || typeof fragment !== "string") return null;

  const raw = fragment.startsWith("#") ? fragment.slice(1) : fragment;
  if (raw === "") return null;

  const candidates = [raw];
  try {
    const decoded = decodeURIComponent(raw);
    if (decoded !== raw) candidates.push(decoded);
  } catch {
    // 復号できない値は、生のままの候補だけで探す（IMP-223 の手順 1）。
  }

  for (const candidate of candidates) {
    const prefixed = findById(markdown, DOC_ID_PREFIX + candidate);
    if (prefixed) return prefixed;

    if (candidate.startsWith(DOC_ID_PREFIX)) {
      const exact = findById(markdown, candidate);
      if (exact) return exact;
    }
  }

  return null;
}

// findHeading は #markdown の中の h1〜h6 で id が完全に一致する最初の要素を返す。無ければ null
// （IMP-223, IMP-224, IMP-222）。
//
// アウトラインとスクロール連動は Heading.ID（user-content- 付き）を持っているため、これを使う。
// **生 HTML の `<div id="user-content-test">` が `## Test` の移動先を奪うこともない**——見出しの
// 要素だけを見る。
export function findHeading(id) {
  const markdown = document.getElementById("markdown");
  if (!markdown || typeof id !== "string" || id === "") return null;

  const escaped = CSS.escape(id);
  return markdown.querySelector(HEADING_TAGS.map((tag) => `${tag}#${escaped}`).join(", "));
}

// findById は root の中で id が一致する最初の要素を文書の順で返す。
//
// **CSS.escape を通す。** 見出しのスラッグは日本語や記号を含み（MD-021）、生 HTML の id は
// 任意の文字列である。
function findById(root, id) {
  return root.querySelector(`#${CSS.escape(id)}`);
}

// openAncestorDetails は、要素を包む <details> を根まですべて開く
// （MD-026, IMP-241, IMP-223, IMP-224）。
//
// **閉じた <details> の中の要素は scrollIntoView の対象にならない。**
// getBoundingClientRect は大きさも位置も返す（実測で 723x35）のに、スクロールは
// 起きず、<details> が自動で開くこともない。例外も警告も出さない。
// ハイライトも件数も正しく作れてしまうため、「移動できた」と誤認しやすい。
// **移動の前に、移動先が見える状態を作る。**
//
// **「大きさを持っているか」で判定しない**——持っている。判定できるのは
// 「祖先に閉じた <details> があるか」だけである。
//
// **入れ子に対応する。** 1 段だけ開いても、その外側が閉じていれば見えない。
//
// **開いたものを閉じ直さない**（IMP-241）。検索を閉じた時点で畳むと、
// 利用者が中身を読んでいる最中に閉じてしまう。
//
// **検索・アンカー移動・アウトラインの 3 経路が同じ関数を使う**（IMP-241）。
// 片方だけ直すと「検索では行けるがリンクでは行けない」という、説明の付かない
// 差が残る。**search.js に置くと outline.js との循環参照になるため、ここに置く。**
export function openAncestorDetails(element) {
  if (!element) return;

  let node = element.closest("details");
  while (node) {
    node.open = true;
    node = node.parentElement ? node.parentElement.closest("details") : null;
  }
}

// clear は子要素をすべて取り除く。
//
// innerHTML = "" ではなく明示的に外す。Go を経由しない文字列を innerHTML へ
// 渡さないという規約（IMP-220）を、空文字も含めて例外なく守るため。
export function clear(node) {
  while (node.firstChild) {
    node.removeChild(node.firstChild);
  }
}

// span は文言を入れた <span> を作る。
//
// **textContent で入れる。** Go を経由しない文字列を innerHTML へ渡さない
// （IMP-220）。className を省くと class を付けない。
export function span(className, text) {
  const element = document.createElement("span");
  if (className) element.className = className;
  element.textContent = text || "";

  return element;
}

// icon は SVG シンボル（IMP-203）を参照する <svg> を作る。
//
// 絵柄の定義は index.html が 1 か所に持ち、ここでは参照だけを組み立てる。
export function icon(id, className) {
  const svg = document.createElementNS(SVG_NS, "svg");
  svg.setAttribute("class", className || "icon");
  svg.setAttribute("aria-hidden", "true");

  const use = document.createElementNS(SVG_NS, "use");
  use.setAttribute("href", "#" + id);
  svg.appendChild(use);

  return svg;
}

// baseName はパスの末尾の要素だけを返す。
//
// **パスの解釈ではなく表示名の切り出しである**（IMP-300）。区切りで分ける以上の
// ことはしない。結合・正規化・大文字小文字の規則は Go 側にある。
export function baseName(path) {
  if (!path) return "";
  const parts = path.split(/[\\/]/).filter(Boolean);

  return parts.length > 0 ? parts[parts.length - 1] : "";
}

// formatSize はバイト数を MB 表記にする（DSP-181）。
//
// **単位記号であり文言ではない**ため strings.js には置かない。Go 側の
// formatSize（errors.go）と同じ桁数に揃えてある。
export function formatSize(bytes) {
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

// formatBuildTime は RFC 3339（UTC）のビルド日時を表示形へ直す（DSP-171）。
//
//	2026-08-30T12:00:00Z  ->  2026-08-30 12:00:00 UTC
//
// **単位・書式であり文言ではない**ため strings.js には置かない（formatSize と
// 同じ扱い）。開発ビルドの "unknown" のように形が合わない値はそのまま返す。
export function formatBuildTime(value) {
  const match = /^(\d{4}-\d{2}-\d{2})T(\d{2}:\d{2}:\d{2})/.exec(value || "");

  return match ? `${match[1]} ${match[2]} UTC` : value || "";
}
