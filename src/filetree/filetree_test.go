package filetree

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeserialize(t *testing.T) {
	dbJSON := `{
        "name": "root",
        "is_dir": true,
        "children": [
            {
                "name": "docs",
                "is_dir": true,
                "children": [
                    {"name": "readme.md", "is_dir": false, "file_id": 1001}
                ]
            },
            {
                "name": "src",
                "is_dir": true,
                "children": [
                    {"name": "main.go", "is_dir": false, "file_id": 1002}
                ]
            }
        ]
    }`

	tree, err := Deserialize(dbJSON, 20)
	require.NoError(t, err, "deserialization failed")
	assert.Len(t, tree.Children, 2, "root should have 2 children")
}

func TestAddFile(t *testing.T) {
	tree := &FileNode{
		Name:  "root",
		IsDir: true,
		Children: []*FileNode{
			{
				Name:     "src",
				IsDir:    true,
				Children: []*FileNode{},
			},
		},
	}

	err := tree.AddFile("src", "utils.go", 1003)
	require.NoError(t, err, "failed to add file")

	node := tree.FindNode("src/utils.go")
	require.NotNil(t, node, "cannot find newly added file")
	assert.Equal(t, uint64(1003), node.FileID, "FileID should be 1003")
}

func TestAddDir(t *testing.T) {
	tree := &FileNode{
		Name:  "root",
		IsDir: true,
		Children: []*FileNode{
			{
				Name:     "src",
				IsDir:    true,
				Children: []*FileNode{},
			},
		},
	}
	require.NoError(t, tree.SetMaxDepth(20))

	err := tree.AddDir("src", "pkg")
	require.NoError(t, err, "failed to add directory")

	node := tree.FindNode("src/pkg")
	require.NotNil(t, node, "cannot find newly added directory")
	assert.True(t, node.IsDir, "should be a directory")
}

func TestAddDirDepthLimit(t *testing.T) {
	t.Run("default max depth is 20", func(t *testing.T) {
		tree := &FileNode{Name: "root", IsDir: true, Children: []*FileNode{}}
		require.NoError(t, tree.SetMaxDepth(20))

		parent := "/"
		for i := 1; i <= 20; i++ {
			dirName := "d" + strconv.Itoa(i)
			require.NoError(t, tree.AddDir(parent, dirName))
			if parent == "/" {
				parent = "/" + dirName
			} else {
				parent = parent + "/" + dirName
			}
		}

		err := tree.AddDir(parent, "overflow")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "directory depth exceeds maximum: 20")
	})

	t.Run("max depth can be configured", func(t *testing.T) {
		tree := &FileNode{Name: "root", IsDir: true, Children: []*FileNode{}}
		require.NoError(t, tree.SetMaxDepth(3))

		require.NoError(t, tree.AddDir("/", "a"))
		require.NoError(t, tree.AddDir("/a", "b"))
		require.NoError(t, tree.AddDir("/a/b", "c"))

		err := tree.AddDir("/a/b/c", "d")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "directory depth exceeds maximum: 3")
	})
}

