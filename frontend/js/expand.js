// expand.js — 拡大画面（FR-122, UI-104, IMP-253, DSP-173）。対象 1 つだけを、ステータス領域を除くウィンドウ
// いっぱいに表示する。独立した OS ウィンドウではなく #expand-view に描く（AR-060。全面を覆う #overlay は使わない）。
//
// **対象は複製せず、本文の要素そのものを移す**（IMP-253）。Mermaid の SVG は id を持ち、中の <style> と
// 矢印（url(#…)）が参照する。複製すると id が 2 つになる。画像は複製すると取り直しが起きうる（FR-120）。
//
// **viewer.js / main.js / docswitch.js / shortcuts.js / zoom.js を import しない**（それぞれから呼ばれ、循環する）。
// 本文ペインへのフォーカスは main.js が渡す deps.focusViewer で戻す（IMP-220）。

import { naturalSize } from "./media.js";
import { S } from "./strings.js";
import { $, icon } from "./util.js";

// 倍率の範囲と 1 段（FR-122, UI-104）、方向キーの移動量、舞台に残す最小の幅、タッチパッドの細かいホイールを
// 1 段にまとめる間隔（IMP-253）。
const [MIN_SCALE, MAX_SCALE, STEP, PAN, KEEP, WHEEL_MS] = [0.1, 10, 1.25, 40, 48, 50];

// KEYS は UI-104 の表。**ツールチップのキー表記もここから組み立てる**（strings.js に書かない。IMP-290）。
// keys の表記は shortcuts.js の keyOf と同じ形。**`+` は配列で届き方が違う**（US は Shift+`=`、JIS は
// Shift+`;`、テンキーは Shift なし）。Esc は閉じるボタンの表記のために載せるが、ここでは処理しない
// （閉じるのは IMP-244 の振り分け）。
const KEYS = {
  fit: { label: "F", keys: ["f", "Shift+f"] },
  actual: { label: "0", keys: ["0", "Ctrl+0"] },
  zoomOut: { label: "-", keys: ["-", "Shift+-", "_", "Shift+_", "Ctrl+-", "Ctrl+Shift+-", "Ctrl+_", "Ctrl+Shift+_"] },
  zoomIn: { label: "+", keys: ["+", "Shift++", "=", "Shift+=", "Ctrl++", "Ctrl+Shift++", "Ctrl+=", "Ctrl+Shift+="] },
  close: { label: "Esc", keys: [] },
};

// 方向キーはスクロールと同じ向きに見える範囲を動かす（→ で右側が見える。中身は左へ動く）。
const ARROWS = { ArrowLeft: [PAN, 0], ArrowRight: [-PAN, 0], ArrowUp: [0, PAN], ArrowDown: [0, -PAN] };

let deps = { focusViewer() {} };
let parts = null;

// view は開いている間の状態。閉じていれば null。
//   key / target / opener / placeholder / savedMaxWidth  移した要素と、戻すための控え
//   width / height  本来の大きさ。scale / x / y / fit  倍率と位置（IMP-253）
//   waiting  再描画の後、同じ鍵の対象の知らせを待っているか
let view = null;

let lastWheel = -Infinity;
let drag = null;
let dragFrame = 0;
let stageSize = { width: 0, height: 0 };

// initExpand は操作バーと舞台を組み立てて配線する（IMP-211, IMP-253）。deps: { focusViewer }。
export function initExpand(given) {
  deps = Object.assign({ focusViewer() {} }, given);

  const root = $("expand-view");
  root.setAttribute("aria-label", S.expandTitle);

  const bar = document.createElement("div");
  bar.className = "expand-bar";
  const zoomValue = document.createElement("span");
  zoomValue.className = "expand-zoom-value";
  parts = {
    root,
    bar,
    zoomValue,
    fit: textButton("expand-fit", S.expandFit, S.tipExpandFit, "fit", () => applyFit()),
    actual: textButton("expand-actual", S.expandActual, S.tipExpandActual, "actual", () => zoomAtCenter(1)),
    zoomOut: iconButton("expand-zoom-out", "icon-zoom-out", S.tipZoomOut, "zoomOut", () => zoomAtCenter(view.scale / STEP)),
    zoomIn: iconButton("expand-zoom-in", "icon-zoom-in", S.tipZoomIn, "zoomIn", () => zoomAtCenter(view.scale * STEP)),
    close: iconButton("expand-close", "icon-close", S.tipExpandClose, "close", () => closeExpand()),
    stage: document.createElement("div"),
    content: document.createElement("div"),
  };
  bar.append(parts.fit, parts.actual, parts.zoomOut, zoomValue, parts.zoomIn, parts.close);
  parts.stage.className = "expand-stage";
  parts.content.className = "expand-content";
  parts.stage.appendChild(parts.content);
  root.append(bar, parts.stage);

  root.addEventListener("keydown", trapTab);
  parts.stage.addEventListener("wheel", onWheel, { passive: false });
  parts.stage.addEventListener("pointerdown", onPointerDown);
  parts.stage.addEventListener("pointermove", onPointerMove);
  parts.stage.addEventListener("pointerup", endDrag);
  parts.stage.addEventListener("pointercancel", endDrag);
  // 画像の既定のドラッグ（ドラッグ＆ドロップ）を始めない。移動に使う（UI-104）
  parts.stage.addEventListener("dragstart", (event) => event.preventDefault());
  window.addEventListener("resize", onResize);
}

