// overlay.js — 状態画面と情報ダイアログ（UI-052, UI-100, IMP-250, IMP-251）。
//
// 状態画面は本文ペインの中に出す。**情報ダイアログも独立したウィンドウでは
// なく、メインウィンドウ内に描くモーダルオーバーレイである**（AR-060,
// UI-100, FR-053）。アプリケーションは終始 1 つのウィンドウしか持たない。

import { S } from "./strings.js";
import { closeSearch } from "./search.js";
import { $, clear, icon, formatSize, formatBuildTime, baseName } from "./util.js";

// 情報ダイアログのリンクを踏んだときの処理（UI-102）。
//
// **判定は Go 側にある**（IMP-312）。本文中のリンクとまったく同じ経路を通す。
let onLink = null;

// ダイアログを開く前にフォーカスがあった要素。閉じたら戻す（IMP-251）。
let restoreFocus = null;

// initOverlay は情報ダイアログを配線する（IMP-211）。
export function initOverlay(deps) {
  onLink = (deps || {}).onLink;

  // 暗幕そのものを押したときだけ閉じる（DSP-170）。中身のクリックで
  // 閉じないよう、対象が #overlay 自身であることを確かめる。
  $("overlay").addEventListener("click", (event) => {
    if (event.target === $("overlay")) hideAbout();
  });

  // **Tab がダイアログの外へ出ないようにする**（IMP-251）。背後の
  // ツールバーやツリーへフォーカスが移ると、操作を受け付けない見た目と
  // 実際の挙動が食い違う（UI-100）。
  $("overlay").addEventListener("keydown", trapTab);
}

// 状態画面の種別ごとの見た目（DSP-181）。
const SCREENS = {
  welcome: { icon: "icon-open", tone: "subtle" },
  "confirm-large": { icon: "icon-warning", tone: "attention" },
  "too-large": { icon: "icon-caution", tone: "danger" },
  "render-error": { icon: "icon-caution", tone: "danger" },
};

// showStateScreen は状態画面を表示する（IMP-250）。
//
// params は ErrorDTO（IMP-307）と、confirm-large で押されたときの onConfirm。
export function showStateScreen(kind, params) {
  const screen = $("state-screen");
  const spec = SCREENS[kind] || SCREENS.welcome;
  const info = params || {};

  // 検索を閉じる（DSP-301）。document 以外の状態では検索は機能しない。
  closeSearch();

  // 本文は空にする（IMP-250）。
  clear($("markdown"));
  clear(screen);

  screen.dataset.kind = kind;
  screen.dataset.tone = spec.tone;
  screen.appendChild(icon(spec.icon, "state-icon"));

  if (kind === "welcome") {
    screen.appendChild(line("state-title", S.welcomeTitle));
    screen.appendChild(line("state-hint", S.welcomeHintOpen));
    screen.appendChild(line("state-hint", S.welcomeHintDrop));
    screen.appendChild(line("state-hint", S.welcomeHintTree));
  } else {
    // 主テキストは対象のファイル名（DSP-181）。
    screen.appendChild(line("state-title", baseName(info.path)));
    buildDetail(screen, kind, info);
  }

  screen.hidden = false;
}

// hideStateScreen は状態画面を隠す（IMP-250）。
export function hideStateScreen() {
  const screen = $("state-screen");
  screen.hidden = true;
  clear(screen);
}

function buildDetail(screen, kind, info) {
  switch (kind) {
    case "confirm-large":
      screen.appendChild(line("state-hint", formatSize(info.size)));
      screen.appendChild(line("state-hint", S.largeTitle));
      screen.appendChild(line("state-hint", S.largeHint));
      screen.appendChild(confirmButton(info));
      return;

    case "too-large":
      screen.appendChild(line("state-hint", formatSize(info.size)));
      screen.appendChild(line("state-hint", S.tooLarge(formatSize(info.limit))));
      return;

    default:
      screen.appendChild(line("state-hint", S.renderError));
      // Go 側が組み立てた理由も添える。文言の正ではなく、原因の手掛かり
      // として出す（IMP-315）。
      screen.appendChild(line("state-detail", info.message));
      return;
  }
}

