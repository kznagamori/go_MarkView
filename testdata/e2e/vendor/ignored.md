# ここはツリーに出てはいけない

`vendor` は既定で除外されるディレクトリです（FR-031, IMP-132）。
除外の一覧は `node_modules` / `vendor` / `.git` / `target` / `dist` / `build` です。

このファイルが**ファイルツリーに現れたら不合格**です（E2E-241）。
