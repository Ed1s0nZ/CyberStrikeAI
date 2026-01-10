/**
 * 内置工具名称常量
 * 所有前端代码中使用内置工具名称的地方都应该使用这些常量，而不是硬编码字符串
 * 
 * 注意：这些常量必须与后端的 internal/mcp/builtin/constants.go 中的常量保持一致
 */

// 内置工具名称常量
const BuiltinTools = {
    // 漏洞管理工具
    RECORD_VULNERABILITY: 'record_vulnerability',
    
    // 知识库工具
    LIST_KNOWLEDGE_RISK_TYPES: 'list_knowledge_risk_types',
    SEARCH_KNOWLEDGE_BASE: 'search_knowledge_base'
};

// 检查是否是内置工具
function isBuiltinTool(toolName) {
    return Object.values(BuiltinTools).includes(toolName);
}

// 获取所有内置工具名称列表
function getAllBuiltinTools() {
    return Object.values(BuiltinTools);
}

