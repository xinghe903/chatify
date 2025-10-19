
-- wrk -t12 -c24 -d10s --script=param.lua --latency "http://ip:8034/chatify/logic/v1/sendSystemPush"

-- param.lua
-- Base64 编码函数 (纯 Lua 实现)
local function base64_encode(data)
    local b='ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/'
    return ((data:gsub('.', function(x)
        local r,b='',x:byte()
        for i=8,1,-1 do r=r..(b%2^i-b%2^(i-1)>0 and '1' or '0') end
        return r;
    end)..'0000'):gsub('%d%d%d?%d?%d?%d?', function(x)
        if (#x < 6) then return '' end
        local c=0
        for i=1,6 do c=c+(x:sub(i,i)=='1' and 2^(6-i) or 0) end
        return b:sub(c+1,c+1)
    end)..({ '', '==', '=' })[#data%3+1])
end

-- 预定义的中文聊天语句列表
local chinese_phrases = {
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
    -- 可以继续添加更多句子...
}

-- 生成随机用户ID (字母数字组合)
local function random_user_id()
    local chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
    local result = "uid"
    for i = 1, 10 do
        result = result .. chars:sub(math.random(1, #chars), math.random(1, #chars))
    end
    return result
end

-- 生成随机 content_id
local function random_content_id()
    local chars = "0123456789"
    local result = "content"
    for i = 1, 3 do
        result = result .. chars:sub(math.random(1, #chars), math.random(1, #chars))
    end
    return result
end

-- wrk request 函数 (每次请求调用)
request = function()
    print("---- DEBUG: request function called ----") -- 添加调试打印
    -- 1. 随机选择一个中文聊天内容
    local raw_content = chinese_phrases[math.random(#chinese_phrases)]

    -- 2. 对中文内容进行 Base64 编码
    local encoded_content = base64_encode(raw_content)

    -- 3. 获取当前时间戳 (秒级)，用于 timestamp 和 expire_time
    local now = os.time() -- 当前时间戳 (秒)
    local expire_time = now + 86400 -- 过期时间：当前时间 + 1天 (86400秒)

    -- 4. 生成随机字段
    local content_id = random_content_id()
    local from_user_id = random_user_id()
    local to_user_id = random_user_id() -- 简化：只推送给一个用户
    local push_type = tostring(math.random(1, 3)) -- 假设 push_type 为 1, 2, 3

    -- 5. 构建符合您示例的动态 JSON body
    -- 注意：JSON 中字符串需要用双引号包围，使用 [[ ]] 可以避免转义
    local dynamic_body = [[
{
  "content_id": "]] .. content_id .. [[",
  "content": "]] .. encoded_content .. [[",
  "timestamp": ]] .. now .. [[,
  "push_type": "]] .. push_type .. [[",
  "from_user_id": "]] .. from_user_id .. [[",
  "to_user_ids": [
    "uidhSSWsdYgB9"
  ],
  "expire_time": "]] .. expire_time .. [["
}
]]
    local headers = {}
    headers["Content-Type"] = "application/json"
    print("Final body: ", dynamic_body) -- 打印最终的 body
    -- 返回格式化的 POST 请求
    return wrk.format("POST", nil, headers, dynamic_body)
end