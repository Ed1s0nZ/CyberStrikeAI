# CTF 资源精华速查

> 精选自 [awesome-ctf-resources](https://github.com/devploit/awesome-ctf-resources) 和 [awesome-ctf](https://github.com/apsdehal/awesome-ctf)
> 按 CTF 题目类型分类，只保留最实用的工具和资源。

---

## 综合框架

| 工具 | 用途 | 链接 |
|------|------|------|
| Pwntools | Exploit 开发框架（Python） | https://github.com/Gallopsled/pwntools |
| ctf-tools | 一键安装 CTF 工具集 | https://github.com/zardus/ctf-tools |
| Ciphey | AI 自动解密 | https://github.com/ciphey/ciphey |
| CyberChef | 在线编解码/加解密 | https://gchq.github.io/CyberChef/ |

---

## Web 类

### 工具
| 工具 | 用途 |
|------|------|
| Burp Suite | HTTP 拦截/重放/扫描 |
| SQLMap | SQL 注入 |
| XSStrike | XSS 检测 |
| dirsearch | 目录发现 |
| JWT_Tool | JWT 攻击 |
| SSRFmap | SSRF 利用 |

### 常见考点
- SQL 注入（联合查询/盲注/时间盲注/堆叠）
- XSS（反射/存储/DOM）
- SSRF（内网探测/云元数据）
- 文件上传（绕过后缀/MIME/内容检测）
- 反序列化（PHP/Java/Python pickle）
- 模板注入（SSTI）
- JWT 伪造/密钥混淆

### Payload 参考
- https://github.com/swisskyrepo/PayloadsAllTheThings
- https://book.hacktricks.wiki/

---

## Reverse 类

### 工具
| 工具 | 用途 |
|------|------|
| IDA Pro / Ghidra | 反编译 |
| radare2 / r2 | CLI 分析 |
| angr | 符号执行 |
| Frida | 动态 Hook |
| GDB + pwndbg | 调试 |
| uncompyle6 | Python 反编译 |
| jadx | Android 反编译 |
| dnSpy | .NET 反编译 |

### 常见考点
- 算法还原（加密/编码/自定义）
- 反调试/反虚拟机绕过
- 壳/混淆（UPX/VMProtect/OLLVM）
- 符号执行求解约束
- 动态 Hook 绕过检查
- Go/Rust 逆向（符号恢复）

---

## Pwn 类

### 工具
| 工具 | 用途 |
|------|------|
| Pwntools | Exploit 编写 |
| GDB + pwndbg/GEF | 调试 |
| ROPgadget | ROP 链构造 |
| one_gadget | libc one-shot |
| checksec | 保护检测 |
| LibcSearcher | libc 版本识别 |

### 常见考点
- 栈溢出（ret2text/ret2libc/ret2shellcode/ROP）
- 堆利用（UAF/double free/tcache/fastbin）
- 格式化字符串（任意读写）
- 整数溢出
- 内核 Pwn（提权/条件竞争）
- 沙箱逃逸（seccomp bypass）

### 常用 payload 模式
```python
# ret2libc 模板
from pwn import *
elf = ELF('./vuln')
libc = ELF('./libc.so.6')
p = process('./vuln')
# leak libc base → calculate system/binsh → overwrite ret
```

---

## Crypto 类

### 工具
| 工具 | 用途 |
|------|------|
| SageMath | 数学计算 |
| RsaCtfTool | RSA 自动攻击 | 
| hashcat/john | 哈希破解 |
| CyberChef | 编解码 |
| z3 (SMT solver) | 约束求解 |

### 常见考点
- RSA（小公钥指数/共模/Wiener/Coppersmith）
- AES（ECB/CBC padding oracle/bit flipping）
- 古典密码（Caesar/Vigenere/置换）
- 哈希长度扩展攻击
- 椭圆曲线（ECDSA nonce 复用）
- 格密码（LLL/CVP）

---

## Forensics 类

### 工具
| 工具 | 用途 |
|------|------|
| Volatility | 内存取证 |
| Autopsy/Sleuth Kit | 磁盘取证 |
| Wireshark | 流量分析 |
| binwalk | 固件/文件提取 |
| foremost | 文件恢复 |
| exiftool | 元数据提取 |

### 常见考点
- 内存 dump 分析（进程/密码/恶意代码）
- PCAP 流量分析（HTTP/DNS/TCP 重组）
- 文件系统分析（删除文件恢复/隐藏分区）
- 日志分析（Web 日志/系统日志）
- 磁盘镜像分析

---

## Misc/Stego 类

### 工具
| 工具 | 用途 |
|------|------|
| StegSolve | 图片隐写分析 |
| zsteg | PNG/BMP 隐写 |
| steghide | JPEG 隐写 |
| Audacity | 音频分析 |
| strings/xxd | 基础分析 |
| file/binwalk | 文件类型识别 |

### 常见考点
- LSB 隐写（图片最低有效位）
- 文件头修复/拼接
- 二维码/条形码
- 音频频谱图隐写
- ZIP 伪加密/已知明文攻击
- 编码识别（Base64/Hex/Morse/Braille）

---

## 在线平台

| 平台 | 特点 | 链接 |
|------|------|------|
| CTFTime | 赛事日历 + writeup | https://ctftime.org/ |
| HackTheBox | 实战靶机 | https://www.hackthebox.com/ |
| TryHackMe | 引导式学习 | https://tryhackme.com/ |
| PicoCTF | 入门友好 | https://picoctf.org/ |
| pwnable.kr | Pwn 专项 | http://pwnable.kr/ |
| cryptopals | Crypto 专项 | https://cryptopals.com/ |
| OverTheWire | War 系列挑战 | https://overthewire.org/ |
| Root-Me | 综合挑战 | https://www.root-me.org/ |

---

## Writeup 资源

| 资源 | 链接 |
|------|------|
| CTFTime Writeups | https://ctftime.org/writeups |
| 0xdf hacks stuff | https://0xdf.gitlab.io/ |
| LiveOverflow (YouTube) | https://www.youtube.com/c/LiveOverflow |
| John Hammond (YouTube) | https://www.youtube.com/c/JohnHammond010 |
| IppSec (HTB walkthrough) | https://www.youtube.com/c/ippsec |
