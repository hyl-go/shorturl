package aireport

// TaskType Asynq 任务名，与 worker 注册名一致。
const TaskType = "ai:report:generate"

// QueueName 报告生成使用独立队列，与访问日志/小时聚合解耦。
const QueueName = "reports"

// KeyPrefix Redis 中任务状态 key 前缀。
const KeyPrefix = "shorturl:aireport:"
