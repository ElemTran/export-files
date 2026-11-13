package exporter

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func writeFile(t *testing.T, name, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatalf("创建目录失败: %v", err)
	}
	if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
		t.Fatalf("写入文件失败: %v", err)
	}
}

// export 在 dir 中执行一次导出，返回输出文本、警告信息与错误。
func export(t *testing.T, dir string, opts Options, paths ...string) (string, string, error) {
	t.Helper()
	t.Chdir(dir)
	var buf, warnings bytes.Buffer
	opts.Warnf = func(format string, a ...any) {
		warnings.WriteString(fmt.Sprintf(format, a...) + "\n")
	}
	err := New(&buf, opts).Run(paths)
	return buf.String(), warnings.String(), err
}

func TestRunDirectoryFormat(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.go"), "package main\n")
	writeFile(t, filepath.Join(dir, "sub", "b.py"), "print(1)\n")

	got, _, err := export(t, dir, Options{}, ".")
	if err != nil {
		t.Fatalf("Run 返回错误: %v", err)
	}
	want := "## a.go\n```go\npackage main\n```\n\n" +
		"## sub/b.py\n```python\nprint(1)\n```\n\n"
	if got != want {
		t.Fatalf("输出不符\n期望:\n%q\n实际:\n%q", want, got)
	}
}

func TestRunSingleFileKeepsGivenPath(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "App.vue"), "<template/>\n")

	got, _, err := export(t, dir, Options{}, "src/App.vue")
	if err != nil {
		t.Fatalf("Run 返回错误: %v", err)
	}
	want := "## src/App.vue\n```vue\n<template/>\n```\n\n"
	if got != want {
		t.Fatalf("输出不符\n期望:\n%q\n实际:\n%q", want, got)
	}
}

func TestSortsFullPaths(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "b.txt"), "b\n")
	writeFile(t, filepath.Join(dir, "a", "z.txt"), "z\n")
	writeFile(t, filepath.Join(dir, "a.txt"), "a\n")

	got, _, err := export(t, dir, Options{NoIgnore: true}, ".")
	if err != nil {
		t.Fatalf("Run 返回错误: %v", err)
	}
	last := -1
	for _, marker := range []string{"## a.txt", "## a/z.txt", "## b.txt"} {
		idx := strings.Index(got, marker+"\n")
		if idx < 0 {
			t.Fatalf("输出中缺少 %q:\n%s", marker, got)
		}
		if idx < last {
			t.Fatalf("顺序错误，%q 出现在错误位置:\n%s", marker, got)
		}
		last = idx
	}
}

func TestSkipsHiddenByDefault(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".env"), "A=1\n")
	writeFile(t, filepath.Join(dir, ".github", "ci.yml"), "on: push\n")
	writeFile(t, filepath.Join(dir, "main.txt"), "x\n")

	got, _, err := export(t, dir, Options{NoIgnore: true}, ".")
	if err != nil {
		t.Fatalf("Run 返回错误: %v", err)
	}
	if strings.Contains(got, ".env") || strings.Contains(got, "ci.yml") {
		t.Fatalf("隐藏文件未被跳过:\n%s", got)
	}
	if !strings.Contains(got, "## main.txt") {
		t.Fatalf("普通文件缺失:\n%s", got)
	}

	got, _, err = export(t, dir, Options{NoIgnore: true, IncludeHidden: true}, ".")
	if err != nil {
		t.Fatalf("Run 返回错误: %v", err)
	}
	if !strings.Contains(got, "## .env") || !strings.Contains(got, "## .github/ci.yml") {
		t.Fatalf("IncludeHidden 未生效:\n%s", got)
	}
}

func TestDefaultIgnoresSkipVendorDir(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "node_modules", "pkg", "index.js"), "x\n")
	writeFile(t, filepath.Join(dir, "__pycache__", "m.pyc"), "x\n")
	writeFile(t, filepath.Join(dir, "keep.js"), "x\n")

	got, _, err := export(t, dir, Options{}, ".")
	if err != nil {
		t.Fatalf("Run 返回错误: %v", err)
	}
	if strings.Contains(got, "node_modules") || strings.Contains(got, "__pycache__") {
		t.Fatalf("默认忽略规则未生效:\n%s", got)
	}
	if !strings.Contains(got, "## keep.js") {
		t.Fatalf("未忽略的文件缺失:\n%s", got)
	}
}

