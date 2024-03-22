package engine

import (
	"fmt"
	"runtime"
	"slices"
	"time"
)

type Worker struct {
	id   int
	work bool
	inst *Session
}

type ComboListRecord struct {
	id       int64
	data     string
	login    string
	password string
}

func NewWorker(t int, i *Session) *Worker {
	return &Worker{
		id:   t + 1,
		work: true,
		inst: i,
	}
}

func (w *Worker) execute() {
	defer w.inst.removeWorker(w.id)
	for w.work {
		line := w.inst.database.GetComboLine()
		switch line.data {
		case "-1":
			w.inst.finishing = true
			w.work = false
		case "~":
			time.Sleep(time.Millisecond * 700)
		default:
			status := "RT3"
			for status == "RT3" {
				var (
					log  *string
					prox *ProxyRecord
				)

				if w.inst.proxy.id != -1 {
					gprox := w.inst.proxy.GiveProxy()
					prox = &gprox
				}

				runtime.Gosched()
				status, log = w.inst.module.ExecuteModule(line, prox, w.inst.timeout, w.inst.database.data_type)
				if status != "RT3" {
					if w.inst.database.data_type != "MT2" {
						w.inst.setToQueue(fmt.Sprintf(QUpdateSessionWebApi, status, w.inst.id, line.id))
					} else {
						if slices.Contains([]string{"RT1", "RT6", "RT7"}, status) {
							w.inst.setToQueue(fmt.Sprintf(QUpdateProtocolSessionDataGood, status, w.inst.id, line.id, w.inst.id, line.data))
						} else {
							w.inst.setToQueue(fmt.Sprintf(QUpdateProtocolSessionDataAny, status, w.inst.id, line.id))
						}
					}
					if log != nil {
						w.inst.setToQueue(fmt.Sprintf(QInsertSessionDataLog, w.inst.id, line.id, *log))
					}
				} else {
					w.inst.setToQueue(fmt.Sprintf(QUpdateErrorStat, w.inst.id))
				}
			}
		}
	}
}
