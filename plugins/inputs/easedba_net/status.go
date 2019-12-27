package easedba_net

import (
	"github.com/influxdata/telegraf/plugins/easedbautil"
	"time"
)

type Status struct {
	easedbautl.BaseStatus
}

func New(device string) *Status {
	s := &Status{*easedbautl.NewBaseStatus(device)}
	return s
}

func (g *Status) Fill(netFields map[string]interface{}) error {
	g.Locker.Lock()
	defer g.Locker.Unlock()


	g.LastStatus = g.CurrStatus
	g.LastTime = g.CurrTime

	g.CurrTime = time.Now()
	g.CurrStatus = netFields


	return nil
}