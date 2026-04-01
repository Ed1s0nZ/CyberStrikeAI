package handler

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/database"
	"cyberstrike-ai/internal/skills"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// SkillsHandler Skills handler
type SkillsHandler struct {
	manager    *skills.Manager
	config     *config.Config
	configPath string
	logger     *zap.Logger
	db *database.DB // database connection（）
}

// NewSkillsHandler creates a new Skills handler
func NewSkillsHandler(manager *skills.Manager, cfg *config.Config, configPath string, logger *zap.Logger) *SkillsHandler {
	return &SkillsHandler{
		manager:    manager,
		config:     cfg,
		configPath: configPath,
		logger:     logger,
	}
}

// SetDB database connection（）
func (h *SkillsHandler) SetDB(db *database.DB) {
	h.db = db
}

// GetSkills skillslist（）
func (h *SkillsHandler) GetSkills(c *gin.Context) {
	skillList, err := h.manager.ListSkills()
	if err != nil {
		h.logger.Error("skillstable failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// search params
	searchKeyword := strings.TrimSpace(c.Query("search"))

	// loadskills
	allSkillsInfo := make([]map[string]interface{}, 0, len(skillList))
	for _, skillName := range skillList {
		skill, err := h.manager.LoadSkill(skillName)
		if err != nil {
			h.logger.Warn("loadskill", zap.String("skill", skillName), zap.Error(err))
			continue
		}

		// get file info
		skillPath := skill.Path
		skillFile := filepath.Join(skillPath, "SKILL.md")
		// filename
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

	// if search keyword exists, filter
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

	// pagination params
	limit := 20 // default20
	offset := 0
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsed, err := parseInt(limitStr); err == nil && parsed > 0 {
			// allow larger limit for search scenarios，set a reasonable upper limit（10000）
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

	// calculate pagination range
	total := len(filteredSkillsInfo)
	start := offset
	end := offset + limit
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	// currentskilllist
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

// GetSkill get single skill details
func (h *SkillsHandler) GetSkill(c *gin.Context) {
	skillName := c.Param("name")
	if skillName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "skill name cannot be empty"})
		return
	}

	skill, err := h.manager.LoadSkill(skillName)
	if err != nil {
		h.logger.Warn("loadskill", zap.String("skill", skillName), zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "skill not found: " + err.Error()})
		return
	}

	// get file info
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

// GetSkillBoundRoles skillrolelist
func (h *SkillsHandler) GetSkillBoundRoles(c *gin.Context) {
	skillName := c.Param("name")
	if skillName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "skill name cannot be empty"})
		return
	}

	boundRoles := h.getRolesBoundToSkill(skillName)
	c.JSON(http.StatusOK, gin.H{
		"skill":       skillName,
		"bound_roles": boundRoles,
		"bound_count": len(boundRoles),
	})
}

// getRolesBoundToSkill skillrolelist（）
func (h *SkillsHandler) getRolesBoundToSkill(skillName string) []string {
	if h.config.Roles == nil {
		return []string{}
	}

	boundRoles := make([]string, 0)
	for roleName, role := range h.config.Roles {
		// role
		if role.Name == "" {
			role.Name = roleName
		}

		// roleSkillslistskill
		if len(role.Skills) > 0 {
			for _, skill := range role.Skills {
				if skill == skillName {
					boundRoles = append(boundRoles, roleName)
					break
				}
			}
		}
	}

	return boundRoles
}

