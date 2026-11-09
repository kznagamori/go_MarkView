// blockedit.js — 編集欄の共通の部品（FR-140 の表, FR-142, FR-145, UI-055, UI-090, IMP-264）。
//
// セルの編集欄（celledit.js。IMP-262）とコードブロックの編集欄（codeedit.js）に共通する、**開いている間の扱い
// （拒む・保留・確定の待ち・開き直し）の判断を、このモジュールだけに置く。** 呼び出し側は入口で
// refuseIfEditing / holdIfEditing を呼ぶだけにする。
//
// **celledit.js / codeedit.js / editmode.js / viewer.js / navigate.js / main.js を import しない**（いずれもこの
// モジュールを呼ぶ。IMP-200）。控えたものを当てるのは main.js が渡す deps.applyHeld。

const NO_DEPS = { api: null, notify() {}, focusViewer() {}, isModalOpen: () => false, applyHeld() {} };
let deps = NO_DEPS;

// box は開いている編集欄 { target, cancel(), focus(), root() }。閉じていれば null。**同時に 1 つだけ。**
// **確定の応答を待つ間も開いているとみなす**（IMP-264）——待つ間の再描画も控え、失敗で開き直す先を残す。
let box = null;

// opening は「開きかけ」（SetEditBoxOpen(true) の応答を待つ間）。拒むが、控えない（IMP-264）。
let opening = false;

// submitting は確定の応答を待つ間。控えた再描画で警告を出さない（知らせは応答が決める。4.81.0）。
let submitting = false;

// held は控えたもの（最新の 1 つ）。{ item, switched, own, removed, afterRemoved }。
//   item: main.js の購読が渡した { type: "doc" | "state" | "removed", doc / error }
//   switched: 文書の切り替え（sameDocument が偽・状態画面）を一度でも控えたか
//   own: 控えたのが自分の書き込みの直後の再描画（trigger が edit）だけか
//   removed: 控えた削除の ErrorDTO（閉じた時点まで残す）。afterRemoved: 削除の後に再描画を控えたか
let held = null;

// sending は SetEditBoxOpen の送信の列。**前の呼び出しの応答を待ってから送る**（立て続けの false と true の到着順を
// 入れ替えない。IMP-210 の saveConfig と同じ事情）。
let sending = Promise.resolve();

// initBlockEdit は依存を受け取る（IMP-211）。given: { api, notify(error), focusViewer(), isModalOpen(), applyHeld(item) }。
export function initBlockEdit(given) {
  deps = { ...NO_DEPS, ...given };
}

// openBox は SetEditBoxOpen(true) を待ち、出してよければ開いたと登録して true を返す（IMP-264。4.76.0）。
//
// **待つ間は「開きかけ」**——refuseIfEditing は拒むが、holdIfEditing は控えない（届いた描画はそのまま当て、ここで
// target が DOM から外れたことで検知する）。出せなければ SetEditBoxOpen(false) を送って false。呼び出し側は true が
// 返ってから編集欄を DOM に置く。handlers: { cancel(), focus(), root() }（取り消し・フォーカスを戻す先・編集欄の器）。
export async function openBox(target, handlers) {
  if (box || opening) return false;

  opening = true;
  await send(true);
  opening = false;

  // **待つ間に文書が描かれていれば、target は DOM から外れている**（本文の差し替え。IMP-220 の手順 1）
  if (!target.isConnected || box) {
    send(false);
    return false;
  }

  box = { target, ...handlers };
  held = null;
  submitting = false;
  return true;
}

// closeBox は閉じたと登録し、控えたものを当てる（IMP-264）。SetEditBoxOpen(false) を送る。
//
// 削除の印があり、**その後に再描画を控えていなければ**、その後に編集モードを終える（削除の後に再描画が届いていれば、
// それがファイルが戻った後の Go 側の状態を運んでいる。4.76.0）。
export function closeBox() {
  if (!box) return;

  box = null;
  submitting = false;
  send(false);

  const last = held;
  held = null;
  if (!last) return;
  if (last.item) deps.applyHeld(last.item);
  if (last.removed && !last.afterRemoved) deps.applyHeld({ type: "removed", error: last.removed });
}

// isEditBoxOpen は開いているか（確定の応答を待つ間を含む）。
export function isEditBoxOpen() {
  return box !== null;
}

// isEditBoxFocused は編集欄（入力欄と Save / Cancel）にフォーカスがあるか（IMP-244 の Esc の (3)）。
export function isEditBoxFocused() {
  return box !== null && box.root().contains(document.activeElement);
}

