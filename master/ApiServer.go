package master

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/hirampeng/crontab-go/common"
)

var (
	//单例对象
	G_apiServer *ApiServer
)

//任务的Http接口
type ApiServer struct {
	httpServer *http.Server
}

//初始化服务
func InitApiServer() (err error) {
	var (
		mux *http.ServeMux
	)

	//配置路由
	mux = http.NewServeMux()
	mux.HandleFunc("/job/save", handleJobSave)
	mux.HandleFunc("/job/delete", handleJobDelete)
	mux.HandleFunc("/job/list", handleJobList)
	mux.HandleFunc("/job/kill", handleJobKill)

	//设置静态文件目录
	staticDir := http.Dir(G_config.WebRoot)
	mux.Handle("/", http.StripPrefix("/", http.FileServer(staticDir)))

	//启动监听
	listener, err := net.Listen("tcp", ":"+strconv.Itoa(G_config.ApiPort))
	if err != nil {
		return err
	}

	//创建一个http服务
	httServer := &http.Server{
		ReadTimeout:  time.Duration(G_config.ApiReadTimeOut) * time.Millisecond,
		WriteTimeout: time.Duration(G_config.ApiWriteTimeout) * time.Millisecond,
		Handler:      mux,
	}

	//赋值单例
	G_apiServer = &ApiServer{
		httpServer: httServer,
	}

	//启动服务端
	go httServer.Serve(listener)

	log.Println("服务端开始监听端口：" + strconv.Itoa(G_config.ApiPort))
	return
}

//强制杀死任务
func handleJobKill(responseWriter http.ResponseWriter, request *http.Request) {
	var (
		err         error
		killJobName string
	)
	err = request.ParseForm()
	if err != nil {
		goto ERR
	}
	killJobName = request.PostForm.Get("name")
	err = G_jobManager.KillJob(killJobName)
	if err == nil {
		resp, err := common.BuildResponse(0, "success", killJobName)
		if err != nil {
			goto ERR
		}
		responseWriter.Write(resp)
		return
	}

ERR:
	log.Println("杀死任务失败: ", err.Error())
	resp, _ := common.BuildResponse(-1, err.Error(), nil)
	responseWriter.Write(resp)
}

//查看etcd中的任务
func handleJobList(responseWriter http.ResponseWriter, request *http.Request) {
	var (
		err  error
		jobs []*common.Job
		//Prefix string
		resp []byte
	)

	//err = request.ParseForm()
	//if err != nil {
	//	goto ERR
	//}
	//Prefix = request.Form.Get("Prefix")
	jobs, err = G_jobManager.List()
	if err != nil {
		goto ERR
	}
	resp, err = common.BuildResponse(0, "success", jobs)
	if err == nil {
		responseWriter.Write(resp)
		return
	}
ERR:
	log.Println("获取任务列表失败：", err.Error())
	resp, _ = common.BuildResponse(-1, err.Error(), nil)
	responseWriter.Write(resp)
}

//保存任务接口
//POST job={"name":"job1", "command":"echo hello", "cronExpr":"*****"}
func handleJobSave(responseWriter http.ResponseWriter, request *http.Request) {
	//任务保存在etcd中
	var (
		jobStr string
		err    error
		resp   []byte
		job    *common.Job
		oldJob *common.Job
	)
	//解析POST表单
	//err = request.ParseMultipartForm(int64(10240))
	err = request.ParseForm()
	if err != nil {
		goto ERR
	}

	//2.取表单中的job字段
	jobStr = request.PostForm.Get("job")
	log.Println(request.PostForm)
	//log.Println(request.Form )
	//log.Println(request.MultipartForm )
	log.Println("jobStr=", jobStr)
	job = &common.Job{}
	//3.反序列化job
	err = json.Unmarshal([]byte(jobStr), job)
	if err != nil {
		goto ERR
	}
	log.Println(*job)
	//4.保存到etcd
	oldJob, err = G_jobManager.SaveJob(job)
	if err != nil {
		goto ERR
	}
	//5.返回正常应答
	resp, err = common.BuildResponse(0, "success", oldJob)
	if err == nil {
		responseWriter.Write(resp)
		return
	}
ERR:
	//6.返回错误应答
	log.Println("保存任务失败：", err.Error())
	resp, _ = common.BuildResponse(-1, err.Error(), nil)
	responseWriter.Write(resp)
}

//删除任务接口
//POST /job/delete name=job1
func handleJobDelete(responseWriter http.ResponseWriter, request *http.Request) {
	var (
		err     error
		jobName string
		oldJob  *common.Job
	)

	//解析Form表单
	err = request.ParseForm()
	if err != nil {
		goto ERR
	}

	//删除的任务名
	jobName = request.PostForm.Get("name")
	oldJob, err = G_jobManager.DeleteJob(jobName)
	if err == nil {
		resp, err := common.BuildResponse(0, "success", oldJob)
		if err != nil {
			goto ERR
		}
		responseWriter.Write(resp)
		return
	}

ERR:
	log.Println("删除任务失败：", err.Error())
	resp, _ := common.BuildResponse(0, "success", oldJob)
	responseWriter.Write(resp)
}
