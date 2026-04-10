package handler

import (
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const (
	chatUploadsRootDirName = "chat_uploads"
	maxChatUploadEditBytes = 2 * 1024 * 1024 // 文本编辑上限
)

// ChatUploadsHandler 对话中上传附件（chat_uploads 目录）的管理 API
type ChatUploadsHandler struct {
	logger *zap.Logger
}

// NewChatUploadsHandler 创建处理器
func NewChatUploadsHandler(logger *zap.Logger) *ChatUploadsHandler {
	return &ChatUploadsHandler{logger: logger}
}

func (h *ChatUploadsHandler) absRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Abs(filepath.Join(cwd, chatUploadsRootDirName))
}

// resolveUnderChatUploads 校验 relativePath（使用 / 分隔）对应文件必须在 chat_uploads 根下
func (h *ChatUploadsHandler) resolveUnderChatUploads(relativePath string) (abs string, err error) {
	root, err := h.absRoot()
	if err != nil {
		return "", err
	}
	rel := strings.TrimSpace(relativePath)
	if rel == "" {
		return "", fmt.Errorf("empty path")
	}
	rel = filepath.Clean(filepath.FromSlash(rel))
	if rel == "." || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("invalid path")
	}
	full := filepath.Join(root, rel)
	full, err = filepath.Abs(full)
	if err != nil {
		return "", err
	}
	rootAbs, _ := filepath.Abs(root)
	if full != rootAbs && !strings.HasPrefix(full, rootAbs+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes chat_uploads root")
	}
	return full, nil
}

// ChatUploadFileItem 列表项
type ChatUploadFileItem struct {
	RelativePath   string `json:"relativePath"`
	AbsolutePath   string `json:"absolutePath"` // 服务器上的绝对路径，便于在对话中引用（与附件落盘路径一致）
	Name           string `json:"name"`
	Size           int64  `json:"size"`
	ModifiedUnix   int64  `json:"modifiedUnix"`
	Date           string `json:"date"`
	ConversationID string `json:"conversationId"`
	// SubPath 为日期、会话目录之下的子路径（不含文件名），如 date/conv/a/b/file 则为 "a/b"；无嵌套则为 ""。
	SubPath string `json:"subPath"`
}

