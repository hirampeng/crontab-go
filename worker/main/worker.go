package main

import (
	"flag"
	"fmt"
	"runtime"
	"time"

	"github.com/hirampeng/crontab-go/worker"
)

//初始化线程数量
func initEnv() {
	runtime.GOMAXPROCS(runtime.NumCPU())
}

var (
	confFile string //配置文件路径
)

//解析命令行参数
func initArgs() {
	// worker -config ./worker.json
	//worker -h
	flag.StringVar(&confFile, "config", "./worker.json", "传入master.json")
	flag.Parse()
}

func main() {
	var (
		err error
	)
	//初始化命令行参数
	initArgs()

	//初始化线程
	initEnv()

	//初始化配置
	err = worker.InitConfig(confFile)
	if err != nil {
		goto ERR
	}

	//启动日志协程
	if err = worker.InitLogSink(); err != nil {
		goto ERR
	}

	//启动执行器
	err = worker.InitExecutor()
	if err != nil {
		goto ERR
	}

	//启动调度器
	err = worker.InitScheduler()
	if err != nil {
		goto ERR
	}

	//初始化任务管理器
	err = worker.InitJobManager()
	if err != nil {
		goto ERR
	}

	time.Sleep(time.Hour)
	//正常退出
	return

ERR:
	fmt.Println(err)
}
