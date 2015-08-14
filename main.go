package main

import (
	"fmt"
	"time"

	"github.com/convox/kernel/helpers"
)

func main() {
	go heartbeat()
	go startClusterMonitor()
	go pullAppImages()
	startWeb()
}

func heartbeat() {
	c, t, err := ClusterProperties()

	msg := fmt.Sprintf("%d %s", c, t)

	if err != nil {
		msg = err.Error()
	}

	helpers.SendMixpanelEvent("kernel-heartbeat", msg)

	for _ = range time.Tick(1 * time.Hour) {
		helpers.SendMixpanelEvent("kernel-heartbeat", msg)
	}
}