func TestRemoveNode(t *testing.T) {
	tests := []struct {
		name         string
		tree         *FileNode
		pathToRemove string
		wantErr      bool
		errContains  string
		verifyFunc   func(t *testing.T, tree *FileNode)
	}{
		{
			name: "remove file node",
			tree: &FileNode{
				Name:  "root",
				IsDir: true,
				Children: []*FileNode{
					{
						Name:  "docs",
						IsDir: true,
						Children: []*FileNode{
							{Name: "readme.md", IsDir: false, FileID: 1001},
						},
					},
				},
			},
			pathToRemove: "docs/readme.md",
			wantErr:      false,
			verifyFunc: func(t *testing.T, tree *FileNode) {
				node := tree.FindNode("docs/readme.md")
				assert.Nil(t, node, "file node should be removed")
				docsNode := tree.FindNode("docs")
				require.NotNil(t, docsNode, "parent directory should not be removed")
				assert.Empty(t, docsNode.Children, "parent directory should be empty")
			},
		},
		{
			name: "remove empty directory",
			tree: &FileNode{
				Name:  "root",
				IsDir: true,
				Children: []*FileNode{
					{
						Name:     "empty_dir",
						IsDir:    true,
						Children: []*FileNode{},
					},
					{
						Name:  "src",
						IsDir: true,
						Children: []*FileNode{
							{Name: "main.go", IsDir: false, FileID: 1002},
						},
					},
				},
			},
			pathToRemove: "empty_dir",
			wantErr:      false,
			verifyFunc: func(t *testing.T, tree *FileNode) {
				node := tree.FindNode("empty_dir")
				assert.Nil(t, node, "empty directory should be removed")
				assert.Len(t, tree.Children, 1, "root should have 1 child")
				srcNode := tree.FindNode("src")
				assert.NotNil(t, srcNode, "other directories should not be affected")
			},
		},
		{
			name: "remove directory with children",
			tree: &FileNode{
				Name:  "root",
				IsDir: true,
				Children: []*FileNode{
					{
						Name:  "docs",
						IsDir: true,
						Children: []*FileNode{
							{Name: "readme.md", IsDir: false, FileID: 1001},
							{Name: "guide.md", IsDir: false, FileID: 1002},
							{
								Name:  "api",
								IsDir: true,
								Children: []*FileNode{
									{Name: "spec.md", IsDir: false, FileID: 1003},
								},
							},
						},
					},
				},
			},
			pathToRemove: "docs",
			wantErr:      false,
			verifyFunc: func(t *testing.T, tree *FileNode) {
				node := tree.FindNode("docs")
				assert.Nil(t, node, "directory and all children should be removed")
				assert.Empty(t, tree.Children, "root should be empty")
			},
		},
		{
			name: "remove non-existent node",
			tree: &FileNode{
				Name:  "root",
				IsDir: true,
				Children: []*FileNode{
					{
						Name:     "src",
						IsDir:    true,
						Children: []*FileNode{},
					},
				},
			},
			pathToRemove: "src/nonexistent.go",
			wantErr:      true,
			errContains:  "node does not exist",
		},
		{
			name: "remove root directory",
			tree: &FileNode{
				Name:     "root",
				IsDir:    true,
				Children: []*FileNode{},
			},
			pathToRemove: "/",
			wantErr:      true,
			errContains:  "cannot remove root directory",
		},
		{
			name: "remove empty path",
			tree: &FileNode{
				Name:     "root",
				IsDir:    true,
				Children: []*FileNode{},
			},
			pathToRemove: "",
			wantErr:      true,
			errContains:  "cannot remove root directory",
		},
		{
			name: "remove one of multiple children",
			tree: &FileNode{
				Name:  "root",
				IsDir: true,
				Children: []*FileNode{
					{
						Name:  "src",
						IsDir: true,
						Children: []*FileNode{
							{Name: "main.go", IsDir: false, FileID: 1001},
							{Name: "utils.go", IsDir: false, FileID: 1002},
							{Name: "config.go", IsDir: false, FileID: 1003},
						},
					},
				},
			},
			pathToRemove: "src/utils.go",
			wantErr:      false,
			verifyFunc: func(t *testing.T, tree *FileNode) {
				node := tree.FindNode("src/utils.go")
				assert.Nil(t, node, "utils.go should be removed")
				srcNode := tree.FindNode("src")
				require.NotNil(t, srcNode, "parent directory should not be removed")
				assert.Len(t, srcNode.Children, 2, "src directory should have 2 children")
				assert.NotNil(t, tree.FindNode("src/main.go"), "main.go should not be removed")
				assert.NotNil(t, tree.FindNode("src/config.go"), "config.go should not be removed")
			},
		},
		{
			name: "remove deeply nested node",
			tree: &FileNode{
				Name:  "root",
				IsDir: true,
				Children: []*FileNode{
					{
						Name:  "a",
						IsDir: true,
						Children: []*FileNode{
							{
								Name:  "b",
								IsDir: true,
								Children: []*FileNode{
									{
										Name:  "c",
										IsDir: true,
										Children: []*FileNode{
											{Name: "deep.txt", IsDir: false, FileID: 9999},
										},
									},
								},
							},
						},
					},
				},
			},
			pathToRemove: "a/b/c/deep.txt",
			wantErr:      false,
			verifyFunc: func(t *testing.T, tree *FileNode) {
				node := tree.FindNode("a/b/c/deep.txt")
				assert.Nil(t, node, "deep file should be removed")
				cNode := tree.FindNode("a/b/c")
				require.NotNil(t, cNode, "parent directory c should not be removed")
				assert.Empty(t, cNode.Children, "directory c should be empty")
			},
		},
		{
			name: "remove only child",
			tree: &FileNode{
				Name:  "root",
				IsDir: true,
				Children: []*FileNode{
					{
						Name:  "docs",
						IsDir: true,
						Children: []*FileNode{
							{Name: "only.md", IsDir: false, FileID: 5555},
						},
					},
				},
			},
			pathToRemove: "docs/only.md",
			wantErr:      false,
			verifyFunc: func(t *testing.T, tree *FileNode) {
				docsNode := tree.FindNode("docs")
				require.NotNil(t, docsNode, "parent directory should not be removed")
				assert.Empty(t, docsNode.Children, "docs directory should be empty")
			},
		},
		{
			name: "parent directory does not exist",
			tree: &FileNode{
				Name:  "root",
				IsDir: true,
				Children: []*FileNode{
					{
						Name:     "src",
						IsDir:    true,
						Children: []*FileNode{},
					},
				},
			},
			pathToRemove: "nonexistent/file.txt",
			wantErr:      true,
			errContains:  "parent directory does not exist",
		},
		{
			name: "remove first child",
			tree: &FileNode{
				Name:  "root",
				IsDir: true,
				Children: []*FileNode{
					{
						Name:  "src",
						IsDir: true,
						Children: []*FileNode{
							{Name: "a.go", IsDir: false, FileID: 1},
							{Name: "b.go", IsDir: false, FileID: 2},
							{Name: "c.go", IsDir: false, FileID: 3},
						},
					},
				},
			},
			pathToRemove: "src/a.go",
			wantErr:      false,
			verifyFunc: func(t *testing.T, tree *FileNode) {
				srcNode := tree.FindNode("src")
				assert.Len(t, srcNode.Children, 2, "should have 2 children remaining")
				assert.Equal(t, "b.go", srcNode.Children[0].Name, "first child should be b.go")
			},
		},
		{
			name: "remove last child",
			tree: &FileNode{
				Name:  "root",
				IsDir: true,
				Children: []*FileNode{
					{
						Name:  "src",
						IsDir: true,
						Children: []*FileNode{
							{Name: "a.go", IsDir: false, FileID: 1},
							{Name: "b.go", IsDir: false, FileID: 2},
							{Name: "c.go", IsDir: false, FileID: 3},
						},
					},
				},
			},
			pathToRemove: "src/c.go",
			wantErr:      false,
			verifyFunc: func(t *testing.T, tree *FileNode) {
				srcNode := tree.FindNode("src")
				assert.Len(t, srcNode.Children, 2, "should have 2 children remaining")
				assert.Equal(t, "b.go", srcNode.Children[len(srcNode.Children)-1].Name, "last child should be b.go")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.tree.RemoveNode(tt.pathToRemove)

			if tt.wantErr {
				assert.Error(t, err, "expected an error")
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains, "error message should contain expected text")
				}
			} else {
				assert.NoError(t, err, "expected no error")
				if tt.verifyFunc != nil {
					tt.verifyFunc(t, tt.tree)
				}
			}
		})
	}
}

