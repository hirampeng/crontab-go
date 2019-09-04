package worker

import (
	"context"
	"log"

	"github.com/hirampeng/crontab-go/common"
	"go.etcd.io/etcd/clientv3"
)

//分布式锁（TXN事务）
type JobLock struct {
	kv    clientv3.KV
	lease clientv3.Lease

	cancelFunc context.CancelFunc //用于取消自动续租
	leaseID    clientv3.LeaseID   //租约的ID
	isLocked   bool

	jobName string
}

//初始化锁的结构体
func InitJobLock(jobName string, kv clientv3.KV, lease clientv3.Lease) (jobLock *JobLock) {
	jobLock = &JobLock{
		kv:      kv,
		lease:   lease,
		jobName: jobName,
	}
	return
}

func (jobLock *JobLock) TryLock() (err error) {
	//创建租约
	leaseGrantResponse, err := jobLock.lease.Grant(context.TODO(), 1)
	if err != nil {
		return err
	}
	//自动续租
	cancelCtx, cancelFunc := context.WithCancel(context.TODO())
	leaseID := leaseGrantResponse.ID
	keepAliveResponseChan, err := jobLock.lease.KeepAlive(cancelCtx, leaseID)
	if err != nil {
		jobLock.ReleaseLock(cancelFunc, leaseID)
		return err
	}

	//处理续租应答的协程
	go func() {
		for {
			select {
			case keepAliveResponse := <-keepAliveResponseChan:
				if keepAliveResponse == nil {
					return //退出协程
				} else {
					log.Println(jobLock.jobName, "自动续租成功:", keepAliveResponse.Revision)
				}
			}
		}
	}()

	//创建事务
	txn := jobLock.kv.Txn(context.TODO())

	//事务抢锁
	jobLockKey := common.JOB_LOCK_DIR + jobLock.jobName
	txnResponse, err := txn.If(clientv3.Compare(clientv3.CreateRevision(jobLockKey), "=", 0)).
		Then(clientv3.OpPut(jobLockKey, "", clientv3.WithLease(leaseID))).
		Else(clientv3.OpGet(jobLockKey)).
		Commit()

	if err != nil {
		jobLock.ReleaseLock(cancelFunc, leaseID)
		return err
	}
	//成功返回，失败释放租约
	if !txnResponse.Succeeded {
		jobLock.ReleaseLock(cancelFunc, leaseID)
		return common.ERR_LOCK_ALREADY_REQUIRED
	}

	//成功
	jobLock.cancelFunc = cancelFunc
	jobLock.leaseID = leaseID
	jobLock.isLocked = true
	return nil
}

//释放锁
func (jobLock *JobLock) ReleaseLock(cancelFunc context.CancelFunc, leaseID clientv3.LeaseID) {
	cancelFunc()                                  //取消自动续租
	jobLock.lease.Revoke(context.TODO(), leaseID) //取消租约，避免浪费资源
	jobLock.isLocked = false
}

//释放锁 前提是已经成功上锁
func (jobLock *JobLock) UnLock() {
	if jobLock.isLocked {
		jobLock.ReleaseLock(jobLock.cancelFunc, jobLock.leaseID)
		log.Println(jobLock.jobName, "已执行完成，取消续租")
	}
}
