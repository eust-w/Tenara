package detectors

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if mkdirErr := os.MkdirAll(filepath.Dir(full), 0o755); mkdirErr != nil {
			t.Fatal(mkdirErr)
		}
		if writeErr := os.WriteFile(full, []byte(content), 0o644); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	return root
}

func TestDetectServiceTable(t *testing.T) {
	cases := []struct {
		name      string
		files     map[string]string
		framework string
		runtime   string
		port      int
	}{
		{
			name:      "next project",
			files:     map[string]string{"package.json": `{"dependencies":{"next":"14.2.0"}}`},
			framework: "next",
			runtime:   "node",
			port:      3000,
		},
		{
			name:      "fastapi requirements",
			files:     map[string]string{"requirements.txt": "fastapi==0.100\nuvicorn\n"},
			framework: "fastapi",
			runtime:   "python",
			port:      8000,
		},
		{
			name:      "go module defaults to 8080",
			files:     map[string]string{"go.mod": "module example\n", "main.go": "package main\n"},
			framework: "",
			runtime:   "go",
			port:      8080,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := writeTree(t, tc.files)
			got := DetectService(root)
			if got.Framework != tc.framework || got.Runtime != tc.runtime || got.Port != tc.port {
				t.Fatalf("got %+v, want %s/%s/%d",
					got, tc.framework, tc.runtime, tc.port)
			}
		})
	}
}

func TestNuxtConfigBelongsToVueFamily(t *testing.T) {
	root := writeTree(t, map[string]string{
		"package.json":   `{"dependencies":{}}`,
		"nuxt.config.ts": "export default {}\n",
	})
	got := DetectService(root)
	if got.Framework != "nuxt" {
		t.Fatalf("framework = %q, want nuxt", got.Framework)
	}
}

func TestDockerfileExposeOverrides(t *testing.T) {
	root := writeTree(t, map[string]string{
		"package.json": `{"dependencies":{"next":"14"}}`,
		"Dockerfile":   "FROM node\nEXPOSE 9000\n",
	})
	got := DetectService(root)
	if got.Port != 9000 {
		t.Fatalf("port = %d, want 9000", got.Port)
	}
}

func TestMongoThreeWayDetection(t *testing.T) {
	t.Run("compose mongo service", func(t *testing.T) {
		root := writeTree(t, map[string]string{
			"docker-compose.yml": "services:\n  db:\n    image: mongo:7\n",
		})
		got := DetectMongoDB(root)
		if !got.Found {
			t.Fatal("compose mongo missed")
		}
	})

	t.Run("mongoose dependency in nested manifest", func(t *testing.T) {
		root := writeTree(t, map[string]string{
			"apps/api/package.json": `{"dependencies":{"mongoose":"8"}}`,
		})
		got := DetectMongoDB(root)
		if !got.Found {
			t.Fatal("mongoose dep missed")
		}
	})

	t.Run("env MONGODB_URI alone hits", func(t *testing.T) {
		root := writeTree(t, map[string]string{
			".env.example": "MONGODB_URI=mongodb://localhost/test\n",
		})
		got := DetectMongoDB(root)
		if !got.Found {
			t.Fatal("env MONGODB_URI missed")
		}
	})

	t.Run("negative clean repo", func(t *testing.T) {
		root := t.TempDir()
		got := DetectMongoDB(root)
		if got.Found {
			t.Fatal("false positive on clean repo")
		}
	})
}
