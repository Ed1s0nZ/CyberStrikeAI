# JS 混淆识别与还原指南

## 混淆类型速查

### 1. OB 混淆 (obfuscator.io)

**识别特征**：
- 大量 `_0x` 前缀变量名（如 `_0x4a3b2c`）
- 顶部有十六进制字符串数组（`var _0xabc = ['...', '...', ...]`）
- 字符串数组旋转函数
- 十六进制属性访问（`obj['_0x1234']` 替代 `obj.method`）

**还原步骤**：
1. 定位字符串数组和旋转函数
2. 执行旋转函数得到最终字符串数组
3. 全局替换十六进制索引为实际字符串
4. 简化数学表达式和逻辑运算
5. 变量重命名提高可读性

**MCP 辅助**：
```
[camoufox-reverse] search_code(keyword="_0x") → 定位混淆入口
[camoufox-reverse] evaluate_js → 在浏览器中执行字符串数组还原
[camoufox-reverse] save_script → 保存到本地进一步分析
```

### 2. 控制流平坦化 (CFF)

**识别特征**：
```javascript
var state = initialState;
while (true) {
    switch (state) {
        case 'A': /* ... */ state = 'C'; break;
        case 'B': /* ... */ state = 'D'; break;
        case 'C': /* ... */ state = 'B'; break;
        // ...
    }
}
```

**还原步骤**：
1. 找到初始状态值
2. 按状态转移顺序排列代码块
3. 去掉 switch-case 包装，还原为顺序代码
4. 简化多余的变量赋值

**MCP 辅助**：
```
[camoufox-reverse] set_breakpoint_via_hook(target_function="状态机入口函数") → 设置伪断点
[camoufox-reverse] trace_function(function_path="状态机函数", log_args=true, log_return=true) → 追踪状态转移
[camoufox-reverse] get_trace_data → 查看状态值变化
```

### 3. eval / Function 打包

**识别特征**：
```javascript
eval(function(p,a,c,k,e,d){...}('encoded_string',...))
// 或
new Function('return ' + decryptedCode)()
```

**还原步骤**：
1. Hook `eval` 和 `Function` 构造器，拦截实际执行的代码
2. 或者将 `eval()` 替换为 `console.log()` 查看解密后的代码
3. 可能有多层嵌套，需要逐层解包

**MCP 辅助**：
```
[camoufox-reverse] add_init_script(script="eval/Function Hook")
[camoufox-reverse] get_console_logs → 读取解包后的代码
```

### 4. AAEncode

**识别特征**：
```javascript
ﾟωﾟﾉ= /｀ｍ´）ﾉ ~┻━┻   //*´∇｀*/ ['_'];
// 全是日文颜文字字符
```

**还原**：直接在浏览器 console 执行，或去掉最外层执行函数改为输出。

### 5. JJEncode

**识别特征**：
```javascript
$=~[];$={___:++$,$$$$:(![]+"")[$],...
// 全是 $ 和特殊字符
```

**还原**：同 AAEncode，直接执行或替换执行为输出。

### 6. JSFuck

**识别特征**：
```javascript
[][(![]+[])[+[]]+(![]+[])[!+[]+!+[]]+(![]+[])[+!+[]]+(!![]+[])[+[]]]...
// 仅使用 []()!+ 六种字符
```

**还原**：直接在浏览器执行。

### 7. 自定义 VM / 字节码解释器

**识别特征**：
- 超大数组作为"字节码"
- 解释器循环，包含 `switch` 或函数查找表
- 通常在 IIFE 中
- 无法通过简单字符串替换还原

**还原策略**：
1. **不要尝试反编译字节码**
2. 找到解释器的输入和输出接口
3. 通过 Hook 解释器的关键操作（函数调用、赋值、返回）来理解行为
4. 直接在 Node.js 中运行字节码解释器

**MCP 辅助**：
```
[camoufox-reverse] trace_function(function_path="解释器函数", log_args=true, log_return=true)
[camoufox-reverse] get_trace_data → 观察每步操作的输入输出
[camoufox-reverse] set_breakpoint_via_hook(target_function="解释器核心函数") → 捕获关键调用
```

### 8. JSVMP（JS 虚拟机保护）

**识别特征**：
- 超大 JS 文件（200KB+）
- 包含自定义解释器和操作码表
- 函数名和变量名完全无意义
- 改写浏览器原生 API

**还原策略**：
1. **不要反编译，通过 I/O 定位**
2. Hook 所有出口（XHR、Cookie、fetch）
3. 追踪加密函数的输入和输出
4. 用已知 I/O 反推算法

**MCP 辅助**：
```
[camoufox-reverse] inject_hook_preset(preset="xhr") → 一键 Hook XHR 请求
[camoufox-reverse] get_request_initiator(request_id=N) → 获取请求的 JS 调用栈（黄金路径）
[camoufox-reverse] add_init_script → 注入全局 Hook
```

## 通用反混淆 Node.js 工具

### 使用 AST 进行基础还原

```javascript
const parser = require('@babel/parser');
const traverse = require('@babel/traverse').default;
const generate = require('@babel/generator').default;
const t = require('@babel/types');

function deobfuscate(code) {
    const ast = parser.parse(code);
    
    traverse(ast, {
        // 还原十六进制字符串
        StringLiteral(path) {
            if (/^\\x/.test(path.node.extra?.raw || '')) {
                path.node.extra = undefined;
            }
        },
        // 还原计算属性为点号访问
        MemberExpression(path) {
            if (t.isStringLiteral(path.node.property) && /^[a-zA-Z_$]/.test(path.node.property.value)) {
                path.node.computed = false;
                path.node.property = t.identifier(path.node.property.value);
            }
        },
        // 折叠常量表达式
        BinaryExpression(path) {
            if (t.isNumericLiteral(path.node.left) && t.isNumericLiteral(path.node.right)) {
                const result = eval(`${path.node.left.value} ${path.node.operator} ${path.node.right.value}`);
                if (typeof result === 'number' && isFinite(result)) {
                    path.replaceWith(t.numericLiteral(result));
                }
            }
        }
    });
    
    return generate(ast, { comments: false }).code;
}
```

## 混淆代码分析的 MCP 工作流

```
1. save_script → 保存混淆代码到本地
2. search_code → 搜索可能的入口函数
3. set_breakpoint_via_hook → 在入口设伪断点
4. get_breakpoint_data → 查看捕获的参数和返回值
5. evaluate_js → 在浏览器执行还原操作
6. trace_function → 追踪关键函数调用链
```
