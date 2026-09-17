// refs.js — 目印の照合（IMP-260, IMP-120）。
//
// **目印（data-ref / data-link）の鍵を読むのはこのモジュールだけにする**（IMP-260）。
// サニタイズを通った後の HTML では、生 HTML で書いた属性と、GFM や図のブロックに
// Go 側が付けた属性を**形で区別できない**。区別できるのは、変換ごとに作る乱数の鍵
// （state.doc.refKey。IMP-102）が合うかどうかだけである。照合を 1 か所でも忘れると、
// 生 HTML の要素が編集・並べ替え・図の描画・コピーの対象になる（NFR-030, BUG-014）。
//
// **state.js 以外を import しない。** lazy.js / copy.js / media.js からも使うため、
// editmode.js などへ依存すると循環しうる（viewer.js → lazy.js → editmode.js → …）。

import { state } from "./state.js";

// REF_SHAPES は data-ref の鍵より後ろの形（IMP-120 の表）。番号は 0 起点の 10 進数だけを受け付ける
// （Go 側の ParseRef と同じ。`+3` や空を通さない）。
const REF_SHAPES = {
  task: /^task:[0-9]+$/,
  table: /^table:[0-9]+$/,
  cell: /^cell:[0-9]+:[0-9]+:[0-9]+$/,
  mermaid: /^mermaid:[0-9]+$/,
  plantuml: /^plantuml:[0-9]+$/,
};

// currentKey は表示中の文書の鍵を返す。文書が無い・鍵が空なら null。
//
// **空の鍵を「合う」と扱わない。** 空のまま接頭辞を作ると `:task:0` のような生 HTML が通る。
function currentKey() {
  const key = state.doc && state.doc.refKey;
  return typeof key === "string" && key !== "" ? key : null;
}

// keyedValue は element の属性 name が「<鍵>:」で始まれば、その後ろを返す。合わなければ null。
function keyedValue(element, name) {
  const key = currentKey();
  if (!key || !element || typeof element.getAttribute !== "function") return null;

  const value = element.getAttribute(name);
  const prefix = key + ":";
  if (typeof value !== "string" || !value.startsWith(prefix)) return null;

  return value.slice(prefix.length);
}

// isOwnRef は data-ref の鍵が state.doc.refKey と合い、種類が kind か
// （task / table / cell / mermaid / plantuml）を返す。
//
// **鍵と種類の両方が合い、番号の形が IMP-120 の表のとおりのときだけ真とする。** 種類を見ないと、
// セルの目印を持つ要素がチェックボックスとして扱われる（Go 側の Check も種類を拒む。UT-115）。
export function isOwnRef(element, kind) {
  const shape = Object.prototype.hasOwnProperty.call(REF_SHAPES, kind) ? REF_SHAPES[kind] : null;
  if (!shape) return false;

  const rest = keyedValue(element, "data-ref");
  return rest !== null && shape.test(rest);
}

// ownRef は isOwnRef が真なら data-ref の値そのもの（`<鍵>:<種類>:<番号…>`）を、偽なら null を返す
// （IMP-260, IMP-316）。チェックボックスとセルの書き込みの指示に載せて Go へ渡す。
//
// **値を分解しない。** 解くのは Go 側（document.ParseRef）であり、鍵の照合を省く経路を作らない（IMP-316）。
// **呼び出し側で getAttribute('data-ref') を読まない**——data-ref を読む箇所をこのモジュールだけに保つ。
export function ownRef(element, kind) {
  return isOwnRef(element, kind) ? element.getAttribute("data-ref") : null;
}

// ownLinkTarget は data-link の鍵が合えば書かれたとおりのリンク先を、合わなければ null を返す
// （IMP-249, FR-063）。
//
// **リンク先はコロンを含みうる**（`https://…`）。鍵の直後の 1 つだけを区切りとし、残りをそのまま
// 返す。生 HTML の `<a>` は目印を持たないため null になり、呼び出し側は href を使う（IMP-120）。
export function ownLinkTarget(anchor) {
  return keyedValue(anchor, "data-link");
}

// ownSource は鍵の合う図のブロック（mermaid / plantuml）なら data-source を、それ以外は null を返す
// （IMP-221, IMP-230）。
//
// **data-mermaid / data-plantuml / data-source の有無では判断しない**（IMP-260）。これらは生 HTML でも
// 書けるため、有無で判断すると Go 側の検査（MD-084）を経ない原文が描画され、見えている内容と違う
// 原文がコピーされる（BUG-014）。
export function ownSource(block) {
  if (!isOwnRef(block, "mermaid") && !isOwnRef(block, "plantuml")) return null;

  const source = block.getAttribute("data-source");
  return typeof source === "string" ? source : null;
}
