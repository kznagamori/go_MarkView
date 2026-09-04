// panes.js — ペインの開閉とリサイズ（UI-026, UI-030, UI-040, IMP-240, IMP-246）。
//
// 表示・非表示は hidden 属性で切り替える。style.display を直接触らない（IMP-202）。
//
// **利用者の意思と、幅不足による一時的な非表示を別の変数で持つ**（IMP-246）。
//   state.outlineVisible … 利用者の意思。設定に保存する（UI-110）
//   outlineSuppressed    … 幅不足による一時的な非表示。保存しない
//   実際の表示 = outlineVisible && !outlineSuppressed
// ここを 1 つの変数にまとめると、ウィンドウを広げても戻らなくなり、
// 設定にも誤った値が保存される。

import { state, saveConfig } from "./state.js";
import { syncActive } from "./outline.js";
import { $ } from "./util.js";

// 本文ペインがこれを下回るとアウトラインを隠す（UI-026, DSP-380）。
const CONTENT_MIN = 240;

// ペイン幅の下限と、ウィンドウ幅に対する上限（UI-030, UI-040, DSP-380）。
const PANE_MIN = 160;
const PANE_MAX_RATIO = 0.4;

let outlineSuppressed = false;
let responsiveFrame = 0;

// initPanes はリサイザとウィンドウ幅の追随を配線する（IMP-211）。
export function initPanes() {
  bindResizer("outline");
  bindResizer("filetree");

  // resize は連続して発火するため 1 フレームにまとめる（NFR-012）。
  window.addEventListener("resize", scheduleResponsive);

  // 起動直後の幅でも判定する。狭いウィンドウで開き直した場合に効く。
  if (applyResponsive()) refresh();
}

// applyPanes は設定値をペインへ反映する（IMP-211 の起動順序）。
export function applyPanes(config) {
  state.outlineVisible = config.outlineVisible;
  state.fileTreeVisible = config.fileTreeVisible;
  state.outlineWidth = config.outlineWidth;
  state.fileTreeWidth = config.fileTreeWidth;

  refresh();
}

// togglePane は表示を切り替え、設定へ残す（FR-034, FR-043, UI-110, UI-114）。
//
// **抑制中でも利用者の意思は通常どおり切り替える**（IMP-246）。抑制が解けた
// ときにその値が反映される。
//
// **戻り値は「表示になったか」**（IMP-240）。ファイルツリーが非表示から表示に
// なったときはツリーを読み直す必要があるが（FR-035 の 1 番目の契機）、
// **その判断は main.js に置く。** ここから filetree.js を呼ぶと、ペインの
// 開閉というひとつの関心にツリーの読み込みが混ざる（IMP-201）。
export function togglePane(name) {
  let shown = false;

  keepScroll(() => {
    if (name === "outline") {
      state.outlineVisible = !state.outlineVisible;
      shown = state.outlineVisible;
    } else {
      state.fileTreeVisible = !state.fileTreeVisible;
      shown = state.fileTreeVisible;
    }
    applyResponsive();
    refresh();
  });

  saveConfig();

  return shown;
}

// setPaneWidth は幅を変え、設定へ残す（UI-030, UI-040）。
export function setPaneWidth(name, px) {
  if (name === "outline") {
    state.outlineWidth = clampWidth(px);
  } else {
    state.fileTreeWidth = clampWidth(px);
  }

  // 幅が変わると折り返しも変わるため、相対位置を維持する（DSP-311, DSP-350）。
  keepScroll(refresh);
  saveConfig();
}

// applyResponsive はウィンドウ幅に応じた一時的な非表示を判定する（IMP-246, UI-026）。
//
// **outlineVisible も設定値も aria-pressed も変えない。** 変えると、
// ウィンドウを広げても戻らなくなる。
export function applyResponsive() {
  // 「アウトラインを出したとしたら本文が何 px になるか」で判断する。
  // 隠れている間も同じ式で測るため、行ったり来たりしない。
  const outline = state.outlineVisible ? effectiveWidth("outline") : 0;
  const tree = state.fileTreeVisible ? effectiveWidth("filetree") : 0;
  const content = window.innerWidth - outline - tree;

  // ファイルツリーは抑制の対象としない（UI-026）。ファイルを選ぶ手段を失わせない。
  const suppressed = state.outlineVisible && content < CONTENT_MIN;
  if (suppressed === outlineSuppressed) return false;

  outlineSuppressed = suppressed;

  return true;
}

// isOutlineSuppressed は検査用に現在の抑制状態を返す。
export function isOutlineSuppressed() {
  return outlineSuppressed;
}

