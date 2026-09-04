// main.js — 起動処理とイベント配線（IMP-211）。
//
// 各モジュールが export する初期化関数をここから順に呼ぶ（IMP-201）。
// グローバル変数を作らない。window への代入を行わない。

import * as api from "./api.js";
import { state } from "./state.js";
import { S, errorText } from "./strings.js";
import { initToolbar, canEdit } from "./toolbar.js";
import { initTooltip } from "./tooltip.js";
import { applyTheme, toggleTheme } from "./theme.js";
import { initZoom, setZoom, stepZoom } from "./zoom.js";
import { applyPanes, initPanes, togglePane } from "./panes.js";
import { initFileTree, loadTreeRoot, revealCurrent } from "./filetree.js";
import { initOutline } from "./outline.js";
import { initViewer, renderDocument, scrollToAnchor } from "./viewer.js";
import { initSearch, openSearch, closeSearch, isSearchOpen, jump } from "./search.js";
import { initShortcuts } from "./shortcuts.js";
import {
  initOverlay,
  showStateScreen,
  showAbout,
  hideAbout,
  isAboutOpen,
  showEditors,
  hideEditors,
  isEditorsOpen,
} from "./overlay.js";
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
  filetree: () => toggleFileTree(),
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
  edit: () => showEditorDialog(),
  quit: () => api.quit(),
});

// ダイアログ表示中に**既定の動作を奪ってはならない**割り当て（UI-103）。
//
// `Enter` の既定の動作は「フォーカスしている操作要素を実行する」であり、
// これは背後ではなく**ダイアログ自身の操作**である。UI-103 は「初期選択が
// あれば `Open` にフォーカスを置き、ボタンと `Enter` の 2 操作で開ける」と
// 定めており、ここで `preventDefault` すると**その 2 操作目が効かなくなる。**
// 情報ダイアログの `Close` も同じである（UI-100）。
//
// 割り当て自体は止まる（背後の検索へは進まない）ので、両立する。
const KEEP_DEFAULT_IN_DIALOG = new Set(["searchNext", "searchPrev"]);

// guardAll はダイアログ表示中の割り当てを止める（UI-100, UI-103）。
//
// 「表示中は背後のメインウィンドウの操作を受け付けない」を、マウス（暗幕が
// 覆う）だけでなくキーボードでも成り立たせる。**Esc だけは通す。**
//
// **2 つのダイアログを同じ条件で止める。** 片方だけ止めると、開いている
// ダイアログの種類で背後の効き方が変わる（IMP-252）。
function guardAll(handlers) {
  const guarded = {};

  for (const [id, handler] of Object.entries(handlers)) {
    guarded[id] = id === "close" ? handler : () => guard(id, handler);
  }

  return guarded;
}

// guard は 1 つの割り当てをダイアログ表示中だけ止める（UI-100, UI-103）。
//
// 止めるときは原則 true を返し、既定の動作も抑止する。false を返すと、
// たとえば `Ctrl` + `+` が WebView 自身のページ拡大として処理されてしまう。
// **例外は KEEP_DEFAULT_IN_DIALOG だけである。**
function guard(id, handler) {
  if (!isDialogOpen()) return handler();

  return !KEEP_DEFAULT_IN_DIALOG.has(id);
}

// isDialogOpen はどちらかのダイアログが開いているかを返す（IMP-252）。
//
// **判定をこの 1 か所にまとめる。** 呼ぶ場所ごとに 2 つを並べて書くと、
// 片方を足し忘れたときに「あるダイアログの表示中だけ背後が効く」になる。
function isDialogOpen() {
  return isAboutOpen() || isEditorsOpen();
}

