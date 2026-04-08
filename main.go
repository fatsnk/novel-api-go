package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"novel-api/api"
	"novel-api/config"
	"novel-api/logs"

	"gopkg.in/yaml.v2"
)

var cfg config.Config

func main() {
	data, err := ioutil.ReadFile(".env")
	if err != nil {
		log.Fatal(err)
	}
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Config loaded successfully")
	fmt.Printf("Translation config: Enable=%v, URL=%s, Model=%s\n", cfg.Translation.Enable, cfg.Translation.URL, cfg.Translation.Model)

	// 初始化日志系统
	if err := logs.InitLogger(); err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	fmt.Println("Logger initialized successfully")
	defer logs.Close()

	// 启动路由 - API路由
	http.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		api.Completions(w, r, &cfg)
	})
	http.HandleFunc("/v1/images/generations", func(w http.ResponseWriter, r *http.Request) {
		api.Generations(w, r, &cfg)
	})

	// A1111 兼容路由 (对外无验证)
	http.HandleFunc("/sdapi/v1/txt2img", func(w http.ResponseWriter, r *http.Request) {
		api.A1111Txt2Img(w, r, &cfg)
	})
	http.HandleFunc("/sdapi/v1/sd-models", api.A1111Models)
	http.HandleFunc("/sdapi/v1/progress", api.A1111Progress)

	// 日志管理API路由
	http.HandleFunc("/api/login", func(w http.ResponseWriter, r *http.Request) {
		api.Login(w, r, &cfg)
	})
	http.HandleFunc("/api/logs", api.QueryLogs)
	http.HandleFunc("/api/logs/detail", api.GetLogDetail)
	
	// 配置管理API路由
	http.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			api.GetConfig(w, r, &cfg)
		} else if r.Method == http.MethodPost {
			api.UpdateConfig(w, r, &cfg)
		} else if r.Method == http.MethodOptions {
			api.GetConfig(w, r, &cfg)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// 本地图片静态资源路由
	localPath := cfg.Local.Path
	if localPath == "" {
		localPath = "./images"
	}
	http.Handle("/images/", http.StripPrefix("/images/", http.FileServer(http.Dir(localPath))))

	// 前端页面路由
	http.HandleFunc("/logs", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/logs.html")
	})
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/logs.html")
	})

	log.Println("Starting server on : ", cfg.Server.Addr)
	log.Println("日志查询页面: http://localhost:" + cfg.Server.Addr + "/logs")
	log.Println("默认管理密码: " + cfg.LogsAdmin.Password)

	if err := http.ListenAndServe(":"+cfg.Server.Addr, nil); err != nil {
		log.Fatal(err)
	}

}
