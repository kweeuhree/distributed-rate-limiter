-- Input parameters
local tokens_key = KEYS[1]..":tokens"           -- Key for the bucket's token counter
local last_timestamp_key = KEYS[1]..":timestamp"   -- Key for the bucket's last access time

local max_tokens = tonumber(ARGV[1])  -- Maximum number of tokens allowed 
local window_size = tonumber(ARGV[2]) -- Window size (expiration time)
local now = tonumber(ARGV[3])         -- Current timestamp in microseconds
local requested = tonumber(ARGV[4])   -- Tokens requested (1 per request)

-- Convert window size to microseconds for token replenishment calculations
local expiration_in_micros = window_size * 1000000

-- Fetch the current token count
local last_tokens = tonumber(redis.call("get", tokens_key))
if last_tokens == nil then
    -- Initialize token count for new user with full capacity
    last_tokens = max_tokens
    redis.call("setex", tokens_key, window_size, last_tokens)
end

-- Fetch the last access time
local last_access = tonumber(redis.call("get", last_timestamp_key))
if last_access == nil then
    -- Initialize last access time for new user with current time
    last_access = now
    redis.call("setex", last_timestamp_key, window_size, last_access)
end

-- Calculate tokens added since last request based on elapsed time and window size
local elapsed = math.max(0, now - last_access)
local add_tokens = math.floor(elapsed * max_tokens / expiration_in_micros) 
local new_tokens = math.min(max_tokens, last_tokens + add_tokens)

-- Check if enough tokens are available
local allowed = new_tokens >= requested

if allowed then
    new_tokens = new_tokens - requested
    -- Update tokens and last access time with current values
    redis.call("setex", tokens_key, window_size, new_tokens)
    redis.call("setex", last_timestamp_key, window_size, now) 
end

-- Return 1 if the operation is allowed, 0 otherwise.
return allowed and 1 or 0