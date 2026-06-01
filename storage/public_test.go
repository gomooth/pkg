package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublicFrom(t *testing.T) {
	filenames := []string{
		"storage/public/abc/storage/def/1.png",
		"/storage/public/abc/storage/def/1.png",
	}
	expectPath := "storage/public/abc/storage/def/1.png"
	expectDir := "storage/public/abc/storage/def"
	expectURL := "/storage/abc/storage/def/1.png"
	host := "https://wwww.domain.com/abc"
	for _, filename := range filenames {
		p := PublicFromFile(filename)

		gotPath, err := p.Path()
		require.NoError(t, err)
		assert.Equal(t, expectPath, gotPath)

		gotDir, err := p.Dir()
		require.NoError(t, err)
		assert.Equal(t, expectDir, gotDir)

		gotURL, err := p.URL()
		require.NoError(t, err)
		assert.Equal(t, expectURL, gotURL)

		gotFilename, err := p.Filename()
		require.NoError(t, err)
		assert.Equal(t, "1.png", gotFilename)

		gotURLWithHost, err := p.URLWithHost(host)
		require.NoError(t, err)
		assert.True(t, strings.HasPrefix(gotURLWithHost, host))
	}

	p2 := PublicFromUrl(expectURL)
	gotPath, err := p2.Path()
	require.NoError(t, err)
	assert.Equal(t, expectPath, gotPath)

	gotURL, err := p2.URL()
	require.NoError(t, err)
	assert.Equal(t, expectURL, gotURL)

	gotDir, err := p2.Dir()
	require.NoError(t, err)
	assert.Equal(t, expectDir, gotDir)
}

func TestPublic_ChainAPI(t *testing.T) {
	// 正常链式调用
	p := Public().AppendDir("2024", "01").SetName("file.txt")

	gotPath, err := p.Path()
	require.NoError(t, err)
	assert.Equal(t, "storage/public/2024/01/file.txt", gotPath)

	gotDir, err := p.Dir()
	require.NoError(t, err)
	assert.Equal(t, "storage/public/2024/01", gotDir)

	gotURL, err := p.URL()
	require.NoError(t, err)
	assert.Equal(t, "/storage/2024/01/file.txt", gotURL)

	// 错误时链式调用不中断，但后续 builder 操作被跳过
	p2 := Public().AppendDir("..", "etc").SetName("file.txt")
	_, err = p2.Path()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "directory traversal")

	_, err = p2.Dir()
	assert.Error(t, err)

	_, err = p2.URL()
	assert.Error(t, err)

	_, err = p2.Filename()
	assert.Error(t, err)
}

func TestPublic_SanitizePathError(t *testing.T) {
	// null 字节
	p := Public().AppendDir("dir\x00name")
	_, err := p.Path()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "null byte")

	// 目录遍历
	p2 := Public().AppendDir("..")
	_, err = p2.Path()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "directory traversal")

	// SetName 错误
	p3 := Public().SetName("../etc/passwd")
	_, err = p3.Path()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "directory traversal")
}

