package storage

import (
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
