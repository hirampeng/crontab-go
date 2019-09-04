package worker

import (
	"fmt"
	"github.com/hirampeng/crontab-go/common"
	"log"
	"time"
)

//任务调度
type Scheduler struct {
	jobEventChan         chan *common.JobEvent
	jobExecuteResultChan chan *common.JobExecuteResult
	jobPlanTable         map[string]*common.JobSchedulePlan
	jobExecutingTable    map[string]*common.JobExecuteInfo
}

//处理任务事件
func (scheduler *Scheduler) handleJobEvent(jobEvent *common.JobEvent) {
	switch jobEvent.EventType {
	case common.JOB_EVENT_SAVE: //保存任务事件
		jobSchedulePlan, err := common.BuildJobSchedulePlan(jobEvent.EventJob)
		if err == nil {
			scheduler.jobPlanTable[jobEvent.EventJob.Name] = jobSchedulePlan
		}
	case common.JOB_EVENT_DELETE: //删除任务事件
		_, jobExisted := scheduler.jobPlanTable[jobEvent.EventJob.Name]
		if jobExisted {
			delete(scheduler.jobPlanTable, jobEvent.EventJob.Name)
		}
	case common.JOB_EVENT_KILL: //强杀任务事件 取消掉Command执行
		//判断任务是否在计划列表中
		_, jobExisted := scheduler.jobPlanTable[jobEvent.EventJob.Name]
		if jobExisted { //如果存在就删除任务 并且取消执行
			//delete(scheduler.jobPlanTable, jobEvent.EventJob.Name)
			jobExecuteInfo, jobIsExecuting := scheduler.jobExecutingTable[jobEvent.EventJob.Name]
			if jobIsExecuting {
				jobExecuteInfo.CancelFunc() //取消common执行，杀死shell子进程
			}
		}
	}
}

//调度协程
func (scheduler *Scheduler) scheduleLoop() {

	//处理过期任务并计算下次调度时间
	schedulerAfter := scheduler.TrySchedule()

	//调度的延时定时器
	scheduleTimer := time.NewTimer(schedulerAfter)

	//定时任务common.Job
	for {
		select {
		case jobEvent := <-scheduler.jobEventChan:
			//对我们维护的列表做增删改查
			scheduler.handleJobEvent(jobEvent)

		case <-scheduleTimer.C: //最近的任务到期了
			//调度一次任务，并计算下次调度任务的时间
			schedulerAfter = scheduler.TrySchedule()
			scheduleTimer.Reset(schedulerAfter)

		case jobExecuteResult := <-scheduler.jobExecuteResultChan:
			//调度任务完成
			scheduler.handleJobExecuteResult(jobExecuteResult)

		}
	}
}

//推送任务变化事件
func (scheduler *Scheduler) PushJobEvent(jobEvent *common.JobEvent) {
	scheduler.jobEventChan <- jobEvent
}

//推送任务结果
func (scheduler *Scheduler) PushJobExecuteResult(jobExecuteResult *common.JobExecuteResult) {
	scheduler.jobExecuteResultChan <- jobExecuteResult
}

//重新计算任务调度状况
func (scheduler *Scheduler) TrySchedule() (schedulerAfter time.Duration) {
	//如果没有任务 返回一秒，让他睡一秒
	if len(scheduler.jobPlanTable) == 0 {
		return 1 * time.Second
	}
	//当前时间
	now := time.Now()
	//最近一个任务要过期的时 间
	var nearTime *time.Time
	//1,遍历所有任务
	for _, jobPlan := range scheduler.jobPlanTable {
		if jobPlan.NextTime.Before(now) || jobPlan.NextTime.Equal(now) {
			//2,过期的任务立即执行
			//log.Println("执行任务：", jobPlan.Job.Name)
			scheduler.TryStartJob(jobPlan)
			jobPlan.NextTime = jobPlan.Expr.Next(now) //更新任务下次执行时间
		}
		//3,统计最近的要过期的任务的时间，返回最近要过期任务的时间
		if nearTime == nil || jobPlan.NextTime.Before(*nearTime) {
			nearTime = &jobPlan.NextTime
		}
	}

	return (*nearTime).Sub(time.Now())
}

//尝试执行任务
func (scheduler *Scheduler) TryStartJob(jobSchedulePlan *common.JobSchedulePlan) {
	//调度和执行时两件事情

	//执行任务可能会需要很久， 1分钟可能会调度60次， 但是只能执行一次，防止并发！

	//如果任务正在执行，跳过本次调度
	jobExecuteInfo, jobExecuting := scheduler.jobExecutingTable[jobSchedulePlan.Job.Name]
	if !jobExecuting {
		//构建执行状态信息

		jobExecuteInfo := common.BuildJobExecuteInfo(jobSchedulePlan)
		scheduler.jobExecutingTable[jobSchedulePlan.Job.Name] = jobExecuteInfo
		//执行任务
		//TODO
		G_executor.ExecuteJob(jobExecuteInfo)
		//fmt.Println("执行任务:", jobExecuteInfo.Job.Name, jobExecuteInfo.PlanTime, jobExecuteInfo.RealTime)
		return
	}

	fmt.Println("尚未退出，跳过执行：", jobExecuteInfo.Job.Name)
}

//处理任务结果
func (scheduler *Scheduler) handleJobExecuteResult(jobExecuteResult *common.JobExecuteResult) {
	//删除任务执行状态
	delete(scheduler.jobExecutingTable, jobExecuteResult.ExecuteInfo.Job.Name)
	//生成执行日志
	if jobExecuteResult.Err != common.ERR_LOCK_ALREADY_REQUIRED {
		jobLog := &common.JobLog{
			JobName:      jobExecuteResult.ExecuteInfo.Job.Name,
			Command:      jobExecuteResult.ExecuteInfo.Job.Command,
			Output:       string(jobExecuteResult.Output),
			PlanTime:     jobExecuteResult.ExecuteInfo.PlanTime.Unix(), //统一用unix存储时间
			ScheduleTime: jobExecuteResult.ExecuteInfo.RealTime.Unix(),
			StartTime:    jobExecuteResult.StartTime.Unix(),
			EndTime:      jobExecuteResult.EndTime.Unix(),
		}

		if jobExecuteResult.Err != nil {
			jobLog.Err = jobExecuteResult.Err.Error()
		}
		log.Println("生成日志：", jobLog)
		//存储到mongodb
		G_logSink.Append(jobLog)
	}

	log.Println("任务执行完成:", jobExecuteResult.ExecuteInfo.Job.Name, " 输出：", string(jobExecuteResult.Output), " 错误：", jobExecuteResult.Err)
}

var (
	G_scheduler *Scheduler
)

//初始化调度器
func InitScheduler() (err error) {
	G_scheduler = &Scheduler{
		jobEventChan:         make(chan *common.JobEvent, 1000),
		jobExecuteResultChan: make(chan *common.JobExecuteResult, 1000),
		jobPlanTable:         make(map[string]*common.JobSchedulePlan),
		jobExecutingTable:    make(map[string]*common.JobExecuteInfo),
	}
	//启动调度协程
	go G_scheduler.scheduleLoop()
	log.Println("初始化Scheduler成功 ： ", *G_scheduler)
	return
}
