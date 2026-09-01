// wailsjs/runtime/runtime.js の代役（BR-054）。
//
// api.js は EventsOn を**名前指定で**取り込む。名前指定の import は
// モジュールを繋ぐ時点で存在を検査されるため、空のモジュールでは
// SyntaxError になる。呼ばれることはないが、名前だけは出しておく。
export function EventsOn() {
  return () => {};
}