// openExpand は対象を拡大画面へ移して開く（FR-122）。target: svg / img、key: IMP-228 の鍵、opener: 押したボタン。
export function openExpand(target, key, opener) {
  if (view || !parts || !target || !target.isConnected) return;

  view = { key, opener, target: null, placeholder: null, savedMaxWidth: undefined, scale: 1, x: 0, y: 0, fit: true, waiting: false };
  moveIn(target);

  parts.root.hidden = false;
  // ステータス領域は覆わないが、操作も受け付けない。表示の更新は止まらない（UI-104）
  $("statusbar").toggleAttribute("inert", true);

  measureStage();
  applyFit();
  parts.fit.focus({ preventScroll: true });
}

// closeExpand は拡大画面を閉じる。閉じたら true、開いていなければ false（IMP-244 の Esc の振り分け）。
export function closeExpand() {
  if (!view) return false;

  const { key, opener } = view;
  restoreTarget(true);
  hide();

  // **閉じた状態にしてから戻す**（focusViewer は拡大画面の表示中は何もしない。IMP-220）。再描画の後のボタンは
  // 画面の外にありうるため preventScroll（DSP-350）。
  const sameKey = document.querySelector(`.media-actions[data-media-key="${key}"] .media-btn-expand`);
  const back = [opener, sameKey].find((button) => button && button.isConnected && !button.hidden);
  if (back) back.focus({ preventScroll: true });
  else deps.focusViewer();
  return true;
}

export function isExpandOpen() {
  return view !== null;
}

// handleExpandKey は UI-104 の表のキーを扱う（IMP-244 から。扱ったら true）。
//
// **Esc は扱わない**（IMP-244 の振り分けが先に閉じる）。**Enter / Space はフォーカスしているボタンの既定の
// 実行に任せる**（false）。
export function handleExpandKey(event) {
  if (!view) return false;

  const combo = comboOf(event);
  if (Object.prototype.hasOwnProperty.call(ARROWS, combo)) {
    const [dx, dy] = ARROWS[combo];
    moveTo(view.x + dx, view.y + dy);
    return true;
  }

  const id = Object.keys(KEYS).find((name) => KEYS[name].keys.includes(combo));
  if (!id) return false;

  parts[id].click();
  return true;
}

// onTargetSettled は IMP-228 から、対象の準備が済んだ（target が null なら失敗）ことを受ける。
//
// **開いている鍵と同じ鍵**なら、新しい要素と差し替えて倍率と位置を保つ（SHOULD）。失敗なら閉じる（FR-120 の
// 「描画が終わってから判断する」）。
export function onTargetSettled(key, target) {
  if (!view || key !== view.key) return;

  view.waiting = false;
  if (!target) {
    closeExpand();
    return;
  }
  if (target === view.target || !target.isConnected) return;

  // 古い要素は捨てる（元の位置はもう無い）。本来の大きさが変わりうるため測り直し、倍率と位置は保つ
  restoreTarget(false);
  moveIn(target);
  render();
}

// onImagesNumbered は IMP-228 の numberImages から（再描画の手順 5a）。開いている画像の番号が count 以上なら閉じる。
//
// **再描画が始まった合図にもする**——以後、同じ鍵の対象の知らせを待つ。
export function onImagesNumbered(count) {
  if (!view) return;

  view.waiting = true;
  const match = /^image:(\d+)$/.exec(view.key);
  if (match && Number(match[1]) >= count) closeExpand();
}

// onAllDiagramsSettled は IMP-228 から。**開いている図の知らせが届いていなければ閉じる**（同じ鍵の図が文書に無い）。
export function onAllDiagramsSettled() {
  if (view && view.waiting && !view.key.startsWith("image:")) closeExpand();
}

