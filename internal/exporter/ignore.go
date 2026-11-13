package exporter

import (
	"path"
	"path/filepath"
	"strings"
)

// defaultIgnores 是默认忽略规则：版本控制与依赖目录、构建缓存，
// 以及常见二进制（目标文件、图片、字体、压缩包、音视频、数据库）。
var defaultIgnores = []string{
	// 版本控制与依赖目录
	".git", ".svn", ".hg", "node_modules", "vendor", "bower_components",
	// Python 缓存与虚拟环境
	"__pycache__", ".pytest_cache", ".mypy_cache", ".ruff_cache", ".venv", "venv", ".tox",
	// IDE 与构建缓存
	".idea", ".vscode", ".gradle", ".next", ".nuxt", ".cache", "target",
	// 目标文件与可执行文件
	"*.pyc", "*.pyo", "*.pyd", "*.class", "*.o", "*.obj", "*.a", "*.lib",
	"*.so", "*.so.*", "*.dylib", "*.dll", "*.exe", "*.wasm", "*.jar", "*.war",
	// 图片
	"*.png", "*.jpg", "*.jpeg", "*.gif", "*.webp", "*.ico", "*.bmp",
	"*.tif", "*.tiff", "*.psd",
	// 字体
	"*.ttf", "*.otf", "*.woff", "*.woff2", "*.eot",
	// 文档与压缩包
	"*.pdf", "*.zip", "*.tar", "*.gz", "*.tgz", "*.bz2", "*.xz", "*.zst",
	"*.7z", "*.rar", "*.whl", "*.egg",
	// 音视频
	"*.mp3", "*.wav", "*.flac", "*.ogg", "*.m4a", "*.mp4", "*.mkv", "*.avi",
	"*.mov", "*.webm",
	// 数据库
	"*.db", "*.sqlite", "*.sqlite3",
}

// matchIgnores 判断相对路径（以 / 分隔）是否命中任一忽略规则。
func matchIgnores(patterns []string, rel string) bool {
	rel = strings.TrimPrefix(filepath.ToSlash(rel), "./")
	if rel == "" {
		return false
	}
	segments := strings.Split(strings.Trim(rel, "/"), "/")
	for _, p := range patterns {
		if matchGlob(p, segments) {
			return true
		}
	}
	return false
}

// matchGlob 用 path.Match 语义匹配单个忽略规则，规则作用在路径的任意片段窗口上：
//   - 不含 / 的规则（*.pyc、node_modules）对任意一段路径匹配，等价于 basename 匹配，
//     因此也能命中目录名并连带其下所有内容（gitignore 语义）；
//   - 含 / 的规则对任意连续片段窗口匹配，且自动忽略开头的 "*/"，
//     因此 "*/node_modules/*" 同时命中 node_modules/x.js 与 a/b/node_modules/x.js；
//   - 以 / 结尾的规则按目录自身匹配。
func matchGlob(pattern string, segments []string) bool {
	p := strings.TrimSpace(filepath.ToSlash(pattern))
	if p == "" || strings.HasPrefix(p, "#") {
		return false
	}
	p = strings.TrimSuffix(strings.TrimPrefix(p, "./"), "/")
	if p == "" {
		return false
	}
	for p != "" {
		if matchAnyWindow(p, segments) {
			return true
		}
		// "*/node_modules/*" 这类前缀通配再放宽一档
		if !strings.HasPrefix(p, "*/") {
			return false
		}
		p = strings.TrimPrefix(p, "*/")
	}
	return false
}

// matchAnyWindow 在路径的所有连续片段窗口上尝试一条 glob。
func matchAnyWindow(p string, segments []string) bool {
	if !strings.Contains(p, "/") {
		for _, seg := range segments {
			if ok, err := path.Match(p, seg); err == nil && ok {
				return true
			}
		}
		return false
	}
	for _, w := range segmentWindows(segments) {
		if ok, err := path.Match(p, w); err == nil && ok {
			return true
		}
		if ok, err := path.Match(p, "/"+w); err == nil && ok {
			return true
		}
	}
	return false
}

// segmentWindows 返回所有连续片段窗口，例：a/b/c ->
// [a b c a/b b/c a/b/c]
func segmentWindows(segments []string) []string {
	out := make([]string, 0, len(segments)*(len(segments)+1)/2)
	for i := range segments {
		for j := i + 1; j <= len(segments); j++ {
			out = append(out, strings.Join(segments[i:j], "/"))
		}
	}
	return out
}
