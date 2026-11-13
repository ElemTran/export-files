package exporter

import "testing"

func TestLanguage(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		// 原 bash 版覆盖的扩展名
		{"a.py", "python"},
		{"a.js", "javascript"},
		{"a.ts", "typescript"},
		{"a.tsx", "tsx"},
		{"a.jsx", "jsx"},
		{"a.vue", "vue"},
		{"A.java", "java"},
		{"a.cpp", "cpp"},
		{"a.cc", "cpp"},
		{"a.cxx", "cpp"},
		{"a.c", "c"},
		{"a.h", "cpp"},
		{"a.hpp", "cpp"},
		{"a.go", "go"},
		{"a.rs", "rust"},
		{"a.sh", "bash"},
		{"a.bash", "bash"},
		{"a.md", "markdown"},
		{"a.json", "json"},
		{"a.yaml", "yaml"},
		{"a.yml", "yaml"},
		{"a.html", "html"},
		{"a.htm", "html"},
		{"a.css", "css"},
		{"a.xml", "xml"},
		{"a.sql", "sql"},
		{"a.php", "php"},
		{"a.rb", "ruby"},

		// 补充的扩展名
		{"a.kt", "kotlin"},
		{"a.kts", "kotlin"},
		{"a.swift", "swift"},
		{"a.cs", "csharp"},
		{"a.dart", "dart"},
		{"a.proto", "protobuf"},
		{"a.graphql", "graphql"},
		{"a.gql", "graphql"},
		{"a.toml", "toml"},
		{"a.ini", "ini"},
		{"a.cfg", "ini"},
		{"a.conf", "ini"},
		{".env", "ini"},
		{"prod.env", "ini"},
		{"a.tf", "hcl"},
		{"a.tfvars", "hcl"},
		{"a.scss", "scss"},
		{"a.sass", "sass"},
		{"a.less", "less"},
		{"a.ps1", "powershell"},
		{"build.mk", "makefile"},
		{"a.cmake", "cmake"},

		// 按完整文件名判定
		{"Dockerfile", "dockerfile"},
		{"dockerfile.dev", "dockerfile"},
		{"app.dockerfile", "dockerfile"},
		{"Makefile", "makefile"},
		{"GNUmakefile", "makefile"},
		{"CMakeLists.txt", "cmake"},
		{"docker-compose.yml", "yaml"},
		{"docker-compose.override.yaml", "yaml"},
		{"compose.yaml", "yaml"},

		// 目录路径与大小写
		{"internal/exporter/language.go", "go"},
		{"dir/MAIN.JAVA", "java"},

		// 无扩展名 / 未知扩展名
		{"README", ""},
		{"LICENSE", ""},
		{"a.unknownext", ""},
	}
	for _, tt := range tests {
		if got := Language(tt.name); got != tt.want {
			t.Errorf("Language(%q) = %q, 期望 %q", tt.name, got, tt.want)
		}
	}
}