func TestNoIgnoreAndCustomIgnore(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "node_modules", "index.js"), "x\n")
	writeFile(t, filepath.Join(dir, "dist", "bundle.js"), "x\n")
	writeFile(t, filepath.Join(dir, "keep.js"), "x\n")

	got, _, err := export(t, dir, Options{NoIgnore: true}, ".")
	if err != nil {
		t.Fatalf("Run 返回错误: %v", err)
	}
	if !strings.Contains(got, "## node_modules/index.js") {
		t.Fatalf("NoIgnore 未生效:\n%s", got)
	}

	got, _, err = export(t, dir, Options{Ignore: []string{"dist/*"}}, ".")
	if err != nil {
		t.Fatalf("Run 返回错误: %v", err)
	}
	if strings.Contains(got, "bundle.js") {
		t.Fatalf("Ignore 'dist/*' 未生效:\n%s", got)
	}
	if !strings.Contains(got, "## keep.js") {
		t.Fatalf("不该被忽略的文件缺失:\n%s", got)
	}
}

func TestBinaryFileSkipped(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bin.dat"), []byte{0x00, 0x01, 0x02, 0xff}, 0o644); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "text.dat"), "text\n")

	got, _, err := export(t, dir, Options{NoIgnore: true}, ".")
	if err != nil {
		t.Fatalf("Run 返回错误: %v", err)
	}
	wantBinary := "## bin.dat\n```\n" + binaryPlaceholder + "\n```\n\n"
	if !strings.Contains(got, wantBinary) {
		t.Fatalf("二进制占位输出不符，实际:\n%q", got)
	}
	if !strings.Contains(got, "## text.dat\n```\ntext\n```\n\n") {
		t.Fatalf("文本 .dat 文件应按无语言标记输出，实际:\n%q", got)
	}
}

func TestIsBinarySniffFirst8KB(t *testing.T) {
	if isBinary(bytes.Repeat([]byte("a"), binarySniffLen+1)) {
		t.Fatal("纯文本不应判为二进制")
	}
	// 嗅探窗口是前 8KB（下标 0..8191）
	inside := append(bytes.Repeat([]byte("a"), binarySniffLen-1), 0x00)
	if !isBinary(inside) {
		t.Fatal("窗口内的 NUL 应判为二进制")
	}
	outside := append(bytes.Repeat([]byte("a"), binarySniffLen), 0x00)
	if isBinary(outside) {
		t.Fatal("窗口外的 NUL 不应判为二进制")
	}
	if isBinary(nil) {
		t.Fatal("空文件不应判为二进制")
	}
}

func TestAddsTrailingNewline(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "noeol.txt"), "no newline at eof")

	got, _, err := export(t, dir, Options{NoIgnore: true}, ".")
	if err != nil {
		t.Fatalf("Run 返回错误: %v", err)
	}
	want := "## noeol.txt\n```\nno newline at eof\n```\n\n"
	if got != want {
		t.Fatalf("输出不符\n期望:\n%q\n实际:\n%q", want, got)
	}
}

func TestEmptyFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "empty.txt"), "")

	got, _, err := export(t, dir, Options{NoIgnore: true}, ".")
	if err != nil {
		t.Fatalf("Run 返回错误: %v", err)
	}
	want := "## empty.txt\n```\n```\n\n"
	if got != want {
		t.Fatalf("输出不符\n期望:\n%q\n实际:\n%q", want, got)
	}
}

func TestNamesWithSpacesAndUnicode(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a b.txt"), "hi\n")
	writeFile(t, filepath.Join(dir, "中文 目录", "文 件.go"), "package x\n")

	got, _, err := export(t, dir, Options{NoIgnore: true}, ".")
	if err != nil {
		t.Fatalf("Run 返回错误: %v", err)
	}
	for _, want := range []string{
		"## a b.txt\n```\nhi\n```\n\n",
		"## 中文 目录/文 件.go\n```go\npackage x\n```\n\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("输出缺少:\n%q\n完整输出:\n%q", want, got)
		}
	}
}

func TestNameWithNewline(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows 不允许文件名包含换行")
	}
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "new\nline.txt"), "x\n")

	got, _, err := export(t, dir, Options{NoIgnore: true}, ".")
	if err != nil {
		t.Fatalf("Run 返回错误: %v", err)
	}
	want := "## new\nline.txt\n```\nx\n```\n\n"
	if !strings.Contains(got, want) {
		t.Fatalf("输出缺少:\n%q\n完整输出:\n%q", want, got)
	}
}

func TestMaxSize(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "small.txt"), "12345")
	writeFile(t, filepath.Join(dir, "big.txt"), strings.Repeat("x", 100))

	got, _, err := export(t, dir, Options{NoIgnore: true, MaxSize: 10}, ".")
	if err != nil {
		t.Fatalf("Run 返回错误: %v", err)
	}
	if !strings.Contains(got, "## small.txt") {
		t.Fatalf("小文件被跳过:\n%s", got)
	}
	if strings.Contains(got, "big.txt") {
		t.Fatalf("超限文件未被跳过:\n%s", got)
	}
}

func TestExcludeOutputFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"), "a\n")
	out := filepath.Join(dir, "out.md")
	writeFile(t, out, "## 旧内容\n")

	got, _, err := export(t, dir, Options{NoIgnore: true, Exclude: []string{out}}, ".")
	if err != nil {
		t.Fatalf("Run 返回错误: %v", err)
	}
	if strings.Contains(got, "out.md") || strings.Contains(got, "旧内容") {
		t.Fatalf("输出文件自身未被排除:\n%s", got)
	}
	if !strings.Contains(got, "## a.txt") {
		t.Fatalf("正常文件缺失:\n%s", got)
	}
}

func TestMissingPathWarnsAndContinues(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"), "a\n")

	got, warnings, err := export(t, dir, Options{NoIgnore: true}, "nope", ".")
	if err == nil {
		t.Fatal("路径缺失应返回错误")
	}
	var notFound *PathsNotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("错误类型不符: %T", err)
	}
	if len(notFound.Paths) != 1 || notFound.Paths[0] != "nope" {
		t.Fatalf("缺失路径列表不符: %v", notFound.Paths)
	}
	if !strings.Contains(warnings, "路径不存在: nope") {
		t.Fatalf("缺少警告: %q", warnings)
	}
	if !strings.Contains(got, "## a.txt") {
		t.Fatalf("其余路径未被处理:\n%s", got)
	}
}

func TestSymlinkSkippedByDefault(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows 创建符号链接需要额外权限")
	}
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "target.txt"), "x\n")
	if err := os.Symlink(filepath.Join(dir, "target.txt"), filepath.Join(dir, "link.txt")); err != nil {
		t.Skipf("无法创建符号链接: %v", err)
	}

	got, warnings, err := export(t, dir, Options{NoIgnore: true}, ".")
	if err != nil {
		t.Fatalf("Run 返回错误: %v", err)
	}
	if strings.Contains(got, "link.txt") {
		t.Fatalf("符号链接未被跳过:\n%s", got)
	}
	if !strings.Contains(warnings, "跳过符号链接: link.txt") {
		t.Fatalf("缺少符号链接警告: %q", warnings)
	}

	got, _, err = export(t, dir, Options{NoIgnore: true, Follow: true}, ".")
	if err != nil {
		t.Fatalf("Run 返回错误: %v", err)
	}
	if !strings.Contains(got, "## link.txt") {
		t.Fatalf("Follow 未生效:\n%s", got)
	}
}

func TestSymlinkLoopDoesNotHang(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows 创建符号链接需要额外权限")
	}
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "sub", "a.txt"), "a\n")
	if err := os.Symlink(dir, filepath.Join(dir, "sub", "loop")); err != nil {
		t.Skipf("无法创建符号链接: %v", err)
	}

	got, warnings, err := export(t, dir, Options{NoIgnore: true, Follow: true}, ".")
	if err != nil {
		t.Fatalf("Run 返回错误: %v", err)
	}
	if !strings.Contains(got, "## sub/a.txt") {
		t.Fatalf("正常文件缺失:\n%s", got)
	}
	if !strings.Contains(warnings, "符号链接环") {
		t.Fatalf("缺少链接环警告: %q", warnings)
	}
}

func TestMultipleRootsKeepExplicitRelPaths(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "a.go"), "package a\n")
	writeFile(t, filepath.Join(dir, "docs", "b.md"), "# b\n")

	got, _, err := export(t, dir, Options{NoIgnore: true}, "src", "docs/b.md")
	if err != nil {
		t.Fatalf("Run 返回错误: %v", err)
	}
	if !strings.Contains(got, "## src/a.go\n```go\n") {
		t.Fatalf("目录根路径前缀不符:\n%s", got)
	}
	if !strings.Contains(got, "## docs/b.md\n```markdown\n") {
		t.Fatalf("单文件路径不符:\n%s", got)
	}
}

func TestUnreadableDirectoryWarns(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("当前环境无法构造不可读目录")
	}
	dir := t.TempDir()
	locked := filepath.Join(dir, "locked")
	if err := os.MkdirAll(locked, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(locked, "a.txt"), "a\n")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	_, warnings, err := export(t, dir, Options{NoIgnore: true}, ".")
	if err != nil {
		t.Fatalf("不可读目录不应导致整体失败: %v", err)
	}
	if !strings.Contains(warnings, "无法读取目录") {
		t.Fatalf("缺少警告: %q", warnings)
	}
}