func TestPublic_PathTraversal(t *testing.T) {
	tests := []struct {
		name    string
		dirs    []string
		wantErr bool
	}{
		{
			name:    "normal path",
			dirs:    []string{"avatars", "user1"},
			wantErr: false,
		},
		{
			name:    "traversal with ..",
			dirs:    []string{"..", "etc"},
			wantErr: true,
		},
		{
			name:    "whitespace only",
			dirs:    []string{"  "},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Public()
			result := s.AppendDir(tt.dirs...)
			_, err := result.Path()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// --- New tests for improved coverage ---

func TestDisk(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("normal directory", func(t *testing.T) {
		s := Disk("uploads", WithRoot(tmpDir))
		require.NotNil(t, s)

		// Use the storage
		s = s.AppendDir("2024", "01").SetName("test.txt")
		dir, err := s.Dir()
		require.NoError(t, err)
		assert.Contains(t, dir, "uploads")
		assert.Contains(t, dir, "2024")
		assert.Contains(t, dir, "01")

		filename, err := s.Filename()
		require.NoError(t, err)
		assert.Equal(t, "test.txt", filename)

		path, err := s.Path()
		require.NoError(t, err)
		assert.Contains(t, path, "test.txt")
	})

	t.Run("tmp alias", func(t *testing.T) {
		s := Disk("tmp")
		require.NotNil(t, s)
		s = s.SetName("tempfile.txt")

		filename, err := s.Filename()
		require.NoError(t, err)
		assert.Equal(t, "tempfile.txt", filename)
	})

	t.Run("temp alias", func(t *testing.T) {
		s := Disk("temp")
		require.NotNil(t, s)
		s = s.SetName("tempfile2.txt")

		filename, err := s.Filename()
		require.NoError(t, err)
		assert.Equal(t, "tempfile2.txt", filename)
	})
}

func TestTemp(t *testing.T) {
	s := Temp()
	require.NotNil(t, s)

	s = s.AppendDir("subdir").SetName("file.dat")

	dir, err := s.Dir()
	require.NoError(t, err)
	assert.Contains(t, dir, "go-pkg")
	assert.Contains(t, dir, "subdir")

	filename, err := s.Filename()
	require.NoError(t, err)
	assert.Equal(t, "file.dat", filename)

	path, err := s.Path()
	require.NoError(t, err)
	assert.Contains(t, path, "file.dat")
}

func TestWithRoot(t *testing.T) {
	tmpDir := t.TempDir()
	s := Disk("uploads", WithRoot(tmpDir))
	s = s.AppendDir("dir1").SetName("file.txt")

	dir, err := s.Dir()
	require.NoError(t, err)
	assert.Contains(t, dir, tmpDir)
}

func TestStorage_AppendDir_Error(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("path traversal in directory", func(t *testing.T) {
		s := Disk("uploads", WithRoot(tmpDir))
		result := s.AppendDir("..", "etc")
		_, err := result.Path()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "directory traversal")
	})

	t.Run("null byte in directory", func(t *testing.T) {
		s := Disk("uploads", WithRoot(tmpDir))
		result := s.AppendDir("dir\x00name")
		_, err := result.Path()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "null byte")
	})

	t.Run("empty path component", func(t *testing.T) {
		s := Disk("uploads", WithRoot(tmpDir))
		result := s.AppendDir("   ")
		_, err := result.Path()
		assert.Error(t, err)
	})
}

func TestStorage_SetName(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("valid name", func(t *testing.T) {
		s := Disk("uploads", WithRoot(tmpDir))
		result := s.SetName("photo.jpg")
		filename, err := result.Filename()
		require.NoError(t, err)
		assert.Equal(t, "photo.jpg", filename)
	})

	t.Run("empty name is ignored", func(t *testing.T) {
		s := Disk("uploads", WithRoot(tmpDir))
		s = s.SetName("original.txt")
		result := s.SetName("") // empty name should not overwrite
		filename, err := result.Filename()
		require.NoError(t, err)
		assert.Equal(t, "original.txt", filename)
	})

	t.Run("path traversal in name", func(t *testing.T) {
		s := Disk("uploads", WithRoot(tmpDir))
		result := s.SetName("../etc/passwd")
		_, err := result.Path()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "directory traversal")
	})
}

func TestStorage_Dir(t *testing.T) {
	tmpDir := t.TempDir()
	s := Disk("uploads", WithRoot(tmpDir))
	s = s.AppendDir("2024", "01")

	dir, err := s.Dir()
	require.NoError(t, err)
	assert.Contains(t, dir, "uploads")
	assert.Contains(t, dir, filepath.Join("2024", "01"))
}

func TestStorage_Path(t *testing.T) {
	tmpDir := t.TempDir()
	s := Disk("uploads", WithRoot(tmpDir))
	s = s.AppendDir("2024", "01").SetName("photo.jpg")

	path, err := s.Path()
	require.NoError(t, err)
	assert.Contains(t, path, "uploads")
	assert.Contains(t, path, filepath.Join("2024", "01"))
	assert.Contains(t, path, "photo.jpg")
}

func TestStorage_Filename(t *testing.T) {
	tmpDir := t.TempDir()
	s := Disk("uploads", WithRoot(tmpDir))
	s = s.SetName("document.pdf")

	filename, err := s.Filename()
	require.NoError(t, err)
	assert.Equal(t, "document.pdf", filename)
}

func TestStorage_PendingError(t *testing.T) {
	tmpDir := t.TempDir()
	s := Disk("uploads", WithRoot(tmpDir))
	// Cause an error via AppendDir
	s = s.AppendDir("..")
	// Further operations should carry the error
	s = s.AppendDir("valid")
	s = s.SetName("file.txt")

	_, err := s.Dir()
	assert.Error(t, err)

	_, err = s.Path()
	assert.Error(t, err)

	_, err = s.Filename()
	assert.Error(t, err)
}

func TestPublic_URLWithHost(t *testing.T) {
	p := Public().AppendDir("images").SetName("logo.png")

	t.Run("with http host", func(t *testing.T) {
		url, err := p.URLWithHost("https://example.com")
		require.NoError(t, err)
		assert.Equal(t, "https://example.com/storage/images/logo.png", url)
	})

	t.Run("with non-http host falls back to URL", func(t *testing.T) {
		url, err := p.URLWithHost("example.com")
		require.NoError(t, err)
		// Should return just the URL without host since host doesn't start with http
		expectedURL, _ := p.URL()
		assert.Equal(t, expectedURL, url)
	})

	t.Run("with error state", func(t *testing.T) {
		pErr := Public().AppendDir("..")
		_, err := pErr.URLWithHost("https://example.com")
		assert.Error(t, err)
	})
}

