// main.js — 起動処理とイベント配線（IMP-211）。
//
// 各モジュールが export する初期化関数をここから順に呼ぶ（IMP-201）。
// グローバル変数を作らない。window への代入を行わない。
//
// **文書を開く経路と結果の反映（状態画面を出すことを含む）は navigate.js にある**（IMP-250）。
// ここは起動・Go からのイベントの購読・ショートカットとダイアログの配線だけにする。

import * as api from "./api.js";
import { state } from "./state.js";
import { S } from "./strings.js";
import { initToolbar, applyHistory } from "./toolbar.js";
import { initTooltip } from "./tooltip.js";
import { applyTheme, toggleTheme } from "./theme.js";
import { initZoom, setZoom, stepZoom } from "./zoom.js";
import { applyPanes, initPanes, togglePane } from "./panes.js";
import { initFileTree, loadTreeRoot } from "./filetree.js";
import { initTreeMenu, runTreeAction, treeMenuItems } from "./treemenu.js";
import { browseEditor, closeEditorDialog, openEditorsForTree, openInEditor, showEditorDialog } from "./editorflow.js";
import { closeTreeFilter, initTreeFilter, isTreeFilterFocused, isTreeFilterOpen } from "./treefilter.js";
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
import { closeContextMenu, closeSubmenu, initContextMenu } from "./contextmenu.js";
import { COLORS, applyColor, canFormat, initFormat, selectionState, toggleStrike } from "./format.js";
import { cancelEditBox, holdIfEditing, initBlockEdit, refuseIfEditing } from "./blockedit.js";
import { initCellEdit } from "./celledit.js";
import { initCodeEdit } from "./codeedit.js";
import {
  initEditMode,
  leaveEditMode,
  redoEdit,
  toggleEditMode,
  undoEdit,
} from "./editmode.js";
import { notifyError, updateStatus } from "./status.js";
import { $ } from "./util.js";

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
  strike: () => toggleStrike(),
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
  // ツリーのフィルター（FR-037。IMP-211。4.76.0）。**写さないと、最初の saveConfig が既定値を送って保存値を
  // 上書きする**（Go 側の ReadDir は保存値で絞っており、枚のチェックボックスとツリーの中身も食い違う）
  state.treeShowDiagrams = init.config.treeShowDiagrams;
  state.treeShowImages = init.config.treeShowImages;

  initToolbar({
    onBack: () => goBack(),
    onForward: () => goForward(),
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
  // ツリーのフィルターの枚（IMP-271）。文言の拡張子は Go の定義から（FR-037）。上で写したフィルターの値を使う
  initTreeFilter(init.extensions, { focusViewer });
  initOutline();
  initViewer({ onFollow: followLink });
  initPanes();
  initSearch();
  initZoom();
  // エディタ選択ウィンドウを Open 以外で閉じたら Go の控えを捨てさせる（IMP-252）
  initOverlay({ onLink: followLink, onBrowse: browseEditor, onOpenEditor: openInEditor, onCloseEditors: closeEditorDialog });
  initDnd();
  // 図と画像のボタンから拡大画面を開き、対象の準備と数を拡大画面へ知らせる（IMP-228, IMP-253）。
  // 拡大画面を開くときはフィルターの枚を閉じる（同時に開かない。IMP-271）
  const onExpand = (target, key, opener) => {
    closeTreeFilter(false);
    openExpand(target, key, opener);
  };
  initMedia({ onExpand, onTargetSettled, onImagesNumbered, onAllDiagramsSettled });
  initExpand({ focusViewer });
  initTableSort({ closeSearch });
  // 編集モード（IMP-260〜IMP-263）。書き込みの失敗はステータス領域に出す（DSP-321）
  initEditMode({ api, notify: notifyError, focusViewer });
  // 編集欄の共通の部品（IMP-264）とセルの編集欄（IMP-262）。控えたものは届いたときと同じ処理で当てる
  initBlockEdit({ api, notify: notifyError, focusViewer, isModalOpen: () => isDialogOpen() || isExpandOpen(), applyHeld });
  initCellEdit({ api, notify: notifyError, focusViewer });
  initCodeEdit({ api, notify: notifyError, focusViewer });
  // 選んだ文字の修飾（IMP-265）。ツールバーのボタンと選択の監視
  initFormat({ api, notify: notifyError });
  // 右クリックメニュー（IMP-249）。Copy は WebView 自身のコピー（execCommand）であり、Go を通すのは
  // Copy link address・execCommand が効かなかったときの代わりの経路・Paste の読み取りだけ（AR-062）。
  // セルの編集欄から開いてメニューの外を押したら、フォーカスが外れたものとして取り消す（IMP-262）。
  // ファイルツリーの右クリックメニュー（IMP-270）。項目と実行は treemenu.js、器と開閉は contextmenu.js
  initTreeMenu({ api, notify: notifyError, openFromTree, openEditorsForTree, applyTreeAction });
  initContextMenu({
    copyText: api.copyToClipboard,
    readClipboard: api.readClipboard,
    notify: notifyError,
    treeMenu: { items: treeMenuItems, run: runTreeAction },
    format: { enabled: () => state.editMode, canFormat, selectionState, toggleStrike, applyColor, colors: COLORS },
  });
  // 戻る・進むの淡色（IMP-323）。起動時の値は InitialStateDTO.History で受け取る——購読より前に起きた
  // 積む処理を取りこぼさない。history:changed と同じ applyHistory を通す
  applyHistory(init.history);
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
      notifyError(init.error);
    }
  }

  if (init.treeRoot) await loadTreeRoot(init.treeRoot);
}

