// strings.js — UI 文言の一元定義（IMP-290）。
//
// 利用者に見える文言はすべてここに置く。他のモジュールに文字列リテラルを書かない。
// ツールチップのみ英日併記とし、キー表記は shortcuts.js から組み立てる（UI-024）。
// ロケール判定・言語切り替えの仕組みは持たない（NFR-062）。
//
// 引数を取る文言は関数として定義し、呼び出し側で文字列を組み立てない。
// 表示文言の全体像がこのファイルだけで読める状態を保つ。

import { formatSize } from "./util.js";

export const S = {
  // ツールチップのみ英日併記（UI-024）。
  // キー表記は含めない。shortcuts.js の定義から組み立てる（IMP-244）
  tipBack: "Back / 戻る", // UI-024（4.75.0）
  tipForward: "Forward / 進む",
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
  tipStrike: "Strikethrough / 取り消し線", // UI-024, FR-146（4.75.0）
  tipColor: "Text color / 文字色", // UI-024, FR-147（ショートカットを持たない）
  tipAbout: "About / アプリケーション情報",

  // ファイルツリーペインの見出しのボタンとフィルターの選択肢も英日併記（UI-024。4.75.0）。
  // kinds は下の kind* の並び（表示している種類）
  tipFilter: (kinds) => `Filter / フィルター (${kinds.join(", ")})`,
  tipFilterMarkdown: "Markdown / マークダウン (always shown)",
  tipFilterDiagrams: "Diagrams / 図 (PlantUML, Mermaid)",
  tipFilterImages: "Images / 画像",

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
  dropHint: "Drop a file to open", // UI-070（4.75.0。種類を名指ししない）
  outsideTree: "(outside tree)",
  treeMore: (n) => `… and ${n} more`, // DSP-112

  // ファイルツリーのフィルター（UI-033。4.75.0）。拡張子は ExtensionsDTO から渡す（FR-037）
  filterTitle: "Show in tree",
  filterMarkdown: (exts) => `Markdown (${exts})`,
  filterDiagrams: (exts) => `Diagrams (${exts})`,
  filterImages: (exts) => `Images (${exts})`,
  kindMarkdown: "Markdown",
  kindDiagrams: "Diagrams",
  kindImages: "Images",

  // ファイルツリーの右クリックメニュー（UI-032。4.75.0）
  menuOpen: "Open",
  menuOpenInEditor: "Open in editor...",
  menuOpenContaining: "Open containing folder",
  menuOpenInFileManager: "Open in file manager",
  menuSetTreeRoot: "Set as tree root",
  menuSetParentRoot: "Set parent as tree root",
  menuCopyPath: "Copy path",

  // 図のファイル（FR-025, FR-026。4.75.0）。図が 1 つも無いとき、noDiagrams を見てフロントエンドが出す
  noDiagrams: "No diagrams found.",

  // コードブロック（DSP-251）
  copy: "Copy",

  // 図と画像・表のボタン（UI-053, UI-054）。ツールチップは英語だけ（UI-024）
  tipActualSize: "Actual size",
  tipExpand: "Expand",
  tipSort: "Sort",
  tipEditBlock: "Edit", // コードブロックと図の編集のボタン（UI-053。4.75.0）

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
  menuStrikethrough: "Strikethrough", // FR-146（4.75.0）
  menuTextColor: "Text color", // FR-147。サブメニューを持つ

  // 文字色の一覧（FR-147, DSP-141。4.75.0）。名前と順は FR-147 の表。**色の値はここに書かない**
  // （ファイルへ書く値は Go 側、画面の色は CSS のトークンが決める。IMP-265）
  colorRed: "Red",
  colorOrange: "Orange",
  colorYellow: "Yellow",
  colorGreen: "Green",
  colorBlue: "Blue",
  colorPurple: "Purple",
  colorGray: "Gray",
  colorDefault: "Default",

  // 編集モード（UI-055, DSP-126）
  editModeBadge: "Edit mode",
  save: "Save", // 編集欄（UI-055。4.75.0）。Cancel は下の cancel を使う

  // 見出しのアンカー（IMP-227, DSP-023）。アイコンだけのリンクに
  // 読み上げ名を与える（IMP-295）
  headingAnchor: "Link to this section",

  // ステータス領域（DSP-150）
  statusLines: (n) => `${n} lines`,
  statusZoom: (z) => `${z}%`,
  // エディタの起動に成功したときだけ出す（FR-090, DSP-151）。**背面の
  // ウィンドウで開くことがあり、何も出ないと「押しても無反応」に見える**
  statusEditor: (name) => `Opened in ${name}`,
  // 画像ファイルの右（UI-060, DSP-150。4.75.0）。寸法は読み込みの後に足す（IMP-228）
  statusImageSize: (w, h) => `${w} × ${h} px`,
  // 1024 未満は B、1 MB 未満は整数の KB、それ以上は確認画面と同じ MB（util.js の formatSize。DSP-150）
  statusFileSize: (bytes) => formatSize(bytes),

  // 状態画面（DSP-181）
  welcomeTitle: "Open a file", // UI-052（4.75.0。種類を名指ししない）
  welcomeHintOpen: "Press Ctrl+O to choose a file",
  welcomeHintDrop: "Or drop a file onto this window",
  welcomeHintTree: "Use the file tree to browse documents",
  openAnyway: "Open anyway",
  rendering: "Rendering...", // Open anyway を押した後（FR-016, UI-052。4.81.0）
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
  // not-supported（4.75.0。v1.1.0 の「Markdown でない」種別とそのキーを、名前ごと改めた。**古いキーを
  // 残さない**——参照が残れば読み込みの時点で気づける。IMP-290）
  errNotSupported: (p) => `Not a supported file: ${p}`,
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
  // 図種別に対応していないか、上限（MD-083）を超えた。**どちらかは区別しない**——処理系の英文で
  // 見分けることになり、その文言は版で変わる（IMP-233）。上限の値は puml.js の定数を渡す（BUG-030）
  pumlUnsupported: (maxPx) =>
    `PlantUML could not render this diagram. It may be unsupported or larger than ${maxPx.toLocaleString("en-US")} px.`,
  pumlFailed: "PlantUML rendering did not complete.",

  errEditorFailed: "Failed to start the editor.",
  errEditorSelf: "MarkView cannot be used as an editor.",
  // 編集モードの書き込み（FR-143, IMP-315）。どちらもステータスに出す
  errEditConflict: "The file changed on disk and was not saved.", // edit-conflict
  errEditFailed: (p) => `Failed to save: ${p}`, // edit-failed
  // 右クリックメニューの Paste でクリップボードを読めなかった（FR-063, IMP-315 の paste）
  errPaste: "Failed to paste.",
  // 4.75.0。いずれも警告（UI-060, DSP-151。下の WARNING_KINDS）
  errEditRejected: "This change would alter the document structure and was not saved.", // edit-rejected
  errCodeLocked: "This code block cannot be edited in MarkView.", // code-locked
  errFormatRejected: "This formatting cannot be applied here.", // format-rejected
  warnFormatWidened: "The whole strikethrough was removed.", // format-widened
  warnEditBusy: "Save or cancel the open edit first.", // edit-busy
  warnEditHeld: "The file changed on disk. Cancel the edit to see the changes.", // edit-held
  warnEditRemoved: "The file was deleted. The edit cannot be saved.", // edit-removed
  warnRemovedEditing: (p) => `File was deleted: ${p}. Edit mode ends when you close the edit.`, // removed-editing（FR-110）
  warnEncodingEditing: "Some characters were replaced. Edit mode ends when you close the edit.", // encoding-editing（FR-110）
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
  "not-supported": (e) => S.errNotSupported(e.path),
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
  "edit-rejected": () => S.errEditRejected,
  "code-locked": () => S.errCodeLocked,
  "format-rejected": () => S.errFormatRejected,
  "format-widened": () => S.warnFormatWidened,
  "edit-busy": () => S.warnEditBusy,
  // 以下の 4 つはフロントエンドだけが作る（編集欄はフロントエンドにしか無い。IMP-315）
  "edit-held": () => S.warnEditHeld,
  "edit-removed": () => S.warnEditRemoved,
  "removed-editing": (e) => S.warnRemovedEditing(e.path),
  "encoding-editing": () => S.warnEncodingEditing,
};

