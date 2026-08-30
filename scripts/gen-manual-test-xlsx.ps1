# 41-e2e-manual.md から手動テスト仕様書（Excel）を生成する
param(
    [Parameter(Mandatory=$true)][string]$Src,
    [Parameter(Mandatory=$true)][string]$Out
)
$ErrorActionPreference = 'Stop'

# ---- Markdown のパース ----------------------------------------------------
$lines = Get-Content -LiteralPath $Src -Encoding UTF8
$cases = New-Object System.Collections.ArrayList
$group = ''
$cur = $null
$mode = ''

function Flush {
    # グループが未設定のもの（41.1 節の方針項目）はケースとして扱わない
    if ($null -ne $script:cur -and $script:cur.Group -ne '') {
        [void]$script:cases.Add($script:cur)
    }
    $script:cur = $null
}

foreach ($raw in $lines) {
    $line = $raw.TrimEnd()
    if ($line -match '^##\s+41\.\d+\s+(G\d+\s+.+)$') { Flush; $group = $Matches[1]; continue }
    if ($line -match '^##\s+41\.\d+\s+') { Flush; $group = ''; continue }
    if ($line -match '^###\s+(E2E-\d{3}):\s*(.+)$') {
        Flush
        $cur = [ordered]@{
            ID = $Matches[1]; Group = $group; Title = $Matches[2]
            Env = ''; Priority = ''; Req = ''; Summary = ''; Pre = ''
            Steps = New-Object System.Collections.ArrayList
            Expect = New-Object System.Collections.ArrayList
        }
        $mode = ''
        continue
    }
    if ($null -eq $cur) { continue }
    if ($line -match '^-\s+\*\*環境\*\*:\s*(.+)$')     { $cur.Env = $Matches[1]; $mode=''; continue }
    if ($line -match '^-\s+\*\*優先度\*\*:\s*(.+)$')   { $cur.Priority = $Matches[1]; $mode=''; continue }
    if ($line -match '^-\s+\*\*関連要求\*\*:\s*(.+)$') { $cur.Req = $Matches[1]; $mode=''; continue }
    if ($line -match '^-\s+\*\*概要\*\*:\s*(.+)$')     { $cur.Summary = $Matches[1]; $mode=''; continue }
    if ($line -match '^-\s+\*\*前提条件\*\*:\s*(.+)$') { $cur.Pre = $Matches[1]; $mode=''; continue }
    if ($line -match '^-\s+\*\*手順\*\*:\s*$')         { $mode = 'steps';  continue }
    if ($line -match '^-\s+\*\*確認内容\*\*:\s*$')     { $mode = 'expect'; continue }
    if ($mode -eq 'steps'  -and $line -match '^\s+\d+\.\s+(.+)$') { [void]$cur.Steps.Add($Matches[1]);  continue }
    if ($mode -eq 'expect' -and $line -match '^\s+-\s+(.+)$')     { [void]$cur.Expect.Add($Matches[1]); continue }
}
Flush

Write-Output ("ケース数: " + $cases.Count)
if ($cases.Count -eq 0) { throw "ケースを抽出できませんでした" }
$missing = $cases | Where-Object { $_.Env -eq '' -or $_.Priority -eq '' -or $_.Steps.Count -eq 0 -or $_.Expect.Count -eq 0 }
if ($missing) { $missing | ForEach-Object { Write-Output ("  [警告] 項目不足: " + $_.ID) } }

function Clean([string]$s) {
    if ([string]::IsNullOrEmpty($s)) { return '' }
    $s = $s -replace '\*\*(.+?)\*\*', '$1'
    $s = $s -replace '`(.+?)`', '$1'
    return $s
}
function JoinNum($list) { $i=0; (($list | ForEach-Object { $i++; "$i. " + (Clean $_) }) -join "`n") }
function JoinBul($list) { (($list | ForEach-Object { "・" + (Clean $_) }) -join "`n") }