function scheduleResponsive() {
  if (responsiveFrame) return;

  responsiveFrame = requestAnimationFrame(() => {
    responsiveFrame = 0;

    // **ウィンドウのリサイズそのものによる位置のずれは維持できない。**
    // ここが呼ばれる時点で本文は既に新しい幅で組み直されており、変更前の
    // 相対位置を読めないためである。DSP-311 / DSP-350 が求めているのは
    // ペインの開閉とリサイズであり、それは下の keepScroll で満たす。
    keepScroll(() => {
      applyResponsive();
      refresh();
    });
  });
}

// refresh は状態を DOM へ書き出す。**表示に関わる変更は必ずここを通す。**
function refresh() {
  applyPane("outline", state.outlineVisible && !outlineSuppressed, state.outlineVisible);
  applyPane("filetree", state.fileTreeVisible, state.fileTreeVisible);

  const app = $("app");
  app.style.setProperty("--pane-outline-width", `${effectiveWidth("outline")}px`);
  app.style.setProperty("--pane-filetree-width", `${effectiveWidth("filetree")}px`);
}

// applyPane はペイン・リサイザ・トグルボタンをまとめて更新する。
//
// shown は実際に見えるか、pressed は利用者の意思。**抑制中は両者が食い違う。**
// ボタンは ON のままにする（UI-021, UI-026, DSP-311）。
function applyPane(name, shown, pressed) {
  $(`pane-${name}`).hidden = !shown;
  $(`resizer-${name}`).hidden = !shown;
  $(`btn-${name}`).setAttribute("aria-pressed", String(pressed));
}

// effectiveWidth は実際に適用する幅を返す（DSP-380）。
//
// **利用者が決めた幅（state）は書き換えない。** ウィンドウを縮めたときは
// 上限に合わせて縮むが、広げれば元の幅に戻る。縮んだ値を保存してしまうと、
// 一度ウィンドウを狭めただけで設定が失われる。
function effectiveWidth(name) {
  const wanted = name === "outline" ? state.outlineWidth : state.fileTreeWidth;

  return clampWidth(wanted);
}

function clampWidth(px) {
  const max = Math.floor(window.innerWidth * PANE_MAX_RATIO);

  return Math.max(PANE_MIN, Math.min(Math.round(px), Math.max(PANE_MIN, max)));
}

// keepScroll は本文のスクロール位置を相対位置で維持する（IMP-240, DSP-311）。
//
// 幅が変わると折り返し位置が変わるため、絶対座標では読んでいた箇所がずれる。
function keepScroll(mutate) {
  const viewer = $("viewer");
  const before = viewer.scrollHeight - viewer.clientHeight;
  const ratio = before > 0 ? viewer.scrollTop / before : 0;

  mutate();

  // scrollHeight を読むことでレイアウトが確定する。
  const after = viewer.scrollHeight - viewer.clientHeight;
  viewer.scrollTop = Math.round(ratio * after);

  syncActive();
}

// bindResizer はドラッグによる幅変更を配線する（IMP-240, DSP-114）。
function bindResizer(name) {
  $(`resizer-${name}`).addEventListener("pointerdown", (event) => startDrag(name, event));
}

function startDrag(name, event) {
  const resizer = $(`resizer-${name}`);
  const pane = $(`pane-${name}`);

  event.preventDefault();
  resizer.setPointerCapture(event.pointerId);
  $("app").classList.add("dragging");

  const startX = event.clientX;
  const startWidth = pane.getBoundingClientRect().width;
  const viewer = $("viewer");
  const before = viewer.scrollHeight - viewer.clientHeight;
  const ratio = before > 0 ? viewer.scrollTop / before : 0;

  let latest = startWidth;
  let frame = 0;

  const onMove = (moveEvent) => {
    latest = startWidth + (moveEvent.clientX - startX);

    // pointermove を 1 フレームにまとめる（NFR-012）。
    if (frame) return;
    frame = requestAnimationFrame(() => {
      frame = 0;
      $("app").style.setProperty(`--pane-${name}-width`, `${clampWidth(latest)}px`);
    });
  };

  const onUp = () => {
    if (frame) cancelAnimationFrame(frame);
    resizer.removeEventListener("pointermove", onMove);
    resizer.removeEventListener("pointerup", onUp);
    resizer.removeEventListener("pointercancel", onUp);
    $("app").classList.remove("dragging");

    // **通知は pointerup で 1 回だけ**（UI-114）。ドラッグ中は Go を呼ばない。
    setPaneWidth(name, latest);
    applyResponsive();
    refresh();

    const after = viewer.scrollHeight - viewer.clientHeight;
    viewer.scrollTop = Math.round(ratio * after);
    syncActive();
  };

  resizer.addEventListener("pointermove", onMove);
  resizer.addEventListener("pointerup", onUp);
  resizer.addEventListener("pointercancel", onUp);
}