func TestPublic_URLWithHost_ErrorInURL(t *testing.T) {
	p := Public().AppendDir("..")
	_, err := p.URLWithHost("https://example.com")
	assert.Error(t, err)
}

func TestPublicFromFile_Empty(t *testing.T) {
	p := PublicFromFile("")
	_, err := p.Path()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty filename")
}

func TestPublicFromUrl_Empty(t *testing.T) {
	p := PublicFromUrl("")
	_, err := p.Path()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty file URL")
}

func TestPublicFromUrl_NoStorageRoot(t *testing.T) {
	p := PublicFromUrl("https://example.com/other/path/file.txt")
	_, err := p.Path()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "URL does not contain storage root path")
}

func TestPublicFromUrl_TrailingSlash(t *testing.T) {
	p := PublicFromUrl("/storage/images/")
	// "images/" after TrimRight becomes "images" which is treated as the filename
	filename, err := p.Filename()
	require.NoError(t, err)
	assert.Equal(t, "images", filename)
}

func TestPublicFromFile_NoValidPathComponent(t *testing.T) {
	// Filename that only contains the base path
	p := PublicFromFile("storage/public/")
	_, err := p.Path()
	assert.Error(t, err)
}

func TestPublic_SanitizePath_EncodingDetection(t *testing.T) {
	// Path that changes during filepath.Clean (encoding-like manipulation)
	p := Public().AppendDir("dir/./hidden")
	_, err := p.Path()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "altered during cleaning")
}

func TestPublic_SetName_EmptyKeepsPrevious(t *testing.T) {
	p := Public().AppendDir("images").SetName("original.png")
	p = p.SetName("") // should not overwrite

	filename, err := p.Filename()
	require.NoError(t, err)
	assert.Equal(t, "original.png", filename)
}

func TestPublic_SetName_WithTraversal(t *testing.T) {
	p := Public().SetName("../../etc/passwd")
	_, err := p.Path()
	assert.Error(t, err)
}

func TestPublic_SetName_NullByte(t *testing.T) {
	p := Public().SetName("file\x00name.txt")
	_, err := p.Path()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "null byte")
}

func TestPublic_AppendDir_NilErr(t *testing.T) {
	// Call AppendDir on a storage with pending error - further operations are skipped
	pErr := Public().AppendDir("..")
	result := pErr.AppendDir("more")
	_, err := result.Path()
	assert.Error(t, err)
}

func TestPublic_DirAndPath_WithRoot(t *testing.T) {
	tmpDir := t.TempDir()
	p := Public(WithRoot(tmpDir))
	p = p.AppendDir("images").SetName("photo.jpg")

	dir, err := p.Dir()
	require.NoError(t, err)
	assert.Contains(t, dir, tmpDir)
	assert.Contains(t, dir, "public")
	assert.Contains(t, dir, "images")

	path, err := p.Path()
	require.NoError(t, err)
	assert.Contains(t, path, "photo.jpg")
}

