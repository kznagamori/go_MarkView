// main.js — 起動処理とイベント配線（IMP-211）。
//
// 各モジュールが export する初期化関数をここから順に呼ぶ（IMP-201）。
// グローバル変数を作らない。window への代入を行わない。
//
// **文書を開く経路と結果の反映（状態画面を出すことを含む）は navigate.js にある**（IMP-250）。
// ここは起動・Go からのイベントの購読・ショートカットとダイアログの配線だけにする。

import * as api from "./api.js";
import { state } from "./state.js";
import { S, errorText } from "./strings.js";
import { initToolbar, canEdit } from "./toolbar.js";
import { initTooltip } from "./tooltip.js";
import { applyTheme, toggleTheme } from "./theme.js";
import { initZoom, setZoom, stepZoom } from "./zoom.js";
import { applyPanes, initPanes, togglePane } from "./panes.js";
import { initFileTree, loadTreeRoot } from "./filetree.js";
import {
  enterStateScreen,
  followLink,
  goBack,
  goForward,
  isStateScreenError,
  openFromTree,
  openViaDialog,
  reloadCurrent,
  showDocument,
  showError,
} from "./navigate.js";
import { initOutline } from "./outline.js";
import { focusViewer, initViewer } from "./viewer.js";
import { initSearch, openSearch, closeSearch, isSearchOpen, jump } from "./search.js";
import { initShortcuts } from "./shortcuts.js";
import {
  initOverlay,
  showAbout,
  hideAbout,
  isAboutOpen,
  showEditors,
  hideEditors,
  isEditorsOpen,
} from "./overlay.js";
import { initDnd } from "./dnd.js";
import {
  closeExpand,
  initExpand,
  isExpandOpen,
  onAllDiagramsSettled,
  onImagesNumbered,
  onTargetSettled,
  openExpand,
} from "./expand.js";
import { initMedia } from "./media.js";
import { initTableSort } from "./tablesort.js";
import { closeContextMenu, initContextMenu } from "./contextmenu.js";
import {
  cancelCellEdit,
  initEditMode,
  isCellEditing,
  leaveEditMode,
  redoEdit,
  toggleEditMode,
  undoEdit,
} from "./editmode.js";
import { showMessage } from "./status.js";

// SHORTCUTS の id と処理の対応（UI-090, IMP-244）。
//
// **未実装のものは載せない。** copySelection（Ctrl+C）は WebView の既定に
// 任せるためここには現れない（FR-062）。Alt+F4 と閉じるボタンは OS が処理する。
//
// editMode はツールバーのボタンと同じ入口（toggleEditMode。IMP-260）を通し、開始できない状態なら false を
// 返す。undo / redo は編集モードでなければ false を返し、既定の動作を止めない（IMP-263）。**入力欄では
// shortcuts.js が素通しする**（FR-144）。**右クリックメニューと拡大画面を開いている間のキーは
// shortcuts.js が扱う**（IMP-244, IMP-249, IMP-253）。
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
  editMode: () => toggleEditMode(),
  undo: () => undoEdit(),
  redo: () => redoEdit(),
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
    onEditMode: () => toggleEditMode(),
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
  // 図と画像のボタンから拡大画面を開き、対象の準備と数を拡大画面へ知らせる（IMP-228, IMP-253）
  initMedia({ onExpand: openExpand, onTargetSettled, onImagesNumbered, onAllDiagramsSettled });
  initExpand({ focusViewer });
  initTableSort({ closeSearch });
  // 編集モード（IMP-260〜IMP-263）。書き込みの失敗はステータス領域に出す（DSP-321）
  initEditMode({ api, notify: (error) => showMessage(errorText(error), "error"), focusViewer });
  // 右クリックメニュー（IMP-249）。Copy は WebView 自身のコピー（execCommand）であり、Go を通すのは
  // Copy link address・execCommand が効かなかったときの代わりの経路・Paste の読み取りだけ（AR-062）。
  // セルの編集欄から開いてメニューの外を押したら、フォーカスが外れたものとして取り消す（IMP-262）。
  initContextMenu({
    copyText: api.copyToClipboard,
    readClipboard: api.readClipboard,
    notify: (error) => showMessage(errorText(error), "error"),
    cancelCellEdit,
  });
  initShortcuts(SHORTCUT_HANDLERS);

  subscribe();

  state.treeRoot = init.treeRoot;

  if (init.document) {
    showDocument(init.document);
  } else {
    enterStateScreen(init.stateKind || "welcome", init.error);
    // 起動時のパスが読めなかった場合、操作案内はそのままに理由を添える
    // （FR-012, IMP-193）。
    if (init.error && !isStateScreenError(init.error)) {
      showMessage(errorText(init.error), "error");
    }
  }

  if (init.treeRoot) await loadTreeRoot(init.treeRoot);
}

