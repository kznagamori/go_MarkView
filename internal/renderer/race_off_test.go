//go:build !race

package renderer

// raceEnabled は競合検出つきでビルドされたかを示す。詳細は race_on_test.go。
const raceEnabled = false
