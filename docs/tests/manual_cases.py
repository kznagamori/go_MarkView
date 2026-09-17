"""41 章（docs/specs/41-e2e-manual.md）の手動テストのケース定義を読み取り、照合する（E2E-200）。

gen_manual_test_xlsx.py が使う。**Excel を書き出す処理は持たない**——書き出しと読み取りを分けておくと、
41 章の書式の決まり（グループの節・番号付きリスト・テストケース一覧との照合）をここだけで読める。

gen_manual_test_xlsx.py が 400 行の目安（IMP-011）を超えたため分けた。
"""

from __future__ import annotations

import re
from dataclasses import dataclass, field


# --- Markdown のパース -------------------------------------------------------

# 節見出し。**節番号を当てにしない**（E2E-200）。グループを増やすと後続の節
# 番号がずれるため、番号で探すと「ケースを 1 つ足しただけで生成が止まる」。
# 番号が付いていてもいなくても、見出しの文言だけを取り出す。
RE_SECTION_HEADING = re.compile(r"^##\s+(?:\d+(?:\.\d+)*\s+)?(.+?)\s*$")
# グループの節は見出しの `Gn` で見分ける（E2E-200）。
RE_GROUP_TITLE = re.compile(r"^G\d+\s+.+$")
# 一覧の節は見出しの文言で探す（E2E-200）。
INDEX_HEADING = "テストケース一覧"
RE_CASE_HEADING = re.compile(r"^###\s+(E2E-\d{3}):\s*(.+?)\s*$")
RE_FIELD = re.compile(r"^-\s+\*\*(環境|優先度|関連要求|概要|前提条件)\*\*:\s*(.+?)\s*$")
RE_BLOCK = re.compile(r"^-\s+\*\*(手順|確認内容)\*\*:\s*$")
# 手順と確認内容は**どちらも番号付きリスト**である（E2E-200）。番号で指摘できる
# ようにするための書式であり、記録用 Excel にも同じ番号で出す。
RE_STEP = re.compile(r"^\s+\d+\.\s+(.+?)\s*$")
# 「テストケース一覧」節の行: | E2E-211 | G1 | 概要 | 高 |
RE_INDEX_ROW = re.compile(r"^\|\s*(E2E-\d{3})\s*\|\s*(G\d+)\s*\|\s*(.+?)\s*\|\s*(.+?)\s*\|\s*$")

FIELD_ATTR = {
    "環境": "env",
    "優先度": "priority",
    "関連要求": "requirements",
    "概要": "summary",
    "前提条件": "precondition",
}


def section_title(line: str) -> str | None:
    """`## 41.3 G1 起動と終了` から `G1 起動と終了` を取り出す。

    節番号が付いていてもいなくても同じ結果を返す。**番号は当てにしない**
    （E2E-200）。`##` の見出しでなければ None を返す。
    """
    m = RE_SECTION_HEADING.match(line)

    return m.group(1) if m else None


@dataclass
class Case:
    id: str
    title: str
    group: str
    env: str = ""
    priority: str = ""
    requirements: str = ""
    summary: str = ""
    precondition: str = ""
    steps: list[str] = field(default_factory=list)
    expectations: list[str] = field(default_factory=list)

    def missing_fields(self) -> list[str]:
        missing = []
        for label, attr in (
            ("環境", "env"),
            ("優先度", "priority"),
            ("関連要求", "requirements"),
            ("概要", "summary"),
            ("前提条件", "precondition"),
        ):
            if not getattr(self, attr):
                missing.append(label)
        if not self.steps:
            missing.append("手順")
        if not self.expectations:
            missing.append("確認内容")
        return missing


def parse_cases(lines: list[str]) -> list[Case]:
    """グループの節のケース定義を読み取る。

    **グループの見出しは `Gn` で見分ける**（E2E-200）。その下にあるものだけを
    ケースとして扱い、それ以外の節（E2E-200 系の方針や要求一覧）からは
    記録用の行を作らない。**節番号は見ない。**
    """
    cases: list[Case] = []
    group = ""
    current: Case | None = None
    block = ""

    for raw in lines:
        line = raw.rstrip("\n")

        title = section_title(line)
        if title is not None:
            current, block = None, ""
            group = title if RE_GROUP_TITLE.match(title) else ""
            continue

        m = RE_CASE_HEADING.match(line)
        if m:
            block = ""
            if group:
                current = Case(id=m.group(1), title=m.group(2), group=group)
                cases.append(current)
            else:
                current = None
            continue

        if current is None:
            continue

        m = RE_FIELD.match(line)
        if m:
            setattr(current, FIELD_ATTR[m.group(1)], m.group(2))
            block = ""
            continue

        m = RE_BLOCK.match(line)
        if m:
            block = m.group(1)
            continue

        if block == "手順":
            m = RE_STEP.match(line)
            if m:
                current.steps.append(m.group(1))
                continue
        elif block == "確認内容":
            m = RE_STEP.match(line)
            if m:
                current.expectations.append(m.group(1))
                continue

    return cases


