"""
主入口脚本 - 纯 Python 协议请求
使用前请修改 CONFIG 中的配置和 utils/sign.py 中的签名逻辑
"""
import json
import time
from utils.sign import generate_sign, generate_m
from utils.request import RequestClient


CONFIG = {
    "base_url": "https://example.com",
    "api_path": "/api/data",
    "cookies": {
        "session_id": "your_session_id_here",
    },
    "total_pages": 5,
    "delay": 1.5,
}

# 从配置文件加载 Headers 模板
with open("config/headers.json", "r", encoding="utf-8") as f:
    HEADERS_TEMPLATE = json.load(f)

# 从配置文件加载密钥
with open("config/keys.json", "r", encoding="utf-8") as f:
    KEYS = json.load(f)


def fetch_page(client: RequestClient, page: int) -> dict:
    """请求单页数据"""
    timestamp = str(int(time.time()))
    params = {
        "page": str(page),
        "ts": timestamp,
    }

    # 计算签名（根据实际逻辑修改）
    sign = generate_sign(params, KEYS.get("sign_key", ""))
    params["sign"] = sign

    # 如需加密参数 m
    # m = generate_m(params, KEYS.get("encrypt_key", ""), KEYS.get("iv", ""))
    # params["m"] = m

    url = f"{CONFIG['base_url']}{CONFIG['api_path']}"
    response = client.get(url, params=params)
    return response.json()


def extract_data(response_data: dict) -> list:
    """从响应中提取目标数据（根据实际数据结构修改）"""
    return response_data.get("data", {}).get("list", [])


def calculate_result(all_data: list):
    """汇总计算结果（根据实际需求修改）"""
    print(f"\n采集完成，共 {len(all_data)} 条数据")
    for i, item in enumerate(all_data[:3]):
        print(f"  [{i+1}] {item}")
    if len(all_data) > 3:
        print(f"  ... 共 {len(all_data)} 条")


def main():
    client = RequestClient(
        cookies=CONFIG["cookies"],
        headers=HEADERS_TEMPLATE,
    )

    all_data = []

    for page in range(1, CONFIG["total_pages"] + 1):
        try:
            print(f"正在请求第 {page} 页...")
            response_data = fetch_page(client, page)

            # 打印关键中间值（调试用）
            print(f"  状态: {response_data.get('code', 'unknown')}")

            page_data = extract_data(response_data)
            if not page_data:
                print(f"  第 {page} 页无数据，停止翻页")
                break

            all_data.extend(page_data)
            print(f"  获取 {len(page_data)} 条数据")

            if page < CONFIG["total_pages"]:
                time.sleep(CONFIG["delay"])

        except Exception as e:
            print(f"  第 {page} 页请求失败: {e}")
            if hasattr(e, "response") and e.response is not None:
                print(f"  响应状态码: {e.response.status_code}")
                print(f"  响应内容: {e.response.text[:200]}")
            break

    calculate_result(all_data)


if __name__ == "__main__":
    main()
