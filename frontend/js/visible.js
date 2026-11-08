// visible.js — 見える文字の数え方（IMP-266。FR-146, BR-054）。
//
// **Go 側の IMP-123（renderer.Visible / VisibleTexts）と同じ数え方にする。** どちらかを変えたら、
// もう一方と描画スモーク（BR-054。scripts/smoke の UT-817）を同じ変更で直す。
// **自前の他のモジュールを import しない**（IMP-200）。
//
// TODO(v1.2.0): 未実装（テストの先行実装で置いたスタブ）。どこからも import しない。
// 描画スモークの見える文字の照合（E2E-109 の 12）は、実装するまで赤い。

// visibleText はブロックの見える文字（文字列）を返す。
export function visibleText(block) {
  return "";
}

// offsetOf は DOM の位置を、見える文字の中の符号位置へ移す。範囲の外なら -1。
export function offsetOf(block, node, offset) {
  return -1;
}

// rangeAt は見える文字の符号位置を DOM の Range へ移す（再描画の後の当て直し）。
export function rangeAt(block, start, end) {
  return null;
}
