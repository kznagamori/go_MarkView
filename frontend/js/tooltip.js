// tooltip.js — ボタンのツールチップ（UI-024, IMP-247, DSP-102）。
//
// **ブラウザ既定の title 属性を使わない。** DSP-102 はボタンの下 6px・
// 遅延 400ms・反転色・1 行という具体値を定めているが、title の見た目も
// 遅延も位置も OS が決めており、どれも満たせない。英日併記（`Open / 開く`）
// で横に長くなるため、折り返さないことが特に効く。
//
// 対象はツールバーのボタンと、v1.1.0 で足した本文中・拡大画面のボタン（図と画像 UI-053、
// 並べ替え UI-054、拡大画面の操作バー UI-104）とする。ツリー・アウトライン・ステータスの
// 「全文をツールチップで示す」（DSP-113, DSP-151）は、装飾ではなく
// 隠れた文字を読ませるためのもので、title のままでよい。**コードブロックのコピーボタンには出さない**
// （UI-053。data-tip を持たない）。
//
// **data-tip は文書から書けない。** サニタイズの許可リストに無い（IMP-116）ため、本文の中で
// data-tip を持つのは自前で足したボタンだけである。
//
// 要素は 1 つだけ作り、使い回す（IMP-247）。ボタンごとに持たせない。

import { $ } from "./util.js";

// 表示遅延と、ボタンとの間隔（DSP-102）。
const DELAY_MS = 400;
const GAP = 6;

// 画面端からの最小の余白。右端の「?」ボタンで枠外へ出さない。
const MARGIN = 4;

let timer = 0;

// initTooltip は #app へ配線する（IMP-211, IMP-247）。
//
// **委譲で受ける。** ボタンごとにリスナを置くと、ツールチップの文言が
// 変わるボタン（テーマ。IMP-243）や、描画のたびに作り直す本文中のボタン
// （IMP-228, IMP-229）で付け替えが要る。
export function initTooltip() {
  const app = $("app");

  app.addEventListener("pointerover", onOver);
  app.addEventListener("pointerout", onOut);

  // 押したら消す。トグルの状態が変わって文言が古くなるため（IMP-243）。
  app.addEventListener("pointerdown", hide);

  // ウィンドウの外へポインタが出た場合や、キー操作へ移った場合。
  window.addEventListener("blur", hide);
  window.addEventListener("keydown", hide);
}

function onOver(event) {
  const target = event.target.closest("[data-tip]");
  if (!target) return;

  // 同じボタンの中で子要素をまたいだだけなら何もしない。
  if (event.relatedTarget && target.contains(event.relatedTarget)) return;

  hide();
  timer = window.setTimeout(() => show(target), DELAY_MS);
}

function onOut(event) {
  const target = event.target.closest("[data-tip]");
  if (!target) return;
  if (event.relatedTarget && target.contains(event.relatedTarget)) return;

  hide();
}

function show(target) {
  const tip = $("tooltip");

  // Go を経由しない文字列を innerHTML へ渡さない（IMP-220）。
  tip.textContent = target.dataset.tip;
  tip.hidden = false;

  place(tip, target);
}

function hide() {
  window.clearTimeout(timer);
  timer = 0;

  const tip = $("tooltip");
  if (tip) tip.hidden = true;
}

// place はボタンの下・水平中央に置き、画面内へ収める（DSP-102）。
//
// **ウィンドウの下端から出る場合はボタンの上に、同じ間隔で出す**（DSP-102）。本文の最下部の
// ボタンや、拡大画面の操作バーの近くでも読めるようにする。
//
// **表示してから測る。** 幅は文言で変わるため、先に描かないと中央が出せない。
function place(tip, target) {
  const button = target.getBoundingClientRect();
  const box = tip.getBoundingClientRect();

  const center = button.left + button.width / 2 - box.width / 2;
  const limit = window.innerWidth - box.width - MARGIN;

  tip.style.left = `${Math.max(MARGIN, Math.min(center, limit))}px`;
  const below = button.bottom + GAP;
  const fits = below + box.height <= window.innerHeight - MARGIN;
  tip.style.top = `${fits ? below : button.top - GAP - box.height}px`;
}
