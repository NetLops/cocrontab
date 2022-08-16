package master

import (
	"encoding/json"
	"fmt"
	"github.com/NetLops/cocrontab/common"
	"net"
	"net/http"
	"strconv"
	"time"
)

// 任务的HTTP接口
type ApiServer struct {
	httpServer *http.Server
}

var (
	// 单例对象
	G_apiServer *ApiServer
)

// 保存任务
// POST job{"anme":"job1", "command":"echo hello","cronExpr":"* * * * *"}
func handleJobSave(resp http.ResponseWriter, req *http.Request) {
	var (
		err     error
		postJob string
		job     common.Job
		oldJob  *common.Job
		bytes   []byte
	)

	//  保存到etcd 中

	// 解析POST表单
	if err = req.ParseForm(); err != nil {
		goto ERR
	}

	// 获取表单中的job字段
	postJob = req.PostForm.Get("job")
	// 反序列化job
	if err = json.Unmarshal([]byte(postJob), &job); err != nil {
		goto ERR
	}

	// 保存到etcd
	if oldJob, err = G_jobMgr.SaveJob(&job); err != nil {
		goto ERR
	}

	// 返回正常应答({"errno":0, "msg":"","data":{...}})
	if bytes, err = common.BuildResponse(0, "success", oldJob); err == nil {
		if _, err = resp.Write(bytes); err != nil {
			return
		}
	}
	return

ERR:

	// 返回异常应答
	if bytes, err = common.BuildResponse(-1, err.Error(), nil); err == nil {
		if _, err = resp.Write(bytes); err != nil {
			return
		}
	}
}

// 删除任务接口
// POST /job/delete name=job1
func handleJobDelete(resp http.ResponseWriter, req *http.Request) {
	var (
		err    error
		name   string
		oldJob *common.Job
		datas  []byte
	)

	//POST: a=1&b=2&c=3
	if err = req.ParseForm(); err != nil {
		goto ERR
	}
	//
	//删除的任务名
	name = req.PostForm.Get("name")

	/********
	//buffer := bytes.Buffer{}
	//r := struct {
	//	Name string `json:"name"`
	//}{}
	//if _, err = buffer.ReadFrom(req.Body); err != nil {
	//	goto ERR
	//}
	//fmt.Println(string(buffer.String()))
	//if err = json.Unmarshal(buffer.Bytes(), &r); err != nil {
	//	goto ERR
	//}
	//fmt.Println(r)

	 ***********/

	// 去删除任务
	if oldJob, err = G_jobMgr.DeleteJob(name); err != nil {
		goto ERR
	}

	// 正常应答
	if datas, err = common.BuildResponse(0, "success", oldJob); err == nil {
		_, err = resp.Write(datas)
		if err != nil {
			return
		}
	}
	return
ERR:
	if datas, err = common.BuildResponse(-1, err.Error(), nil); err == nil {
		_, err = resp.Write(datas)
		if err != nil {
			return
		}
	}
}

// 列举所有crontab任务
func handleJobList(resp http.ResponseWriter, req *http.Request) {
	var (
		jobList []*common.Job
		err     error
		datas   []byte
	)

	if jobList, err = G_jobMgr.ListJobs(); err != nil {
		goto ERR
	}
	// 正常应答
	if datas, err = common.BuildResponse(0, "success", jobList); err == nil {
		_, err = resp.Write(datas)
		if err != nil {
			return
		}
	}
	return
ERR:
	if datas, err = common.BuildResponse(-1, err.Error(), nil); err == nil {
		_, err = resp.Write(datas)
		if err != nil {
			return
		}
	}
}

// 强制杀死crontab某个任务
// POST /job/kill name=job1
func handleJobKill(resp http.ResponseWriter, req *http.Request) {
	var (
		err   error
		name  string
		datas []byte
	)

	if err = req.ParseForm(); err != nil {
		goto ERR
	}
	// 杀死任务
	name = req.PostForm.Get("name")
	if err = G_jobMgr.KillJob(name); err != nil {
		goto ERR
	}
	if datas, err = common.BuildResponse(0, "success", nil); err == nil {
		_, err = resp.Write(datas)
		if err != nil {
			return
		}
	}
	return
ERR:
	if datas, err = common.BuildResponse(-1, err.Error(), nil); err == nil {
		_, err = resp.Write(datas)
		if err != nil {
			return
		}
	}
}

// 初始化服务
func InitApiServer() (err error) {

	var (
		mux        *http.ServeMux
		listener   net.Listener
		httpServer *http.Server
	)

	// 配置路由
	mux = http.NewServeMux()
	mux.HandleFunc("/job/save", handleJobSave)
	mux.HandleFunc("/job/delete", handleJobDelete)
	mux.HandleFunc("/job/list", handleJobList)
	mux.HandleFunc("/job/kill", handleJobKill)

	//  启动TCP监听
	if listener, err = net.Listen("tcp", ":"+strconv.Itoa(G_config.ApiPort)); err != nil {
		return
	}

	//  创建一个http服务
	httpServer = &http.Server{
		ReadTimeout:  time.Duration(G_config.ApiReadTimeout) * time.Millisecond,
		WriteTimeout: time.Duration(G_config.ApiWriteTimeout) * time.Millisecond,
		Handler:      mux,
	}

	// 赋值单例
	G_apiServer = &ApiServer{
		httpServer: httpServer,
	}
	// 启动了 服务端
	go func() {
		err = httpServer.Serve(listener)
		if err != nil {
			fmt.Println(err)
			return
		}
	}()
	return
}
