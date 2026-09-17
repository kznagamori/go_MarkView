// shortcuts.js — キーボードショートカット（UI-090, IMP-244）。
//
// **キー割り当ての唯一の定義**を持つ。ツールバーのツールチップに載せる
// キー表記もここから組み立てる（IMP-290）。文言側にキーを書くと二重管理になる。
//
// window の keydown に 1 つだけリスナを置き、表を引いて処理する。
// 個々の要素にキーハンドラを分散させない（IMP-244）。

import { handleExpandKey, isExpandOpen } from "./expand.js";
import { isContextMenuOpen } from "./contextmenu.js";

// SHORTCUTS は UI-090 の一覧をそのまま写したもの。
//
// label は代表キー。keys が複数あってもツールチップには label だけを載せる。
// label が空のものは、ツールチップを持たない文脈依存の割り当てである。
//
// keys の表記は keyOf が組み立てる形（`Ctrl+Alt+Shift+` + KeyboardEvent.key）
// に揃える。**倍率のキーに別名が並ぶのはキーボード配列の違いによる。**
// US 配列では `+` が Shift+`=`、JIS 配列では Shift+`;` であり、テンキーの
// `+` には Shift が付かない。どの経路でも同じ動作になるよう、実際に届く
// 組み合わせをすべて表に書く。表以外の場所で例外を作らない。
export const SHORTCUTS = [
  { id: "open", keys: ["Ctrl+o"], label: "Ctrl+O" },
  { id: "reload", keys: ["F5", "Ctrl+r"], label: "F5" },
  { id: "theme", keys: ["Ctrl+Shift+t"], label: "Ctrl+Shift+T" },
  { id: "outline", keys: ["Ctrl+Shift+o"], label: "Ctrl+Shift+O" },
  { id: "filetree", keys: ["Ctrl+Shift+e"], label: "Ctrl+Shift+E" },
  { id: "edit", keys: ["Ctrl+e"], label: "Ctrl+E" },
  { id: "search", keys: ["Ctrl+f"], label: "Ctrl+F" },
  { id: "searchNext", keys: ["Enter"], label: "" },
  { id: "searchPrev", keys: ["Shift+Enter"], label: "" },
  { id: "close", keys: ["Escape"], label: "" },
  { id: "back", keys: ["Alt+ArrowLeft"], label: "Alt+←" },
  { id: "forward", keys: ["Alt+ArrowRight"], label: "Alt+→" },
  { id: "zoomIn", keys: ["Ctrl++", "Ctrl+Shift++", "Ctrl+=", "Ctrl+Shift+="], label: "Ctrl++" },
  { id: "zoomOut", keys: ["Ctrl+-", "Ctrl+Shift+-", "Ctrl+_", "Ctrl+Shift+_"], label: "Ctrl+-" },
  { id: "zoomReset", keys: ["Ctrl+0"], label: "Ctrl+0" },
  { id: "copySelection", keys: ["Ctrl+c"], label: "" },
  { id: "about", keys: ["F1"], label: "F1" },
  { id: "quit", keys: ["Ctrl+q"], label: "Ctrl+Q" },

  // v1.1.0（UI-090, IMP-244）。editMode はツールバーのボタンと同じ入口（IMP-260 の toggleEditMode）を通す。
  // undo / redo は編集モードでなければ false を返し、既定の動作を止めない（IMP-263）。
  //
  // **Shift+F10 とアプリケーションキーは載せない。** WebView がこれらのキーで contextmenu イベントを
  // 出すため、右クリックと同じ経路（IMP-249）で受ける。
  { id: "editMode", keys: ["Ctrl+Shift+m"], label: "Ctrl+Shift+M" },
  { id: "undo", keys: ["Ctrl+z"], label: "" },
  { id: "redo", keys: ["Ctrl+y", "Ctrl+Shift+z"], label: "" },
];

// 入力欄にフォーカスがある間、素通しさせる割り当て（UI-090）。
//
// **テキスト編集に関わるものだけを挙げる。** UI-090 は「テキスト編集に
// 関わるキーを優先する」と定めているのであって、入力中はすべてのショート
// カットを止める、とは定めていない。Ctrl+Shift+T などは入力中も効く。
//
// なお Ctrl+C はもともとハンドラを持たない（WebView の既定に任せる。
// FR-062）。ここに挙げるのは、後から誤って結び付けられないようにするため
// である。
//
// **undo / redo はテキスト編集に関わるキーとして扱う**（FR-144, IMP-244）。入力欄の中の取り消しに任せる。
const PASS_THROUGH_IN_INPUT = new Set(["copySelection", "undo", "redo"]);

// TEXT_INPUTS は「入力欄」として扱う要素（UI-090, IMP-244）。検索バーの入力欄とセルの編集欄だけとする。
//
// **チェックボックスとラジオボタンを入力欄に数えない。** v1.0.0 は input 要素をすべて入力欄とみなしていた。
// v1.1.0 では再描画の後にフォーカスがチェックボックスへ戻る（IMP-220 の手順 11）ため、そのままでは
// チェックボックスを切り替えた直後の Ctrl+Z / Ctrl+Y が素通しされ、既定の動作にも何も無いため黙って
// 何も起きない（FR-144）。
const TEXT_INPUTS = "input.search-input, input.cell-editor";

