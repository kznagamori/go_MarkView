// about.js — 情報ダイアログの中身（UI-100, UI-101, UI-102, IMP-251, DSP-170, DSP-171）。
//
// **開閉・フォーカスの復帰・Tab の制御は overlay.js が持つ**（IMP-251, IMP-252）。ここは #overlay に
// 入れる DOM を組み立てるだけにする。overlay.js を import しないのは、循環参照を作らないためである
// （editors.js と同じ形）。overlay.js が 400 行の目安（IMP-011）を超えていたため分けた。
//
// **Go を呼ばない**（IMP-201）。閉じる処理とリンクの処理は handlers として受け取る。リンクは本文中の
// リンクとまったく同じ経路（IMP-312）へ、overlay.js を経由して main.js が渡す。

import { S } from "./strings.js";
import { formatBuildTime, icon, span } from "./util.js";

// buildAboutDialog はダイアログ本体を組み立てる（DSP-170, DSP-171）。
//
// about は AboutDTO（IMP-306）。handlers は { onClose, onLink }——閉じるボタン 2 つと、リポジトリの
// リンクの処理（UI-102）。
export function buildAboutDialog(about, handlers) {
  const dialog = document.createElement("div");
  dialog.className = "dialog";
  dialog.setAttribute("role", "dialog");
  dialog.setAttribute("aria-modal", "true");
  dialog.setAttribute("aria-labelledby", "about-title");

  dialog.appendChild(closeButton(handlers));
  dialog.appendChild(buildHead(about));
  dialog.appendChild(buildTable(about, handlers));
  dialog.appendChild(buildLicenses(about));
  dialog.appendChild(buildActions(handlers));

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
function buildTable(about, handlers) {
  const list = document.createElement("dl");
  list.className = "about-table";

  addRow(list, S.aboutAuthor, span("", about.author));
  addRow(list, S.aboutRepository, repositoryLink(about.repository, handlers));
  addRow(list, S.aboutLicense, span("", about.license));
  addRow(list, S.aboutEnvironment, span("", about.environment));
  addRow(list, S.aboutBundled, bundled(about.vendors));

  return list;
}

// repositoryLink は既定ブラウザで開くリンクを作る（UI-102, FR-050）。
//
// **href を持たせない。** WebView 内でのページ遷移を一切起こさないという
// 規約（AR-060）に対し、遷移し得ない形にしておくほうが確実である。
function repositoryLink(url, handlers) {
  const link = document.createElement("a");
  link.className = "about-link";
  link.textContent = url || "";
  link.setAttribute("role", "link");
  link.tabIndex = 0;

  const open = () => {
    if (url) handlers.onLink(url);
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

function buildActions(handlers) {
  const actions = document.createElement("div");
  actions.className = "dialog-actions";

  const button = document.createElement("button");
  button.id = "about-close";
  button.type = "button";
  button.className = "dialog-button";
  button.textContent = S.close;
  button.addEventListener("click", handlers.onClose);
  actions.appendChild(button);

  return actions;
}

// closeButton は右上の閉じるボタンを作る（DSP-170）。
function closeButton(handlers) {
  const button = document.createElement("button");
  button.id = "about-x";
  button.type = "button";
  button.className = "dialog-x";
  button.title = S.close;
  button.setAttribute("aria-label", S.close);
  button.appendChild(icon("icon-close"));
  button.addEventListener("click", handlers.onClose);

  return button;
}

function addRow(list, label, value) {
  const term = document.createElement("dt");
  term.textContent = label;
  list.appendChild(term);

  const detail = document.createElement("dd");
  detail.appendChild(value);
  list.appendChild(detail);
}