// boot は起動時の 1 回だけ実行する（IMP-211）。
async function boot() {
  const init = await api.getInitialState();

  // テーマの適用を最優先で行い、既定色から切り替わるちらつきを防ぐ（UI-105）。
  applyTheme(init.config.theme);

  // 倍率は復元しない。常に 100 % から始まる（UI-111, UI-115, IMP-242）。
  // state.zoom の初期値が 100 であり、--zoom も未設定時は 100 として効く
  // （DSP-021）ため、起動時に適用する処理は要らない。
  applyPanes(init.config);

  initToolbar({
    onOpen: openViaDialog,
    onReload: reloadCurrent,
    onTheme: toggleTheme,
    onOutline: () => togglePane("outline"),
    onFileTree: toggleFileTree,
    onEdit: showEditorDialog,
    onAbout: showAboutDialog,
  });
  initTooltip();
  initFileTree({ onOpen: openFromTree });
  initOutline();
  initViewer({ onFollow: followLink });
  initPanes();
  initSearch();
  initZoom();
  initOverlay({ onLink: followLink, onBrowse: browseEditor, onOpenEditor: openInEditor });
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

  // **ツリーも読み直す**（FR-035 の 2 番目の契機。IMP-240）。
  //
  // Reload() は表示中の文書を開き直すだけでツリーに触れない（IMP-310）ため、
  // ここで続けて呼ぶ。**文書の再読み込みが失敗しても行う**——FR-035 が契機と
  // 定めているのは「再読み込み操作を行ったとき」であり、その成否ではない。
  await loadTreeRoot(state.treeRoot);
}

// toggleFileTree はツリーペインを開閉し、**表示になったらツリーを読み直す**
// （FR-035 の 1 番目の契機。IMP-240）。
//
// **ツールバーのボタンとショートカットの両方がここを通る。** 片方だけに足すと、
// 経路によって挙動が変わる。
//
// **表示になったときだけ読む。** ReadDir は毎回ディスクを読む（IMP-310）ため、
// 閉じる操作や、既に開いている状態で読み直すと、大きなディレクトリで引っかかる
// （NFR-020）。
//
// loadTreeRoot はツリーを作り直すため、利用者が開いていたディレクトリの展開状態は
// 失われ、表示中の文書までの経路だけが開き直される（revealCurrent。DSP-331）。
// FR-035 は展開状態の保持を求めていない。
async function toggleFileTree() {
  if (!togglePane("filetree")) return;

  await loadTreeRoot(state.treeRoot);
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
// 順序は重なり順（DSP-015）に従う。ダイアログのほうが検索バーより上に
// あるため先に閉じる。閉じるものがなければ false を返す（IMP-244）。
//
// **2 つのダイアログは同時に開かない**（IMP-252）ため、どちらを先に見ても
// 結果は変わらない。開いていないほうは false を返して素通りする。
function closeTop() {
  if (hideAbout()) return true;
  if (hideEditors()) return true;
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

// --- エディタで開く（FR-090, FR-091, IMP-331） ---

// showEditorDialog はエディタ選択ウィンドウを開く（FR-091, UI-103）。
//
// **Go を呼ぶのはここだけとし、overlay.js は受け取った値を描くだけにする**
// （IMP-201）。**押すたびに一覧を取り直す**（IMP-310, NFR-013）。
//
// ボタンとショートカットの両方がここへ来る。**押せるかどうかの判定も
// ここ 1 か所に置く。** ボタンは淡色で防げるが（UI-021）、Ctrl+E は
// それだけでは止まらない。
async function showEditorDialog() {
  if (!canEdit()) return;

  const list = await api.listEditors();

  // 一覧すら作れなかったときはウィンドウを出さない。**選べるものが無い
  // ウィンドウを出さない。** 理由はステータス領域へ出す（IMP-315）。
  if (list && list.error) {
    showError(list.error);
    return;
  }

  showEditors(list);
}

// browseEditor は実行ファイルを選ぶダイアログを開き、新しい一覧を返す
// （FR-091, IMP-310）。**描き直すのは overlay.js の役目**である（IMP-252）。
async function browseEditor() {
  const list = await api.browseEditor();

  // 作れなかったときは null を返し、**いま出ている一覧を保つ**（IMP-252）。
  if (list && list.error) {
    showError(list.error);
    return null;
  }

  return list;
}

// openInEditor は選ばれたエディタで開き、結果をステータス領域へ出す
// （FR-090, DSP-151, IMP-331）。
//
// **成功したときも出す。** エディタが背面のウィンドウで開くことがあり、
// 何も出ないと「押しても無反応」に見える（FR-090, UI-060）。
async function openInEditor(id) {
  const result = await api.openInEditor(id);
  if (!result) return;

  if (result.error) {
    showError(result.error);
    return;
  }

  showMessage(S.statusEditor(result.name), "info");
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