// CreateSkill create new skill
func (h *SkillsHandler) CreateSkill(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		Content     string `json:"content" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request parameters: " + err.Error()})
		return
	}

	// skill（only allow letters, numbers, hyphens and underscores）
	if !isValidSkillName(req.Name) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "skill name can only contain letters, numbers, hyphens and underscores"})
		return
	}

	// get skills directory
	skillsDir := h.config.SkillsDir
	if skillsDir == "" {
		skillsDir = "skills"
	}
	configDir := filepath.Dir(h.configPath)
	if !filepath.IsAbs(skillsDir) {
		skillsDir = filepath.Join(configDir, skillsDir)
	}

	// create skill directory
	skillDir := filepath.Join(skillsDir, req.Name)
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		h.logger.Error("failed to create skill directory", zap.String("skill", req.Name), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create skill directory: " + err.Error()})
		return
	}

	// check if already exists
	skillFile := filepath.Join(skillDir, "SKILL.md")
	if _, err := os.Stat(skillFile); err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "skill already exists"})
		return
	}

	// build SKILL.md content
	var content strings.Builder
	content.WriteString("---\n")
	content.WriteString(fmt.Sprintf("name: %s\n", req.Name))
	if req.Description != "" {
		// if description contains special chars, needs quoting
		desc := req.Description
		if strings.Contains(desc, ":") || strings.Contains(desc, "\n") {
			desc = fmt.Sprintf(`"%s"`, strings.ReplaceAll(desc, `"`, `\"`))
		}
		content.WriteString(fmt.Sprintf("description: %s\n", desc))
	}
	content.WriteString("version: 1.0.0\n")
	content.WriteString("---\n\n")
	content.WriteString(req.Content)

	// write file
	if err := os.WriteFile(skillFile, []byte(content.String()), 0644); err != nil {
		h.logger.Error("failed to create skill file", zap.String("skill", req.Name), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create skill file: " + err.Error()})
		return
	}
	h.manager.InvalidateSkill(req.Name)

	h.logger.Info("skill created successfully", zap.String("skill", req.Name))
	c.JSON(http.StatusOK, gin.H{
		"message": "skillcreated",
		"skill": map[string]interface{}{
			"name": req.Name,
			"path": skillDir,
		},
	})
}

// UpdateSkill update skill
func (h *SkillsHandler) UpdateSkill(c *gin.Context) {
	skillName := c.Param("name")
	if skillName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "skill name cannot be empty"})
		return
	}

	var req struct {
		Description string `json:"description"`
		Content     string `json:"content" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request parameters: " + err.Error()})
		return
	}

	// get skills directory
	skillsDir := h.config.SkillsDir
	if skillsDir == "" {
		skillsDir = "skills"
	}
	configDir := filepath.Dir(h.configPath)
	if !filepath.IsAbs(skillsDir) {
		skillsDir = filepath.Join(configDir, skillsDir)
	}

	// find skill file
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
			c.JSON(http.StatusNotFound, gin.H{"error": "skill not found"})
			return
		}
	}

	// read existing file to preserve name in front matter
	existingContent, err := os.ReadFile(skillFile)
	if err != nil {
		h.logger.Error("failed to read skill file", zap.String("skill", skillName), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read skill file: " + err.Error()})
		return
	}

	// parse，name
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

	// build new SKILL.md content
	var newContent strings.Builder
	newContent.WriteString("---\n")
	newContent.WriteString(fmt.Sprintf("name: %s\n", existingName))
	if req.Description != "" {
		// if description contains special chars, needs quoting
		desc := req.Description
		if strings.Contains(desc, ":") || strings.Contains(desc, "\n") {
			desc = fmt.Sprintf(`"%s"`, strings.ReplaceAll(desc, `"`, `\"`))
		}
		newContent.WriteString(fmt.Sprintf("description: %s\n", desc))
	}
	newContent.WriteString("version: 1.0.0\n")
	newContent.WriteString("---\n\n")
	newContent.WriteString(req.Content)

	// write file（SKILL.md）
	targetFile := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(targetFile, []byte(newContent.String()), 0644); err != nil {
		h.logger.Error("update skill", zap.String("skill", skillName), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update skill: " + err.Error()})
		return
	}

	// SKILL.md，delete
	if skillFile != targetFile {
		os.Remove(skillFile)
	}
	h.manager.InvalidateSkill(skillName)

	h.logger.Info("update skill", zap.String("skill", skillName))
	c.JSON(http.StatusOK, gin.H{
		"message": "skill updated",
	})
}

