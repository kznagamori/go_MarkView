// main.js — 起動処理とイベント配線（IMP-211）。
//
// 各モジュールが export する初期化関数をここから順に呼ぶ（IMP-201）。
// グローバル変数を作らない。window への代入を行わない。

import * as api from "./api.js";
import { state } from "./state.js";
import { errorText } from "./strings.js";
import { initToolbar } from "./toolbar.js";
import { initTooltip } from "./tooltip.js";
import { applyTheme, toggleTheme } from "./theme.js";
import { applyZoom, initZoom, setZoom, stepZoom } from "./zoom.js";
import { applyPanes, initPanes, togglePane } from "./panes.js";
import { initFileTree, loadTreeRoot, revealCurrent } from "./filetree.js";
import { initOutline } from "./outline.js";
import { initViewer, renderDocument, scrollToAnchor } from "./viewer.js";
import { initSearch, openSearch, closeSearch, isSearchOpen, jump } from "./search.js";
import { initShortcuts } from "./shortcuts.js";
import { initOverlay, showStateScreen, showAbout, hideAbout, isAboutOpen } from "./overlay.js";
import { initDnd } from "./dnd.js";
import { updateStatus, showMessage } from "./status.js";
import { $ } from "./util.js";

// ErrorDTO.Kind と状態画面の対応（IMP-315 の「表示先」）。
// ここにない Kind はステータス領域に 1 行で出す。
const STATE_SCREENS = {
  "needs-confirm": "confirm-large",
  "too-large": "too-large",
  "render-error": "render-error",
};

// SHORTCUTS の id と処理の対応（UI-090, IMP-244）。
//
// **未実装のものは載せない。** copySelection（Ctrl+C）は WebView の既定に
// 任せるためここには現れない（FR-062）。Alt+F4 と閉じるボタンは OS が処理する。
const SHORTCUT_HANDLERS = guardAll({
  open: () => openViaDialog(),
  reload: () => reloadCurrent(),
  theme: () => toggleTheme(),
  outline: () => togglePane("outline"),
  filetree: () => togglePane("filetree"),
  search: () => openSearch(),
  searchNext: () => jump(1),
  searchPrev: () => jump(-1),
  close: () => closeTop(),
  back: () => goBack(),
  forward: () => goForward(),
  zoomIn: () => stepZoom(1),
  zoomOut: () => stepZoom(-1),
  zoomReset: () => setZoom(100),
  about: () => showAboutDialog(),
  quit: () => api.quit(),
});

// guardAll は情報ダイアログ表示中の割り当てを止める（UI-100）。
//
// 「表示中は背後のメインウィンドウの操作を受け付けない」を、マウス（暗幕が
// 覆う）だけでなくキーボードでも成り立たせる。**Esc だけは通す。**
//
// 止めるときも true を返し、既定の動作は抑止する。false を返すと、たとえば
// Ctrl + `+` が WebView 自身のページ拡大として処理されてしまう。
function guardAll(handlers) {
  const guarded = {};

  for (const [id, handler] of Object.entries(handlers)) {
    guarded[id] = id === "close" ? handler : () => (isAboutOpen() ? true : handler());
  }

  return guarded;
}

// boot は起動時の 1 回だけ実行する（IMP-211）。
async function boot() {
  const init = await api.getInitialState();

  // テーマの適用を最優先で行い、既定色から切り替わるちらつきを防ぐ（UI-105）。
  applyTheme(init.config.theme);
  applyZoom(init.config.zoom);
  applyPanes(init.config);

  initToolbar({
    onOpen: openViaDialog,
    onReload: reloadCurrent,
    onTheme: toggleTheme,
    onOutline: () => togglePane("outline"),
    onFileTree: () => togglePane("filetree"),
    onAbout: showAboutDialog,
  });
  initTooltip();
  initFileTree({ onOpen: openFromTree });
  initOutline();
  initViewer({ onFollow: followLink });
  initPanes();
  initSearch();
  initZoom();
  initOverlay({ onLink: followLink });
  initDnd();
  initShortcuts(SHORTCUT_HANDLERS);

  subscribe();

  state.treeRoot = init.treeRoot;

  if (init.document) {
    showDocument(init.document);
  } else {
    showStateScreen(init.stateKind || "welcome", withConfirm(init.error));
    updateStatus();
    // 起動時のパスが読めなかった場合、操作案内はそのままに理由を添える
    // （FR-012, IMP-193）。
    if (init.error && !STATE_SCREENS[init.error.kind]) {
      showMessage(errorText(init.error), "error");
    }
  }

  if (init.treeRoot) await loadTreeRoot(init.treeRoot);
}

