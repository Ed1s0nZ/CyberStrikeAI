"""
签名函数封装
根据逆向分析结论修改具体的签名逻辑
"""
import hashlib
import hmac
import base64
import json
import time

# 如需执行提取的 JS 代码
# import execjs


def md5(text: str) -> str:
    """MD5 哈希（返回 32 位小写 hex）"""
    return hashlib.md5(text.encode("utf-8")).hexdigest()


def sha256(text: str) -> str:
    """SHA-256 哈希（返回 64 位小写 hex）"""
    return hashlib.sha256(text.encode("utf-8")).hexdigest()


def hmac_sha256(key: str, message: str) -> str:
    """HMAC-SHA256 签名（返回 hex）"""
    return hmac.new(
        key.encode("utf-8"),
        message.encode("utf-8"),
        hashlib.sha256
    ).hexdigest()


def aes_encrypt(plaintext: str, key: str, iv: str) -> str:
    """AES-CBC 加密（返回 Base64）"""
    from Crypto.Cipher import AES
    from Crypto.Util.Padding import pad

    cipher = AES.new(
        key.encode("utf-8"),
        AES.MODE_CBC,
        iv.encode("utf-8")
    )
    padded = pad(plaintext.encode("utf-8"), AES.block_size)
    encrypted = cipher.encrypt(padded)
    return base64.b64encode(encrypted).decode("utf-8")


def aes_decrypt(ciphertext_b64: str, key: str, iv: str) -> str:
    """AES-CBC 解密"""
    from Crypto.Cipher import AES
    from Crypto.Util.Padding import unpad

    cipher = AES.new(
        key.encode("utf-8"),
        AES.MODE_CBC,
        iv.encode("utf-8")
    )
    decrypted = cipher.decrypt(base64.b64decode(ciphertext_b64))
    return unpad(decrypted, AES.block_size).decode("utf-8")


def generate_sign(params: dict, secret_key: str) -> str:
    """
    生成请求签名（根据实际逻辑修改）

    示例签名逻辑：
      1. 参数按 key 字典序排序
      2. 拼接为 key1=value1&key2=value2 格式
      3. 末尾追加密钥
      4. 对拼接字符串做 MD5

    Args:
        params: 请求参数字典
        secret_key: 签名密钥
    Returns:
        签名字符串
    """
    sorted_params = sorted(params.items(), key=lambda x: x[0])
    param_str = "&".join(f"{k}={v}" for k, v in sorted_params)
    sign_str = f"{param_str}&key={secret_key}"

    # 打印签名明文（调试用，正式使用时删除）
    print(f"  签名明文: {sign_str}")

    result = md5(sign_str)
    print(f"  签名结果: {result}")
    return result


def generate_m(params: dict, encrypt_key: str, iv: str) -> str:
    """
    生成加密参数 m（根据实际逻辑修改）

    示例：将参数 JSON 序列化后 AES 加密
    """
    plaintext = json.dumps(params, separators=(",", ":"), sort_keys=True)
    return aes_encrypt(plaintext, encrypt_key, iv)


# ====== execjs 方式（当 JS 逻辑过于复杂时使用）======
#
# def generate_sign_via_js(params: dict) -> str:
#     """使用 execjs 执行提取的 JS 代码生成签名"""
#     with open("config/sign_logic.js", "r", encoding="utf-8") as f:
#         js_code = f.read()
#     ctx = execjs.compile(js_code)
#     return ctx.call("generateSign", params)
