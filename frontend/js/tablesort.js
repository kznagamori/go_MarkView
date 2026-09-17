// tablesort.js — 表の表示上の並べ替え（FR-130, IMP-229, UI-054, DSP-125）。
//
// **並べ替えるのは表示だけであり、ファイルは書き換えない**（FR-130）。`tbody` の `tr` を付け替えるだけで
// 行い、セルの中身を作り直さない（ボタン・`<mark>`・図のボタンが消える）。
//
// **対象は鍵の合う GFM の表だけとする**（IMP-260 の `isOwnRef(table, "table")`）。鍵をこのモジュールで
// 読まない。生 HTML の `<table>` は目印を持たないか鍵が合わず、見出し行と本体の行の区別もセルの結合の
// 有無も保証されない（NFR-030）。
//
// **viewer.js / overlay.js / main.js / navigate.js / docswitch.js を import しない**（IMP-250。docswitch.js
// から呼ばれるため、循環する）。

import { isOwnRef } from "./refs.js";
import { S } from "./strings.js";
import { icon } from "./util.js";

// 並べ替えの向き（FR-130）。元の順は向きを持たない（状態から消す）。
const ASC = "asc";
const DESC = "desc";

// 向きごとのアイコンと aria-sort の値（IMP-203, IMP-295）。**アイコンの形で昇順と降順を区別する**
// （色だけに頼らない。UI-054）。
const ICONS = { [ASC]: "icon-sort-asc", [DESC]: "icon-sort-desc" };
const ICON_NONE = "icon-sort";
const ARIA_SORT = { [ASC]: "ascending", [DESC]: "descending" };

// DocumentDTO.trigger のうち、行の並びを保つもの（IMP-302, IMP-195 の 8）。
const TRIGGER_EDIT = "edit";

// 行の元の順を持つ属性（IMP-229）。先頭のセルの data-ref に頼らない——縦棒だけの行はセルがすべて
// 補われ、data-ref を持たない（IMP-120）。
const ROW_ATTR = "data-sort-row";

// 数値として解釈する形（FR-130, IMP-229）。`\d` は u フラグが無ければ ASCII の数字だけに合う。
const NUMBER = /^[+-]?(\d+(\.\d*)?|\.\d+)$/;

// 数値でない文字列の比べ方（IMP-229 の手順 4）。大文字小文字を区別せず、数字の並びを数の大小で比べる。
const collator = new Intl.Collator(undefined, { numeric: true, sensitivity: "base" });

// sorts は表の番号（`table:<t>` の t。鍵の合う表の文書の中の順番）ごとの並べ替えの状態
// （{ col, dir, cols, order }）。並べ替えていない表は持たない。**設定に保存しない**（UI-111, NFR-042）。
const sorts = new Map();

// attached は最後に attachSortButtons が準備した表（番号ごと。対象でない表は null）。
// 各要素は { rows, headers, cols, buttons }。**rows は元の順**（添字が data-sort-row の値）であり、並べ替えは
// 常にここから始める。DOM の並びから元の順を読み直さない（IMP-229）。
let attached = [];

let deps = { closeSearch() {} };

// initTableSort は依存を受け取る（{ closeSearch }。IMP-211）。
export function initTableSort(given) {
  deps = Object.assign({ closeSearch() {} }, given);
}

// attachSortButtons は鍵の合う GFM の表に並べ替えのボタンを付ける（renderDocument の手順 6c）。
//
// **表の番号は目印の番号を読まずに数える。** 鍵を読むのは refs.js だけであり（IMP-260）、鍵の合う表を
// 文書の順に並べたときの位置は、目印の番号（Go 側が文書の順に振る。IMP-120）と一致する。行が 2 行未満の
// 表も番号には数える。
export function attachSortButtons(root) {
  attached = [...root.querySelectorAll("table")]
    .filter((table) => isOwnRef(table, "table"))
    .map((table, number) => prepareTable(table, number));
}

