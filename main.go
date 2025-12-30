package main

import (
	router "chaos/api/api"
	"chaos/api/chain"
	"chaos/api/config"
	"chaos/api/log"
	"chaos/api/model"
	"chaos/api/service"
	"chaos/api/system"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"
)

func main() {
	log.Info("starting...")

	// 创建主上下文和取消函数
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 创建等待组，用于等待所有goroutine完成
	var wg sync.WaitGroup

	// 启动链上消费者
	chain.StartTopupConsumer(ctx)

	// 启动数据库更新处理goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				log.Info("Database update goroutine shutting down...")
				return
			case info := <-chain.UpdateTopupTx():
				if info == nil {
					continue
				}
				log.Info("prepare for db update", info)
				err := service.UpdateAccountBalance(info)
				if err != nil {
					log.Error("update account balance and token flow status failed", err)
				}
			}
		}
	}()

	// start to check database pending transactions
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(24 * time.Hour) // 每10秒检查一次
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				log.Info("Database check goroutine shutting down...")
				return
			case <-ticker.C:
				checkTx()
			}
		}
	}()

	// 启动HTTP服务器
	server := router.Init()

	// 创建HTTP服务器实例
	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", config.GetConfig().Http.Port),
		Handler: server,
	}

	// 启动HTTP服务器
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Info("HTTP server starting...")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("HTTP server failed to start", err)
		}
	}()

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 等待信号
	sig := <-sigChan
	log.Info("Received signal:", sig)
	log.Info("Starting graceful shutdown...")

	// 取消上下文，通知所有goroutine停止
	cancel()

	// 创建关闭超时上下文
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// 优雅关闭HTTP服务器
	log.Info("Shutting down HTTP server...")

	// 先停止接受新连接
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error("HTTP server graceful shutdown failed:", err)

		// 如果优雅关闭失败，强制关闭
		log.Warn("Forcing HTTP server to close...")
		forceCtx, forceCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer forceCancel()

		if err := httpServer.Shutdown(forceCtx); err != nil {
			log.Error("HTTP server forced shutdown failed:", err)
		}
	}

	// 等待所有goroutine完成
	log.Info("Waiting for all tasks to complete...")
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	// 等待任务完成或超时
	select {
	case <-done:
		log.Info("All tasks completed successfully")
	case <-shutdownCtx.Done():
		log.Warn("Shutdown timeout reached, forcing exit")
		log.Warn("Some tasks may not have completed properly")
	}

	log.Info("Server shutdown complete")
}

func checkTx() {
	db := system.GetDb()

	var chaosTrans []model.AccountBalanceFlow
	db.Model(&model.AccountBalanceFlow{}).Where("status = ?", model.BalanceFlowStatusPending).Limit(100).Find(&chaosTrans)
	if len(chaosTrans) == 0 {
		return
	}
	log.Info("[CheckTX] found pending transactions to process:", len(chaosTrans))

	// var wg sync.WaitGroup
	for _, chaos := range chaosTrans {
		// wg.Add(1)
		func(cflow model.AccountBalanceFlow) {
			// defer wg.Done()
			log.Infof("[CheckTX] processing transaction: %d - %s", cflow.ID, cflow.TxHash)

			txHash := cflow.TxHash
			op := cflow.Op

			chainID, err := strconv.ParseUint(cflow.ChainID, 10, 64)
			if err != nil {
				log.Error("[CheckTX] parse chain id failed", err)
				return
			}
			if len(txHash) == 0 {
				log.Errorf("[CheckTX] transaction hash is empty: %d - %s", cflow.ID, cflow.TxHash)
				return
			}

			switch op {
			case model.BalanceFlowOpRecharge:
				chain.AppendTopupTx(chainID, txHash, cflow.MainID, cflow.ID, op)
			case model.BalanceFlowOpWithdraw:
				chain.AppendTopupTx(chainID, txHash, cflow.MainID, cflow.RefFlowID, op)
			case model.BalanceFlowOpUnfreeze:
				chain.AppendTopupTx(chainID, txHash, cflow.MainID, cflow.RefFlowID, op)
			case model.BalanceFlowOpFreeze:
				chain.AppendTopupTx(chainID, txHash, cflow.MainID, cflow.ID, op)
			default:
				log.Warnf("[CheckTX] unsupported operation type: %d", op)
				return
			}

		}(chaos)
	}
}