// subscribe は Go からのイベントを購読する。**起動時の 1 回のみ**（IMP-322）。
function subscribe() {
  api.on(api.EVENT.documentOpened, showDocument);
  api.on(api.EVENT.documentChanged, showDocument);

  // 削除されても本文とタイトルは維持する（FR-110, UI-013）。
  api.on(api.EVENT.documentRemoved, (error) => showMessage(errorText(error), "error"));

  api.on(api.EVENT.treeRootChanged, async (root) => {
    state.treeRoot = root;
    await loadTreeRoot(root);
  });

  api.on(api.EVENT.error, showError);
}

// showDocument は本文を描画し、ツリーの選択と展開を合わせる（DSP-331）。
//
// **描画とツリーの追随を 1 か所にまとめる。** 経路ごとに呼び分けると、
// どれか 1 つで追随を書き忘れたときに気付けない。
function showDocument(doc) {
  renderDocument(doc);
  revealCurrent();
}

// handleResult は OpenResultDTO を描画へ落とす（IMP-308）。
//
// document も error も null のときは何も起きなかったことを意味する。
// 表示を変えない（履歴の端、ダイアログの取り消しなど）。
function handleResult(result) {
  if (!result) return;

  if (result.error) {
    showError(result.error);
    return;
  }
  if (result.document) {
    showDocument(result.document);
  }
}

// showError は ErrorDTO を表示先へ振り分ける（IMP-315）。
function showError(error) {
  if (!error) return;

  const screen = STATE_SCREENS[error.kind];
  if (screen) {
    showStateScreen(screen, withConfirm(error));
    return;
  }

  showMessage(errorText(error), "error");
}

// withConfirm は確認画面のボタンに処理を結び付ける（FR-016, IMP-314）。
function withConfirm(error) {
  if (!error) return null;

  return Object.assign({}, error, { onConfirm: openConfirmed });
}

async function openConfirmed(path) {
  handleResult(await api.openConfirmed(path));
}

async function openViaDialog() {
  await leaveDocument();
  handleResult(await api.openFileDialog());
}

async function reloadCurrent() {
  await leaveDocument();
  handleResult(await api.reload());
}

// goBack / goForward は表示履歴をたどる（FR-051, UI-090）。
//
// ツールバーにボタンはなく（UI-020）、Alt+← / Alt+→ が唯一の経路である。
// 端に居るときは document も error も null が返り、表示は変わらない
// （IMP-308）。
async function goBack() {
  await leaveDocument();
  handleResult(await api.historyBack());
}

async function goForward() {
  await leaveDocument();
  handleResult(await api.historyForward());
}

// closeTop は Esc の受け口（UI-090）。**上に重なっているものから閉じる。**
//
// 順序は重なり順（DSP-015）に従う。情報ダイアログのほうが検索バーより
// 上にあるため先に閉じる。閉じるものがなければ false を返す（IMP-244）。
function closeTop() {
  if (hideAbout()) return true;
  if (!isSearchOpen()) return false;

  closeSearch();
  return true;
}

// showAboutDialog は情報ダイアログを開く（FR-100, UI-100）。
//
// **Go を呼ぶのはここだけとし、overlay.js は受け取った値を描くだけにする**
// （IMP-201）。
async function showAboutDialog() {
  showAbout(await api.getAbout());
}

// openFromTree はツリーで選ばれたファイルを開く（FR-033）。
// **ツリールートは変わらない**（FR-030）。判断は Go 側にある。
async function openFromTree(path) {
  await leaveDocument();
  handleResult(await api.openFromTree(path));
}

// followLink は本文中のリンクをたどる（FR-050, IMP-330）。
//
// 判断は Go 側にある。ここは結果を描画へ落とすだけとする。
async function followLink(href) {
  await leaveDocument();
  handleLink(await api.followLink(href));
}

// handleLink は LinkResultDTO を種類ごとに振り分ける（IMP-305）。
function handleLink(result) {
  if (!result) return;

  switch (result.kind) {
    case "document":
      showDocument(result.document);
      return;

    case "anchor":
      scrollToAnchor(result.anchor);
      return;

    case "external":
      // Go 側が既に OS へ委譲済み。表示は変えない（FR-053）。
      return;

    default:
      showError(result.error);
  }
}

// leaveDocument は現在のスクロール位置を履歴へ記録する（IMP-311）。
//
// **文書を離れる直前に 1 回だけ呼ぶ。** スクロールのたびには呼ばない。
async function leaveDocument() {
  if (!state.doc) return;

  await api.setScrollTop($("viewer").scrollTop);
}

boot();
