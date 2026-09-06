// zoom.js — 表示倍率（FR-081, IMP-242, DSP-021）。
//
// --zoom は #markdown にだけ設定する。ツールバー・ペイン・ステータスへ
// 波及させない（FR-081）。

import { state } from "./state.js";
import { updateStatus } from "./status.js";
import { $ } from "./util.js";

// 倍率の範囲と刻み（FR-081）。**このモジュールだけが持つ。**
//
// 倍率は保存されないため（UI-111, UI-115）、config パッケージ側に対応する定数を
// 置かない（IMP-153, IMP-242）。ここにあるのは操作の上限・下限である。
const MIN = 50;
const MAX = 300;
const STEP = 10;

// applyZoom は表示倍率を本文へ反映する。値は百分率（100 が等倍）。
//
// 渡された値をそのまま反映する。丸めは setZoom が行う（IMP-242）。
//
// **公開しない。** 操作の入口を setZoom / stepZoom に限ることを、モジュールの
// 外から呼べないことで保証する。倍率は復元しないため（UI-111）、丸めを経ない
// 反映を外部から行う理由がない。
function applyZoom(zoom) {
  state.zoom = zoom;
  $("markdown").style.setProperty("--zoom", String(zoom));
}

// initZoom は Ctrl + ホイールを配線する（FR-081, IMP-242）。
//
// window で受ける。本文の外（ペインやツールバーの上）でも、WebView 既定の
// ページ全体の拡大が起きてはならない（AR-060）。
export function initZoom() {
  window.addEventListener("wheel", onWheel, { passive: false });
}

// setZoom は倍率を決めて反映する（IMP-242）。操作の入口はここと stepZoom。
//
// 10 の倍数へ丸めてから範囲へ収める。上限・下限がどちらも 10 の倍数のため、
// 順序を入れ替えても結果は変わらない。
//
// **設定へ保存しない**（UI-111, UI-115）。saveConfig を呼ばず、state.zoom は
// セッション内でのみ保持する。起動のたびに 100 % から始まる。
//
// **値が変わらないときは何もしない。** 上限・下限に張り付いた状態でキーを
// 押し続けたときに、同じ値の反映とステータス更新を繰り返さないため。
export function setZoom(percent) {
  const stepped = Math.round(percent / STEP) * STEP;
  const clamped = Math.max(MIN, Math.min(MAX, stepped));

  if (clamped === state.zoom) return;

  applyZoom(clamped);

  // 100 % 以外のときだけステータスへ出す（FR-081, DSP-150）。
  updateStatus();
}

// stepZoom は 1 段だけ変える（FR-081）。delta は +1 / -1。
export function stepZoom(delta) {
  setZoom(state.zoom + delta * STEP);
}

function onWheel(event) {
  if (!event.ctrlKey) return;

  // WebView 既定の拡大を抑止する（IMP-242）。
  event.preventDefault();

  // deltaY は下方向が正。手前へ回す（負）と拡大する。
  stepZoom(event.deltaY < 0 ? 1 : -1);
}
