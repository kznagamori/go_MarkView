// wailsjs/runtime/runtime.js の代役（BR-054）。
//
// api.js は EventsOn を、dnd.js は OnFileDrop を**名前指定で**取り込む
// （IMP-245）。名前指定の import はモジュールを繋ぐ時点で存在を検査される
// ため、空のモジュールでは SyntaxError になる。呼ばれることはないが、
// 名前だけは出しておく。
export function EventsOn() {
  return () => {};
}

export function OnFileDrop() {}
