package main

import (
	"bytes"
	"os"
	"path/filepath"
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

func TestParseArgsInterspersedFlags(t *testing.T) {
	cfg, err := parseArgs([]string{".", "--ignore", "dist/*", "-L", "--ignore=*.log", "--max-size", "5M", "-o", "out.md"})
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(cfg.paths) != 1 || cfg.paths[0] != "." {
		t.Fatalf("路径解析错误: %v", cfg.paths)
	}
	if len(cfg.ignores) != 2 || cfg.ignores[0] != "dist/*" || cfg.ignores[1] != "*.log" {
		t.Fatalf("忽略规则解析错误: %v", cfg.ignores)
	}
	if cfg.maxSize != 5<<20 {
		t.Fatalf("--max-size 解析错误: %d", cfg.maxSize)
	}
	if !cfg.follow {
		t.Fatal("-L 未生效")
	}
	if cfg.output != "out.md" {
		t.Fatalf("--output 解析错误: %q", cfg.output)
	}
}

func TestParseArgsLongForms(t *testing.T) {
	cfg, err := parseArgs([]string{"--output", "o.md", "--follow", "--no-ignore", "--include-hidden", "src"})
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if cfg.output != "o.md" || !cfg.follow || !cfg.noIgnore || !cfg.includeHidden {
		t.Fatalf("长选项解析错误: %+v", cfg)
	}
}

func TestParseArgsDoubleDash(t *testing.T) {
	cfg, err := parseArgs([]string{"--no-ignore", "--", "-weird-name"})
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(cfg.paths) != 1 || cfg.paths[0] != "-weird-name" {
		t.Fatalf("-- 之后的参数应视为路径: %v", cfg.paths)
	}
}

func TestParseArgsErrors(t *testing.T) {
	cases := [][]string{
		{"--unknown"},
		{"-o"},
		{"--max-size"},
		{"--max-size", "abc"},
		{"--ignore", ""},
		{"-h=1"},
	}
	for _, args := range cases {
		if _, err := parseArgs(args); err == nil {
			t.Errorf("参数 %v 应报错", args)
		}
	}
}

func TestRunNoArgsExitsOne(t *testing.T) {
	var stdout, stderr strings.Builder
	if code := run(nil, &stdout, &stderr); code != 1 {
		t.Fatalf("退出码 = %d, 期望 1", code)
	}
	if !strings.Contains(stderr.String(), "请指定至少一个路径") {
		t.Fatalf("缺少错误提示: %q", stderr.String())
	}
}

func TestRunHelp(t *testing.T) {
	var stdout, stderr strings.Builder
	if code := run([]string{"--help"}, &stdout, &stderr); code != 0 {
		t.Fatalf("退出码 = %d, 期望 0", code)
	}
	if !strings.Contains(stdout.String(), "--include-hidden") {
		t.Fatalf("帮助信息不完整: %q", stdout.String())
	}
}

func TestRunStdout(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.go"), "package a\n")
	t.Chdir(dir)

	var stdout, stderr strings.Builder
	if code := run([]string{"."}, &stdout, &stderr); code != 0 {
		t.Fatalf("退出码 = %d, stderr = %q", code, stderr.String())
	}
	want := "## a.go\n```go\npackage a\n```\n\n"
	if stdout.String() != want {
		t.Fatalf("输出不符\n期望:\n%q\n实际:\n%q", want, stdout.String())
	}
}

func TestRunOutputFileExcludesItself(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.go"), "package a\n")
	t.Chdir(dir)
	target := filepath.Join(dir, "out.md")
	// 预置旧内容，确保它不会被递归扫进新输出
	writeFile(t, target, "旧内容\n")

	var stdout, stderr strings.Builder
	if code := run([]string{".", "-o", target}, &stdout, &stderr); code != 0 {
		t.Fatalf("退出码 = %d, stderr = %q", code, stderr.String())
	}
	first, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	want := "## a.go\n```go\npackage a\n```\n\n"
	if string(first) != want {
		t.Fatalf("文件输出不符\n期望:\n%q\n实际:\n%q", want, string(first))
	}

	// 再跑一次应完全一致（幂等，说明输出文件被排除）
	if code := run([]string{".", "-o", target}, &stdout, &stderr); code != 0 {
		t.Fatalf("第二次退出码 = %d, stderr = %q", code, stderr.String())
	}
	second, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("两次输出不一致:\n%q\n%q", string(first), string(second))
	}
	// 临时文件不应残留
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, de := range entries {
		if strings.HasPrefix(de.Name(), ".export-files-") {
			t.Fatalf("残留临时文件: %s", de.Name())
		}
	}
}

func TestRunMissingPathExitsNonZero(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"), "a\n")
	t.Chdir(dir)

	var stdout, stderr strings.Builder
	if code := run([]string{"nope", "."}, &stdout, &stderr); code == 0 {
		t.Fatal("缺失路径应返回非 0 退出码")
	}
	if !strings.Contains(stderr.String(), "路径不存在: nope") {
		t.Fatalf("缺少警告: %q", stderr.String())
	}
	if !strings.Contains(stdout.String(), "## a.txt") {
		t.Fatalf("其余路径未被处理: %q", stdout.String())
	}
}

func TestRunMissingPathStillWritesOutput(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"), "a\n")
	t.Chdir(dir)
	target := filepath.Join(dir, "out.md")

	var stdout, stderr strings.Builder
	if code := run([]string{"nope", ".", "-o", target}, &stdout, &stderr); code == 0 {
		t.Fatal("缺失路径应返回非 0 退出码")
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("输出文件应仍被写出: %v", err)
	}
	if !strings.Contains(string(data), "## a.txt") {
		t.Fatalf("输出内容不符: %q", string(data))
	}
}
