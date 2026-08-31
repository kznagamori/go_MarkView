// strings.js — UI 文言の一元定義（IMP-290）。
//
// 利用者に見える文言はすべてここに置く。他のモジュールに文字列リテラルを書かない。
// ツールチップのみ英日併記とし、キー表記は shortcuts.js から組み立てる（UI-024）。
// ロケール判定・言語切り替えの仕組みは持たない（NFR-062）。
//
// 引数を取る文言は関数として定義し、呼び出し側で文字列を組み立てない。
// 表示文言の全体像がこのファイルだけで読める状態を保つ。

export const S = {
  // ツールチップのみ英日併記（UI-024）。
  // キー表記は含めない。shortcuts.js の定義から組み立てる（IMP-244）
  tipOpen: "Open / 開く",
  tipReload: "Reload / 再読み込み",
  tipThemeDark: "Dark theme / ダークテーマ",
  tipThemeLight: "Light theme / ライトテーマ",
  tipOutline: "Outline / アウトライン",
  tipFileTree: "File tree / ファイルツリー",
  tipAbout: "About / アプリケーション情報",

  // それ以外はすべて英語（UI-024）
  paneFiles: "Files",
  paneOutline: "Outline",
  noHeadings: "No headings",
  searchPlaceholder: "Find in document",
  searchNoResults: "No results",
  searchCount: (i, n) => `${i} / ${n}`, // DSP-160
  searchPrevious: "Previous match",
  searchNext: "Next match",
  searchClose: "Close search",
  dropHint: "Drop a Markdown file to open",
  outsideTree: "(outside tree)",
  treeMore: (n) => `… and ${n} more`, // DSP-112

  // コードブロック（DSP-251）
  copy: "Copy",

  // ステータス領域（DSP-150）
  statusLines: (n) => `${n} lines`,
  statusZoom: (z) => `${z}%`,

  // 状態画面（DSP-181）
  welcomeTitle: "Open a Markdown file",
  welcomeHintOpen: "Press Ctrl+O to choose a file",
  welcomeHintDrop: "Or drop a Markdown file onto this window",
  welcomeHintTree: "Use the file tree to browse documents",
  openAnyway: "Open anyway",
  largeTitle: "This file is large.",
  largeHint: "Rendering may take a while.",
  tooLarge: (limit) => `Maximum size is ${limit}.`,
  renderError: "Failed to render this document.",

  // 情報ダイアログ（DSP-171）
  appName: "MarkView", // 見出し。固有名だが画面に出るためここに置く
  aboutVersion: (version, commit) => (commit ? `Version ${version} (${commit})` : `Version ${version}`),
  aboutVendor: (name, version) => `${name} ${version}`, // Bundled 行の 1 項目
  aboutAuthor: "Author",
  aboutRepository: "Repository",
  aboutLicense: "License",
  aboutEnvironment: "Environment",
  aboutBundled: "Bundled",
  aboutLicenses: "Third-party licenses",
  close: "Close",

  // エラー（IMP-315 の Kind に対応）
  errNotFound: (p) => `File not found: ${p}`,
  errPermission: (p) => `Cannot access: ${p}`,
  errNotMarkdown: (p) => `Not a Markdown file: ${p}`,
  errLinkNotFound: (h) => `Link target not found: ${h}`,
  errClipboard: "Failed to copy.",
  errRemoved: (p) => `File was deleted: ${p}`,
  warnEncoding: "Some characters were replaced.",
};

// IMP-315 の Kind と上の定義を 1 対 1 で対応させる表。
//
// **ステータス領域に 1 行で出す種別だけをここに置く。** 状態画面へ出す
// needs-confirm / too-large / render-error は、サイズなどの要素を伴って
// overlay.js が組み立てる（IMP-250, DSP-181）。
const statusText = {
  "not-found": (e) => S.errNotFound(e.path),
  permission: (e) => S.errPermission(e.path),
  "not-markdown": (e) => S.errNotMarkdown(e.path),
  "link-not-found": (e) => S.errLinkNotFound(e.path),
  clipboard: () => S.errClipboard,
  removed: (e) => S.errRemoved(e.path),
  encoding: () => S.warnEncoding,
};

// errorText は ErrorDTO（IMP-307）をステータス領域の 1 行に変える。
//
// **未知の Kind は ErrorDTO.message をそのまま出す**（IMP-290, IMP-315）。
// Go 側も英語で組み立てており、文言が空欄になるより理由が読めるほうがよい。
export function errorText(dto) {
  if (!dto) return "";
  const make = statusText[dto.kind];
  return make ? make(dto) : dto.message;
}

// warningText は DocumentDTO.warnings の Kind を文言に変える（IMP-302）。
//
// **未知の Kind は無視する。** warnings にはフォールバックの message がない。
export function warningText(kind) {
  const make = statusText[kind];
  return make ? make({ kind }) : "";
}