// DeleteSkill deleteskill
func (h *SkillsHandler) DeleteSkill(c *gin.Context) {
	skillName := c.Param("name")
	if skillName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "skill name cannot be empty"})
		return
	}

	// roleskill，if so, auto-remove binding
	affectedRoles := h.removeSkillFromRoles(skillName)
	if len(affectedRoles) > 0 {
		h.logger.Info("roleskill",
			zap.String("skill", skillName),
			zap.Strings("roles", affectedRoles))
	}

	// get skills directory
	skillsDir := h.config.SkillsDir
	if skillsDir == "" {
		skillsDir = "skills"
	}
	configDir := filepath.Dir(h.configPath)
	if !filepath.IsAbs(skillsDir) {
		skillsDir = filepath.Join(configDir, skillsDir)
	}

	// deleteskill
	skillDir := filepath.Join(skillsDir, skillName)
	if err := os.RemoveAll(skillDir); err != nil {
		h.logger.Error("deleteskill", zap.String("skill", skillName), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "deleteskill: " + err.Error()})
		return
	}
	h.manager.InvalidateSkill(skillName)

	responseMsg := "skilldelete"
	if len(affectedRoles) > 0 {
		responseMsg = fmt.Sprintf("skilldelete，auto-removed from %d role: %s",
			len(affectedRoles), strings.Join(affectedRoles, ", "))
	}

	h.logger.Info("deleteskill", zap.String("skill", skillName))
	c.JSON(http.StatusOK, gin.H{
		"message":        responseMsg,
		"affected_roles": affectedRoles,
	})
}

// GetSkillStats get skills call statistics
func (h *SkillsHandler) GetSkillStats(c *gin.Context) {
	skillList, err := h.manager.ListSkills()
	if err != nil {
		h.logger.Error("skillstable failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// get skills directory
	skillsDir := h.config.SkillsDir
	if skillsDir == "" {
		skillsDir = "skills"
	}
	configDir := filepath.Dir(h.configPath)
	if !filepath.IsAbs(skillsDir) {
		skillsDir = filepath.Join(configDir, skillsDir)
	}

	// load
	var skillStatsMap map[string]*database.SkillStats
	if h.db != nil {
		dbStats, err := h.db.LoadSkillStats()
		if err != nil {
			h.logger.Warn("loadSkills", zap.Error(err))
			skillStatsMap = make(map[string]*database.SkillStats)
		} else {
			skillStatsMap = dbStats
		}
	} else {
		skillStatsMap = make(map[string]*database.SkillStats)
	}

	// （skills，record）
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

// ClearSkillStats clearSkills
func (h *SkillsHandler) ClearSkillStats(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database connection"})
		return
	}

	if err := h.db.ClearSkillStats(); err != nil {
		h.logger.Error("clearSkills", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "clear: " + err.Error()})
		return
	}

	h.logger.Info("clearSkills")
	c.JSON(http.StatusOK, gin.H{
		"message": "clearSkills",
	})
}

// ClearSkillStatsByName clearskill
func (h *SkillsHandler) ClearSkillStatsByName(c *gin.Context) {
	skillName := c.Param("name")
	if skillName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "skill name cannot be empty"})
		return
	}

	if h.db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database connection"})
		return
	}

	if err := h.db.ClearSkillStatsByName(skillName); err != nil {
		h.logger.Error("clearskill", zap.String("skill", skillName), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "clear: " + err.Error()})
		return
	}

	h.logger.Info("clearskill", zap.String("skill", skillName))
	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("clearskill '%s' ", skillName),
	})
}