function confirmButton(info) {
  const button = document.createElement("button");
  button.type = "button";
  button.className = "state-button";
  button.textContent = S.openAnyway;
  button.addEventListener("click", () => {
    if (info.onConfirm) info.onConfirm(info.path);
  });

  return button;
}

function line(className, text) {
  const element = document.createElement("p");
  element.className = className;
  element.textContent = text || "";

  return element;
}

// --- 情報ダイアログ（UI-100, UI-101, UI-102, IMP-251, DSP-170, DSP-171） ---

// showAbout は情報ダイアログを表示する（FR-100, IMP-251）。
//
// about は AboutDTO（IMP-306）。**Go を呼ぶのは main.js の役目**とし、
// ここは受け取った値を描くだけにする（IMP-201）。
export function showAbout(about) {
  const overlay = $("overlay");

  restoreFocus = document.activeElement;

  clear(overlay);
  overlay.appendChild(buildDialog(about || {}));
  overlay.hidden = false;

  // 開いた直後は閉じるボタンにフォーカスを置く（IMP-295）。
  $("about-close").focus();
}

// hideAbout はダイアログを閉じ、フォーカスを戻す（IMP-251）。
export function hideAbout() {
  const overlay = $("overlay");
  if (overlay.hidden) return false;

  overlay.hidden = true;
  clear(overlay);

  // **戻し先は「?」ボタンを既定とする**（IMP-251）。開く前に別の操作要素へ
  // フォーカスがあった場合だけ、そこへ戻す。
  //
  // F1 で開いたときの直前のフォーカスは <body> であることが多く、そのまま
  // 戻すと**どこにもフォーカスがない状態になる**。ダイアログを閉じた直後に
  // Tab を押しても、ツールバーの先頭から辿り直すことになってしまう。
  const target = isControl(restoreFocus) ? restoreFocus : $("btn-about");
  restoreFocus = null;
  if (target) target.focus();

  return true;
}

// isControl はフォーカスを戻す先として使える要素かを判定する。
function isControl(element) {
  return Boolean(element) && element.isConnected && element !== document.body && element.tabIndex >= 0;
}

// isAboutOpen はダイアログが開いているかを返す。
export function isAboutOpen() {
  return !$("overlay").hidden;
}

function buildDialog(about) {
  const dialog = document.createElement("div");
  dialog.className = "dialog";
  dialog.setAttribute("role", "dialog");
  dialog.setAttribute("aria-modal", "true");
  dialog.setAttribute("aria-labelledby", "about-title");

  dialog.appendChild(closeButton("about-x", "dialog-x", S.close));
  dialog.appendChild(buildHead(about));
  dialog.appendChild(buildTable(about));
  dialog.appendChild(buildLicenses(about));
  dialog.appendChild(buildActions());

  return dialog;
}

// buildHead はアイコン・名称・バージョン行を組み立てる（DSP-171）。
function buildHead(about) {
  const head = document.createElement("header");
  head.className = "about-head";

  // アイコンは assetsrv が配信する（IMP-160）。**外部 URL を参照しない**。
  // 装飾目的のため alt は空とし、読み上げの対象にしない（IMP-251）。
  const image = document.createElement("img");
  image.className = "about-icon";
  image.src = "/appicon.png";
  image.alt = "";
  head.appendChild(image);

  const box = document.createElement("div");

  const title = document.createElement("h2");
  title.id = "about-title";
  title.className = "about-title";
  title.textContent = S.appName;
  box.appendChild(title);

  const version = document.createElement("p");
  version.className = "about-version";
  version.appendChild(span("", S.aboutVersion(about.version, about.commit)));
  version.appendChild(span("about-buildtime", formatBuildTime(about.buildTime)));
  box.appendChild(version);

  head.appendChild(box);

  return head;
}

