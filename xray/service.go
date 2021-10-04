package xray

import (
	"Txray/core"
	"Txray/core/manage"
	"Txray/core/protocols"
	"Txray/core/setting"
	"Txray/log"
	"bufio"
	"os/exec"
	"time"
)

var Xray *exec.Cmd

var isFirstRun = true

func Start(key string) {
	testUrl := setting.TestUrl()
	testTimeout := setting.TestTimeout()
	manager := manage.Manager
	indexList := core.IndexList(key, manager.NodeLen())
	if len(indexList) == 0 {
		log.Warn("没有选取到节点")
	} else if len(indexList) == 1 {
		index := indexList[0]
		node := manager.GetNode(index)
		manager.SetSelectedIndex(index)
		manager.Save()
		exe := run(node.Protocol)
		if exe {
			if setting.Http() == 0 {
				log.Infof("启动成功, 监听socks端口: %d, 所选节点: %d", setting.Socks(), manager.SelectedIndex())
			} else {
				log.Infof("启动成功, 监听socks/http端口: %d/%d, 所选节点: %d", setting.Socks(), setting.Http(), manager.SelectedIndex())
			}
			result, status := TestNode(testUrl, setting.Socks(), testTimeout)
			log.Infof("%6s [ %s ] 延迟: %dms", status, testUrl, result)
		}
	} else {
		min := 100000
		i := -1
		for _, index := range indexList {
			node := manager.GetNode(index)
			exe := run(node.Protocol)
			if exe {
				result, status := TestNode(testUrl, setting.Socks(), testTimeout)
				log.Infof("%6s [ %s ] 节点: %d, 延迟: %dms", status, testUrl, index, result)
				if result != -1 && min > result {
					i = index
					min = result
				}
			} else {
				return
			}
		}
		if i != -1 {
			log.Info("延迟最小的节点为：", i, "，延迟：", min, "ms")
			manager.SetSelectedIndex(i)
			manager.Save()
			node := manager.GetNode(i)
			exe := run(node.Protocol)
			if exe {
				if setting.Http() == 0 {
					log.Infof("启动成功, 监听socks端口: %d, 所选节点: %d", setting.Socks(), manager.SelectedIndex())
				} else {
					log.Infof("启动成功, 监听socks/http端口: %d/%d, 所选节点: %d", setting.Socks(), setting.Http(), manager.SelectedIndex())
				}
			} else {
				log.Error("启动失败")
			}
		} else {
			log.Info("所选节点全部不能访问外网")
		}

	}
}

func run(node protocols.Protocol) bool {
	if isFirstRun {
		Kill()
		isFirstRun = false
	} else {
		Stop()
	}
	switch node.GetProtocolMode() {
	case protocols.ModeShadowSocks, protocols.ModeTrojan, protocols.ModeVMess, protocols.ModeSocks, protocols.ModeVLESS, protocols.ModeVMessAEAD:
		if CheckFile() {
			file := GenConfig(node)
			Xray = exec.Command(XrayPath(), "-c", file)
		} else {
			return false
		}
	default:
		log.Infof("暂不支持%v协议", node.GetProtocolMode())
		return false
	}
	stdout, _ := Xray.StdoutPipe()
	_ = Xray.Start()
	r := bufio.NewReader(stdout)
	lines := new([]string)
	go readInfo(r, lines)
	status := make(chan struct{})
	go checkProc(Xray, status)
	stopper := time.NewTimer(time.Millisecond * 300)
	select {
	case <-stopper.C:
		return true
	case <-status:
		log.Error("开启xray服务失败, 查看下面报错信息来检查出错问题")
		for _, x := range *lines {
			log.Error(x)
		}
		return false
	}
}

// Stop 停止服务
func Stop() {
	if Xray != nil {
		Xray.Process.Kill()
		Xray = nil
	}
}

func Kill() {
	KillProcessByName("xray")
}

func readInfo(r *bufio.Reader, lines *[]string) {
	for i := 0; i < 20; i++ {
		line, _, _ := r.ReadLine()
		if len(string(line[:])) != 0 {
			*lines = append(*lines, string(line[:]))
		}
	}
}

// 检查进程状态
func checkProc(c *exec.Cmd, status chan struct{}) {
	c.Wait()
	status <- struct{}{}
}
