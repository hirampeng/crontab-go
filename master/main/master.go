package main

import (
	"flag"
	"fmt"
	"runtime"
	"time"

	"github.com/hirampeng/crontab-go/master"
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
	// master -config ./worker.json
	//master -h
	flag.StringVar(&confFile, "config", "./master.json", "传入master.json")
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
	err = master.InitConfig(confFile)
	if err != nil {
		goto ERR
	}

	//初始化worker管理任务
	err = master.InitWorkerManager()
	if err != nil {
		goto ERR
	}

	//启动作业日志服务
	err = master.InitJobLogManager()
	if err != nil {
		goto ERR
	}

	//启动作业管理服务
	err = master.InitJobManager()
	if err != nil {
		goto ERR
	}

	//启动Api Http服务
	err = master.InitApiServer()
	if err != nil {
		goto ERR
	}

	time.Sleep(time.Hour)
	//正常退出
	return

ERR:
	fmt.Println(err)
}
