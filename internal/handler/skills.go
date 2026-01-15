package handler

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/database"
	"cyberstrike-ai/internal/skills"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// SkillsHandler Skills处理器
type SkillsHandler struct {
	manager    *skills.Manager
	config     *config.Config
	configPath string
	logger     *zap.Logger
	db         *database.DB // 数据库连接（用于获取调用统计）
}

// NewSkillsHandler 创建新的Skills处理器
func NewSkillsHandler(manager *skills.Manager, cfg *config.Config, configPath string, logger *zap.Logger) *SkillsHandler {
	return &SkillsHandler{
		manager:    manager,
		config:     cfg,
		configPath: configPath,
		logger:     logger,
	}
}

// SetDB 设置数据库连接（用于获取调用统计）
func (h *SkillsHandler) SetDB(db *database.DB) {
	h.db = db
}

// GetSkills 获取所有skills列表（支持分页和搜索）
func (h *SkillsHandler) GetSkills(c *gin.Context) {
	skillList, err := h.manager.ListSkills()
	if err != nil {
		h.logger.Error("获取skills列表失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 搜索参数
	searchKeyword := strings.TrimSpace(c.Query("search"))
	
	// 先加载所有skills的详细信息用于搜索过滤
	allSkillsInfo := make([]map[string]interface{}, 0, len(skillList))
	for _, skillName := range skillList {
		skill, err := h.manager.LoadSkill(skillName)
		if err != nil {
			h.logger.Warn("加载skill失败", zap.String("skill", skillName), zap.Error(err))
			continue
		}

		// 获取文件信息
		skillPath := skill.Path
		skillFile := filepath.Join(skillPath, "SKILL.md")
		// 尝试其他可能的文件名
		if _, err := os.Stat(skillFile); os.IsNotExist(err) {
			alternatives := []string{
				filepath.Join(skillPath, "skill.md"),
				filepath.Join(skillPath, "README.md"),
				filepath.Join(skillPath, "readme.md"),
			}
			for _, alt := range alternatives {
				if _, err := os.Stat(alt); err == nil {
					skillFile = alt
					break
				}
			}
		}

		fileInfo, _ := os.Stat(skillFile)
		var fileSize int64
		var modTime string
		if fileInfo != nil {
			fileSize = fileInfo.Size()
			modTime = fileInfo.ModTime().Format("2006-01-02 15:04:05")
		}

		skillInfo := map[string]interface{}{
			"name":        skill.Name,
			"description": skill.Description,
			"path":        skill.Path,
			"file_size":   fileSize,
			"mod_time":    modTime,
		}
		allSkillsInfo = append(allSkillsInfo, skillInfo)
	}

	// 如果有搜索关键词，进行过滤
	filteredSkillsInfo := allSkillsInfo
	if searchKeyword != "" {
		keywordLower := strings.ToLower(searchKeyword)
		filteredSkillsInfo = make([]map[string]interface{}, 0)
		for _, skillInfo := range allSkillsInfo {
			name := strings.ToLower(fmt.Sprintf("%v", skillInfo["name"]))
			description := strings.ToLower(fmt.Sprintf("%v", skillInfo["description"]))
			path := strings.ToLower(fmt.Sprintf("%v", skillInfo["path"]))
			
			if strings.Contains(name, keywordLower) ||
				strings.Contains(description, keywordLower) ||
				strings.Contains(path, keywordLower) {
				filteredSkillsInfo = append(filteredSkillsInfo, skillInfo)
			}
		}
	}

	// 分页参数
	limit := 20 // 默认每页20条
	offset := 0
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsed, err := parseInt(limitStr); err == nil && parsed > 0 {
			// 允许更大的limit用于搜索场景，但设置一个合理的上限（10000）
			if parsed <= 10000 {
				limit = parsed
			} else {
				limit = 10000
			}
		}
	}
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if parsed, err := parseInt(offsetStr); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	// 计算分页范围
	total := len(filteredSkillsInfo)
	start := offset
	end := offset + limit
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	// 获取当前页的skill列表
	var paginatedSkillsInfo []map[string]interface{}
	if start < end {
		paginatedSkillsInfo = filteredSkillsInfo[start:end]
	} else {
		paginatedSkillsInfo = []map[string]interface{}{}
	}

	c.JSON(http.StatusOK, gin.H{
		"skills": paginatedSkillsInfo,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}


// GetSkill 获取单个skill的详细信息
func (h *SkillsHandler) GetSkill(c *gin.Context) {
	skillName := c.Param("name")
	if skillName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "skill名称不能为空"})
		return
	}

	skill, err := h.manager.LoadSkill(skillName)
	if err != nil {
		h.logger.Warn("加载skill失败", zap.String("skill", skillName), zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "skill不存在: " + err.Error()})
		return
	}

	// 获取文件信息
	skillPath := skill.Path
	skillFile := filepath.Join(skillPath, "SKILL.md")
	if _, err := os.Stat(skillFile); os.IsNotExist(err) {
		alternatives := []string{
			filepath.Join(skillPath, "skill.md"),
			filepath.Join(skillPath, "README.md"),
			filepath.Join(skillPath, "readme.md"),
		}
		for _, alt := range alternatives {
			if _, err := os.Stat(alt); err == nil {
				skillFile = alt
				break
			}
		}
	}

	fileInfo, _ := os.Stat(skillFile)
	var fileSize int64
	var modTime string
	if fileInfo != nil {
		fileSize = fileInfo.Size()
		modTime = fileInfo.ModTime().Format("2006-01-02 15:04:05")
	}

	c.JSON(http.StatusOK, gin.H{
		"skill": map[string]interface{}{
			"name":        skill.Name,
			"description": skill.Description,
			"content":     skill.Content,
			"path":        skill.Path,
			"file_size":   fileSize,
			"mod_time":    modTime,
		},
	})
}

