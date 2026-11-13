package exporter

import (
	"strings"
	"testing"
)

// glob 是 matchGlob 的测试适配层：把相对路径切成片段。
func glob(pattern, rel string) bool {
	return matchGlob(pattern, strings.Split(strings.Trim(rel, "/"), "/"))
}

func TestMatchGlob(t *testing.T) {
	tests := []struct {
		pattern string
		rel     string
		want    bool
	}{
		// 不含 / 的规则：任意一段匹配即可（basename 语义）
		{"*.pyc", "a/b/c.pyc", true},
		{"*.pyc", "c.pyc", true},
		{"*.pyc", "c.py", false},
		{"node_modules", "node_modules", true},
		{"node_modules", "a/node_modules", true},
		{"node_modules", "node_modules/pkg/index.js", true},
		{"node_modules", "a/node_modules/pkg/index.js", true},

		// 含 / 的规则：任意连续片段窗口匹配
		{"dist/*", "dist/a.js", true},
		{"dist/*", "other/dist/a.js", true},
		{"dist/*", "dist/a/b.js", true},
		{"./dist/*", "dist/a.js", true},
		{"src/*", "vendor/src/a.go", true},
		{"dist/*", "src/a.js", false},

		// 需求点：*/node_modules/* 必须生效
		{"*/node_modules/*", "node_modules/pkg/index.js", true},
		{"*/node_modules/*", "a/node_modules/pkg/index.js", true},
		{"*/node_modules/*", "a/b/node_modules/index.js", true},
		{"*/node_modules/*", "a/node_modules", false},

		// 目录规则（尾随 / 视为目录自身，其内容一并忽略）
		{"build/", "build", true},
		{"build/", "build/x.js", true},
		{"build/", "src/build/x.js", true},
		{"build/", "src/builder/x.js", false},

		// 空规则与注释
		{"", "a.txt", false},
		{"   ", "a.txt", false},
		{"# comment", "# comment", false},

		// 非法 glob 不 panic，也不匹配
		{"[abc", "a.txt", false},
	}
	for _, tt := range tests {
		if got := glob(tt.pattern, tt.rel); got != tt.want {
			t.Errorf("matchGlob(%q, %q) = %v, 期望 %v", tt.pattern, tt.rel, got, tt.want)
		}
	}
}

func TestMatchIgnoresMultiplePatterns(t *testing.T) {
	patterns := []string{"dist/*", "*.log"}
	if !matchIgnores(patterns, "dist/a.js") {
		t.Fatal("应命中 dist/*")
	}
	if !matchIgnores(patterns, "logs/app.log") {
		t.Fatal("应命中 *.log")
	}
	if matchIgnores(patterns, "src/main.go") {
		t.Fatal("不应命中任何规则")
	}
	if matchIgnores(nil, "src/main.go") {
		t.Fatal("空规则集不应命中")
	}
}

func TestDefaultIgnoresCoverBinaries(t *testing.T) {
	for _, rel := range []string{
		"node_modules/x/index.js",
		"__pycache__/m.pyc",
		"/node_modules/pkg/index.js",
		"a/main.pyc",
		"a/main.o",
		"a/lib.so",
		"a/app.exe",
		"a/logo.png",
		"a/doc.pdf",
		"a/pack.tar.gz",
		"a/font.woff2",
		"a/movie.mp4",
		"a/data.sqlite3",
	} {
		if !matchIgnores(defaultIgnores, rel) {
			t.Errorf("默认规则应忽略 %q", rel)
		}
	}
	for _, rel := range []string{"main.go", "README.md", "src/app.js", "a/icon.svg"} {
		if matchIgnores(defaultIgnores, rel) {
			t.Errorf("默认规则不应忽略 %q", rel)
		}
	}
}

func TestSegmentWindows(t *testing.T) {
	got := segmentWindows([]string{"a", "b", "c"})
	want := []string{"a", "a/b", "a/b/c", "b", "b/c", "c"}
	if len(got) != len(want) {
		t.Fatalf("窗口数量不符: %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("segmentWindows 第 %d 项 = %q, 期望 %q（全部: %v）", i, got[i], want[i], got)
		}
	}
}
