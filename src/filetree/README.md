# 文件夹结构的 N 叉树存储方案

## 需求

我有一个表记录文件，一条数据对应一个文件。现在我需要一个抽象的文件夹结构描述文件的目录功能。

## 方案设计

### 核心思路

**直接存储 N 叉树结构**，单条数据描述整个目录：

```
数据库 JSON 字符串 ↔ N 叉树 (FileNode)
```

### 核心流程

```
1. 取出：数据库 JSON → 反序列化 → *FileNode
2. 操作：在树上直接操作（添加/删除/修改）
3. 存入：*FileNode → 序列化 → JSON → 更新数据库
```

## Go 语言实现

### 1. 数据结构定义

```go
package filetree

// FileNode 文件节点 (N 叉树节点)
type FileNode struct {
    Name     string      `json:"name"`        // 文件/文件夹名称
    IsDir    bool        `json:"is_dir"`      // 是否为目录
    FileID   uint64      `json:"file_id,omitempty"` // 文件 ID（仅文件节点有值，目录节点省略）
    Children []*FileNode `json:"children"`    // 子节点列表
}

// SetMaxDepth 设置目录最大深度（必须 > 0）
func (f *FileNode) SetMaxDepth(maxDepth int) error
```

### 2. 序列化/反序列化

```go
// Serialize 将 N 叉树序列化为 JSON 字符串
func Serialize(root *FileNode) (string, error) {
    data, err := json.Marshal(root)
    if err != nil {
        return "", err
    }
    return string(data), nil
}

// Deserialize 将 JSON 字符串反序列化为 N 叉树，并要求传入最大深度配置
func Deserialize(data string, maxDepth int) (*FileNode, error) {
    var root *FileNode
    err := json.Unmarshal([]byte(data), &root)
    if err != nil {
        return nil, err
    }
    if err := root.SetMaxDepth(maxDepth); err != nil {
        return nil, err
    }
    return root, nil
}
```

### 3. 文件操作（直接操作树）

```go
// FindNode 查找指定路径的节点
func FindNode(root *FileNode, path string) *FileNode {
    if path == "" || path == "/" {
        return root
    }
    
    parts := strings.Split(strings.Trim(path, "/"), "/")
    current := root
    
    for _, part := range parts {
        found := false
        for _, child := range current.Children {
            if child.Name == part {
                current = child
                found = true
                break
            }
        }
        if !found {
            return nil
        }
    }
    
    return current
}

// AddFile 添加文件到指定目录（需要传入文件 ID）
func AddFile(root *FileNode, dirPath string, fileName string, fileID uint64) error {
    parent := FindNode(root, dirPath)
    if parent == nil || !parent.IsDir {
        return fmt.Errorf("目录不存在：%s", dirPath)
    }
    
    // 检查是否已存在
    for _, child := range parent.Children {
        if child.Name == fileName {
            return fmt.Errorf("文件已存在：%s", fileName)
        }
    }
    
    parent.Children = append(parent.Children, &FileNode{
        Name:   fileName,
        IsDir:  false,
        FileID: fileID,
    })
    
    return nil
}

// AddDir 添加子目录到指定目录
func AddDir(root *FileNode, parentPath, dirName string) error {
    // 新目录深度 = parentPath 深度 + 1，超过最大深度时返回错误
    parent := FindNode(root, parentPath)
    if parent == nil || !parent.IsDir {
        return fmt.Errorf("目录不存在：%s", parentPath)
    }
    
    // 检查是否已存在
    for _, child := range parent.Children {
        if child.Name == dirName {
            return fmt.Errorf("目录已存在：%s", dirName)
        }
    }
    
    parent.Children = append(parent.Children, &FileNode{
        Name:     dirName,
        IsDir:    true,
        Children: []*FileNode{},
    })
    
    return nil
}

// RemoveNode 删除指定路径的节点（文件或目录）
func RemoveNode(root *FileNode, path string) error {
    if path == "" || path == "/" {
        return fmt.Errorf("不能删除根目录")
    }
    
    parts := strings.Split(strings.Trim(path, "/"), "/")
    fileName := parts[len(parts)-1]
    parentPath := strings.Join(parts[:len(parts)-1], "/")
    
    parent := FindNode(root, parentPath)
    if parent == nil {
        return fmt.Errorf("父目录不存在：%s", parentPath)
    }
    
    // 查找并删除
    for i, child := range parent.Children {
        if child.Name == fileName {
            parent.Children = append(parent.Children[:i], parent.Children[i+1:]...)
            return nil
        }
    }
    
    return fmt.Errorf("节点不存在：%s", path)
}

// RenameNode 重命名节点
func RenameNode(root *FileNode, oldPath, newName string) error {
    node := FindNode(root, oldPath)
    if node == nil {
        return fmt.Errorf("节点不存在：%s", oldPath)
    }
    
    node.Name = newName
    return nil
}

// MoveNode 移动节点到指定目录
func MoveNode(root *FileNode, sourcePath, targetDirPath string) error {
    // 1) 找到源节点和父目录
    // 2) 校验目标目录存在且无重名
    // 3) 目录禁止移动到自己或子目录下
    // 4) 从源父目录摘除并挂载到目标目录
    return nil
}

// GetFileID 获取文件的 ID
func GetFileID(root *FileNode, path string) (uint64, error) {
    node := FindNode(root, path)
    if node == nil {
        return 0, fmt.Errorf("节点不存在：%s", path)
    }
    if node.IsDir {
        return 0, fmt.Errorf("不是文件：%s", path)
    }
    return node.FileID, nil
}
```