// List GET /api/chat-uploads
func (h *ChatUploadsHandler) List(c *gin.Context) {
	conversationFilter := strings.TrimSpace(c.Query("conversation"))
	root, err := h.absRoot()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// 保证根目录存在，否则「按文件夹」浏览时无法 mkdir，且首次列表为空时界面无路径工具栏
	if err := os.MkdirAll(root, 0755); err != nil {
		h.logger.Warn("创建 chat_uploads 根目录失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	var files []ChatUploadFileItem
	var folders []string
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		relSlash := filepath.ToSlash(rel)
		if d.IsDir() {
			folders = append(folders, relSlash)
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		parts := strings.Split(relSlash, "/")
		var dateStr, convID string
		if len(parts) >= 2 {
			dateStr = parts[0]
		}
		if len(parts) >= 3 {
			convID = parts[1]
		}
		var subPath string
		if len(parts) >= 4 {
			subPath = strings.Join(parts[2:len(parts)-1], "/")
		}
		if conversationFilter != "" && convID != conversationFilter {
			return nil
		}
		absPath, _ := filepath.Abs(path)
		files = append(files, ChatUploadFileItem{
			RelativePath:   relSlash,
			AbsolutePath:   absPath,
			Name:           d.Name(),
			Size:           info.Size(),
			ModifiedUnix:   info.ModTime().Unix(),
			Date:           dateStr,
			ConversationID: convID,
			SubPath:        subPath,
		})
		return nil
	})
	if err != nil {
		h.logger.Warn("列举对话附件失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if conversationFilter != "" {
		filteredFolders := make([]string, 0, len(folders))
		for _, rel := range folders {
			parts := strings.Split(rel, "/")
			if len(parts) >= 2 && parts[1] == conversationFilter {
				filteredFolders = append(filteredFolders, rel)
				continue
			}
			if len(parts) == 1 {
				prefix := rel + "/"
				for _, f := range files {
					if strings.HasPrefix(f.RelativePath, prefix) {
						filteredFolders = append(filteredFolders, rel)
						break
					}
				}
			}
		}
		folders = filteredFolders
	}
	sort.Strings(folders)
	sort.Slice(files, func(i, j int) bool {
		return files[i].ModifiedUnix > files[j].ModifiedUnix
	})
	c.JSON(http.StatusOK, gin.H{"files": files, "folders": folders})
}

// Download GET /api/chat-uploads/download?path=...
func (h *ChatUploadsHandler) Download(c *gin.Context) {
	p := c.Query("path")
	abs, err := h.resolveUnderChatUploads(p)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	st, err := os.Stat(abs)
	if err != nil || st.IsDir() {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}
	c.FileAttachment(abs, filepath.Base(abs))
}

type chatUploadPathBody struct {
	Path string `json:"path"`
}

// Delete DELETE /api/chat-uploads
func (h *ChatUploadsHandler) Delete(c *gin.Context) {
	var body chatUploadPathBody
	if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.Path) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	abs, err := h.resolveUnderChatUploads(body.Path)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	st, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if st.IsDir() {
		if err := os.RemoveAll(abs); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	} else {
		if err := os.Remove(abs); err != nil {
			if os.IsNotExist(err) {
				c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type chatUploadMkdirBody struct {
	Parent string `json:"parent"`
	Name   string `json:"name"`
}

// Mkdir POST /api/chat-uploads/mkdir — 在 parent 目录下新建子目录（parent 为 chat_uploads 下相对路径，空表示根目录；name 为单段目录名）
func (h *ChatUploadsHandler) Mkdir(c *gin.Context) {
	var body chatUploadMkdirBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" || strings.ContainsAny(name, `/\`) || name == "." || name == ".." {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid name"})
		return
	}
	if utf8.RuneCountInString(name) > 200 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name too long"})
		return
	}

	parent := strings.TrimSpace(body.Parent)
	parent = filepath.ToSlash(filepath.Clean(filepath.FromSlash(parent)))
	parent = strings.Trim(parent, "/")
	if parent == "." {
		parent = ""
	}

	root, err := h.absRoot()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if parent != "" {
		absParent, err := h.resolveUnderChatUploads(parent)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		st, err := os.Stat(absParent)
		if err != nil || !st.IsDir() {
			c.JSON(http.StatusBadRequest, gin.H{"error": "parent not found"})
			return
		}
	}

	var rel string
	if parent == "" {
		rel = name
	} else {
		rel = parent + "/" + name
	}
	absNew, err := h.resolveUnderChatUploads(rel)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if _, err := os.Stat(absNew); err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "already exists"})
		return
	}
	if err := os.Mkdir(absNew, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	relOut, _ := filepath.Rel(root, absNew)
	c.JSON(http.StatusOK, gin.H{"ok": true, "relativePath": filepath.ToSlash(relOut)})
}

type chatUploadRenameBody struct {
	Path    string `json:"path"`
	NewName string `json:"newName"`
}