// subscribe は Go からのイベントを購読する。**起動時の 1 回のみ**（IMP-322）。
function subscribe() {
  api.on(api.EVENT.documentOpened, showDocument);
  api.on(api.EVENT.documentChanged, showDocument);

  // 削除されても本文とタイトルは維持する（FR-110, UI-013）。**編集モードは終える**——Go 側は既に終えており、
  // 次に読み込めるまで開始させない（IMP-260, IMP-320, FR-140 の表）。
  api.on(api.EVENT.documentRemoved, (error) => {
    leaveEditMode();
    showMessage(errorText(error), "error");
  });

  api.on(api.EVENT.treeRootChanged, async (root) => {
    state.treeRoot = root;
    await loadTreeRoot(root);
  });

  api.on(api.EVENT.error, showError);
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

// closeTop は Esc の受け口（UI-090）。**上に重なっているものから閉じる。**
//
// **振り分けは UI-090 の順序に固定し、この 1 か所に書く**（IMP-244）。1 回の押下で 1 つだけに働かせ、
// 最初に当てはまったところで戻る。**他のモジュールは Esc を扱わない**——2 か所に書くと、
// stopPropagation の有無で順序が入れ替わる。閉じるものがなければ false を返す。
//
//   (1) 右クリックメニュー
//   (2) 拡大画面・情報ダイアログ・エディタ選択ダイアログ
//   (3) セルの編集欄（取り消して、本文ペインへフォーカスを戻す）
//   (4) 検索バー
//
// **2 つのダイアログは同時に開かない**（IMP-252）ため、どちらを先に見ても
// 結果は変わらない。開いていないほうは false を返して素通りする。
function closeTop() {
  // (1) 開く前のフォーカスへ戻す（IMP-249）
  if (closeContextMenu()) return true;

  // (2) 拡大画面は情報ダイアログ・エディタ選択ダイアログと同時に開かない（IMP-251, IMP-253）
  if (closeExpand()) return true;
  if (hideAbout()) return true;
  if (hideEditors()) return true;

  // (3) 取り消して本文ペインへ戻す（IMP-262, UI-055）
  if (isCellEditing()) {
    cancelCellEdit();
    focusViewer();
    return true;
  }

  // (4)
  if (!isSearchOpen()) return false;

  closeSearch();
  return true;
}

// showAboutDialog は情報ダイアログを開く（FR-100, UI-100）。
//
// **Go を呼ぶのはここだけとし、overlay.js は受け取った値を描くだけにする**
// （IMP-201）。
async function showAboutDialog() {
  const about = await api.getAbout();

  // **応答を待つ間に拡大画面が開いていれば出さない**（IMP-251）。同じ層（DSP-015）で #expand-view が
  // #overlay より後ろにあるため、出すと拡大画面の下に隠れる。
  if (isExpandOpen()) return;
  showAbout(about);
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

  // 応答を待つ間に拡大画面が開いていれば出さない（IMP-252。情報ダイアログと同じ理由）
  if (isExpandOpen()) return;
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

boot();