// removeSkillFromRoles roleskill
// returnsrolelist
func (h *SkillsHandler) removeSkillFromRoles(skillName string) []string {
	if h.config.Roles == nil {
		return []string{}
	}

	affectedRoles := make([]string, 0)
	rolesToUpdate := make(map[string]config.RoleConfig)

	// role，skill
	for roleName, role := range h.config.Roles {
		// role
		if role.Name == "" {
			role.Name = roleName
		}

		// roleSkillslistdeleteskill
		if len(role.Skills) > 0 {
			updated := false
			newSkills := make([]string, 0, len(role.Skills))
			for _, skill := range role.Skills {
				if skill != skillName {
					newSkills = append(newSkills, skill)
				} else {
					updated = true
				}
			}
			if updated {
				role.Skills = newSkills
				rolesToUpdate[roleName] = role
				affectedRoles = append(affectedRoles, roleName)
			}
		}
	}

	// role，
	if len(rolesToUpdate) > 0 {
		// update in-memory config
		for roleName, role := range rolesToUpdate {
			h.config.Roles[roleName] = role
		}
		// role
		if err := h.saveRolesConfig(); err != nil {
			h.logger.Error("role", zap.Error(err))
		}
	}

	return affectedRoles
}

// saveRolesConfig role（SkillsHandler）
func (h *SkillsHandler) saveRolesConfig() error {
	configDir := filepath.Dir(h.configPath)
	rolesDir := h.config.RolesDir
	if rolesDir == "" {
		rolesDir = "roles" // default directory
	}

	// if relative path, relative to config file directory
	if !filepath.IsAbs(rolesDir) {
		rolesDir = filepath.Join(configDir, rolesDir)
	}

	// ensure directory exists
	if err := os.MkdirAll(rolesDir, 0755); err != nil {
		return fmt.Errorf("role: %w", err)
	}

	// role
	if h.config.Roles != nil {
		for roleName, role := range h.config.Roles {
			// role
			if role.Name == "" {
				role.Name = roleName
			}

			// rolefilename（filename，）
			safeFileName := sanitizeRoleFileName(role.Name)
			roleFile := filepath.Join(rolesDir, safeFileName+".yaml")

			// roleYAML
			roleData, err := yaml.Marshal(&role)
			if err != nil {
				h.logger.Error("role", zap.String("role", roleName), zap.Error(err))
				continue
			}

			// process icon field：\Uicon（YAMLparseUnicode）
			roleDataStr := string(roleData)
			if role.Icon != "" && strings.HasPrefix(role.Icon, "\\U") {
				// match icon: \UXXXXXXXX format（without quotes），exclude already quoted cases
				re := regexp.MustCompile(`(?m)^(icon:\s+)(\\U[0-9A-F]{8})(\s*)$`)
				roleDataStr = re.ReplaceAllString(roleDataStr, `${1}"${2}"${3}`)
				roleData = []byte(roleDataStr)
			}

			// write file
			if err := os.WriteFile(roleFile, roleData, 0644); err != nil {
				h.logger.Error("role", zap.String("role", roleName), zap.String("file", roleFile), zap.Error(err))
				continue
			}

			h.logger.Info("roleconfig saved", zap.String("role", roleName), zap.String("file", roleFile))
		}
	}

	return nil
}

// sanitizeRoleFileName rolefilename
func sanitizeRoleFileName(name string) string {
	// replace potentially unsafe chars
	replacer := map[rune]string{
		'/':  "_",
		'\\': "_",
		':':  "_",
		'*':  "_",
		'?':  "_",
		'"':  "_",
		'<':  "_",
		'>':  "_",
		'|':  "_",
		' ':  "_",
	}

	var result []rune
	for _, r := range name {
		if replacement, ok := replacer[r]; ok {
			result = append(result, []rune(replacement)...)
		} else {
			result = append(result, r)
		}
	}

	fileName := string(result)
	// filename，default
	if fileName == "" {
		fileName = "role"
	}

	return fileName
}

// isValidSkillName validate skill name
func isValidSkillName(name string) bool {
	if name == "" || len(name) > 100 {
		return false
	}
	// only allow letters, numbers, hyphens and underscores
	for _, r := range name {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_') {
			return false
		}
	}
	return true
}
