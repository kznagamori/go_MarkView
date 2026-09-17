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
  // エディタで開く（UI-024）。**tipEdit を残さない**——v1.0.0 の `Edit / 編集` は編集モードと
  // 並ぶと区別がつかない（IMP-290）。キーの名前ごと改め、古いキーの参照が残れば気づけるようにした
  tipOpenInEditor: "Open in editor / エディタで開く",
  tipEditMode: "Edit mode / 編集モード", // UI-024, FR-140
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

  // 図と画像・表のボタン（UI-053, UI-054）。ツールチップは英語だけ（UI-024）
  tipActualSize: "Actual size",
  tipExpand: "Expand",
  tipSort: "Sort",

  // 拡大画面（UI-104, DSP-173）。キーの表記は expand.js のキーの表から足す（IMP-253）
  expandTitle: "Expanded view", // #expand-view の aria-label
  expandFit: "Fit",
  expandActual: "1:1",
  tipExpandFit: "Fit", // ツールチップは `Fit (F)` の形に組み立てる
  tipExpandActual: "Actual size",
  tipZoomOut: "Zoom out",
  tipZoomIn: "Zoom in",
  tipExpandClose: "Close",
  expandZoom: (z) => `${z}%`,

  // 右クリックメニュー（FR-063, UI-085）
  menuCut: "Cut",
  menuCopy: "Copy",
  menuPaste: "Paste",
  menuSelectAll: "Select all",
  menuCopyLink: "Copy link address",

  // 編集モード（UI-055, DSP-126）
  editModeBadge: "Edit mode",

  // 見出しのアンカー（IMP-227, DSP-023）。アイコンだけのリンクに
  // 読み上げ名を与える（IMP-295）
  headingAnchor: "Link to this section",

  // ステータス領域（DSP-150）
  statusLines: (n) => `${n} lines`,
  statusZoom: (z) => `${z}%`,
  // エディタの起動に成功したときだけ出す（FR-090, DSP-151）。**背面の
  // ウィンドウで開くことがあり、何も出ないと「押しても無反応」に見える**
  statusEditor: (name) => `Opened in ${name}`,

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

  // エディタ選択ダイアログ（UI-103, DSP-172）
  //
  // **エディタ名をここに置かない。** 一覧に出る名前は固有名詞であり、
  // Go 側のプリセット表（IMP-172）から EditorDTO.name で届く。写すと
  // 定義が 2 か所になり、片方だけ増えたときに一覧から名前が消える
  editorTitle: "Choose an editor",
  editorOther: "Other...",
  editorMissing: "(not installed)", // 検出できなかった行に添える
  editorNone: "(no file chosen)", // Other... をまだ選んでいないとき
  editorBrowse: "Browse",
  editorOpen: "Open", // ダイアログのボタン。tipOpen とは別物
  cancel: "Cancel",

  // エラー（IMP-315 の Kind に対応）
  errNotFound: (p) => `File not found: ${p}`,
  errPermission: (p) => `Cannot access: ${p}`,
  errNotMarkdown: (p) => `Not a Markdown file: ${p}`,
  errLinkNotFound: (h) => `Link target not found: ${h}`,
  // リンク先を OS の既定のハンドラへ渡せなかった（open-failed。FR-050, FR-053, IMP-315）。
  // **ステータスに出す種別であり、状態画面にしない**——本文を残す（BUG-013）。h はリンクの生値
  errOpenFailed: (h) => `Cannot open: ${h}`,
  errClipboard: "Failed to copy.",
  errRemoved: (p) => `File was deleted: ${p}`,
  // パスを伴う上の文言と違い、**どちらもパスを含めない。** ここに載せられる対象は
  // エディタの実行ファイルのパスしかなく、画面へ出してはならない（NFR-035 の 3）
  // PlantUML を描かなかった理由の 3 種（DSP-272, IMP-233）。
  // **図が出ていない**ときにだけ添える。構文エラーは PlantUML 自身が
  // 行番号付きのエラー図を返すため、こちらでは何も書かない（FR-024）。
  pumlInclude: "Include directives are not supported.",
  pumlUnsupported: "PlantUML could not render this diagram.",
  pumlFailed: "PlantUML rendering did not complete.",

  errEditorFailed: "Failed to start the editor.",
  errEditorSelf: "MarkView cannot be used as an editor.",
  // 編集モードの書き込み（FR-143, IMP-315）。どちらもステータスに出す
  errEditConflict: "The file changed on disk and was not saved.", // edit-conflict
  errEditFailed: (p) => `Failed to save: ${p}`, // edit-failed
  // 右クリックメニューの Paste でクリップボードを読めなかった（FR-063, IMP-315 の paste）
  errPaste: "Failed to paste.",
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
  "open-failed": (e) => S.errOpenFailed(e.path),
  clipboard: () => S.errClipboard,
  removed: (e) => S.errRemoved(e.path),
  "editor-failed": () => S.errEditorFailed,
  "editor-self": () => S.errEditorSelf,
  "edit-conflict": () => S.errEditConflict,
  "edit-failed": (e) => S.errEditFailed(e.path),
  paste: () => S.errPaste,
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
