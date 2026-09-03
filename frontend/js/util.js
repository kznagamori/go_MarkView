// util.js — 共通ユーティリティ。
//
// 利用者に見える文言をここに書かない（IMP-290）。

const SVG_NS = "http://www.w3.org/2000/svg";

// $ は ID で要素を取る。
export function $(id) {
  return document.getElementById(id);
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