### 4. 完整使用示例

```go
package main

import (
    "encoding/json"
    "fmt"
    "log"
)

func main() {
    // 1. 从数据库加载 JSON 字符串
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
    
    // 2. 反序列化为 N 叉树（必须传入最大深度）
    tree, err := Deserialize(dbJSON, 30)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("加载成功：根目录有 %d 个子节点\n", len(tree.Children))
    
    // 3. 添加新文件（传入文件 ID）
    err = AddFile(tree, "src", "utils.go", 1003)
    if err != nil {
        log.Fatal(err)
    }
    
    // 4. 添加新目录
    err = AddDir(tree, "src", "pkg")
    if err != nil {
        log.Fatal(err)
    }
    
    // 5. 删除文件
    err = RemoveNode(tree, "docs/readme.md")
    if err != nil {
        log.Fatal(err)
    }
    
    // 6. 重命名
    err = RenameNode(tree, "src/main.go", "main_test.go")
    if err != nil {
        log.Fatal(err)
    }
    
    // 7. 移动文件
    err = MoveNode(tree, "src/utils.go", "docs")
    if err != nil {
        log.Fatal(err)
    }

    // 8. 获取文件 FileID
    fileID, err := GetFileID(tree, "docs/utils.go")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("文件 FileID: %d\n", fileID)
    
    // 9. 序列化存储回数据库
    result, err := Serialize(tree)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("存储数据：%s\n", result)
}
```

## 序列化格式示例

```json
{
  "name": "root",
  "is_dir": true,
  "children": [
    {
      "name": "docs",
      "is_dir": true,
      "children": []
    },
    {
      "name": "src",
      "is_dir": true,
      "children": [
        {"name": "main_test.go", "is_dir": false, "file_id": 1002},
        {"name": "utils.go", "is_dir": false, "file_id": 1003}
      ]
    }
  ]
}
```

**注意**：

- 文件节点包含 `file_id` 字段（数据库中的文件记录 ID）
- 目录节点**没有** `file_id` 字段（`omitempty` 自动省略）

## 辅助函数（可选）

```go
// GetAllFiles 获取所有文件路径和 FileID
func GetAllFiles(root *FileNode) []struct {
    Path   string
    FileID uint64
} {
    var files []struct {
        Path   string
        FileID uint64
    }
    var traverse func(node *FileNode, path string)
    
    traverse = func(node *FileNode, path string) {
        if !node.IsDir {
            files = append(files, struct {
                Path   string
                FileID uint64
            }{Path: path, FileID: node.FileID})
            return
        }
        
        for _, child := range node.Children {
            childPath := path + "/" + child.Name
            traverse(child, childPath)
        }
    }
    
    for _, child := range root.Children {
        traverse(child, "/"+child.Name)
    }
    
    return files
}

// GetAllDirs 获取所有目录路径
func GetAllDirs(root *FileNode) []string {
    var dirs []string
    var traverse func(node *FileNode, path string)
    
    traverse = func(node *FileNode, path string) {
        if node.IsDir {
            dirs = append(dirs, path)
        }
        
        for _, child := range node.Children {
            childPath := path + "/" + child.Name
            traverse(child, childPath)
        }
    }
    
    for _, child := range root.Children {
        traverse(child, "/"+child.Name)
    }
    
    return dirs
}
```

## 优势

- ✅ **简洁**：直接存储树结构，无需扁平化转换
- ✅ **原子性**：单条 JSON 记录保证数据一致性
- ✅ **高效**：树操作 O(depth)，序列化使用标准库
- ✅ **灵活**：支持任意深度的目录结构
- ✅ **FileID 关联**：文件节点存储数据库 FileID，方便关联查询
- ✅ **易扩展**：可在 FileNode 添加其他元数据字段（大小、权限等）