// CELL_EDITOR はセルの編集欄（IMP-262）。**Enter / Shift+Enter を検索の移動へ回さない**（FR-142, UI-090）。
// 編集欄自身の keydown が確定を受け持つ。
const CELL_EDITOR = "input.cell-editor";
const SEARCH_MOVES = new Set(["searchNext", "searchPrev"]);

// 拡大画面を開いている間も既定の動作を残す割り当て（Enter はフォーカスしているボタンの実行。IMP-253）。
const KEEP_DEFAULT_IN_EXPAND = SEARCH_MOVES;

// keys から id を引く表。起動時に 1 度だけ組み立てる。
const byKey = new Map();
for (const shortcut of SHORTCUTS) {
  for (const key of shortcut.keys) byKey.set(key, shortcut.id);
}

let handlers = {};

// initShortcuts は keydown を配線する（IMP-211, IMP-244）。
//
// handlers は id をキーとする処理の表。**未実装のものは渡さない。**
// 押しても無反応になるが、動かない処理を仮に繋ぐより状態がはっきりする。
//
// Alt+F4 と閉じるボタンは OS とウィンドウマネージャが処理するため、
// ここでは扱わない（UI-090）。
export function initShortcuts(deps) {
  handlers = deps || {};

  window.addEventListener("keydown", onKeyDown);
}

// keyLabel はツールチップに載せる代表キーを返す。未定義なら空文字。
export function keyLabel(id) {
  const found = SHORTCUTS.find((s) => s.id === id);
  return found ? found.label : "";
}

function onKeyDown(event) {
  // IME の変換中は何も割り当てない。変換確定の Enter を検索の移動として
  // 拾ってしまう。
  if (event.isComposing) return;

  const id = byKey.get(keyOf(event));

  // **右クリックメニューを開いている間は、Esc 以外の割り当てを働かせず、既定の動作も止めない**（IMP-244）。
  // メニューの中の ↑ / ↓ / Enter / Space / Tab は #contextmenu の keydown が受ける（IMP-249）。止めないと、
  // 検索バーの入力欄でメニューを開いて Paste で Enter を押したとき、Enter が検索の移動へ回る。
  if (isContextMenuOpen() && id !== "close") return;

  // **拡大画面を開いている間は、Esc を下の振り分け（close）で先に扱い、UI-104 の表の残りのキーを
  // expand.js へ渡し、それ以外の割り当てを止める**（Ctrl+Q を除く。IMP-244）。止めるときは既定の動作も
  // 抑止する（Ctrl + `+` が WebView 自身の拡大になる）。ただし Enter / Shift+Enter はフォーカスしている
  // 操作バーのボタンの実行に任せる（IMP-253。ダイアログの KEEP_DEFAULT_IN_DIALOG と同じ理由）。
  if (isExpandOpen() && id !== "close" && id !== "quit") {
    if (handleExpandKey(event) || (id && !KEEP_DEFAULT_IN_EXPAND.has(id))) event.preventDefault();
    return;
  }

  if (!id) return;

  if (isTextInput(event.target) && PASS_THROUGH_IN_INPUT.has(id)) return;
  if (SEARCH_MOVES.has(id) && matches(event.target, CELL_EDITOR)) return;

  const handler = handlers[id];
  if (!handler) return;

  // **preventDefault はここで一括して呼ぶ**（IMP-244）。個々のハンドラに
  // 書き漏らす余地を残さない。
  //
  // ただしハンドラが false を返したときは「何もしなかった」とみなし、
  // 既定の動作を止めない。検索が閉じているときの Enter が、
  // フォーカス中のボタンの実行（UI-021）を妨げないようにするためである。
  if (handler() === false) return;

  event.preventDefault();
}

// keyOf は KeyboardEvent を SHORTCUTS の表記へ変える。
//
// 修飾子の順序は Ctrl → Alt → Shift に固定する。表と同じ順で並べないと
// 引けない。
//
// **1 文字のキーは小文字へ揃える。** Shift と CapsLock で KeyboardEvent.key の
// 大小が変わるためであり、Shift の有無は修飾子側で区別する。
// CapsLock が入っているだけで Ctrl+O が効かない、という事態を避ける。
function keyOf(event) {
  const parts = [];

  if (event.ctrlKey) parts.push("Ctrl");
  if (event.altKey) parts.push("Alt");
  if (event.shiftKey) parts.push("Shift");
  parts.push(event.key.length === 1 ? event.key.toLowerCase() : event.key);

  return parts.join("+");
}

// isTextInput はテキストの入力欄にフォーカスがあるかを返す（UI-090, IMP-244）。
function isTextInput(target) {
  return matches(target, TEXT_INPUTS);
}

// matches は target が要素で、selector に合うかを返す。
function matches(target, selector) {
  return Boolean(target) && typeof target.matches === "function" && target.matches(selector);
}
