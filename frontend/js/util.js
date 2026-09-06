// util.js — 共通ユーティリティ。
//
// 利用者に見える文言をここに書かない（IMP-290）。

const SVG_NS = "http://www.w3.org/2000/svg";

// $ は ID で要素を取る。
export function $(id) {
  return document.getElementById(id);
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