func TestRemoveNodeTwice(t *testing.T) {
	tree := &FileNode{
		Name:  "root",
		IsDir: true,
		Children: []*FileNode{
			{
				Name:  "docs",
				IsDir: true,
				Children: []*FileNode{
					{Name: "readme.md", IsDir: false, FileID: 1001},
				},
			},
		},
	}

	// First removal should succeed
	err := tree.RemoveNode("docs/readme.md")
	require.NoError(t, err, "first removal failed")

	// Second removal of the same node should fail
	err = tree.RemoveNode("docs/readme.md")
	assert.Error(t, err, "second removal should fail")
	assert.Contains(t, err.Error(), "node does not exist", "error message should contain 'node does not exist'")
}

func TestRenameNode(t *testing.T) {
	tree := &FileNode{
		Name:  "root",
		IsDir: true,
		Children: []*FileNode{
			{
				Name:     "src",
				IsDir:    true,
				Children: []*FileNode{},
			},
		},
	}

	err := tree.AddFile("src", "main.go", 1002)
	require.NoError(t, err, "failed to add file")

	err = tree.RenameNode("src/main.go", "main_test.go")
	require.NoError(t, err, "failed to rename")

	node := tree.FindNode("src/main_test.go")
	assert.NotNil(t, node, "cannot find renamed file")

	oldNode := tree.FindNode("src/main.go")
	assert.Nil(t, oldNode, "old name node should not exist")
}

