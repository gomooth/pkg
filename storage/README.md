# 本地文件存储 Storage

基于项目的文件存储管理工具，提供公开文件、私有文件和临时文件三种存储类型，内置路径安全防护。

## 存储类型

| 类型 | 创建方式 | 根目录 | 特点 |
|------|---------|--------|------|
| 公开文件 | `Public()` | `{root}/public/` | 支持 URL 生成 |
| 私有文件 | `Disk(dir)` | `{root}/{dir}/` | 内部文件，不对外暴露 |
| 临时文件 | `Temp()` | `{os.TempDir()}/go-pkg/` | 系统临时目录，重启后清理 |

存储根目录默认为 `storage`，可通过 `WithRoot()` 修改。

## 接口

### IStorage — 基础接口

```go
type IStorage interface {
    Dir() (string, error)      // 获得文件存储的目录
    Path() (string, error)     // 获得文件存储的完整路径
    Filename() (string, error) // 获得文件名
}
```

### IPrivateStorage — 私有文件接口

```go
type IPrivateStorage interface {
    IStorage
    AppendDir(dirs ...string) IPrivateStorage // 追加目录（链式调用）
    SetName(name string) IPrivateStorage      // 设置文件名（链式调用）
}
```

### IPublicStorage — 公开文件接口

```go
type IPublicStorage interface {
    IStorage
    AppendDir(dirs ...string) IPublicStorage  // 追加目录（链式调用）
    SetName(name string) IPublicStorage       // 设置文件名（链式调用）
    URL() (string, error)                     // 访问链接（不含 host）
    URLWithHost(host string) (string, error)  // 访问链接（含 host）
}
```

## 使用示例

### 公开文件

```go
// 链式创建
p := storage.Public().AppendDir("avatars", "2024").SetName("photo.jpg")

path, _ := p.Path()       // storage/public/avatars/2024/photo.jpg
dir, _ := p.Dir()         // storage/public/avatars/2024
name, _ := p.Filename()   // photo.jpg
url, _ := p.URL()         // /storage/avatars/2024/photo.jpg
fullUrl, _ := p.URLWithHost("https://example.com")  // https://example.com/storage/avatars/2024/photo.jpg

// 从已有文件路径创建
p := storage.PublicFromFile("storage/public/avatars/2024/photo.jpg")

// 从 URL 创建
p := storage.PublicFromUrl("/storage/avatars/2024/photo.jpg")
```

### 私有文件

```go
d := storage.Disk("exports").AppendDir("reports").SetName("data.csv")
path, _ := d.Path()    // storage/exports/reports/data.csv
dir, _ := d.Dir()      // storage/exports/reports
```

### 临时文件

```go
t := storage.Temp().AppendDir("uploads").SetName("temp_file.dat")
path, _ := t.Path()    // {os.TempDir()}/go-pkg/uploads/temp_file.dat
```

### 自定义根目录

```go
p := storage.Public(storage.WithRoot("files"))
path, _ := p.Path()    // files/public/...
```

## 安全防护

所有路径组件经过 `sanitizePath` 检查，防止：

- **目录遍历**：拒绝 `..`、`../` 等路径组件
- **Null 字节注入**：拒绝包含 `\x00` 的路径
- **路径编码绕过**：验证 `filepath.Clean` 后的路径与原始路径一致

错误发生时，链式调用不会中断（返回自身），但后续所有操作都会返回错误：

```go
p := storage.Public().AppendDir("..", "etc").SetName("passwd")
_, err := p.Path()  // error: directory traversal detected
```

## 注意事项

- 公开文件和私有文件的路径相对于项目运行时目录
- 临时文件存储于系统临时目录，受操作系统影响，重启后可能被清理
- `Disk("tmp")` 或 `Disk("temp")` 等价于 `Temp()`