// WARNING_KINDS は警告の色（DSP-151）で出す種別（IMP-315 の「ステータス（警告）」。UI-060。4.75.0。
// 利用者の判断）。拒んだ知らせはすべて警告とする。ここに無い種別は "error" で出す。
const WARNING_KINDS = new Set([
  "edit-rejected",
  "code-locked",
  "format-rejected",
  "format-widened",
  "edit-busy",
  "edit-held",
  "edit-removed",
  "removed-editing",
  "encoding-editing",
]);

// errorText は ErrorDTO（IMP-307）をステータス領域の 1 行に変える。
//
// **未知の Kind は ErrorDTO.message をそのまま出す**（IMP-290, IMP-315）。
// Go 側も英語で組み立てており、文言が空欄になるより理由が読めるほうがよい。
export function errorText(dto) {
  if (!dto) return "";
  const make = statusText[dto.kind];
  return make ? make(dto) : dto.message;
}

// errorLevel は ErrorDTO を出す一時メッセージの種類（"warning" / "error"。DSP-151）を返す（IMP-315）。
// **種別と文言の対応（上の表）と同じ場所で決める**——呼び出し側ごとに決めると、同じ種別が経路によって
// 違う色で出る。
export function errorLevel(dto) {
  return dto && WARNING_KINDS.has(dto.kind) ? "warning" : "error";
}

// warningText は DocumentDTO.warnings の Kind を文言に変える（IMP-302）。
//
// **未知の Kind は無視する。** warnings にはフォールバックの message がない。
export function warningText(kind) {
  const make = statusText[kind];
  return make ? make({ kind }) : "";
}
