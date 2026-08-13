"""
HTTP 请求封装
包含 Cookie 管理、重试机制、频率控制
"""
import time
import random
import requests
from typing import Optional


class RequestClient:
    """HTTP 客户端封装"""

    def __init__(
        self,
        cookies: Optional[dict] = None,
        headers: Optional[dict] = None,
        max_retries: int = 3,
        retry_delay: float = 2.0,
    ):
        self.session = requests.Session()
        self.max_retries = max_retries
        self.retry_delay = retry_delay

        default_headers = {
            "User-Agent": (
                "Mozilla/5.0 (Windows NT 10.0; Win64; x64) "
                "AppleWebKit/537.36 (KHTML, like Gecko) "
                "Chrome/120.0.0.0 Safari/537.36"
            ),
            "Accept": "application/json, text/plain, */*",
            "Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
            "Accept-Encoding": "gzip, deflate, br",
            "Connection": "keep-alive",
        }

        if headers:
            default_headers.update(headers)
        self.session.headers.update(default_headers)

        if cookies:
            for name, value in cookies.items():
                self.session.cookies.set(name, value)

    def request(self, method: str, url: str, **kwargs) -> requests.Response:
        """带重试的请求"""
        last_error = None

        for attempt in range(1, self.max_retries + 1):
            try:
                response = self.session.request(method, url, **kwargs)

                if response.status_code == 429:
                    wait = self.retry_delay * (2 ** attempt) + random.uniform(0, 1)
                    print(f"  触发频率限制，等待 {wait:.1f}s 后重试 ({attempt}/{self.max_retries})")
                    time.sleep(wait)
                    continue

                if response.status_code in (403, 412):
                    print(f"  请求被拒绝 ({response.status_code})，可能需要检查 Cookie 或签名")
                    response.raise_for_status()

                if response.status_code >= 500:
                    wait = self.retry_delay * attempt + random.uniform(0, 1)
                    print(f"  服务端错误 ({response.status_code})，等待 {wait:.1f}s 后重试 ({attempt}/{self.max_retries})")
                    time.sleep(wait)
                    continue

                return response

            except requests.RequestException as e:
                last_error = e
                if attempt < self.max_retries:
                    wait = self.retry_delay * attempt
                    print(f"  请求异常: {e}，等待 {wait:.1f}s 后重试 ({attempt}/{self.max_retries})")
                    time.sleep(wait)

        raise last_error or Exception("请求失败，已达最大重试次数")

    def get(self, url: str, **kwargs) -> requests.Response:
        return self.request("GET", url, **kwargs)

    def post(self, url: str, **kwargs) -> requests.Response:
        return self.request("POST", url, **kwargs)

    def request_with_delay(
        self, method: str, url: str, delay: float = 1.0, **kwargs
    ) -> requests.Response:
        """带延迟的请求（避免触发频率限制）"""
        jitter = random.uniform(0, delay * 0.3)
        time.sleep(delay + jitter)
        return self.request(method, url, **kwargs)

    def set_cookie(self, name: str, value: str, domain: str = ""):
        """设置 Cookie"""
        self.session.cookies.set(name, value, domain=domain or None)

    def update_cookies_from_response(self, response: requests.Response):
        """从响应中更新 Cookie"""
        self.session.cookies.update(response.cookies)

    def get_cookie_string(self) -> str:
        """获取 Cookie 字符串"""
        return "; ".join(f"{c.name}={c.value}" for c in self.session.cookies)


# ====== curl_cffi 版本（对抗 TLS 指纹检测时使用）======
#
# from curl_cffi import requests as curl_requests
#
# class CurlRequestClient:
#     """使用 curl_cffi 的 HTTP 客户端，支持浏览器 TLS 指纹模拟"""
#
#     def __init__(self, impersonate: str = "chrome120"):
#         self.session = curl_requests.Session(impersonate=impersonate)
#
#     def get(self, url: str, **kwargs):
#         return self.session.get(url, **kwargs)
#
#     def post(self, url: str, **kwargs):
#         return self.session.post(url, **kwargs)
