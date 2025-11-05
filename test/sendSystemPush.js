import http from 'k6/http';
import { check, sleep } from 'k6';
import encoding from 'k6/encoding';

// 预定义中文聊天语句列表
const chinesePhrases = [
  "你好啊，今天过得怎么样？",
  "最近在看什么好看的电视剧吗？",
  "今天天气真不错，适合出去走走。",
  "吃饭了吗？",
  "工作好忙啊，感觉快累垮了。",
  "周末有什么计划吗？",
  "哈哈，这个笑话太搞笑了！",
  "我刚学会做一道新菜，味道还不错。",
  "你去过上海吗？那边的外滩很漂亮。",
  "最近压力有点大，想找人聊聊天。",
  "听说新上映的电影很不错，要不要一起去看？",
  "今天学到了一个新知识，感觉很有意思。",
  "你的新发型真好看！",
  "明天一起去爬山吧？",
  "这个项目什么时候能完成？"
];

// 生成随机用户ID (uid + 10位字母数字)
function randomUserId() {
  const chars = 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789';
  let result = 'uid';
  for (let i = 0; i < 10; i++) {
    result += chars.charAt(Math.floor(Math.random() * chars.length));
  }
  return result;
}

// 生成随机 content_id (content + 3位数字)
function randomContentId() {
  const digits = '0123456789';
  let result = 'content';
  for (let i = 0; i < 3; i++) {
    result += digits.charAt(Math.floor(Math.random() * digits.length));
  }
  return result;
}

// 主 VU 函数
export default function () {
  // 1. 随机选择中文内容
  const rawContent = chinesePhrases[Math.floor(Math.random() * chinesePhrases.length)];

  // 2. Base64 编码（k6 自动处理 UTF-8）
  const encodedContent = encoding.b64encode(rawContent, 'utf8');

  // 3. 时间戳（秒级）
  const now = Math.floor(Date.now() / 1000); // k6 使用毫秒，转为秒
  const expireTime = now + 86400; // 1天后过期

  // 4. 随机字段
  const contentId = randomContentId();
  const fromUserId = randomUserId();
  const toUserIds = ["uidhSSWsdYgB9"]; // 固定目标用户（与你的 Lua 脚本一致）
  const pushType = String(Math.floor(Math.random() * 3) + 1); // "1", "2", or "3"

  // 5. 构建 JSON body
  const payload = {
    content_id: contentId,
    content: encodedContent,
    timestamp: now,
    push_type: pushType,
    from_user_id: fromUserId,
    to_user_ids: toUserIds,
    expire_time: String(expireTime), // 注意：你的 Lua 脚本中 expire_time 是字符串
  };

  const url = 'http://localhost:8034/chatify/logic/v1/sendSystemPush';
  const params = {
    headers: {
      'Content-Type': 'application/json',
    },
  };

  // 发送请求
  const res = http.post(url, JSON.stringify(payload), params);

  // 可选：检查响应状态
  check(res, {
    'status is 200': (r) => r.status === 200,
  });

  // 可选：模拟思考时间（如果需要）
  // sleep(0.1);
}