package worker

import (
	"github.com/hirampeng/crontab-go/common"
	"math/rand"
	"os/exec"
	"time"
)

//任务执行器
type Executor struct {
}

var (
	G_executor *Executor
)

//执行一个任务
func (executor *Executor) ExecuteJob(info *common.JobExecuteInfo) {
	go func() {
		//执行shell命令
		cmd := exec.CommandContext(info.CancelCtx, `C:\Windows\system32\cmd.exe`, "-c", info.Job.Command)

		//初始化分布式锁
		jobLock := G_jobManager.CreateJobLock(info.Job.Name)
		//任务执行完 释放锁
		defer jobLock.UnLock()

		//抢锁做一个随机睡眠，为了实现公平锁
		time.Sleep(time.Duration(rand.Intn(1000)) * time.Millisecond)
		//抢锁  目的是为了只让一个服务执行任务
		if err := jobLock.TryLock(); err != nil { //抢锁失败
			jobExecuteResult := common.BuildJobExecuteResult(info, nil, err, time.Now(), time.Now())
			G_scheduler.PushJobExecuteResult(jobExecuteResult)
			return
		}

		//抢锁成功
		//任务开始时间
		startTime := time.Now()

		//执行并捕获输出
		output, err := cmd.CombinedOutput()

		//任务结束时间
		endTime := time.Now()

		//任务执行完成后，把执行的结果返回给Scheduler， Scheduler会从executingTable删除掉执行记录
		jobExecuteResult := common.BuildJobExecuteResult(info, output, err, startTime, endTime)
		G_scheduler.PushJobExecuteResult(jobExecuteResult)

	}()

}

//初始化执行器
func InitExecutor() (err error) {
	G_executor = &Executor{}
	return
}
