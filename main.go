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

	// A1111 兼容路由 (包含动态前缀校验)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		
		// 检查静态资源
		if path == "/" || path == "/logs" {
			http.ServeFile(w, r, "web/logs.html")
			return
		}
		
		// 检查 A1111 路由前缀
		a1111Prefix := ""
		if cfg.NovelAI.A1111Path != "" {
			a1111Prefix = "/" + cfg.NovelAI.A1111Path
		}
		
		if len(path) >= len(a1111Prefix)+6 && path[len(a1111Prefix):len(a1111Prefix)+6] == "/sdapi" {
			if a1111Prefix != "" && path[:len(a1111Prefix)] != a1111Prefix {
				http.Error(w, "Unauthorized: Invalid subpath", http.StatusUnauthorized)
				return
			}
			
			subPath := path[len(a1111Prefix):]
			switch subPath {
			case "/sdapi/v1/txt2img":
				api.A1111Txt2Img(w, r, &cfg)
			case "/sdapi/v1/sd-models":
				api.A1111Models(w, r)
			case "/sdapi/v1/progress":
				api.A1111Progress(w, r)
			default:
				http.NotFound(w, r)
			}
			return
		}
		
		http.NotFound(w, r)
	})

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


	log.Println("Starting server on : ", cfg.Server.Addr)
	log.Println("日志查询页面: http://localhost:" + cfg.Server.Addr + "/logs")
	log.Println("默认管理密码: " + cfg.LogsAdmin.Password)

	if err := http.ListenAndServe(":"+cfg.Server.Addr, nil); err != nil {
		log.Fatal(err)
	}

}
