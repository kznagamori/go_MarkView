// zoom.js — 表示倍率（FR-081, IMP-242, DSP-021）。
//
// --zoom は #markdown にだけ設定する。ツールバー・ペイン・ステータスへ
// 波及させない（FR-081）。

import { state, saveConfig } from "./state.js";
import { updateStatus } from "./status.js";
import { $ } from "./util.js";

// 倍率の範囲と刻み（FR-081）。config.Config の MinZoom / MaxZoom / ZoomStep
// （IMP-153）と同じ値だが、**Go 側が範囲外を既定値へ戻すのは壊れた設定への
// 備えであり**、ここは操作の上限・下限である。役割が違うため各々で持つ。
const MIN = 50;
const MAX = 300;
const STEP = 10;

// applyZoom は表示倍率を本文へ反映する。値は百分率（100 が等倍）。
//
// **設定へ保存しない。** 起動時の適用と操作による変更を同じ関数で行う。
export function applyZoom(zoom) {
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

// setZoom は倍率を決めて反映し、設定へ残す（IMP-242, UI-114）。
//
// 10 の倍数へ丸めてから範囲へ収める。上限・下限がどちらも 10 の倍数のため、
// 順序を入れ替えても結果は変わらない。
//
// **値が変わらないときは何もしない。** 上限・下限に張り付いた状態でキーを
// 押し続けたときに、同じ値の保存を Go へ送り続けないため。
export function setZoom(percent) {
  const stepped = Math.round(percent / STEP) * STEP;
  const clamped = Math.max(MIN, Math.min(MAX, stepped));

  if (clamped === state.zoom) return;

  applyZoom(clamped);

  // 100 % 以外のときだけステータスへ出す（FR-081, DSP-150）。
  updateStatus();
  saveConfig();
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