// CreateSkill 创建新skill
func (h *SkillsHandler) CreateSkill(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		Content     string `json:"content" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数: " + err.Error()})
		return
	}

	// 验证skill名称（只允许字母、数字、连字符和下划线）
	if !isValidSkillName(req.Name) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "skill名称只能包含字母、数字、连字符和下划线"})
		return
	}

	// 获取skills目录
	skillsDir := h.config.SkillsDir
	if skillsDir == "" {
		skillsDir = "skills"
	}
	configDir := filepath.Dir(h.configPath)
	if !filepath.IsAbs(skillsDir) {
		skillsDir = filepath.Join(configDir, skillsDir)
	}

	// 创建skill目录
	skillDir := filepath.Join(skillsDir, req.Name)
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		h.logger.Error("创建skill目录失败", zap.String("skill", req.Name), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建skill目录失败: " + err.Error()})
		return
	}

	// 检查是否已存在
	skillFile := filepath.Join(skillDir, "SKILL.md")
	if _, err := os.Stat(skillFile); err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "skill已存在"})
		return
	}

	// 构建SKILL.md内容
	var content strings.Builder
	content.WriteString("---\n")
	content.WriteString(fmt.Sprintf("name: %s\n", req.Name))
	if req.Description != "" {
		// 如果描述包含特殊字符，需要加引号
		desc := req.Description
		if strings.Contains(desc, ":") || strings.Contains(desc, "\n") {
			desc = fmt.Sprintf(`"%s"`, strings.ReplaceAll(desc, `"`, `\"`))
		}
		content.WriteString(fmt.Sprintf("description: %s\n", desc))
	}
	content.WriteString("version: 1.0.0\n")
	content.WriteString("---\n\n")
	content.WriteString(req.Content)

	// 写入文件
	if err := os.WriteFile(skillFile, []byte(content.String()), 0644); err != nil {
		h.logger.Error("创建skill文件失败", zap.String("skill", req.Name), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建skill文件失败: " + err.Error()})
		return
	}

	h.logger.Info("创建skill成功", zap.String("skill", req.Name))
	c.JSON(http.StatusOK, gin.H{
		"message": "skill已创建",
		"skill": map[string]interface{}{
			"name": req.Name,
			"path": skillDir,
		},
	})
}