// captureSort は並べ替えの状態の写しを返す（手順 0a。同じ文書の再描画のときだけ呼ぶ）。無ければ null。
export function captureSort() {
  if (sorts.size === 0) return null;

  const snapshot = new Map();
  for (const [number, sort] of sorts) snapshot.set(number, { ...sort, order: [...sort.order] });
  return snapshot;
}

// restoreSort は写しを当て直す（手順 6c。attachSortButtons の後に呼ぶ）。trigger は DocumentDTO.trigger。
//
// **同じ番号の表があり、列数が同じで、行が 2 行以上ある**ときだけ当て直し、そうでなければその表の状態を
// 捨てる（FR-130）。書き込みの直後（edit）は写しの行の並びをそのまま使い、並べ直さない——直したセルの
// 行が動くと、いま直した行を見失う。**行の数が変わっていれば並べ直す。** それ以外（watch / reload /
// open）は列と向きで並べ直す。
export function restoreSort(snapshot, trigger) {
  sorts.clear();
  if (!snapshot) return;

  for (const [number, kept] of snapshot) {
    const table = attached[number];
    if (!table || table.cols !== kept.cols) continue;

    if (trigger === TRIGGER_EDIT && kept.order.length === table.rows.length) {
      arrange(table, kept.order.map((index) => table.rows[index]));
      remember(number, table, kept.col, kept.dir, kept.order);
      continue;
    }

    sortTable(number, table, kept.col, kept.dir);
  }
}

// clearSort は並べ替えの状態を空にする（文書の切り替えと状態画面への移行。IMP-250 の leaveDocument）。
export function clearSort() {
  sorts.clear();
  attached = [];
}

// compareCells は 2 つのセルの文字列を昇順で比べる。純粋な関数（IMP-229）。
//
// **向きを引数に足さない。** 降順は呼び出し側が符号を反転する（空のセルを除く）。描画スモーク（UT-814）は
// 昇順の値を見る。
export function compareCells(a, b) {
  const left = trimmed(a);
  const right = trimmed(b);

  // 1. 空のセルは後ろ（昇順・降順とも。FR-130 の「逆順の例外」）
  if (left === "" && right === "") return 0;
  if (left === "") return 1;
  if (right === "") return -1;

  // 2. 両方が数なら数で比べる
  const x = parseNumber(left);
  const y = parseNumber(right);
  if (x !== null && y !== null) return x < y ? -1 : x > y ? 1 : 0;

  // 3. 片方だけが数なら数を先に
  if (x !== null) return -1;
  if (y !== null) return 1;

  // 4. どちらも数でなければ文字列として
  return collator.compare(left, right);
}

// parseNumber は数値として解釈できれば数を、できなければ null を返す（FR-130, IMP-229）。
//
// 前後の空白を除き、`,` をすべて除き、末尾の `%` を 1 つだけ除いてから形を見る。**解釈できないものは
// 0 ではなく null とする**——空のセルや文字列を 0 として数の間に並べない。
export function parseNumber(text) {
  let value = trimmed(text).replaceAll(",", "");
  if (value.endsWith("%")) value = value.slice(0, -1);

  return NUMBER.test(value) ? Number(value) : null;
}

// prepareTable は 1 つの表に行の元の順を振り、見出しセルにボタンを置く。対象でなければ null を返す。
//
// **直下の thead / tbody だけを見る**（tHead / tBodies）。セルの中に生 HTML の表があっても、その行を
// 数えない。
function prepareTable(table, number) {
  const head = table.tHead && table.tHead.rows[0];
  const body = table.tBodies[0];
  if (!head || !body || body.rows.length < 2) return null;

  const rows = [...body.rows];
  rows.forEach((row, index) => row.setAttribute(ROW_ATTR, String(index)));

  const headers = [...head.cells];
  const prepared = { rows, headers, cols: headers.length, buttons: [] };

  prepared.buttons = headers.map((cell, col) => {
    const button = sortButton();
    button.addEventListener("click", () => cycle(number, prepared, col));
    cell.appendChild(button);
    return button;
  });

  return prepared;
}