// onDocumentSwitched は文書の切り替えと状態画面への移行で閉じる（IMP-220 の手順 0a, IMP-250）。
//
// **移していた要素は戻さず捨てる。フォーカスも戻さない**（新しい文書の IMP-220 の手順 11 が移す）。
export function onDocumentSwitched() {
  if (!view) return;

  restoreTarget(false);
  hide();
}

// moveIn は要素を拡大画面へ移し、元の位置に同じ大きさの代わりの要素を置く（IMP-253）。
//
// **本文のレイアウトを動かさない**（FR-122 の「本文のスクロール位置を保つ」）。Mermaid の SVG のインラインの
// max-width は控えて外す（IMP-228 と同じ扱い）。
function moveIn(target) {
  const rect = target.getBoundingClientRect();
  const placeholder = document.createElement("span");
  placeholder.className = "expand-placeholder";
  placeholder.style.setProperty("--placeholder-width", `${rect.width}px`);
  placeholder.style.setProperty("--placeholder-height", `${rect.height}px`);
  target.replaceWith(placeholder);

  view.savedMaxWidth = target.style ? target.style.maxWidth : "";
  if (view.savedMaxWidth) target.style.removeProperty("max-width");

  const size = naturalSize(target);
  view.width = size.width || rect.width || 1;
  view.height = size.height || rect.height || 1;
  view.target = target;
  view.placeholder = placeholder;
  parts.content.replaceChildren(target);
}

// restoreTarget は移していた要素を代わりの要素の位置へ戻す。**戻す先が無い（再描画で消えた）か、戻さない
// 指示なら捨てる**（IMP-253）。
function restoreTarget(putBack) {
  const { target, placeholder, savedMaxWidth } = view;
  if (savedMaxWidth) target.style.maxWidth = savedMaxWidth;

  if (putBack && placeholder.isConnected) placeholder.replaceWith(target);
  else {
    target.remove();
    placeholder.remove();
  }
}

function hide() {
  view = null;
  endDrag();
  parts.root.hidden = true;
  $("statusbar").toggleAttribute("inert", false);
}

// applyFit は全体が舞台に収まる倍率で中央に置く（FR-122 の Fit。F / Fit ボタン / 開いた時点）。
function applyFit() {
  if (!view) return;

  const scale = clamp(Math.min(stageSize.width / view.width, stageSize.height / view.height), MIN_SCALE, MAX_SCALE);
  view.scale = scale;
  view.x = (stageSize.width - view.width * scale) / 2;
  view.y = (stageSize.height - view.height * scale) / 2;
  view.fit = true;
  render();
}

// zoomAtCenter は画面（舞台）の中心を基準に倍率を変える（UI-104 の + / - / 0）。
function zoomAtCenter(scale) {
  zoomAt(stageSize.width / 2, stageSize.height / 2, scale);
}

// zoomAt は舞台の中の点 (px, py) の下にある箇所を動かさずに倍率を変える（FR-122 のホイール）。範囲の端で丸める。
function zoomAt(px, py, scale) {
  if (!view) return;

  const next = clamp(scale, MIN_SCALE, MAX_SCALE);
  const ratio = next / view.scale;
  view.x = px - (px - view.x) * ratio;
  view.y = py - (py - view.y) * ratio;
  view.scale = next;
  view.fit = false;
  moveTo(view.x, view.y);
}

// moveTo は位置を変える。**対象の少なくとも 48px が舞台の中に残る範囲に丸める**（UI-104, IMP-253）。
function moveTo(x, y) {
  const width = view.width * view.scale;
  const height = view.height * view.scale;
  const keepX = Math.min(KEEP, width);
  const keepY = Math.min(KEEP, height);
  view.x = clamp(x, keepX - width, stageSize.width - keepX);
  view.y = clamp(y, keepY - height, stageSize.height - keepY);
  render();
}

// render は倍率と位置を画面へ写す。**倍率は transform の scale で掛けず、器の幅と高さを変える**——ビットマップの
// 引き伸ばしで SVG の輪郭がぼやけうる（FR-122）。位置の移動にだけ translate を使う（DSP-173）。
function render() {
  const style = parts.content.style;
  style.setProperty("--expand-width", `${view.width * view.scale}px`);
  style.setProperty("--expand-height", `${view.height * view.scale}px`);
  style.setProperty("--expand-x", `${view.x}px`);
  style.setProperty("--expand-y", `${view.y}px`);

  parts.zoomValue.textContent = S.expandZoom(Math.round(view.scale * 100));
  // 範囲の端は aria-disabled。**disabled 属性は付けない**——フォーカス中のボタンからフォーカスが外れる（IMP-253）
  parts.zoomOut.setAttribute("aria-disabled", String(view.scale <= MIN_SCALE));
  parts.zoomIn.setAttribute("aria-disabled", String(view.scale >= MAX_SCALE));
}