// UpdateSkill 更新skill
func (h *SkillsHandler) UpdateSkill(c *gin.Context) {
	skillName := c.Param("name")
	if skillName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "skill名称不能为空"})
		return
	}

	var req struct {
		Description string `json:"description"`
		Content     string `json:"content" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数: " + err.Error()})
		return
	}

	// 获取skills目录
	skillsDir := h.config.SkillsDir
	if skillsDir == "" {
		skillsDir = "skills"
	}
	configDir := filepath.Dir(h.configPath)
	if !filepath.IsAbs(skillsDir) {
		skillsDir = filepath.Join(configDir, skillsDir)
	}

	// 查找skill文件
	skillDir := filepath.Join(skillsDir, skillName)
	skillFile := filepath.Join(skillDir, "SKILL.md")
	if _, err := os.Stat(skillFile); os.IsNotExist(err) {
		alternatives := []string{
			filepath.Join(skillDir, "skill.md"),
			filepath.Join(skillDir, "README.md"),
			filepath.Join(skillDir, "readme.md"),
		}
		found := false
		for _, alt := range alternatives {
			if _, err := os.Stat(alt); err == nil {
				skillFile = alt
				found = true
				break
			}
		}
		if !found {
			c.JSON(http.StatusNotFound, gin.H{"error": "skill不存在"})
			return
		}
	}

	// 读取现有文件以保留front matter中的name
	existingContent, err := os.ReadFile(skillFile)
	if err != nil {
		h.logger.Error("读取skill文件失败", zap.String("skill", skillName), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取skill文件失败: " + err.Error()})
		return
	}

	// 解析现有内容，提取name
	existingName := skillName
	contentStr := string(existingContent)
	if strings.HasPrefix(contentStr, "---") {
		parts := strings.SplitN(contentStr, "---", 3)
		if len(parts) >= 2 {
			frontMatter := parts[1]
			lines := strings.Split(frontMatter, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "name:") {
					name := strings.TrimSpace(strings.TrimPrefix(line, "name:"))
					name = strings.Trim(name, `"'`)
					if name != "" {
						existingName = name
					}
					break
				}
			}
		}
	}

	// 构建新的SKILL.md内容
	var newContent strings.Builder
	newContent.WriteString("---\n")
	newContent.WriteString(fmt.Sprintf("name: %s\n", existingName))
	if req.Description != "" {
		// 如果描述包含特殊字符，需要加引号
		desc := req.Description
		if strings.Contains(desc, ":") || strings.Contains(desc, "\n") {
			desc = fmt.Sprintf(`"%s"`, strings.ReplaceAll(desc, `"`, `\"`))
		}
		newContent.WriteString(fmt.Sprintf("description: %s\n", desc))
	}
	newContent.WriteString("version: 1.0.0\n")
	newContent.WriteString("---\n\n")
	newContent.WriteString(req.Content)

	// 写入文件（统一使用SKILL.md）
	targetFile := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(targetFile, []byte(newContent.String()), 0644); err != nil {
		h.logger.Error("更新skill文件失败", zap.String("skill", skillName), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新skill文件失败: " + err.Error()})
		return
	}

	// 如果原文件不是SKILL.md，删除旧文件
	if skillFile != targetFile {
		os.Remove(skillFile)
	}

	h.logger.Info("更新skill成功", zap.String("skill", skillName))
	c.JSON(http.StatusOK, gin.H{
		"message": "skill已更新",
	})
}

// DeleteSkill 删除skill
func (h *SkillsHandler) DeleteSkill(c *gin.Context) {
	skillName := c.Param("name")
	if skillName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "skill名称不能为空"})
		return
	}

	// 获取skills目录
	skillsDir := h.config.SkillsDir
	if skillsDir == "" {
		skillsDir = "skills"
	}
	configDir := filepath.Dir(h.configPath)
	if !filepath.IsAbs(skillsDir) {
		skillsDir = filepath.Join(configDir, skillsDir)
	}

	// 删除skill目录
	skillDir := filepath.Join(skillsDir, skillName)
	if err := os.RemoveAll(skillDir); err != nil {
		h.logger.Error("删除skill失败", zap.String("skill", skillName), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除skill失败: " + err.Error()})
		return
	}

	h.logger.Info("删除skill成功", zap.String("skill", skillName))
	c.JSON(http.StatusOK, gin.H{
		"message": "skill已删除",
	})
}