# ---- Excel の生成 ---------------------------------------------------------
$excel = $null; $book = $null
try {
    $excel = New-Object -ComObject Excel.Application
    $excel.Visible = $false
    $excel.DisplayAlerts = $false
    $book = $excel.Workbooks.Add()
    while ($book.Worksheets.Count -lt 3) { [void]$book.Worksheets.Add() }
    while ($book.Worksheets.Count -gt 3) { $book.Worksheets.Item($book.Worksheets.Count).Delete() }

    $shCover = $book.Worksheets.Item(1); $shCover.Name = '表紙'
    $shCase  = $book.Worksheets.Item(2); $shCase.Name  = 'テスト仕様・結果'
    $shEnv   = $book.Worksheets.Item(3); $shEnv.Name   = '環境定義'
    Write-Output "  シート作成 OK"

    # ---- 表紙 ----
    $cover = @(
        @('MarkView 手動テスト仕様書 兼 結果記録',''),
        @('',''),
        @('対象プロダクト','MarkView'),
        @('リポジトリ','https://github.com/kznagamori/go_MarkView'),
        @('生成元','docs/specs/41-e2e-manual.md（こちらが正）'),
        @('総ケース数', [string]$cases.Count),
        @('',''),
        @('■ 実施情報（実施時に記入）',''),
        @('ソフトウェアバージョン',''),
        @('実施期間（開始）',''),
        @('実施期間（終了）',''),
        @('実施者',''),
        @('',''),
        @('■ 結果サマリ（実施時に記入）',''),
        @('OK',''), @('NG',''), @('対象外',''), @('未実施',''),
        @('',''),
        @('■ 注意',''),
        @('1','本ファイルは docs/specs/41-e2e-manual.md から生成した記録用ファイルです。'),
        @('2','ケースの追加・変更は 41 章で行い、本ファイルを作り直してください。'),
        @('3','本ファイルを直接編集してケースを変更しないでください。'),
        @('4','記入するのは J〜P 列（実施日・バージョン・実施環境・実施者・結果・備考・不具合番号）です。'),
        @('5','実施済みのファイルは MarkView_手動テスト結果_v<バージョン>.xlsx として別名保存してください。')
    )
    for ($i=0; $i -lt $cover.Count; $i++) {
        $shCover.Cells.Item($i+1,1).Value2 = [string]$cover[$i][0]
        $shCover.Cells.Item($i+1,2).Value2 = [string]$cover[$i][1]
    }
    $shCover.Cells.Item(1,1).Font.Size = 16
    $shCover.Cells.Item(1,1).Font.Bold = $true
    foreach ($r in @(8,14,20)) { $shCover.Cells.Item($r,1).Font.Bold = $true }
    $shCover.Columns.Item(1).ColumnWidth = 28
    $shCover.Columns.Item(2).ColumnWidth = 72
    Write-Output "  表紙 OK"

    # ---- 環境定義 ----
    $envRows = @(
        @('コード','環境','手動テストでの位置づけ'),
        @('W1','Windows 11 / amd64','主環境。全ケースを実施する'),
        @('L1','Ubuntu 24.04 / amd64','OS 差の出るケースを実施する'),
        @('WA','Windows 11 / arm64','入手できた場合に起動確認を行う'),
        @('LA','Ubuntu 24.04 / arm64','入手できた場合に起動確認を行う'),
        @('','',''),
        @('優先度','意味',''),
        @('高','壊れていると配布物として成立しない。NG ならリリースしない',''),
        @('中','主要機能。NG なら原則リリースしない',''),
        @('低','補助的な振る舞い。NG でも影響を判断のうえリリースしてよい',''),
        @('','',''),
        @('結果','意味',''),
        @('OK','確認内容をすべて満たした',''),
        @('NG','満たさない項目があった。不具合番号を記入する',''),
        @('対象外','環境が用意できない等で実施しなかった。理由を備考に記入する',''),
        @('未実施','まだ実施していない','')
    )
    for ($i=0; $i -lt $envRows.Count; $i++) {
        for ($j=0; $j -lt 3; $j++) {
            $shEnv.Cells.Item($i+1,$j+1).Value2 = [string]$envRows[$i][$j]
        }
    }
    foreach ($r in @(1,7,12)) { $shEnv.Rows.Item($r).Font.Bold = $true }
    $shEnv.Columns.Item(1).ColumnWidth = 12
    $shEnv.Columns.Item(2).ColumnWidth = 55
    $shEnv.Columns.Item(3).ColumnWidth = 40
    Write-Output "  環境定義 OK"

    # ---- テスト仕様・結果 ----
    $headers = @('ID','テストグループ','テスト環境','優先度','テスト概要','前提条件',
                 'テスト手順','テスト確認内容','関連要求',
                 'テスト実施日','ソフトウェアバージョン','実施環境','実施者','テスト確認結果','実測・備考','不具合番号')
    for ($j=0; $j -lt $headers.Count; $j++) { $shCase.Cells.Item(1,$j+1).Value2 = [string]$headers[$j] }

    $row = 2
    foreach ($c in $cases) {
        $shCase.Cells.Item($row,1).Value2 = [string]$c.ID
        $shCase.Cells.Item($row,2).Value2 = [string]$c.Group
        $shCase.Cells.Item($row,3).Value2 = [string]$c.Env
        $shCase.Cells.Item($row,4).Value2 = [string]$c.Priority
        $shCase.Cells.Item($row,5).Value2 = [string](Clean $c.Summary)
        $shCase.Cells.Item($row,6).Value2 = [string](Clean $c.Pre)
        $shCase.Cells.Item($row,7).Value2 = [string](JoinNum $c.Steps)
        $shCase.Cells.Item($row,8).Value2 = [string](JoinBul $c.Expect)
        $shCase.Cells.Item($row,9).Value2 = [string]$c.Req
        $shCase.Cells.Item($row,14).Value2 = '未実施'
        $row++
    }
    $last = $row - 1
    Write-Output ("  ケース書き込み OK（最終行 " + $last + "）")

    $head = $shCase.Range($shCase.Cells.Item(1,1), $shCase.Cells.Item(1,$headers.Count))
    $head.Font.Bold = $true
    $head.Interior.Color = 15849925
    $head.HorizontalAlignment = -4108

    $all = $shCase.Range($shCase.Cells.Item(1,1), $shCase.Cells.Item($last,$headers.Count))
    $all.VerticalAlignment = -4160
    $all.WrapText = $true
    for ($b=7; $b -le 12; $b++) { $all.Borders.Item($b).LineStyle = 1 }
    Write-Output "  書式 OK"

    $widths = @(10,20,12,8,40,32,48,48,24,14,20,10,10,14,36,12)
    for ($k=1; $k -le $widths.Count; $k++) { $shCase.Columns.Item($k).ColumnWidth = $widths[$k-1] }
    $shCase.Rows.Item(1).RowHeight = 30
    for ($r=2; $r -le $last; $r++) { [void]$shCase.Rows.Item($r).AutoFit() }
    Write-Output "  列幅・行高 OK"

    [void]$head.AutoFilter()
    $resultCol = $shCase.Range($shCase.Cells.Item(2,14), $shCase.Cells.Item($last,14))
    $resultCol.Validation.Delete()
    [void]$resultCol.Validation.Add(3, 1, 1, "OK,NG,対象外,未実施")
    $resultCol.Validation.IgnoreBlank = $true
    $resultCol.Validation.InCellDropdown = $true
    Write-Output "  フィルタ・入力規則 OK"

    $shCase.PageSetup.Orientation = 2
    $shCase.PageSetup.Zoom = $false
    $shCase.PageSetup.FitToPagesWide = 1
    $shCase.PageSetup.FitToPagesTall = $false
    $shCase.PageSetup.PrintTitleRows = '$1:$1'
    Write-Output "  印刷設定 OK"

    $shCase.Activate()
    $shCase.Range("A2").Select()
    $excel.ActiveWindow.FreezePanes = $true
    $shCover.Activate()
    Write-Output "  ウィンドウ枠固定 OK"

    $full = [System.IO.Path]::GetFullPath($Out)
    $dir = Split-Path -Parent $full
    if (-not (Test-Path $dir)) { New-Item -ItemType Directory -Path $dir -Force | Out-Null }
    if (Test-Path $full) { Remove-Item -LiteralPath $full -Force }
    $book.SaveAs($full, 51)
    Write-Output ("生成しました: " + $full)
}
catch {
    Write-Output ("エラー行 " + $_.InvocationInfo.ScriptLineNumber + ": " + $_.Exception.Message)
    throw
}
finally {
    if ($null -ne $book)  { try { $book.Close($false) } catch {} }
    if ($null -ne $excel) { try { $excel.Quit() } catch {} }
    [GC]::Collect(); [GC]::WaitForPendingFinalizers()
}