// sortButton は並べ替えのボタンを作る（UI-054, DSP-125）。
//
// **ツールチップの文言は向きで変えない**（UI-054）。向きは見出しセルの aria-sort で伝える（IMP-295）。
function sortButton() {
  const button = document.createElement("button");
  button.type = "button";
  button.className = "sort-btn";
  button.dataset.tip = S.tipSort;
  button.setAttribute("aria-label", S.tipSort);
  button.appendChild(icon(ICON_NONE));
  return button;
}

// cycle はボタンの押下で、その列を 昇順 → 降順 → 元の順 と巡回する（FR-130）。
// 別の列のボタンでは、その列の昇順から始める。
function cycle(number, table, col) {
  const current = sorts.get(number);
  let dir = ASC;
  if (current && current.col === col) dir = current.dir === ASC ? DESC : null;

  // **並べ替える前に検索を閉じる**（FR-080, DSP-361）。ヒットの順序が本文と食い違う。
  deps.closeSearch();
  sortTable(number, table, col, dir);
}

// sortTable は元の順から並べ直し、状態と見出しの表示を合わせる。dir が null なら元の順に戻す。
//
// **元の順から始めて安定に並べる**（Array.prototype.sort は安定）。比較が等しい行は元の順を保つ（FR-130）。
function sortTable(number, table, col, dir) {
  if (!dir) {
    arrange(table, table.rows);
    sorts.delete(number);
    showState(table, -1, null);
    return;
  }

  const keyed = table.rows.map((row) => ({ row, text: cellText(row, col) }));
  keyed.sort((a, b) => {
    const order = compareCells(a.text, b.text);
    // 降順は符号を反転する。**ただし空のセルは反転しない**（昇順・降順とも末尾。IMP-229）
    return dir === DESC && a.text !== "" && b.text !== "" ? -order : order;
  });

  const rows = keyed.map((item) => item.row);
  arrange(table, rows);
  remember(number, table, col, dir, rows.map((row) => Number(row.getAttribute(ROW_ATTR))));
}

// remember は状態を持ち、見出しの表示を合わせる。
function remember(number, table, col, dir, order) {
  sorts.set(number, { col, dir, cols: table.cols, order: [...order] });
  showState(table, col, dir);
}

// arrange は rows の順に tr を付け替える（appendChild。IMP-229）。
function arrange(table, rows) {
  const body = table.rows[0].parentNode;
  for (const row of rows) body.appendChild(row);
}

// showState は並べ替えている列の th に aria-sort を付け、ボタンのアイコンを向きに合わせる
// （IMP-229, IMP-295, DSP-125）。それ以外の列からは aria-sort を外す。
function showState(table, col, dir) {
  table.headers.forEach((cell, index) => {
    const sorted = index === col && dir !== null;
    if (sorted) cell.setAttribute("aria-sort", ARIA_SORT[dir]);
    else cell.removeAttribute("aria-sort");

    table.buttons[index].querySelector("use").setAttribute("href", "#" + (sorted ? ICONS[dir] : ICON_NONE));
  });
}

// cellText は比べる文字列（セルの textContent の前後の空白を除いたもの）を返す（IMP-229）。
//
// **自前で足した要素（.sort-btn / .media-actions）はテキストを持たない**ため、textContent をそのまま使う。
// セルの足りない行は空のセルとして扱う。
function cellText(row, col) {
  const cell = row.cells[col];
  return cell ? cell.textContent.trim() : "";
}

// trimmed は文字列の前後の空白を除く。文字列でなければ空文字とする。
function trimmed(value) {
  return typeof value === "string" ? value.trim() : "";
}