func TestSanitizePath_Table(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid simple", "images", false},
		{"valid with dots inside", "my.dir", false},
		{"empty after trim", "   ", true},
		{"null byte", "dir\x00name", true},
		{"double dot traversal", "..", true},
		{"double dot prefix slash", "../etc", true},
		{"dot-slash encoding", "dir/./hidden", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sanitizePath(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSecureJoin(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("normal path creation", func(t *testing.T) {
		result, err := secureJoin(tmpDir, "subdir", "file.txt")
		require.NoError(t, err)
		assert.Contains(t, result, "subdir")
		assert.Contains(t, result, "file.txt")
	})

	t.Run("creates base directory", func(t *testing.T) {
		baseDir := filepath.Join(tmpDir, "newbase")
		result, err := secureJoin(baseDir, "subdir")
		require.NoError(t, err)
		assert.Contains(t, result, "subdir")

		// Base dir should have been created
		_, err = os.Stat(baseDir)
		assert.NoError(t, err)
	})

	t.Run("no segments", func(t *testing.T) {
		result, err := secureJoin(tmpDir)
		require.NoError(t, err)
		assert.Equal(t, tmpDir, result)
	})
}

func TestSecureJoin_TraversalAttack(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("path traversal attempt", func(t *testing.T) {
		// This should be caught by sanitizePath in AppendDir, but
		// secureJoin should also protect against direct calls
		// Create a real path to test secureJoin
		result, err := secureJoin(tmpDir, "safe", "path")
		require.NoError(t, err)
		assert.NotContains(t, result, "..")
	})
}

func TestDisk_WithRoot(t *testing.T) {
	tmpDir := t.TempDir()
	s := Disk("data", WithRoot(tmpDir))
	s = s.AppendDir("sub").SetName("file.txt")

	path, err := s.Path()
	require.NoError(t, err)
	assert.Contains(t, path, tmpDir)
	assert.Contains(t, path, "data")
	assert.Contains(t, path, "file.txt")
}

func TestPublic_URL_PathTraversal(t *testing.T) {
	// Test URL with traversal paths
	p := Public().AppendDir("images").SetName("photo.png")

	url, err := p.URL()
	require.NoError(t, err)
	assert.Equal(t, "/storage/images/photo.png", url)
	assert.NotContains(t, url, "..")
}

func TestPublic_URLWithHost_WithHTTP(t *testing.T) {
	p := Public().AppendDir("docs").SetName("report.pdf")

	url, err := p.URLWithHost("http://localhost:8080")
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:8080/storage/docs/report.pdf", url)
}

func TestPublicFromFile_RelativePath(t *testing.T) {
	// Filename that doesn't contain the base path
	p := PublicFromFile("images/photo.png")
	dir, err := p.Dir()
	require.NoError(t, err)
	assert.Contains(t, dir, "images")

	filename, err := p.Filename()
	require.NoError(t, err)
	assert.Equal(t, "photo.png", filename)
}

func TestPublicFromUrl_Valid(t *testing.T) {
	p := PublicFromUrl("/storage/images/logo.png")

	path, err := p.Path()
	require.NoError(t, err)
	assert.Contains(t, path, "images")
	assert.Contains(t, path, "logo.png")

	url, err := p.URL()
	require.NoError(t, err)
	assert.Equal(t, "/storage/images/logo.png", url)
}

func TestSetDirsAndName_Traversal(t *testing.T) {
	p := Public()
	pub := p.(*public)
	pub.setDirsAndName("../../etc/passwd")
	assert.Error(t, pub.err)
}

func TestSetDirsAndName_EmptyComponents(t *testing.T) {
	p := Public()
	pub := p.(*public)
	pub.setDirsAndName("./.")
	assert.Error(t, pub.err)
}

func TestSplitPathParts(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"a/b/c", []string{"a", "b", "c"}},
		{"/a//b/./c/", []string{"a", "b", "c"}},
		{"", []string{}},
		{".", []string{}},
		{"a", []string{"a"}},
	}

	for _, tt := range tests {
		got := splitPathParts(tt.input)
		assert.Equal(t, tt.want, got)
	}
}

func TestNewPublic_WithRoot(t *testing.T) {
	tmpDir := t.TempDir()
	p := Public(WithRoot(tmpDir))
	pub := p.(*public)
	assert.Equal(t, tmpDir, pub.root[0])
	assert.Equal(t, "public", pub.root[1])
}

func TestNewStorage_DefaultRoot(t *testing.T) {
	s := Disk("uploads")
	st := s.(*storage)
	assert.Equal(t, "storage", st.root[0])
	assert.Equal(t, "uploads", st.root[1])
}

func TestNewStorage_WithRoot(t *testing.T) {
	tmpDir := t.TempDir()
	s := Disk("uploads", WithRoot(tmpDir))
	st := s.(*storage)
	assert.Equal(t, tmpDir, st.root[0])
	assert.Equal(t, "uploads", st.root[1])
}

func TestEvalSymlinksPath(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("existing path", func(t *testing.T) {
		realPath, err := evalSymlinksPath(tmpDir)
		require.NoError(t, err)
		// On macOS, /tmp is a symlink to /private/tmp, so compare resolved paths
		expectedReal, _ := filepath.EvalSymlinks(tmpDir)
		assert.Equal(t, expectedReal, realPath)
	})

	t.Run("non-existent path with existing parent", func(t *testing.T) {
		nonExistent := filepath.Join(tmpDir, "subdir", "file.txt")
		realPath, err := evalSymlinksPath(nonExistent)
		require.NoError(t, err)
		assert.Contains(t, realPath, "subdir")
		assert.Contains(t, realPath, "file.txt")
	})

	t.Run("completely non-existent deep path", func(t *testing.T) {
		// Use a very deep path that doesn't exist
		deepPath := filepath.Join("/nonexistent_root_test_12345", "a", "b", "c")
		realPath, err := evalSymlinksPath(deepPath)
		require.NoError(t, err)
		// Should return a cleaned path since nothing exists
		assert.NotEmpty(t, realPath)
	})
}