func TestMoveNode(t *testing.T) {
	tests := []struct {
		name        string
		tree        *FileNode
		sourcePath  string
		targetPath  string
		wantErr     bool
		errContains string
		verifyFunc  func(t *testing.T, tree *FileNode)
	}{
		{
			name: "move file to another directory",
			tree: &FileNode{
				Name:  "root",
				IsDir: true,
				Children: []*FileNode{
					{
						Name:  "docs",
						IsDir: true,
						Children: []*FileNode{
							{Name: "readme.md", IsDir: false, FileID: 1001},
						},
					},
					{
						Name:     "archive",
						IsDir:    true,
						Children: []*FileNode{},
					},
				},
			},
			sourcePath: "docs/readme.md",
			targetPath: "archive",
			verifyFunc: func(t *testing.T, tree *FileNode) {
				assert.Nil(t, tree.FindNode("docs/readme.md"), "source path should no longer exist")
				moved := tree.FindNode("archive/readme.md")
				require.NotNil(t, moved, "moved file should exist in target")
				assert.Equal(t, uint64(1001), moved.FileID, "file ID should be preserved")
			},
		},
		{
			name: "move directory with children",
			tree: &FileNode{
				Name:  "root",
				IsDir: true,
				Children: []*FileNode{
					{
						Name:  "src",
						IsDir: true,
						Children: []*FileNode{
							{Name: "main.go", IsDir: false, FileID: 1002},
						},
					},
					{
						Name:     "backup",
						IsDir:    true,
						Children: []*FileNode{},
					},
				},
			},
			sourcePath: "src",
			targetPath: "backup",
			verifyFunc: func(t *testing.T, tree *FileNode) {
				assert.Nil(t, tree.FindNode("src"), "old directory path should not exist")
				movedDir := tree.FindNode("backup/src")
				require.NotNil(t, movedDir, "moved directory should exist")
				assert.NotNil(t, tree.FindNode("backup/src/main.go"), "nested children should move together")
			},
		},
		{
			name: "move root should fail",
			tree: &FileNode{
				Name:     "root",
				IsDir:    true,
				Children: []*FileNode{},
			},
			sourcePath:  "/",
			targetPath:  "docs",
			wantErr:     true,
			errContains: "cannot move root directory",
		},
		{
			name: "target directory does not exist",
			tree: &FileNode{
				Name:  "root",
				IsDir: true,
				Children: []*FileNode{
					{
						Name:  "docs",
						IsDir: true,
						Children: []*FileNode{
							{Name: "readme.md", IsDir: false, FileID: 1001},
						},
					},
				},
			},
			sourcePath:  "docs/readme.md",
			targetPath:  "missing",
			wantErr:     true,
			errContains: "directory does not exist",
		},
		{
			name: "move directory into itself should fail",
			tree: &FileNode{
				Name:  "root",
				IsDir: true,
				Children: []*FileNode{
					{
						Name:  "src",
						IsDir: true,
						Children: []*FileNode{
							{
								Name:     "pkg",
								IsDir:    true,
								Children: []*FileNode{},
							},
						},
					},
				},
			},
			sourcePath:  "src",
			targetPath:  "src/pkg",
			wantErr:     true,
			errContains: "cannot move directory into itself",
		},
		{
			name: "name conflict in target directory",
			tree: &FileNode{
				Name:  "root",
				IsDir: true,
				Children: []*FileNode{
					{
						Name:  "docs",
						IsDir: true,
						Children: []*FileNode{
							{Name: "readme.md", IsDir: false, FileID: 1001},
						},
					},
					{
						Name:  "archive",
						IsDir: true,
						Children: []*FileNode{
							{Name: "readme.md", IsDir: false, FileID: 2001},
						},
					},
				},
			},
			sourcePath:  "docs/readme.md",
			targetPath:  "archive",
			wantErr:     true,
			errContains: "node already exists in target directory",
		},
		{
			name: "move to same directory is no-op",
			tree: &FileNode{
				Name:  "root",
				IsDir: true,
				Children: []*FileNode{
					{
						Name:  "docs",
						IsDir: true,
						Children: []*FileNode{
							{Name: "readme.md", IsDir: false, FileID: 1001},
						},
					},
				},
			},
			sourcePath: "docs/readme.md",
			targetPath: "docs",
			verifyFunc: func(t *testing.T, tree *FileNode) {
				node := tree.FindNode("docs/readme.md")
				require.NotNil(t, node, "file should remain in same directory")
				docs := tree.FindNode("docs")
				require.NotNil(t, docs, "docs should exist")
				assert.Len(t, docs.Children, 1, "should not duplicate children")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, tt.tree.SetMaxDepth(20))
			err := tt.tree.MoveNode(tt.sourcePath, tt.targetPath)

			if tt.wantErr {
				assert.Error(t, err, "expected an error")
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains, "error message should contain expected text")
				}
				return
			}

			require.NoError(t, err, "expected no error")
			if tt.verifyFunc != nil {
				tt.verifyFunc(t, tt.tree)
			}
		})
	}
}

