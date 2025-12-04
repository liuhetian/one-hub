package main

import (
	"fmt"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"

	"github.com/valyala/fasthttp"
)

var responseBody = []byte(`{"id":"chatcmpl-B9MBs8CjcvOU2jLn4n570S5qMJKcT","object":"chat.completion","created":1741569952,"model":"gpt-4.1-2025-04-14","choices":[{"index":0,"message":{"role":"assistant","content":"Hello! How can I assist you today?","refusal":null,"annotations":[]},"logprobs":null,"finish_reason":"stop"}],"usage":{"prompt_tokens":19,"completion_tokens":10,"total_tokens":29,"prompt_tokens_details":{"cached_tokens":0,"audio_tokens":0},"completion_tokens_details":{"reasoning_tokens":0,"audio_tokens":0,"accepted_prediction_tokens":0,"rejected_prediction_tokens":0}},"service_tier":"default"}`)

var healthBody = []byte(`{"status":"ok"}`)

var (
	contentLength = fmt.Sprintf("%d", len(responseBody))
	healthLength  = fmt.Sprintf("%d", len(healthBody))
	requestCount  uint64
)

func chatCompletionHandler(ctx *fasthttp.RequestCtx) {
	atomic.AddUint64(&requestCount, 1)

	ctx.Response.Header.SetCanonical([]byte("Content-Type"), []byte("application/json"))
	ctx.Response.Header.SetCanonical([]byte("Content-Length"), []byte(contentLength))
	ctx.Response.Header.SetCanonical([]byte("openai-model"), []byte("gpt-4.1-2025-04-14"))
	ctx.Response.Header.SetCanonical([]byte("openai-organization"), []byte("mock-org"))
	ctx.Response.Header.SetCanonical([]byte("openai-processing-ms"), []byte("50"))
	ctx.Response.Header.SetCanonical([]byte("openai-version"), []byte("2020-10-01"))

	ctx.Response.SetBodyRaw(responseBody)
}

func healthHandler(ctx *fasthttp.RequestCtx) {
	ctx.Response.Header.SetCanonical([]byte("Content-Type"), []byte("application/json"))
	ctx.Response.Header.SetCanonical([]byte("Content-Length"), []byte(healthLength))
	ctx.Response.SetBodyRaw(healthBody)
}

func metricsHandler(ctx *fasthttp.RequestCtx) {
	count := atomic.LoadUint64(&requestCount)
	ctx.Response.Header.SetCanonical([]byte("Content-Type"), []byte("application/json"))
	fmt.Fprintf(ctx, `{"requests":%d}`, count)
}

func requestHandler(ctx *fasthttp.RequestCtx) {
	path := string(ctx.Path())

	if path == "/health" {
		healthHandler(ctx)
		return
	}

	if path == "/metrics" {
		metricsHandler(ctx)
		return
	}

	chatCompletionHandler(ctx)
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	s := &fasthttp.Server{
		Handler:               requestHandler,
		Name:                  "OpenAI-Mock-FastHTTP",
		DisableKeepalive:      false,
		MaxRequestBodySize:    4 * 1024,
		Concurrency:           256 * 1024,
		ReadBufferSize:        4096,
		WriteBufferSize:       4096,
		ReduceMemoryUsage:     false,
		NoDefaultServerHeader: false,
		NoDefaultContentType:  true,
	}

	addr := ":" + port
	fmt.Printf("🚀 OpenAI Mock 服务启动 (fasthttp 极速版)，监听端口 %s\n", port)
	fmt.Printf("📡 API端点: http://localhost:%s/v1/chat/completions\n", port)
	fmt.Printf("💚 健康检查: http://localhost:%s/health\n", port)
	fmt.Printf("📊 指标监控: http://localhost:%s/metrics\n", port)
	fmt.Printf("⚡ 性能配置: 最大并发=%d, 缓冲区=%d字节\n", s.Concurrency, s.ReadBufferSize)
	fmt.Println("📌 按 Ctrl+C 退出")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Printf("\n🛑 收到退出信号，总请求数: %d\n", atomic.LoadUint64(&requestCount))
		s.Shutdown()
	}()

	if err := s.ListenAndServe(addr); err != nil {
		fmt.Printf("服务已停止: %v\n", err)
	}
}
