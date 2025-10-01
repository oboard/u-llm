import asyncio
import aiohttp
import csv
import time
import os
from typing import List, Dict

# 全局配置
BASE_URL = "https://cloudsearchapi.ulearning.cn/assistant/checkPermissionById"
AUTH_HEADER = "88017833834BB8342D32241E1EBD7246"
CONCURRENCY = 20                       # 降低并发数，避免被限流
CSV_FILE = "authorized_assistants.csv"  # 结果追加写入
TIMEOUT = aiohttp.ClientTimeout(total=10)

# 请求头
HEADERS = {
    "Authorization": AUTH_HEADER,
    "User-Agent": "ulearning-scan/1.0"
}

# 进度计数和文件锁
processed_count = 0
found_count = 0
total_count = 10001
csv_lock = asyncio.Lock()


async def write_to_csv(aid: int, name: str, model_ids: List[int]) -> None:
    """实时写入CSV文件"""
    global found_count
    async with csv_lock:
        # 检查是否需要写表头
        file_exists = os.path.exists(CSV_FILE) and os.path.getsize(CSV_FILE) > 0
        with open(CSV_FILE, "a", newline="", encoding="utf-8") as f:
            writer = csv.writer(f)
            if not file_exists:
                writer.writerow(["id", "name", "models.id"])
            writer.writerow([aid, name, ",".join(map(str, model_ids))])
        found_count += 1
        print(f"✓ 写入: ID {aid} - {name} (已找到 {found_count} 个)")


async def fetch(session: aiohttp.ClientSession, aid: int) -> None:
    """单 ID 探测"""
    global processed_count
    url = f"{BASE_URL}?id={aid}&domain="
    try:
        async with session.get(url, headers=HEADERS, timeout=TIMEOUT) as resp:
            if resp.status == 200:
                data = await resp.json()
                # 判定有权限：code == 1
                if data.get("code") == 1 and "result" in data:
                    res = data["result"]
                    aid_out = res["id"]
                    name = res["name"]
                    # 收集所有启用的模型 id
                    model_ids = [
                        m["modelId"]
                        for m in res.get("modelInfoList", [])
                        if m.get("enable") == 1
                    ]
                    # 立即写入CSV
                    await write_to_csv(aid_out, name, model_ids)
    except asyncio.TimeoutError:
        print(f"✗ 超时: ID {aid}")
    except Exception as e:
        print(f"✗ 错误: ID {aid} - {type(e).__name__}")
    finally:
        processed_count += 1
        if processed_count % 100 == 0:
            progress = (processed_count / total_count) * 100
            print(f"进度: {processed_count}/{total_count} ({progress:.1f}%)")


async def worker(queue: asyncio.Queue, session: aiohttp.ClientSession) -> None:
    """并发 Worker：不断从队列取 ID 探测"""
    while True:
        aid = await queue.get()
        await fetch(session, aid)
        queue.task_done()


async def main() -> None:
    """主协程：0-10000 快速扫描"""
    print(f"开始扫描 ID 0-{total_count-1}，并发数: {CONCURRENCY}")
    print("=" * 50)
    
    queue: asyncio.Queue[int] = asyncio.Queue()

    # 预填充队列
    for i in range(total_count):
        queue.put_nowait(i)

    async with aiohttp.ClientSession() as session:
        # 启动 worker 任务
        tasks = [
            asyncio.create_task(worker(queue, session))
            for _ in range(CONCURRENCY)
        ]
        await queue.join()  # 等待所有 ID 处理完
        # 清理 worker
        for t in tasks:
            t.cancel()
        await asyncio.gather(*tasks, return_exceptions=True)

    print("=" * 50)
    print(f"扫描完成！")
    print(f"总扫描: {processed_count}/{total_count}")
    print(f"发现授权: {found_count} 条")
    print(f"结果保存: {CSV_FILE}")


if __name__ == "__main__":
    s = time.time()
    asyncio.run(main())
    elapsed = time.time() - s
    print(f"总耗时: {elapsed:.2f}秒")
    if processed_count > 0:
        print(f"平均速度: {processed_count/elapsed:.1f} 请求/秒")