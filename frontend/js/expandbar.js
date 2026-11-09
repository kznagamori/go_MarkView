// expandbar.js — 拡大画面の操作バーとキーの表（FR-122, UI-104, IMP-253, DSP-173）。
//
// expand.js が 400 行の目安（IMP-011）を超えたため、操作バーの組み立てとキーの表だけを分けた。**倍率と位置の
// 状態は expand.js が持ち、押されたときの処理は expand.js が渡す。** このモジュールは自前のモジュールのうち
// strings.js と util.js だけを import する（expand.js の import の制約を崩さない。IMP-253）。

import { S } from "./strings.js";
import { icon } from "./util.js";

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

// buildBar は操作バー（.expand-bar）を組み立てる（DSP-173）。actions: { fit, actual, zoomOut, zoomIn, close }
// の処理、isOpen: 拡大画面が開いているか。**aria-disabled のボタンと、閉じている間は押しても何もしない**（IMP-253）。
// 返す値は { bar, zoomValue, fit, actual, zoomOut, zoomIn, close }（ボタンは KEYS と同じ名前）。
export function buildBar(actions, isOpen) {
  const make = (className, tip, id) => barButton(className, tip, id, actions[id], isOpen);
  const parts = {
    bar: document.createElement("div"),
    zoomValue: document.createElement("span"),
    fit: textButton(make("expand-fit", S.tipExpandFit, "fit"), S.expandFit),
    actual: textButton(make("expand-actual", S.tipExpandActual, "actual"), S.expandActual),
    zoomOut: iconButton(make("expand-zoom-out", S.tipZoomOut, "zoomOut"), "icon-zoom-out"),
    zoomIn: iconButton(make("expand-zoom-in", S.tipZoomIn, "zoomIn"), "icon-zoom-in"),
    close: iconButton(make("expand-close", S.tipExpandClose, "close"), "icon-close"),
  };
  parts.bar.className = "expand-bar";
  parts.zoomValue.className = "expand-zoom-value";
  parts.bar.append(parts.fit, parts.actual, parts.zoomOut, parts.zoomValue, parts.zoomIn, parts.close);
  return parts;
}

// commandOf はキーの組み合わせ（comboOf の表記）に当たる KEYS の名前を返す。無ければ null。
export function commandOf(combo) {
  return Object.keys(KEYS).find((name) => KEYS[name].keys.includes(combo)) || null;
}

// comboOf は KeyboardEvent を KEYS の表記へ変える（shortcuts.js の keyOf と同じ規則。1 文字のキーは小文字）。
export function comboOf(event) {
  const names = [];
  if (event.ctrlKey) names.push("Ctrl");
  if (event.altKey) names.push("Alt");
  if (event.shiftKey) names.push("Shift");
  names.push(event.key.length === 1 ? event.key.toLowerCase() : event.key);
  return names.join("+");
}

// textButton / iconButton はボタンに文字かアイコンを入れる。ツールチップと読み上げ名は `Fit (F)` の形
// （IMP-247, IMP-295）。
function textButton(button, text) {
  button.classList.add("expand-text");
  button.textContent = text;
  return button;
}

function iconButton(button, symbol) {
  button.appendChild(icon(symbol));
  return button;
}

function barButton(className, tip, id, onClick, isOpen) {
  const button = document.createElement("button");
  button.type = "button";
  button.className = `tb-btn ${className}`;
  const label = `${tip} (${KEYS[id].label})`;
  button.dataset.tip = label;
  button.setAttribute("aria-label", label);
  button.addEventListener("click", () => {
    if (isOpen() && button.getAttribute("aria-disabled") !== "true") onClick();
  });
  return button;
}
