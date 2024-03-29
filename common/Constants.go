package common

const (
	//请求成功应答
	RESPONSE_SUCCESS = "Success!"
	//任务保存前缀
	JOB_SAVE_DIR = "/cron/jobList/"
	//强杀任务前缀
	JOB_KILL_DIR = "/cron/jobKillList"
	//任务锁前缀
	JOB_LOCK_DIR = "/cron/lock/"
	// 服务注册前缀
	JOB_WORKER_DIR = "/cron/workers/"

	// 保存任务事件
	JOB_EVENT_SAVE = 1

	// 删除任务事件
	JOB_EVENT_DELETE = -1

	// 强杀任务事件
	JOB_EVENT_KILL = 0

	// 强杀任务事件
	JOB_EVENT_CANCEL_KILL = -2
)