// subscribe は Go からのイベントを購読する。**起動時の 1 回のみ**（IMP-322）。
function subscribe() {
  // **編集欄が開いている間は、画面を差し替えるものを控える**（IMP-264 の「保留する」）。閉じた時点で applyHeld が当てる
  const onDocument = (doc) => {
    if (!holdIfEditing({ type: "doc", doc })) showDocument(doc);
  };
  api.on(api.EVENT.documentOpened, onDocument);
  api.on(api.EVENT.documentChanged, onDocument);

  // 削除されても本文とタイトルは維持する（FR-110, UI-013）。**編集モードは終える**——Go 側は既に終えており、
  // 次に読み込めるまで開始させない（IMP-260, IMP-320, FR-140 の表）。編集欄が開いていれば控え、閉じた時点で終える
  api.on(api.EVENT.documentRemoved, (error) => {
    if (holdIfEditing({ type: "removed", error })) return;
    leaveEditMode();
    notifyError(error);
  });

  api.on(api.EVENT.treeRootChanged, async (root) => {
    state.treeRoot = root;
    await loadTreeRoot(root);
  });

  // 状態画面の種別は、編集欄が開いていれば控える（IMP-264）。**ドロップを Go が拒んだ edit-busy**（IMP-245）は、
  // 編集欄が開いていれば押して拒んだときと同じく警告を出し、フォーカスを編集欄へ戻す（ダイアログ・拡大画面の
  // 表示中は動かさない。refuseIfEditing）。閉じた後に届いたら警告だけ
  api.on(api.EVENT.error, (error) => {
    if (error && isStateScreenError(error) && holdIfEditing({ type: "state", error })) return;
    if (error && error.kind === "edit-busy" && refuseIfEditing()) return;
    showError(error);
  });
  // 戻れるか・進めるか（IMP-323）。seq の古い値は applyHistory が捨てる
  api.on(api.EVENT.historyChanged, applyHistory);
}

// applyHeld は、編集欄を閉じた時点で控えたものを当てる（IMP-264）。**届いたときと同じ処理へ渡す**——再描画は
// showDocument、状態画面は showError、削除は編集モードを終える（知らせは控えたときに removed-editing で出した）。
function applyHeld(item) {
  if (item.type === "doc") showDocument(item.doc);
  else if (item.type === "state") showError(item.error);
  else if (item.type === "removed") leaveEditMode();
}

// applyTreeAction はツリーの右クリックメニューでツリールートを変えた結果（TreeActionDTO）を反映する（FR-030, IMP-270）。
// ツリーを新しいルートで読み直し（FR-035 の 4 番目の契機）、ステータス領域のパスを差し替える（FR-052。表示用パスは
// Go が求める）。**表示中の文書・履歴・編集モードは変えない。** メニューを開いた項目が組み直しで消えて
// フォーカスが body に落ちたら、組み直したツリーの着地点（tabindex="0"。IMP-248）へ移す（DSP-330）。
async function applyTreeAction(dto) {
  state.treeRoot = dto.root;
  await loadTreeRoot(dto.root);

  const shown = state.doc ?? state.target;
  if (shown) {
    shown.displayPath = dto.displayPath;
    shown.outsideTree = dto.outsideTree;
  }
  updateStatus();

  if (!document.activeElement || document.activeElement === document.body) {
    const landing = $("tree").querySelector('li.tree-item[tabindex="0"]');
    if (landing) landing.focus({ preventScroll: true });
  }
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
  // 隠したらフィルターの枚も閉じる。枚の中のフォーカスは本文ペインへ（IMP-271）
  if (!togglePane("filetree")) {
    closeTreeFilter(false);
    return;
  }

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
//   (3) 編集欄にフォーカスがあれば取り消して、本文ペインへフォーカスを戻す
//   (4) フィルターの枚と検索バーのうち、フォーカスのある方。どちらにも無ければ枚を先に（4.73.0。IMP-271）
//
// **2 つのダイアログは同時に開かない**（IMP-252）ため、どちらを先に見ても
// 結果は変わらない。開いていないほうは false を返して素通りする。
function closeTop() {
  // (1) サブメニューが開いていればサブメニューだけ。それ以外は開く前のフォーカスへ戻す（IMP-249）
  if (closeSubmenu() || closeContextMenu()) return true;

  // (2) 拡大画面は情報ダイアログ・エディタ選択ダイアログと同時に開かない（IMP-251, IMP-253）
  if (closeExpand()) return true;
  if (hideAbout()) return true;
  if (hideEditors()) return true;

  // (3) 編集欄（セル・コードブロック）にフォーカスがあれば、取り消して本文ペインへ戻す（IMP-262, IMP-264, UI-055）。
  //     **フォーカスが無ければ取り消さない**（4.73.0）——検索バーを閉じるつもりの Esc で入力を捨てない
  if (cancelEditBox()) return true;

  // (4) 利用者が閉じるので、枚はボタンへフォーカスを戻す（IMP-271）
  const searchFocused = isSearchOpen() && $("searchbar").contains(document.activeElement);
  if (isTreeFilterOpen() && (isTreeFilterFocused() || !searchFocused)) return closeTreeFilter(true);
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
  closeTreeFilter(false); // 同時に開かない（IMP-271）
  showAbout(about);
}

boot();