// onWheel は 1 回のホイールを 1 段とし、カーソルの位置を中心に拡大・縮小する（UI-104）。Ctrl の有無を問わない。
function onWheel(event) {
  event.preventDefault();
  if (!view || event.deltaY === 0 || event.timeStamp - lastWheel < WHEEL_MS) return;

  lastWheel = event.timeStamp;
  const rect = parts.stage.getBoundingClientRect();
  zoomAt(event.clientX - rect.left, event.clientY - rect.top, event.deltaY < 0 ? view.scale * STEP : view.scale / STEP);
}

// ドラッグは主ボタンで始め、pointermove を 1 フレームにまとめて位置を変える（NFR-012）。選択を始めない。
function onPointerDown(event) {
  if (!view || event.button !== 0) return;

  event.preventDefault();
  try {
    // ステージの外へ出ても追う。合成したイベントなど、捕捉できないポインタでは例外になる
    parts.stage.setPointerCapture(event.pointerId);
  } catch {
    // 捕捉できなくても、ステージの中の移動は受け取れる
  }
  parts.stage.classList.add("is-dragging");
  drag = { startX: event.clientX, startY: event.clientY, x: view.x, y: view.y, clientX: event.clientX, clientY: event.clientY };
}

function onPointerMove(event) {
  if (!drag) return;

  drag.clientX = event.clientX;
  drag.clientY = event.clientY;
  if (dragFrame) return;
  dragFrame = requestAnimationFrame(() => {
    dragFrame = 0;
    if (drag && view) moveTo(drag.x + drag.clientX - drag.startX, drag.y + drag.clientY - drag.startY);
  });
}

function endDrag() {
  drag = null;
  parts.stage.classList.remove("is-dragging");
}

// onResize は Fit の状態なら合わせ直し、そうでなければ舞台の中心に見えている箇所を保つ（UI-104）。
function onResize() {
  if (!view) return;

  const before = stageSize;
  measureStage();
  if (view.fit) {
    applyFit();
    return;
  }
  moveTo(view.x + (stageSize.width - before.width) / 2, view.y + (stageSize.height - before.height) / 2);
}

function measureStage() {
  const rect = parts.stage.getBoundingClientRect();
  stageSize = { width: rect.width, height: rect.height };
}

// trapTab は Tab を操作バーの中で巡らせる（UI-104。IMP-251 と同じ形）。
function trapTab(event) {
  if (event.key !== "Tab") return;

  const buttons = [...parts.bar.querySelectorAll("button")];
  const index = buttons.indexOf(document.activeElement);
  const next = event.shiftKey ? index - 1 : index + 1;
  event.preventDefault();
  buttons[(next + buttons.length) % buttons.length].focus();
}

// textButton / iconButton は操作バーのボタンを作る。ツールチップと読み上げ名は `Fit (F)` の形（IMP-247, IMP-295）。
// **aria-disabled のボタンを押しても何もしない**（IMP-253）。
function textButton(className, text, tip, id, onClick) {
  const button = barButton(`tb-btn expand-text ${className}`, tip, id, onClick);
  button.textContent = text;
  return button;
}

function iconButton(className, symbol, tip, id, onClick) {
  const button = barButton(`tb-btn ${className}`, tip, id, onClick);
  button.appendChild(icon(symbol));
  return button;
}

function barButton(className, tip, id, onClick) {
  const button = document.createElement("button");
  button.type = "button";
  button.className = className;
  const label = `${tip} (${KEYS[id].label})`;
  button.dataset.tip = label;
  button.setAttribute("aria-label", label);
  button.addEventListener("click", () => {
    if (view && button.getAttribute("aria-disabled") !== "true") onClick();
  });
  return button;
}

// comboOf は KeyboardEvent を KEYS の表記へ変える（shortcuts.js の keyOf と同じ規則。1 文字のキーは小文字）。
function comboOf(event) {
  const names = [];
  if (event.ctrlKey) names.push("Ctrl");
  if (event.altKey) names.push("Alt");
  if (event.shiftKey) names.push("Shift");
  names.push(event.key.length === 1 ? event.key.toLowerCase() : event.key);
  return names.join("+");
}

function clamp(value, min, max) {
  return Math.min(max, Math.max(min, value));
}
