package common

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/gorhill/cronexpr"
)

type JobEvent struct {
	EventType int
	EventJob  *Job
}

//任务的结构体
type Job struct {
	Name     string `json:"name"`     // 任务名
	Command  string `json:"command"`  //shell命令
	CronExpr string `json:"cronExpr"` //cron表达式
}

//HTTP接口应答
type Response struct {
	ErrNo int         `json:"err_no"`
	Msg   string      `json:"msg"`
	Data  interface{} `json:"data"`
}

//任务调度计划
type JobSchedulePlan struct {
	Job      *Job                 //要调度的任务信息
	Expr     *cronexpr.Expression //解析好的cronexpr
	NextTime time.Time            //下次调度时间
}

//任务执行状态
type JobExecuteInfo struct {
	Job        *Job               //任务信息
	PlanTime   time.Time          //理论上的调度时间
	RealTime   time.Time          //实际的调度时间
	CancelCtx  context.Context    // 任务command的context
	CancelFunc context.CancelFunc //  用于取消command执行的cancel函数
}

//任务执行结果
type JobExecuteResult struct {
	ExecuteInfo *JobExecuteInfo //执行状态
	Output      []byte          //脚本输入
	Err         error           //脚本错误原因
	StartTime   time.Time       //启动时间
	EndTime     time.Time       //结束时间
}

//任务执行日志
type JobLog struct {
	JobName      string `json:"jobName" bson:"jobName"`           // 任务名字
	Command      string `json:"command" bson:"command"`           // 脚本命令
	Err          string `json:"err" bson:"err"`                   // 错误原因
	Output       string `json:"output" bson:"output"`             // 脚本输出
	PlanTime     int64  `json:"planTime" bson:"planTime"`         // 计划开始时间
	ScheduleTime int64  `json:"scheduleTime" bson:"scheduleTime"` // 实际调度时间
	StartTime    int64  `json:"startTime" bson:"startTime"`       // 任务执行开始时间
	EndTime      int64  `json:"endTime" bson:"endTime"`           // 任务执行结束时间
}

//日志批次
type LogBatch struct {
	JobLogs []interface{}
}

// 任务日志过滤条件
type JobLogFilter struct {
	JobName string `bson:"jobName"`
}

// 任务日志排序规则
type JobLogSort struct {
	//例：降序 {startTime: -1}
	StartTime int `bson:"startTime"`
}

//>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
//建立应答方法
func BuildResponse(errNo int, msg string, data interface{}) (resp []byte, err error) {
	response := Response{
		ErrNo: errNo,
		Msg:   msg,
		Data:  data,
	}
	bytes, err := json.Marshal(response)
	log.Println("返回的数据：", data, " 全部数据： ", string(bytes))
	return bytes, err
}

//反序列化job
func UnpackJob(jobBytes []byte) (job *Job, err error) {
	tempJob := &Job{}
	err = json.Unmarshal(jobBytes, tempJob)
	if err != nil {
		return nil, err
	}
	return tempJob, err
}

//封装一个事件
func BuildJobEvent(eventType int, job *Job) *JobEvent {
	return &JobEvent{
		EventType: eventType,
		EventJob:  job,
	}
}

//cron/jobList/job1 -> job1
func ExtractString(jobName string, prefix string) string {
	return strings.TrimPrefix(jobName, prefix)
}

//生成任务调度计划对象
func BuildJobSchedulePlan(job *Job) (jobSchedulePlan *JobSchedulePlan, err error) {
	var (
		expr *cronexpr.Expression
	)

	//解析JOB的cron表达式
	expr, err = cronexpr.Parse(job.CronExpr)
	if err != nil {
		return nil, err
	}

	//生成任务调度计划对象
	jobSchedulePlan = &JobSchedulePlan{
		Job:      job,
		Expr:     expr,
		NextTime: expr.Next(time.Now()),
	}
	return jobSchedulePlan, nil
}

//构造执行状态信息
func BuildJobExecuteInfo(jobSchedulePlan *JobSchedulePlan) (jobExecuteInfo *JobExecuteInfo) {
	jobExecuteInfo = &JobExecuteInfo{
		Job:      jobSchedulePlan.Job,
		PlanTime: jobSchedulePlan.NextTime, //计算调度时间
		RealTime: time.Now(),               //真实调度时间
	}
	jobExecuteInfo.CancelCtx, jobExecuteInfo.CancelFunc = context.WithCancel(context.TODO())
	return
}

//构造执行状态信息
func BuildJobExecuteResult(executeInfo *JobExecuteInfo, output []byte, err error, startTime time.Time, endTime time.Time) (jobExecuteResult *JobExecuteResult) {
	jobExecuteResult = &JobExecuteResult{
		ExecuteInfo: executeInfo, //执行状态
		Output:      output,      //脚本输入
		Err:         err,         //脚本错误原因
		StartTime:   startTime,   //启动时间
		EndTime:     endTime,     //结束时间
	}
	return
}