// buildTable は情報テーブルを組み立てる（DSP-171）。
//
// ラベルと値の対であるため <dl> を使う。表示は CSS グリッドで 2 列にする。
function buildTable(about) {
  const list = document.createElement("dl");
  list.className = "about-table";

  addRow(list, S.aboutAuthor, span("", about.author));
  addRow(list, S.aboutRepository, repositoryLink(about.repository));
  addRow(list, S.aboutLicense, span("", about.license));
  addRow(list, S.aboutEnvironment, span("", about.environment));
  addRow(list, S.aboutBundled, bundled(about.vendors));

  return list;
}

// repositoryLink は既定ブラウザで開くリンクを作る（UI-102, FR-050）。
//
// **href を持たせない。** WebView 内でのページ遷移を一切起こさないという
// 規約（AR-060）に対し、遷移し得ない形にしておくほうが確実である。
function repositoryLink(url) {
  const link = document.createElement("a");
  link.className = "about-link";
  link.textContent = url || "";
  link.setAttribute("role", "link");
  link.tabIndex = 0;

  const open = () => {
    if (url && onLink) onLink(url);
  };

  link.addEventListener("click", open);
  link.addEventListener("keydown", (event) => {
    if (event.key !== "Enter" && event.key !== " ") return;
    event.preventDefault();
    open();
  });

  return link;
}

// bundled は同梱資産のバージョンを並べる（UI-100 の Bundled 行, BR-042）。
function bundled(vendors) {
  const box = document.createElement("span");
  box.className = "about-vendors";

  for (const vendor of vendors || []) {
    box.appendChild(span("about-vendor", S.aboutVendor(vendor.name, vendor.version)));
  }

  return box;
}

// buildLicenses は OSS ライセンス表示欄を組み立てる（UI-101, FR-101）。
//
// **<textarea readonly> ではなく <pre> を使う**（IMP-251）。整形を崩さず、
// 選択とコピーができる。内容はビルド時に埋め込まれたものであり、実行時に
// 外部から取得しない（FR-101）。
function buildLicenses(about) {
  const box = document.createElement("div");
  box.className = "about-licenses-box";

  const heading = document.createElement("h3");
  heading.className = "about-licenses-title";
  heading.textContent = S.aboutLicenses;
  box.appendChild(heading);

  const body = document.createElement("pre");
  body.className = "about-licenses";
  body.tabIndex = 0; // キーボードでもスクロールできるようにする（IMP-295）
  body.textContent = about.licenses || "";
  box.appendChild(body);

  return box;
}

function buildActions() {
  const actions = document.createElement("div");
  actions.className = "dialog-actions";

  const button = document.createElement("button");
  button.id = "about-close";
  button.type = "button";
  button.className = "dialog-button";
  button.textContent = S.close;
  button.addEventListener("click", hideAbout);
  actions.appendChild(button);

  return actions;
}

function closeButton(id, className, label) {
  const button = document.createElement("button");
  button.id = id;
  button.type = "button";
  button.className = className;
  button.title = label;
  button.setAttribute("aria-label", label);
  button.appendChild(icon("icon-close"));
  button.addEventListener("click", hideAbout);

  return button;
}

// trapTab は Tab をダイアログの中で循環させる（IMP-251, UI-100）。
function trapTab(event) {
  if (event.key !== "Tab") return;

  const targets = focusable();
  if (targets.length === 0) return;

  const first = targets[0];
  const last = targets[targets.length - 1];

  // 端に来たときだけ折り返す。途中では既定の移動に任せる。
  if (event.shiftKey && document.activeElement === first) {
    event.preventDefault();
    last.focus();
    return;
  }
  if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault();
    first.focus();
  }
}

function focusable() {
  return [...$("overlay").querySelectorAll("button, a, [tabindex]")].filter(
    (element) => element.tabIndex >= 0,
  );
}

function addRow(list, label, value) {
  const term = document.createElement("dt");
  term.textContent = label;
  list.appendChild(term);

  const detail = document.createElement("dd");
  detail.appendChild(value);
  list.appendChild(detail);
}

function span(className, text) {
  const element = document.createElement("span");
  if (className) element.className = className;
  element.textContent = text || "";

  return element;
}