def parse_index(lines: list[str]) -> list[tuple[str, str, str, str]]:
    """「テストケース一覧」節のケース一覧を読み取る（突き合わせ用）。

    **節番号ではなく見出しの文言で探す**（E2E-200）。
    """
    rows = []
    in_index = False

    for raw in lines:
        line = raw.rstrip("\n")

        title = section_title(line)
        if title is not None:
            # 一覧の節に入ったあと、次の見出しが来たらそこで終わる。
            if in_index:
                break
            in_index = title == INDEX_HEADING
            continue

        if in_index:
            m = RE_INDEX_ROW.match(line)
            if m:
                rows.append(m.groups())

    return rows


def verify(cases: list[Case], index: list[tuple[str, str, str, str]]) -> list[str]:
    """ケース定義と「テストケース一覧」節を突き合わせ、食い違いを返す。"""
    problems: list[str] = []

    for case in cases:
        missing = case.missing_fields()
        if missing:
            problems.append(f"{case.id}: 項目が欠けている（{', '.join(missing)}）")

    ids = [c.id for c in cases]
    duplicates = sorted({i for i in ids if ids.count(i) > 1})
    if duplicates:
        problems.append(f"ID が重複している: {', '.join(duplicates)}")

    if not index:
        problems.append(f"「{INDEX_HEADING}」節を読み取れなかった")
        return problems

    defined, listed = set(ids), {row[0] for row in index}
    for missing_id in sorted(listed - defined):
        problems.append(f"{missing_id}: 「{INDEX_HEADING}」節にあるが、ケース定義がない")
    for extra_id in sorted(defined - listed):
        problems.append(f"{extra_id}: ケース定義はあるが、「{INDEX_HEADING}」節にない")

    by_id = {c.id: c for c in cases}
    for case_id, group, summary, priority in index:
        case = by_id.get(case_id)
        if case is None:
            continue
        if not case.group.startswith(group + " "):
            problems.append(
                f"{case_id}: グループが一致しない（定義 '{case.group}' / 一覧 '{group}'）"
            )
        if case.priority != priority:
            problems.append(
                f"{case_id}: 優先度が一致しない（定義 '{case.priority}' / 一覧 '{priority}'）"
            )
        if strip_markup(case.title) != strip_markup(summary):
            problems.append(
                f"{case_id}: 概要が一致しない（定義 '{case.title}' / 一覧 '{summary}'）"
            )

    return problems


# --- 表記の正規化（照合と書き出しの両方が使う） -----------------------------

RE_BOLD = re.compile(r"\*\*(.+?)\*\*")
# コードスパン（CommonMark）。開きと閉じは同じ長さのバッククォートの並びで、前後に別のバッククォートが続かない
RE_CODE = re.compile(r"(?<!`)(`+)(?!`)(.+?)(?<!`)\1(?!`)")
# コードスパンを一時的に置き換える印。Unicode の私用領域の文字であり、41 章には現れない
PLACEHOLDER = 0xE000


def strip_markup(text: str) -> str:
    """セルに入れるため、強調とコード記法の記号を落とす。

    **コードスパンの中は書かれたとおりに残す。** 先に太字を落とすと、E2E-384 の `**fresh**`
    （利用者が入力する記法そのもの）が fresh になり、確認する値が変わってしまう。そこで
    コードスパンを印に置き換えてから太字を落とし、最後に中身へ戻す。太字の範囲がコードスパンを
    含んでいても、コードスパンの中の ** とは組にならない。
    """
    spans: list[str] = []

    def hide(match: re.Match[str]) -> str:
        body = match.group(2)
        # CommonMark と同じく、両端に空白があれば 1 つずつ落とす（2 連のバッククォートで書いたとき）
        if len(body) >= 2 and body[0] == " " and body[-1] == " " and body.strip():
            body = body[1:-1]
        spans.append(body)
        return chr(PLACEHOLDER + len(spans) - 1)

    text = RE_BOLD.sub(r"\1", RE_CODE.sub(hide, text))
    for index, body in enumerate(spans):
        text = text.replace(chr(PLACEHOLDER + index), body)
    return text.strip()