func TestMoveNodeRespectsDepthLimit(t *testing.T) {
	tree := &FileNode{
		Name:  "root",
		IsDir: true,
		Children: []*FileNode{
			{
				Name:  "a",
				IsDir: true,
				Children: []*FileNode{
					{Name: "b", IsDir: true, Children: []*FileNode{}},
				},
			},
			{
				Name:     "target",
				IsDir:    true,
				Children: []*FileNode{},
			},
		},
	}
	require.NoError(t, tree.SetMaxDepth(2))

	err := tree.MoveNode("a", "target")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "directory depth exceeds maximum: 2")
}

func TestGetFileID(t *testing.T) {
	tree := &FileNode{
		Name:  "root",
		IsDir: true,
		Children: []*FileNode{
			{
				Name:  "src",
				IsDir: true,
				Children: []*FileNode{
					{Name: "utils.go", IsDir: false, FileID: 1003},
				},
			},
		},
	}

	fileID, err := tree.GetFileID("src/utils.go")
	require.NoError(t, err, "failed to get FileID")
	assert.Equal(t, uint64(1003), fileID, "FileID should be 1003")
}

func TestGetAllFiles(t *testing.T) {
	tree := &FileNode{
		Name:  "root",
		IsDir: true,
		Children: []*FileNode{
			{
				Name:  "docs",
				IsDir: true,
				Children: []*FileNode{
					{Name: "readme.md", IsDir: false, FileID: 1001},
				},
			},
			{
				Name:  "src",
				IsDir: true,
				Children: []*FileNode{
					{Name: "main.go", IsDir: false, FileID: 1002},
					{Name: "utils.go", IsDir: false, FileID: 1003},
				},
			},
		},
	}

	files := tree.GetAllFiles()
	assert.Len(t, files, 3, "should have 3 files")
}

func TestGetAllDirs(t *testing.T) {
	tree := &FileNode{
		Name:  "root",
		IsDir: true,
		Children: []*FileNode{
			{
				Name:     "docs",
				IsDir:    true,
				Children: []*FileNode{},
			},
			{
				Name:  "src",
				IsDir: true,
				Children: []*FileNode{
					{Name: "pkg", IsDir: true, Children: []*FileNode{}},
				},
			},
		},
	}

	dirs := tree.GetAllDirs()
	assert.Len(t, dirs, 3, "should have 3 directories")
}

func TestSerialize(t *testing.T) {
	tree := &FileNode{
		Name:  "root",
		IsDir: true,
		Children: []*FileNode{
			{
				Name:     "src",
				IsDir:    true,
				Children: []*FileNode{},
			},
		},
	}

	result, err := tree.Serialize()
	require.NoError(t, err, "serialization failed")
	assert.NotEmpty(t, result, "serialization result should not be empty")
}

func TestFindNode(t *testing.T) {
	tree := &FileNode{
		Name:  "root",
		IsDir: true,
		Children: []*FileNode{
			{
				Name:  "src",
				IsDir: true,
				Children: []*FileNode{
					{Name: "main.go", IsDir: false, FileID: 1002},
				},
			},
		},
	}

	node := tree.FindNode("src/main.go")
	require.NotNil(t, node, "cannot find node")
	assert.Equal(t, "main.go", node.Name, "node name should be main.go")
}

func TestAddFileToNonExistentDir(t *testing.T) {
	tree := &FileNode{
		Name:     "root",
		IsDir:    true,
		Children: []*FileNode{},
	}

	err := tree.AddFile("nonexistent", "file.txt", 1001)
	assert.Error(t, err, "adding file to non-existent directory should fail")
}

func TestRemoveRootDir(t *testing.T) {
	tree := &FileNode{
		Name:     "root",
		IsDir:    true,
		Children: []*FileNode{},
	}

	err := tree.RemoveNode("/")
	assert.Error(t, err, "removing root directory should fail")
}
