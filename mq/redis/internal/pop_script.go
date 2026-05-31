package internal

import "github.com/redis/go-redis/v9"

// PopScript 原子性 Pop 脚本：先从 backup 列表取数据，backup 为空则从主队列阻塞取并写入 backup
// 使用 BLMOVE 替代已废弃的 BRPOPLPUSH（Redis 6.2+ 废弃）
var PopScript = redis.NewScript(`
local backup_key = KEYS[2]
local main_key = KEYS[1]
local timeout = tonumber(ARGV[1])

-- 先尝试从 backup 获取
local bak = redis.call('RPOP', backup_key)
if bak then
    return bak
end

-- backup 为空，从主队列阻塞获取
-- BLMOVE source destination LEFT RIGHT timeout 等价于 BRPOPLPUSH source destination timeout
local result = redis.call('BLMOVE', main_key, backup_key, 'LEFT', 'RIGHT', timeout)
return result
`)