// cancelEditBox は編集欄にフォーカスがあれば取り消して true を返す（IMP-244 の Esc の (3)。UI-090）。
// **フォーカスが無ければ取り消さない**——検索バーを閉じるつもりの Esc で入力を捨てない（4.73.0）。
export function cancelEditBox() {
  if (!isEditBoxFocused() || submitting) return false;

  box.cancel();
  return true;
}

// refuseIfEditing は、開いていれば（開きかけを含む）何もせずに edit-busy の警告を出し、フォーカスを編集欄へ戻して
// true を返す（FR-140 の表の「拒む」）。**情報ウィンドウ・エディタ選択ウィンドウ・拡大画面の表示中はフォーカスを
// 動かさない**——フォーカストラップを破らない。
export function refuseIfEditing() {
  if (!box && !opening) return false;

  deps.notify({ kind: "edit-busy" });
  if (box && !submitting && !deps.isModalOpen()) box.focus();
  return true;
}

// canTakeFocus は、編集欄へフォーカスを移してよいかを返す（情報ウィンドウ・エディタ選択ウィンドウ・拡大画面の表示中は
// 偽。FR-140）。失敗で開き直した編集欄（IMP-262, IMP-264）が、開いているダイアログのフォーカストラップを破らないようにする。
export function canTakeFocus() {
  return !deps.isModalOpen();
}

// holdIfEditing は、開いていれば画面を差し替えるものを控えて true を返す（FR-140 の表の「保留する」）。
//
// **控えるのは最新の 1 つだけ**とし、後から届いたもので置き換える。前に控えていたものが文書の切り替えなら、後の
// 再描画も切り替えとして当てる（sameDocument を偽にする）。**削除の印は、その後の再描画で消さない。**
export function holdIfEditing(item) {
  if (!box) return false;

  const next = held || { item: null, switched: false, own: true, removed: null, afterRemoved: false };
  if (item.type === "removed") {
    next.removed = item.error;
    next.afterRemoved = false;
  } else {
    const switching = item.type === "state" || item.doc.sameDocument !== true;
    next.switched = next.switched || switching;
    next.item = item.type === "doc" && next.switched ? { type: "doc", doc: { ...item.doc, sameDocument: false } } : item;
    next.own = next.own && item.type === "doc" && item.doc.trigger === "edit" && !switching;
    if (next.removed) next.afterRemoved = true;
  }
  held = next;

  warnHeld(item);
  return true;
}

// canSubmit は確定してよいかを返す（IMP-264）。控えた外部の更新・切り替え・状態画面があれば edit-held、削除の印が
// あれば edit-removed の警告を出し直して false。**編集欄は閉じず、Go へ送らない**（送っても Go 側が Stale にする。
// 安全は Go 側が保ち、ここは知らせ方を決める）。自分の書き込みの直後の再描画だけを控えているときは確定してよい。
export function canSubmit() {
  if (!held) return true;
  if (held.removed) {
    deps.notify({ kind: "edit-removed" });
    return false;
  }
  if (held.own) return true;

  deps.notify({ kind: "edit-held" });
  return false;
}

// markSubmitting は確定の応答を待つ間かを登録する（呼び出し側が確定の前後で呼ぶ）。
export function markSubmitting(on) {
  submitting = on;
}

// warnHeld は控えたものの警告を 1 つだけ出す（DSP-151 は前のメッセージを置き換える。4.76.0）。順に、削除なら
// removed-editing、不正なバイト列を含んだ再描画（editable が偽）なら encoding-editing、それ以外の外部の更新・
// 切り替え・状態画面なら edit-held。**自分の書き込みの直後は出さない。確定の応答を待つ間は、再描画の警告を出さない**
// （4.81.0。知らせは確定の応答が決める）——**削除は再描画ではなく、待つ間も知らせる**（IMP-264）。
function warnHeld(item) {
  if (submitting && item.type !== "removed") return;
  if (item.type === "removed") {
    deps.notify({ ...item.error, kind: "removed-editing" });
    return;
  }
  if (item.type === "doc" && item.doc.trigger === "edit" && item.doc.sameDocument === true) return;
  if (item.type === "doc" && item.doc.sameDocument === true && item.doc.editable === false) {
    deps.notify({ kind: "encoding-editing" });
    return;
  }
  deps.notify({ kind: "edit-held" });
}

// send は SetEditBoxOpen を列に積み、前の応答を待ってから送る（IMP-264, IMP-313）。呼び出しの失敗は無視する。
function send(open) {
  sending = sending.then(() => (deps.api ? deps.api.setEditBoxOpen(open) : null)).catch(() => null);
  return sending;
}