// GetSkillStats 获取skills调用统计信息
func (h *SkillsHandler) GetSkillStats(c *gin.Context) {
	skillList, err := h.manager.ListSkills()
	if err != nil {
		h.logger.Error("获取skills列表失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 获取skills目录
	skillsDir := h.config.SkillsDir
	if skillsDir == "" {
		skillsDir = "skills"
	}
	configDir := filepath.Dir(h.configPath)
	if !filepath.IsAbs(skillsDir) {
		skillsDir = filepath.Join(configDir, skillsDir)
	}

	// 从数据库加载调用统计
	var skillStatsMap map[string]*database.SkillStats
	if h.db != nil {
		dbStats, err := h.db.LoadSkillStats()
		if err != nil {
			h.logger.Warn("从数据库加载Skills统计信息失败", zap.Error(err))
			skillStatsMap = make(map[string]*database.SkillStats)
		} else {
			skillStatsMap = dbStats
		}
	} else {
		skillStatsMap = make(map[string]*database.SkillStats)
	}

	// 构建统计信息（包含所有skills，即使没有调用记录）
	statsList := make([]map[string]interface{}, 0, len(skillList))
	totalCalls := 0
	totalSuccess := 0
	totalFailed := 0

	for _, skillName := range skillList {
		stat, exists := skillStatsMap[skillName]
		if !exists {
			stat = &database.SkillStats{
				SkillName:    skillName,
				TotalCalls:   0,
				SuccessCalls: 0,
				FailedCalls:  0,
			}
		}

		totalCalls += stat.TotalCalls
		totalSuccess += stat.SuccessCalls
		totalFailed += stat.FailedCalls

		lastCallTimeStr := ""
		if stat.LastCallTime != nil {
			lastCallTimeStr = stat.LastCallTime.Format("2006-01-02 15:04:05")
		}

		statsList = append(statsList, map[string]interface{}{
			"skill_name":     stat.SkillName,
			"total_calls":    stat.TotalCalls,
			"success_calls":  stat.SuccessCalls,
			"failed_calls":   stat.FailedCalls,
			"last_call_time": lastCallTimeStr,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"total_skills":  len(skillList),
		"total_calls":   totalCalls,
		"total_success": totalSuccess,
		"total_failed":  totalFailed,
		"skills_dir":    skillsDir,
		"stats":         statsList,
	})
}

// ClearSkillStats 清空所有Skills统计信息
func (h *SkillsHandler) ClearSkillStats(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库连接未配置"})
		return
	}

	if err := h.db.ClearSkillStats(); err != nil {
		h.logger.Error("清空Skills统计信息失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "清空统计信息失败: " + err.Error()})
		return
	}

	h.logger.Info("已清空所有Skills统计信息")
	c.JSON(http.StatusOK, gin.H{
		"message": "已清空所有Skills统计信息",
	})
}

// ClearSkillStatsByName 清空指定skill的统计信息
func (h *SkillsHandler) ClearSkillStatsByName(c *gin.Context) {
	skillName := c.Param("name")
	if skillName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "skill名称不能为空"})
		return
	}

	if h.db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库连接未配置"})
		return
	}

	if err := h.db.ClearSkillStatsByName(skillName); err != nil {
		h.logger.Error("清空指定skill统计信息失败", zap.String("skill", skillName), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "清空统计信息失败: " + err.Error()})
		return
	}

	h.logger.Info("已清空指定skill统计信息", zap.String("skill", skillName))
	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("已清空skill '%s' 的统计信息", skillName),
	})
}

// isValidSkillName 验证skill名称是否有效
func isValidSkillName(name string) bool {
	if name == "" || len(name) > 100 {
		return false
	}
	// 只允许字母、数字、连字符和下划线
	for _, r := range name {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_') {
			return false
		}
	}
	return true
}