// Rename PUT /api/chat-uploads/rename
func (h *ChatUploadsHandler) Rename(c *gin.Context) {
	var body chatUploadRenameBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	newName := strings.TrimSpace(body.NewName)
	if newName == "" || strings.ContainsAny(newName, `/\`) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid newName"})
		return
	}
	abs, err := h.resolveUnderChatUploads(body.Path)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	dir := filepath.Dir(abs)
	newAbs := filepath.Join(dir, filepath.Base(newName))
	root, _ := h.absRoot()
	newAbs, _ = filepath.Abs(newAbs)
	if newAbs != root && !strings.HasPrefix(newAbs, root+string(filepath.Separator)) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid target path"})
		return
	}
	if err := os.Rename(abs, newAbs); err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	newRel, _ := filepath.Rel(root, newAbs)
	c.JSON(http.StatusOK, gin.H{"ok": true, "relativePath": filepath.ToSlash(newRel)})
}

type chatUploadContentBody struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// GetContent GET /api/chat-uploads/content?path=...
func (h *ChatUploadsHandler) GetContent(c *gin.Context) {
	p := c.Query("path")
	abs, err := h.resolveUnderChatUploads(p)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	st, err := os.Stat(abs)
	if err != nil || st.IsDir() {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}
	if st.Size() > maxChatUploadEditBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "file too large for editor"})
		return
	}
	b, err := os.ReadFile(abs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !utf8.Valid(b) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "binary file not editable in UI"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"content": string(b)})
}

// PutContent PUT /api/chat-uploads/content
func (h *ChatUploadsHandler) PutContent(c *gin.Context) {
	var body chatUploadContentBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	if !utf8.ValidString(body.Content) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content must be valid UTF-8"})
		return
	}
	if len(body.Content) > maxChatUploadEditBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "content too large"})
		return
	}
	abs, err := h.resolveUnderChatUploads(body.Path)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := os.WriteFile(abs, []byte(body.Content), 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func chatUploadShortRand(n int) string {
	const letters = "0123456789abcdef"
	b := make([]byte, n)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = letters[int(b[i])%len(letters)]
	}
	return string(b)
}

// Upload POST /api/chat-uploads multipart: file；conversationId 可选；relativeDir 可选（chat_uploads 下目录的相对路径，将文件直接上传至该目录）
func (h *ChatUploadsHandler) Upload(c *gin.Context) {
	fh, err := c.FormFile("file")
	if err != nil || fh == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing file"})
		return
	}
	root, err := h.absRoot()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var targetDir string
	targetRel := strings.TrimSpace(c.PostForm("relativeDir"))
	if targetRel != "" {
		absDir, err := h.resolveUnderChatUploads(targetRel)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		st, err := os.Stat(absDir)
		if err != nil {
			if os.IsNotExist(err) {
				if err := os.MkdirAll(absDir, 0755); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		} else if !st.IsDir() {
			c.JSON(http.StatusBadRequest, gin.H{"error": "relativeDir is not a directory"})
			return
		}
		targetDir = absDir
	} else {
		convID := strings.TrimSpace(c.PostForm("conversationId"))
		convDir := convID
		if convDir == "" {
			convDir = "_manual"
		} else {
			convDir = strings.ReplaceAll(convDir, string(filepath.Separator), "_")
		}
		dateStr := time.Now().Format("2006-01-02")
		targetDir = filepath.Join(root, dateStr, convDir)
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	baseName := filepath.Base(fh.Filename)
	if baseName == "" || baseName == "." {
		baseName = "file"
	}
	baseName = strings.ReplaceAll(baseName, string(filepath.Separator), "_")
	ext := filepath.Ext(baseName)
	nameNoExt := strings.TrimSuffix(baseName, ext)
	suffix := fmt.Sprintf("_%s_%s", time.Now().Format("150405"), chatUploadShortRand(6))
	var unique string
	if ext != "" {
		unique = nameNoExt + suffix + ext
	} else {
		unique = baseName + suffix
	}
	fullPath := filepath.Join(targetDir, unique)
	src, err := fh.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	defer src.Close()
	dst, err := os.Create(fullPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		_ = os.Remove(fullPath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	rel, _ := filepath.Rel(root, fullPath)
	absSaved, _ := filepath.Abs(fullPath)
	c.JSON(http.StatusOK, gin.H{
		"ok":           true,
		"relativePath": filepath.ToSlash(rel),
		"absolutePath": absSaved,
		"name":         unique,
	})
}
